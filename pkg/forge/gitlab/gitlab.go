package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/forge"
)

// SignedCommitsHelpLinks defaults to GitLab commit signing docs. Orgs may override.
var SignedCommitsHelpLinks = "See https://docs.gitlab.com/user/project/repository/signed_commits/"

// PRChecklistSpec is the global checklist spec. Orgs may override.
var PRChecklistSpec forge.ChecklistSpec

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
func (c *Client) IsAvailable() bool {
	return c.repo != "" && c.mr != "" && isNumeric(c.mr) && forge.ValidPR(c.mr)
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

// MR returns the merge request IID.
func (c *Client) MR() string { return c.mr }

// RepoSpec returns the repository specification for glab.
func (c *Client) RepoSpec() string {
	if c.host != "" && c.host != "gitlab.com" {
		return c.host + "/" + c.repo
	}
	return c.repo
}

// ValidateMRTitle checks the MR title follows Conventional Commits.
func ValidateMRTitle(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	title, err := fetchMRField(c, "title")
	if err != nil {
		return fmt.Errorf("could not fetch MR title: %w", err)
	}
	return forge.ValidatePRTitleString(title)
}

// ValidateMRTitleString validates a single title string against Conventional Commits.
func ValidateMRTitleString(title string) error {
	return forge.ValidatePRTitleString(title)
}

// TitleSuggestion optionally checks additional, non-blocking MR-title conventions.
var TitleSuggestion func(title string) string

// MRTitleSuggestion returns the current MR's non-blocking title suggestion.
func MRTitleSuggestion(c *Client) string {
	if TitleSuggestion == nil || !c.IsAvailable() {
		return ""
	}
	title, err := fetchMRField(c, "title")
	if err != nil || forge.ValidatePRTitleString(title) != nil {
		return ""
	}
	return TitleSuggestion(title)
}

// ValidateMRChecklist validates the MR description checklist per spec.
func ValidateMRChecklist(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	body, err := fetchMRField(c, "description")
	if err != nil {
		return fmt.Errorf("could not fetch MR description: %w", err)
	}
	return forge.ValidateChecklistString(body, PRChecklistSpec)
}

// ValidateMRChecklistString validates a checklist body against a ChecklistSpec.
func ValidateMRChecklistString(body string, spec forge.ChecklistSpec) error {
	return forge.ValidateChecklistString(body, spec)
}

type mrViewResponse struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func fetchMRField(c *Client, field string) (string, error) {
	out, err := c.glab("api", fmt.Sprintf("projects/%s/merge_requests/%s", EncodeProject(c.repo), c.mr))
	if err != nil {
		return "", err
	}
	var data mrViewResponse
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
	out, err := c.glab("api", "--paginate", fmt.Sprintf("projects/%s/merge_requests/%s/commits", encodedProject, c.mr))
	if err != nil {
		return nil, fmt.Errorf("could not fetch MR commits: %w", err)
	}
	var commits []mrCommit
	dec := json.NewDecoder(strings.NewReader(out))
	for dec.More() {
		var page []mrCommit
		if err := dec.Decode(&page); err != nil {
			if apiErr := checkAPIError([]byte(out)); apiErr != nil {
				return nil, apiErr
			}
			return nil, fmt.Errorf("parsing MR commits: %w", err)
		}
		commits = append(commits, page...)
	}

	if len(commits) == 0 {
		return nil, nil
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

	ch := make(chan int, len(commits))
	for i := range commits {
		ch <- i
	}
	close(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for w := 0; w < concurrency; w++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case i, ok := <-ch:
					if !ok {
						return
					}
					cmt := commits[i]
					desc := formatCommitDesc(cmt)
					var sigOut string
					var err error
					for attempt := 0; attempt < 3; attempt++ {
						sigOut, err = c.glabContext(ctx, "", "api", fmt.Sprintf("projects/%s/repository/commits/%s/signature", encodedProject, cmt.ID))
						if err == nil {
							break
						}
						errStr := err.Error()
						if strings.Contains(errStr, "429") || strings.Contains(errStr, "Too Many Requests") {
							select {
							case <-ctx.Done():
								return
							case <-time.After(time.Duration(150*(attempt+1)) * time.Millisecond):
								continue
							}
						}
						break
					}
					if err != nil {
						if errors.Is(ctx.Err(), context.Canceled) {
							return
						}
						errStr := err.Error()
						if strings.Contains(errStr, "404") || strings.Contains(errStr, "Not Found") {
							verdicts[i] = commitVerdict{desc: desc, unsigned: true}
							continue
						}
						verdicts[i] = commitVerdict{desc: desc, err: fmt.Errorf("fetch signature for %s: %w", cmt.ID, err)}
						cancel()
						return
					}
					var sig commitSignature
					if err := json.Unmarshal([]byte(sigOut), &sig); err != nil || sig.VerificationStatus != "verified" {
						verdicts[i] = commitVerdict{desc: desc, unsigned: true}
					}
				}
			}
		}()
	}
	wg.Wait()

	for _, v := range verdicts {
		if v.err != nil {
			return nil, v.err
		}
	}

	var unsigned []string
	for _, v := range verdicts {
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
	return c.glabContext(context.Background(), "", args...)
}

