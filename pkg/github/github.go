package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// SignedCommitsHelpLinks defaults to GitHub docs. Orgs may override.
var SignedCommitsHelpLinks = "See https://docs.github.com/authentication/managing-commit-signature-verification"

// PRChecklistSpec is the global checklist spec. Orgs may override.
var PRChecklistSpec ChecklistSpec

// ChecklistItem defines a single checkbox in the PR template.
type ChecklistItem struct {
	ID           string // unique identifier referenced by SelectOneGroups and Conditionals
	LabelPattern string // regex-safe label, matched against `- [x] <LabelPattern>`
}

// SelectOneGroup defines a group where exactly one option must be checked.
type SelectOneGroup struct {
	Name    string   // human-readable group name (e.g. "Change Type")
	Options []string // IDs of ChecklistItems in this group
}

// ConditionalRequire defines a conditional requirement: if the WhenID item is
// checked, all RequireIDs items must also be checked.
type ConditionalRequire struct {
	WhenID     string   // must be checked for the condition to apply
	RequireIDs []string // must all be checked when WhenID is checked
	Message    string   // error message when the condition is not met
}

// ChecklistSpec defines the PR checklist validation rules.
// An empty (zero-value) spec skips all validation.
type ChecklistSpec struct {
	Items        []ChecklistItem      // all known checkboxes (ID → LabelPattern)
	Required     []string             // IDs of items that must always be checked
	SelectOne    []SelectOneGroup     // exactly-one-of constraints
	Conditionals []ConditionalRequire // conditional dependencies
}

// Client is a thin GitHub API client backed by gh.
type Client struct {
	repo string
	pr   string
	env  func(string) string
}

// NewClient builds a GitHub client. PR may be empty for non-PR runs.
func NewClient(repoURL, pr string) *Client {
	repo := extractRepo(repoURL)
	return &Client{repo: repo, pr: pr, env: func(k string) string {
		if k == "GH_REPO" {
			return repo
		}
		return ""
	}}
}

// NewDisabledClient returns a client that reports unavailable.
func NewDisabledClient() *Client { return &Client{} }

// IsAvailable reports whether the client can talk to GitHub.
func (c *Client) IsAvailable() bool { return c.repo != "" && c.pr != "" }

// Repo returns the owner/repo slug.
func (c *Client) Repo() string { return c.repo }

// ValidatePRTitle checks the PR title follows conventional commits.
func ValidatePRTitle(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	title, err := c.gh("pr", "view", c.pr, "--json", "title", "--jq", ".title")
	if err != nil {
		return fmt.Errorf("could not fetch PR title: %w", err)
	}
	return ValidatePRTitleString(title)
}

// prTitlePattern matches the Conventional Commits prefix. The (?i) flag makes
// the type case-insensitive per spec §15 ("units of information ... MUST NOT be
// treated as case-sensitive"); `revert` is included per the spec's recommended
// revert convention. The type set mirrors @commitlint/config-conventional.
var prTitlePattern = regexp.MustCompile(`(?i)^(feat|fix|docs|style|refactor|test|build|ci|chore|perf|revert)(\(.+\))?!?: .+`)

// ValidatePRTitleString validates a single title string.
func ValidatePRTitleString(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("PR title is empty")
	}
	if prTitlePattern.MatchString(title) {
		return nil
	}
	return fmt.Errorf("PR title %q does not follow Conventional Commits (expected prefix like feat:, fix:, chore:)", title)
}

// TitleSuggestion optionally checks additional, non-blocking PR-title
// conventions once title already satisfies the required Conventional
// Commits prefix (see ValidatePRTitleString) - e.g. an org's ticket-
// reference suffix convention that's encouraged but not enforced. It
// returns a human-readable suggestion, or "" when there's nothing to
// suggest. nil by default (no suggestion checks); an org layer may set
// this from a Configure()-style seam. Unlike ValidatePRTitleString, a
// non-empty return here never blocks the pipeline (see PRTitleSuggestion/
// validator.ComposePRChecksSection's non-blocking rendering of it).
var TitleSuggestion func(title string) string

// PRTitleSuggestion returns the current PR's non-blocking title
// suggestion from the TitleSuggestion hook, or "" when: the client has no
// PR context, no TitleSuggestion hook is configured, the title could not
// be fetched, the required prefix hasn't already passed ValidatePRTitleString
// (a hard failure already covers that case - piling a suggestion on top
// would just be noise), or the hook itself has nothing to suggest.
func PRTitleSuggestion(c *Client) string {
	if TitleSuggestion == nil || !c.IsAvailable() {
		return ""
	}
	title, err := c.gh("pr", "view", c.pr, "--json", "title", "--jq", ".title")
	if err != nil || ValidatePRTitleString(title) != nil {
		return ""
	}
	return TitleSuggestion(title)
}

// ValidatePRChecklist validates the PR body checklist per spec.
func ValidatePRChecklist(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	body, err := c.gh("pr", "view", c.pr, "--json", "body", "--jq", ".body")
	if err != nil {
		return fmt.Errorf("could not fetch PR body: %w", err)
	}
	return ValidatePRChecklistString(body, PRChecklistSpec)
}

// ValidatePRChecklistString validates a checklist body against a
// ChecklistSpec without any GitHub API interaction.
func ValidatePRChecklistString(body string, spec ChecklistSpec) error {
	return validatePRChecklistBody(body, spec)
}

// buildCheckedSet builds a set of checked item IDs from the body.
func buildCheckedSet(body string, items []ChecklistItem) map[string]bool {
	checked := map[string]bool{}
	for _, item := range items {
		re := regexp.MustCompile(`(?im)^\s*-\s*\[x\]\s*` + regexp.QuoteMeta(item.LabelPattern))
		if re.MatchString(body) {
			checked[item.ID] = true
		}
	}
	return checked
}

