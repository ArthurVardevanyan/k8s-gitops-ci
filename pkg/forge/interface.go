package forge

import (
	"regexp"
	"strings"
	"sync"
)

// Affinity levels for forge detection ordering.
const (
	AffinityNone = iota
	AffinityURL
	AffinityEnv
	AffinityExplicit
)

// FileChange represents a single changed file in a PR/MR.
type FileChange struct {
	Filename string `json:"filename"`
	Status   string `json:"status"` // "added", "modified", "removed", "renamed", ...
}

// PRFile is the public alias used by changeset to maintain API compatibility.
type PRFile = FileChange

// errCLINotFound is the internal error type used for CLI-not-found errors.
type errCLINotFound struct{}

func (errCLINotFound) Error() string { return "CLI binary not found in PATH" }

// ErrCLINotFound is returned when the CLI binary is not found in PATH.
var ErrCLINotFound = errCLINotFound{}

// errTitle wraps a title validation error so callers can check via string
// comparison while still satisfying the error interface.
type errTitle string

func (e errTitle) Error() string   { return string(e) }
func (e errTitle) Unwrap() error   { return nil }

// errEmptyTitle is returned when a title string is empty.
var errEmptyTitle = errTitle("PR title is empty")

// errInvalidTitle is returned when a title string does not follow the
// required Conventional-Commits prefix.
var errInvalidTitle = errTitle(
	`PR title does not follow Conventional Commits (expected prefix like feat:, fix:, chore:)`,
)

// ValidatePRTitleString validates a single title string against the
// Conventional Commits prefix. The (?i) flag makes the type case-insensitive
// per spec §15 ("units of information ... MUST NOT be treated as
// case-sensitive"); `revert` is included per the spec's recommended revert
// convention.  The type set mirrors @commitlint/config-conventional.
func ValidatePRTitleString(title string) error {
	if strings.TrimSpace(title) == "" {
		return errEmptyTitle
	}
	if prTitlePattern.MatchString(title) {
		return nil
	}
	return errInvalidTitle
}

// prTitlePattern matches the Conventional Commits prefix.
var prTitlePattern = regexp.MustCompile(
	`(?i)^(feat|fix|docs|style|refactor|test|build|ci|chore|perf|revert)(\(.+\))?!?: .+`,
)

// Forge describes the set of behaviors core packages (pipeline, changeset,
// cireport) need from any source-hosting platform (GitHub, GitLab, Gitea,
// Bitbucket, etc.).  Each method is self-contained and takes (url, pr) as
// arguments — the caller resolves the forge once (via Detect) and reuses it,
// but each method independently accepts the URL/PR pair so a single Forge
// instance can be called concurrently from different goroutines without
// sharing mutable state.
type Forge interface {
	// Name returns the registered name of this forge (e.g. "github").
	Name() string

	// IsAvailable reports whether this forge can operate with the given
	// url and pr context (e.g. a non-empty repo+pr slug).
	IsAvailable(url, pr string) bool

	// ValidateTitle checks that the PR title follows the required
	// convention (typically Conventional Commits).  Returns nil when
	// !IsAvailable(url, pr) so callers can delegate guard-logic to
	// IsAvailable and never worry about nil-checks on the result.
	ValidateTitle(url, pr string) error

	// TitleSuggestion returns a non-blocking title suggestion (or "") when
	// the required prefix already passes ValidateTitle.  An empty string
	// means "nothing to suggest."
	TitleSuggestion(url, pr string) string

	// GetUnsignedCommits returns one "<short-sha> <message-first-line>"
	// identifier per commit on the PR whose signature verification did
	// not succeed.  A nil, nil result means every commit is signed
	// (or the PR has no commits).
	GetUnsignedCommits(url, pr string) ([]string, error)

	// ValidateChecklist validates the PR body checklist.
	ValidateChecklist(url, pr string) error

	// FetchFiles fetches the full list of changed files (with status) from
	// the forge API.  Each entry corresponds to one line in a PR diff.
	FetchFiles(url, pr string) ([]FileChange, error)

	// ResolveRevision returns the git ref to check out for a PR.
	// An empty raw string means "use the default."
	ResolveRevision(raw, pr string) string

	// UpsertComment posts or updates the single CI report comment.
	UpsertComment(url, pr, marker, body string) error

	// DeleteComments deletes comments whose bodies contain any of the
	// given markers.
	DeleteComments(url, pr string, markers ...string) error

	// ExtractRepo parses the owner/repo slug from a URL.  Forges that
	// support nested groups (GitLab) return the full path; GitHub-style
	// forges return the last two path segments.
	ExtractRepo(rawURL string) string

	// FillFromEnv returns this forge's CI environment values.  It is
	// called before URL resolution so a caller can pre-populate URL/PR
	// from the active CI environment (e.g. CI_PROJECT_URL +
	// CI_MERGE_REQUEST_IID for GitLab).  An empty result means "this
	// forge has nothing to report for the current environment."
	FillFromEnv() (url, pr, revision, targetBranch string)

	// AuthHint returns human-readable guidance for authenticating to
	// this forge (e.g. "Set GH_TOKEN or run 'gh auth login'.").
	AuthHint() string

	// Matches reports whether this forge claims (rawURL, explicit)
	// using affinity levels (AffinityExplicit > AffinityEnv >
	// AffinityURL > AffinityNone).  The registry selects the highest-
	// affinity match; ties break by registration order.
	Matches(rawURL, explicit string) int
}

