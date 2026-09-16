package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
)

// ErrCLINotFound is returned when the glab binary is not found in PATH.
var ErrCLINotFound = errors.New("glab CLI not found in PATH")

// SignedCommitsHelpLinks defaults to GitLab commit signing docs. Orgs may override.
var SignedCommitsHelpLinks = "See https://docs.gitlab.com/user/project/repository/signed_commits/"

// TitleSuggestion optionally checks additional, non-blocking MR-title conventions.
var TitleSuggestion func(title string) string

// Client is a thin GitLab API client backed by glab.
type Client struct {
	host string
	repo string
	mr   string
	env  func(string) string
}

// HasGLab reports whether the glab CLI is installed and available in PATH.
func HasGLab() bool {
	_, err := exec.LookPath("glab")
	return err == nil
}

// IsGitLabURL reports whether a repository URL or forge flag targets GitLab.
func IsGitLabURL(rawURL, forge string) bool {
	if strings.EqualFold(forge, "gitlab") {
		return true
	}
	if strings.EqualFold(forge, "github") {
		return false
	}
	return strings.Contains(strings.ToLower(rawURL), "gitlab") ||
		os.Getenv("GITLAB_CI") != "" ||
		os.Getenv("GITLAB_HOST") != "" ||
		os.Getenv("CI_SERVER_HOST") != ""
}

// NewClient builds a GitLab client. MR may be empty for non-MR runs.
func NewClient(repoURL, mr string) *Client {
	host, repo := ExtractProject(repoURL)
	return &Client{
		host: host,
		repo: repo,
		mr:   mr,
		env: func(k string) string {
			if k == "GITLAB_HOST" && host != "" {
				return host
			}
			return ""
		},
	}
}

// NewDisabledClient returns a client that reports unavailable.
func NewDisabledClient() *Client { return &Client{} }

// IsAvailable reports whether the client can talk to GitLab.
func (c *Client) IsAvailable() bool {
	return c.repo != "" && c.mr != "" && isNumeric(c.mr)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Repo returns the project slug (e.g. group/subgroup/project).
func (c *Client) Repo() string { return c.repo }

// Host returns the GitLab host (e.g. gitlab.com).
func (c *Client) Host() string { return c.host }

// RepoSpec returns the repository specification for glab.
func (c *Client) RepoSpec() string {
	if c.host != "" && c.host != "gitlab.com" {
		return c.host + "/" + c.repo
	}
	return c.repo
}

// ValidateMRTitle checks the MR title follows conventional commits.
func ValidateMRTitle(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	title, err := fetchMRField(c, "title")
	if err != nil {
		return fmt.Errorf("could not fetch MR title: %w", err)
	}
	return github.ValidatePRTitleString(title)
}

// MRTitleSuggestion returns the current MR's non-blocking title suggestion from
// the TitleSuggestion hook, or "" when not configured or non-passing.
func MRTitleSuggestion(c *Client) string {
	if TitleSuggestion == nil || !c.IsAvailable() {
		return ""
	}
	title, err := fetchMRField(c, "title")
	if err != nil || github.ValidatePRTitleString(title) != nil {
		return ""
	}
	return TitleSuggestion(title)
}

// ValidateMRChecklist validates the MR description checklist against PRChecklistSpec.
func ValidateMRChecklist(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	body, err := fetchMRField(c, "description")
	if err != nil {
		return fmt.Errorf("could not fetch MR description: %w", err)
	}
	return github.ValidatePRChecklistString(body, github.PRChecklistSpec)
}

type mrFields struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func fetchMRField(c *Client, field string) (string, error) {
	if !isNumeric(c.mr) {
		return "", fmt.Errorf("invalid MR identifier: %q", c.mr)
	}
	out, err := c.glab("mr", "view", c.mr, "-R", c.RepoSpec(), "-F", "json")
	if err != nil {
		return "", err
	}
	var data mrFields
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		if apiErr := checkAPIError([]byte(out)); apiErr != nil {
			return "", apiErr
		}
		return "", fmt.Errorf("parsing MR JSON: %w", err)
	}
	if field == "title" {
		return data.Title, nil
	}
	return data.Description, nil
}

type mrCommit struct {
	ID      string `json:"id"`
	ShortID string `json:"short_id"`
	Title   string `json:"title"`
}

type commitSignature struct {
	VerificationStatus string `json:"verification_status"`
}

