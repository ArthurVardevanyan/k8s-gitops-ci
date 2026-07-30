package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// SignedCommitsHelpLinks defaults to GitHub docs. Orgs may override.
var SignedCommitsHelpLinks = "See https://docs.github.com/authentication/managing-commit-signature-verification"

// PRChecklistSpec is the global checklist spec. Orgs may override.
var PRChecklistSpec ChecklistSpec

// ChecklistSpec configures PR checklist validation rules.
type ChecklistSpec struct {
	Items        []string
	Required     []string
	SelectOne    [][]string
	Conditionals []ConditionalCheck
}

// ConditionalCheck validates a checklist item only when its predicate is present.
type ConditionalCheck struct {
	Predicate string
	Item      string
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

var prTitlePattern = regexp.MustCompile(`^(feat|fix|docs|style|refactor|test|build|ci|chore|perf)(\(.+\))?!?: .+`)

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

// ValidatePRChecklistString validates a checklist body.
func ValidatePRChecklistString(body string, spec ChecklistSpec) error {
	unchecked := uncheckedItems(body)
	for _, item := range spec.Required {
		if _, ok := unchecked[item]; ok {
			return fmt.Errorf("required checklist item %q is unchecked", item)
		}
	}
	for _, group := range spec.SelectOne {
		checked := 0
		for _, item := range group {
			if !unchecked[item] {
				checked++
			}
		}
		if checked == 0 {
			return fmt.Errorf("one of %v must be checked", group)
		}
		if checked > 1 {
			return fmt.Errorf("only one of %v may be checked", group)
		}
	}
	return nil
}

func uncheckedItems(body string) map[string]bool {
	m := make(map[string]bool)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- [") {
			continue
		}
		checked := strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]")
		item := strings.TrimSpace(strings.TrimPrefix(line, "- [ ]"))
		item = strings.TrimSpace(strings.TrimPrefix(item, "- [x]"))
		item = strings.TrimSpace(strings.TrimPrefix(item, "- [X]"))
		m[item] = !checked
	}
	return m
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
	var unsigned []string
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
		cmd.Env = append(cmd.Env, "GH_REPO="+repo)
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