func (c *Client) glabStdin(stdin string, args ...string) (string, error) {
	return c.glabContext(context.Background(), stdin, args...)
}

func (c *Client) glabContext(parentCtx context.Context, stdin string, args ...string) (string, error) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "glab", args...)
	env := make([]string, 0, len(os.Environ())+2)
	hasToken := false
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITLAB_HOST=") {
			continue
		}
		if strings.HasPrefix(e, "GITLAB_TOKEN=") {
			if strings.TrimPrefix(e, "GITLAB_TOKEN=") != "" {
				hasToken = true
				env = append(env, e)
			}
			continue
		}
		if strings.HasPrefix(e, "GLAB_TOKEN=") {
			if strings.TrimPrefix(e, "GLAB_TOKEN=") != "" {
				hasToken = true
				env = append(env, e)
			}
			continue
		}
		if strings.HasPrefix(e, "GITLAB_ACCESS_TOKEN=") {
			if strings.TrimPrefix(e, "GITLAB_ACCESS_TOKEN=") != "" {
				hasToken = true
				env = append(env, e)
			}
			continue
		}
		env = append(env, e)
	}
	if c.host != "" {
		env = append(env, "GITLAB_HOST="+c.host)
	}
	if !hasToken {
		if jobToken := os.Getenv("CI_JOB_TOKEN"); jobToken != "" {
			env = append(env, "GITLAB_TOKEN="+jobToken)
		}
	}
	cmd.Env = env

	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", forge.ErrCLINotFound
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			sanitizedStderr := sanitizeURL(strings.TrimSpace(string(exitErr.Stderr)))
			return "", fmt.Errorf("%w: %s", err, sanitizedStderr)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var userinfoRegexp = regexp.MustCompile(`://[^/@\s]+@`)

func sanitizeURL(s string) string {
	return userinfoRegexp.ReplaceAllString(s, "://***@")
}

type apiErrorResponse struct {
	Message any    `json:"message"`
	Error   string `json:"error"`
}