// GetUnsignedCommits returns identifiers for commits on the MR whose signature
// verification did not succeed.
func GetUnsignedCommits(c *Client) ([]string, error) {
	if !c.IsAvailable() {
		return nil, nil
	}
	encodedProject := EncodeProject(c.repo)
	out, err := c.glab("api", fmt.Sprintf("projects/%s/merge_requests/%s/commits", encodedProject, c.mr))
	if err != nil {
		return nil, fmt.Errorf("could not fetch MR commits: %w", err)
	}
	var commits []mrCommit
	if err := json.Unmarshal([]byte(out), &commits); err != nil {
		if apiErr := checkAPIError([]byte(out)); apiErr != nil {
			return nil, apiErr
		}
		return nil, fmt.Errorf("parsing MR commits: %w", err)
	}

	type commitVerdict struct {
		desc     string
		unsigned bool
		err      error
	}

	verdicts := make([]commitVerdict, len(commits))
	concurrency := 8
	if len(commits) < concurrency {
		concurrency = len(commits)
	}
	if concurrency == 0 {
		return nil, nil
	}

	ch := make(chan int, len(commits))
	for i := range commits {
		ch <- i
	}
	close(ch)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for w := 0; w < concurrency; w++ {
		go func() {
			defer wg.Done()
			for i := range ch {
				cmt := commits[i]
				desc := formatCommitDesc(cmt)
				sigOut, err := c.glab("api", fmt.Sprintf("projects/%s/repository/commits/%s/signature", encodedProject, cmt.ID))
				if err != nil {
					errStr := err.Error()
					if strings.Contains(errStr, "404") || strings.Contains(errStr, "Not Found") {
						verdicts[i] = commitVerdict{desc: desc, unsigned: true}
						continue
					}
					verdicts[i] = commitVerdict{desc: desc, err: fmt.Errorf("fetch signature for %s: %w", cmt.ID, err)}
					continue
				}
				var sig commitSignature
				if err := json.Unmarshal([]byte(sigOut), &sig); err != nil || sig.VerificationStatus != "verified" {
					verdicts[i] = commitVerdict{desc: desc, unsigned: true}
				}
			}
		}()
	}
	wg.Wait()

	var unsigned []string
	for _, v := range verdicts {
		if v.err != nil {
			return nil, v.err
		}
		if v.unsigned {
			unsigned = append(unsigned, v.desc)
		}
	}
	return unsigned, nil
}

func formatCommitDesc(cmt mrCommit) string {
	sha := cmt.ShortID
	if sha == "" {
		sha = cmt.ID
		if len(sha) > 7 {
			sha = sha[:7]
		}
	}
	return strings.TrimSpace(sha + " " + cmt.Title)
}

// CommentOnUnsignedCommits posts a warning about unsigned commits.
func CommentOnUnsignedCommits(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	_, err := c.glabStdin(
		"Some commits appear unsigned. "+SignedCommitsHelpLinks,
		"api", fmt.Sprintf("projects/%s/merge_requests/%s/notes", EncodeProject(c.repo), c.mr),
		"-X", "POST", "-F", "body=@-",
	)
	return err
}

func (c *Client) glab(args ...string) (string, error) {
	return c.glabStdin("", args...)
}

func (c *Client) glabStdin(stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "glab", args...)
	if c.host != "" {
		env := make([]string, 0, len(os.Environ())+1)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "GITLAB_HOST=") {
				env = append(env, e)
			}
		}
		env = append(env, "GITLAB_HOST="+c.host)
		cmd.Env = env
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrCLINotFound
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type apiErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func checkAPIError(data []byte) error {
	var apiErr apiErrorResponse
	if err := json.Unmarshal(data, &apiErr); err == nil {
		if apiErr.Message != "" {
			return fmt.Errorf("gitlab api: %s", apiErr.Message)
		}
		if apiErr.Error != "" {
			return fmt.Errorf("gitlab api: %s", apiErr.Error)
		}
	}
	return nil
}

// ExtractProject parses raw URL into hostname and GitLab project path.
func ExtractProject(raw string) (host, project string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if strings.HasPrefix(raw, "git@") {
		// git@host:group/subgroup/repo.git
		trimmed := strings.TrimPrefix(raw, "git@")
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			host = parts[0]
			project = cleanProjectPath(parts[1])
			return host, project
		}
		return "", cleanProjectPath(trimmed)
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil {
			host = u.Hostname()
			project = cleanProjectPath(u.Path)
			return host, project
		}
	}
	// Bare path or host/path
	parts := strings.Split(raw, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		host = parts[0]
		project = cleanProjectPath(strings.Join(parts[1:], "/"))
		return host, project
	}
	return "", cleanProjectPath(raw)
}

func cleanProjectPath(p string) string {
	p = strings.Trim(p, "/")
	p = strings.TrimSuffix(p, ".git")
	return p
}

// EncodeProject URL-encodes project path for GitLab API endpoints (:id).
func EncodeProject(project string) string {
	return strings.ReplaceAll(project, "/", "%2F")
}