// validatePRChecklistBody validates the body content against a ChecklistSpec.
func validatePRChecklistBody(body string, spec ChecklistSpec) error {
	// Empty spec = no validation.
	if len(spec.Items) == 0 && len(spec.Required) == 0 &&
		len(spec.SelectOne) == 0 && len(spec.Conditionals) == 0 {
		return nil
	}

	if body == "" {
		return fmt.Errorf("PR description is empty")
	}

	checked := buildCheckedSet(body, spec.Items)

	// Required items.
	for _, id := range spec.Required {
		if !checked[id] {
			label := labelForItem(id, spec.Items)
			return fmt.Errorf("required checkbox not checked: %s", label)
		}
	}

	// Select-one groups.
	for _, group := range spec.SelectOne {
		var selected []string
		for _, optID := range group.Options {
			if checked[optID] {
				selected = append(selected, labelForItem(optID, spec.Items))
			}
		}
		if len(selected) == 0 {
			return fmt.Errorf("no %s selected in PR description", group.Name)
		}
		if len(selected) > 1 {
			return fmt.Errorf("multiple %s selected in PR description", group.Name)
		}
	}

	// Conditional requirements.
	for _, cond := range spec.Conditionals {
		if !checked[cond.WhenID] {
			continue
		}
		for _, reqID := range cond.RequireIDs {
			if !checked[reqID] {
				if cond.Message != "" {
					return fmt.Errorf("%s", cond.Message)
				}
				return fmt.Errorf("%s requires %s", labelForItem(cond.WhenID, spec.Items), labelForItem(reqID, spec.Items))
			}
		}
	}

	return nil
}

func labelForItem(id string, items []ChecklistItem) string {
	for _, item := range items {
		if item.ID == id {
			return item.LabelPattern
		}
	}
	return id
}

// prCommit is the subset of the GitHub "list pull request commits" API
// response (GET /repos/{owner}/{repo}/pulls/{pr}/commits) this package reads.
type prCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message      string `json:"message"`
		Verification struct {
			Verified bool `json:"verified"`
		} `json:"verification"`
	} `json:"commit"`
}

// GetUnsignedCommits returns one "<short-sha> <message-first-line>"
// identifier per commit on the PR whose GitHub-computed signature
// verification did not succeed. A nil, nil result means every commit is
// signed (or the PR has no commits).
func GetUnsignedCommits(c *Client) ([]string, error) {
	if !c.IsAvailable() {
		return nil, nil
	}
	out, err := c.gh("api", fmt.Sprintf("repos/%s/pulls/%s/commits", c.repo, c.pr))
	if err != nil {
		return nil, fmt.Errorf("could not fetch PR commits: %w", err)
	}
	return parseUnsignedCommits([]byte(out))
}

// parseUnsignedCommits parses a GitHub "list pull request commits" API
// response body and returns identifiers for every commit whose signature
// verification did not succeed.
func parseUnsignedCommits(data []byte) ([]string, error) {
	var commits []prCommit
	if err := json.Unmarshal(data, &commits); err != nil {
		return nil, fmt.Errorf("parsing PR commits: %w", err)
	}
	unsigned := make([]string, 0, len(commits))
	for _, cmt := range commits {
		if cmt.Commit.Verification.Verified {
			continue
		}
		sha := cmt.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		message := cmt.Commit.Message
		if idx := strings.IndexByte(message, '\n'); idx >= 0 {
			message = message[:idx]
		}
		unsigned = append(unsigned, strings.TrimSpace(sha+" "+message))
	}
	return unsigned, nil
}

// CommentOnUnsignedCommits posts a warning about unsigned commits.
func CommentOnUnsignedCommits(c *Client) error {
	if !c.IsAvailable() {
		return nil
	}
	_, err := c.gh("pr", "comment", c.pr, "--body", "Some commits appear unsigned. "+SignedCommitsHelpLinks)
	return err
}

func (c *Client) gh(args ...string) (string, error) {
	return c.ghStdin("", args...)
}

// ghStdin runs gh with args, writing stdin to the child process if non-empty.
// Use this (with a "@-"-style field/body-file argument) instead of passing
// large content directly as a command-line argument - both to avoid argv
// length limits and because it's the mechanism gh itself expects for
// reading field/body values from stdin (see gh api/gh pr comment --help).
func (c *Client) ghStdin(stdin string, args ...string) (string, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "gh", args...)
	if repo := c.env("GH_REPO"); repo != "" {
		// Start from the parent's environment (os.Environ()), not a nil/empty
		// Cmd.Env - exec.Cmd treats a non-nil Env as the *complete* child
		// environment (no implicit inheritance), so appending onto a nil
		// slice here would previously strip PATH/HOME/GH_TOKEN etc. from the
		// gh subprocess, causing it to fail to find its config/auth and exit
		// with "not authenticated" (status 4) even when `gh auth status` is
		// fine in the parent shell.
		cmd.Env = append(os.Environ(), "GH_REPO="+repo)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func extractRepo(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return ""
}

// PRValidationResult is the result of PR validation checks.
type PRValidationResult struct {
	TitleErr     error
	UnsignedErr  error
	ChecklistErr error
}

// HasErrors reports whether any blocking check failed.
func (r PRValidationResult) HasErrors() bool {
	return r.TitleErr != nil || r.UnsignedErr != nil
}

// RawPayload parses an arbitrary JSON payload (placeholder for webhook support).
func RawPayload(data []byte) (map[string]any, error) {
	var m map[string]any
	err := json.Unmarshal(data, &m)
	return m, err
}