func checkAPIError(data []byte) error {
	var apiErr apiErrorResponse
	if err := json.Unmarshal(data, &apiErr); err == nil {
		if apiErr.Message != nil {
			return fmt.Errorf("gitlab api: %v", apiErr.Message)
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
	p = strings.TrimSpace(p)
	for strings.HasPrefix(p, "/") {
		p = strings.TrimPrefix(p, "/")
	}
	for strings.HasSuffix(p, "/") || strings.HasSuffix(p, ".git") {
		p = strings.TrimSuffix(p, "/")
		p = strings.TrimSuffix(p, ".git")
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

// EncodeProject URL-encodes project path for GitLab API endpoints (:id).
func EncodeProject(project string) string {
	return strings.ReplaceAll(project, "/", "%2F")
}

// HasGLab reports whether the glab CLI is installed and available in PATH.
func HasGLab() bool {
	_, err := exec.LookPath("glab")
	return err == nil
}

// AuthHint returns guidance when GitLab authentication is required or fails.
func AuthHint() string {
	return "Set GITLAB_TOKEN (or CI_JOB_TOKEN) or run 'glab auth login'."
}

// ---------------------------------------------------------------------------
// Forge implementation (implements forge.Forge)
// ---------------------------------------------------------------------------

// ForgeID is the registered forge name for GitLab.
const ForgeID = "gitlab"

func init() {
	forge.Register(NewForge())
}

// NewForge creates a GitLab-specific Forge implementation.
func NewForge() forge.Forge {
	return &glForge{}
}

type glForge struct{}

var _ forge.Forge = (*glForge)(nil)

func (f *glForge) Name() string { return ForgeID }

func (f *glForge) IsAvailable(repoURL, pr string) bool {
	_, repo := ExtractProject(repoURL)
	return repo != "" && pr != "" && isNumeric(pr) && forge.ValidPR(pr)
}

func (f *glForge) ValidateTitle(repoURL, pr string) error {
	c := NewClient(repoURL, pr)
	return ValidateMRTitle(c)
}

func (f *glForge) TitleSuggestion(repoURL, pr string) string {
	c := NewClient(repoURL, pr)
	return MRTitleSuggestion(c)
}

func (f *glForge) GetUnsignedCommits(repoURL, pr string) ([]string, error) {
	c := NewClient(repoURL, pr)
	return GetUnsignedCommits(c)
}

func (f *glForge) ValidateChecklist(repoURL, pr string) error {
	c := NewClient(repoURL, pr)
	return ValidateMRChecklist(c)
}

func (f *glForge) FetchFiles(repoURL, pr string) ([]forge.FileChange, error) {
	c := NewClient(repoURL, pr)
	if !c.IsAvailable() {
		return nil, fmt.Errorf("no repo/MR context available for FetchFiles")
	}
	if !HasGLab() {
		return nil, fmt.Errorf("glab command not available; %s: %w", f.AuthHint(), forge.ErrCLINotFound)
	}
	encoded := EncodeProject(c.repo)
	out, err := c.glab("api", "--paginate", fmt.Sprintf("projects/%s/merge_requests/%s/diffs", encoded, c.mr))
	if err != nil {
		return nil, fmt.Errorf("glab api diffs: %w", err)
	}

	type diffEntry struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
	}

	var files []forge.FileChange
	dec := json.NewDecoder(strings.NewReader(out))
	for dec.More() {
		var page []diffEntry
		if err := dec.Decode(&page); err != nil {
			if apiErr := checkAPIError([]byte(out)); apiErr != nil {
				return nil, apiErr
			}
			return nil, fmt.Errorf("parsing MR diffs response: %w", err)
		}
		for _, diff := range page {
			status := "modified"
			filename := diff.NewPath
			switch {
			case diff.DeletedFile:
				status = "removed"
				filename = diff.OldPath
			case diff.NewFile:
				status = "added"
			case diff.RenamedFile:
				status = "renamed"
			}
			files = append(files, forge.FileChange{
				Filename: filename,
				Status:   status,
			})
		}
	}
	return files, nil
}

// ResolveRevision implements Forge. GitLab serves an MR's head commit at
// refs/merge-requests/<pr>/head, so an MR run without an explicit raw revision
// checks out the MR's actual commits instead of the target repo's
// default branch — which would silently validate the wrong code.
func (f *glForge) ResolveRevision(raw, pr string) string {
	return forge.ResolvePRRef(raw, pr, "refs/merge-requests/%s/head")
}

func (f *glForge) UpsertComment(repoURL, pr, marker, body string) error {
	c := NewClient(repoURL, pr)
	return UpsertComment(c, marker, body)
}

func (f *glForge) DeleteComments(repoURL, pr string, markers ...string) error {
	c := NewClient(repoURL, pr)
	return DeleteComments(c, markers...)
}

func (f *glForge) ExtractRepo(repoURL string) string {
	_, project := ExtractProject(repoURL)
	return project
}

func (f *glForge) FillFromEnv() (url, pr, revision, targetBranch string) {
	url = os.Getenv("CI_PROJECT_URL")
	if url == "" {
		url = os.Getenv("CI_REPOSITORY_URL")
	}
	pr = os.Getenv("CI_MERGE_REQUEST_IID")
	revision = os.Getenv("CI_COMMIT_SHA")
	targetBranch = os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
	if targetBranch == "" {
		targetBranch = os.Getenv("CI_DEFAULT_BRANCH")
	}
	return url, pr, revision, targetBranch
}

func (f *glForge) AuthHint() string {
	return AuthHint()
}

func (f *glForge) Matches(rawURL, explicit string) int {
	if explicit != "" {
		if strings.EqualFold(explicit, ForgeID) {
			return forge.AffinityExplicit
		}
		return forge.AffinityNone
	}
	if rawURL != "" {
		host, _ := ExtractProject(rawURL)
		if strings.Contains(strings.ToLower(host), "gitlab") || strings.Contains(strings.ToLower(rawURL), "gitlab") {
			return forge.AffinityURL
		}
		if glHost := os.Getenv("GITLAB_HOST"); glHost != "" && strings.EqualFold(host, glHost) {
			return forge.AffinityURL
		}
		if srvHost := os.Getenv("CI_SERVER_HOST"); srvHost != "" && strings.EqualFold(host, srvHost) {
			return forge.AffinityURL
		}
	}
	if os.Getenv("GITLAB_CI") != "" || os.Getenv("CI_MERGE_REQUEST_IID") != "" || os.Getenv("GITLAB_HOST") != "" || os.Getenv("CI_SERVER_HOST") != "" {
		return forge.AffinityEnv
	}
	return forge.AffinityNone
}