// Register makes f discoverable by Detect.  Forges must register in
// init(); built-ins call it from their own init() packages.  There is no
// protection against duplicate names — calling Register twice with the
// same name causes Detect to pick whichever one was registered last
// (useful for tests that want to shadow a built-in).
func Register(f Forge) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, f)
}

// Detect returns the registered forge matching (rawURL, explicit).
//
// Selection order:
//
//	1. explicit flag (case-insensitive) match by Name().
//	2. Highest Affinity by Matches(rawURL, explicit).
//
// Returns a no-op null Forge when nothing matches.  Null is a true no-op
// (IsAvailable always returns false, all other methods return nil/""/empty
// except FetchFiles which returns an error) — it preserves the existing
// behavior where an unavailable client silently passes checks.
func Detect(rawURL, explicit string) Forge {
	mu.RLock()
	defer mu.RUnlock()

	if explicit != "" {
		for _, f := range registry {
			if strings.EqualFold(f.Name(), explicit) {
				return f
			}
		}
		return nilForge
	}

	var best Forge
	bestAff := AffinityNone
	for _, f := range registry {
		aff := f.Matches(rawURL, "")
		if aff > bestAff {
			bestAff = aff
			best = f
		}
	}
	if best != nil {
		return best
	}
	return nilForge
}

// nilForge is a no-op forge returned when no registered forge matches.
var nilForge = &nullForge{}

// Ensure nullForge implements Forge at compile time.
var _ Forge = &nullForge{}

type nullForge struct{}

func (n *nullForge) Name() string { return "" }
func (n *nullForge) IsAvailable(_, _ string) bool         { return false }
func (n *nullForge) ValidateTitle(_, _ string) error      { return nil }
func (n *nullForge) TitleSuggestion(_, _ string) string   { return "" }
func (n *nullForge) GetUnsignedCommits(_, _ string) ([]string, error) {
	return nil, nil
}
func (n *nullForge) ValidateChecklist(_, _ string) error              { return nil }
func (n *nullForge) FetchFiles(_, _ string) ([]FileChange, error)     { return nil, errNoForge }
func (n *nullForge) ResolveRevision(raw, _ string) string {
	if raw != "" {
		return raw
	}
	return "HEAD"
}
func (n *nullForge) UpsertComment(_, _, _, _ string) error { return nil }
func (n *nullForge) DeleteComments(_, _ string, _ ...string) error {
	return nil
}
func (n *nullForge) ExtractRepo(s string) string            { return s }
func (n *nullForge) FillFromEnv() (string, string, string, string) {
	return "", "", "", ""
}
func (n *nullForge) AuthHint() string                       { return "" }
func (n *nullForge) Matches(_, _ string) int                { return 0 }

// errNoForge is the error returned by FetchFiles when no forge matched.
var errNoForge = errNoForgeStr{}

type errNoForgeStr struct{}

func (errNoForgeStr) Error() string { return "no forge registered matching this URL" }

// registry holds all registered forges.  It is protected by mu and
// accessed only from Detect and Register (never during normal pipeline
// execution), so the lock is lightweight.
var (
	mu       = new(sync.RWMutex)
	registry []Forge
)

