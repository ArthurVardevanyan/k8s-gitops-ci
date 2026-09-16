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

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
)

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
func (c *Client) IsAvailable() bool { return c.repo != "" && c.mr != "" }

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

func fetchMRField(c *Client, field string) (string, error) {
	out, err := c.glab("mr", "view", c.mr, "-R", c.RepoSpec(), "-F", "json")
	if err != nil {
		return "", err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		if apiErr := checkAPIError([]byte(out)); apiErr != nil {
			return "", apiErr
		}
		return "", fmt.Errorf("parsing MR JSON: %w", err)
	}
	val, _ := data[field].(string)
	return val, nil
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
	var unsigned []string
	for _, cmt := range commits {
		sigOut, err := c.glab("api", fmt.Sprintf("projects/%s/repository/commits/%s/signature", encodedProject, cmt.ID))
		if err != nil {
			// No signature endpoint or signature not found implies unverified
			unsigned = append(unsigned, formatCommitDesc(cmt))
			continue
		}
		var sig commitSignature
		if err := json.Unmarshal([]byte(sigOut), &sig); err != nil || sig.VerificationStatus != "verified" {
			unsigned = append(unsigned, formatCommitDesc(cmt))
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
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "glab", args...)
	if c.host != "" {
		cmd.Env = append(os.Environ(), "GITLAB_HOST="+c.host)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
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
