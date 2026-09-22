package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var urlCredentialsPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/@\s]+(?:@[^/@\s]+)*)@`)

// SanitizeURL strips userinfo (tokens, passwords) from a repository URL or error string for display and logging.
func SanitizeURL(raw string) string {
	if !strings.Contains(raw, "@") || !strings.Contains(raw, "://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err == nil && u.User != nil {
		u.User = nil
		return u.String()
	}
	return urlCredentialsPattern.ReplaceAllString(raw, "${1}")
}

// CloneOptions configures a repository clone.
type CloneOptions struct {
	URL      string
	Revision string
	Verbose  bool
}

// Clone clones a repo to a temporary directory and checks out Revision.
// Revision may be a branch name, tag, commit SHA, or any other single
// refspec `git fetch` accepts - including refs that aren't real
// branches/tags, like a PR head ("refs/pull/42/head"). An empty Revision
// checks out the remote's default branch. Caller must call Cleanup(dir).
//
// Unlike a plain `git clone --branch <revision>` (which requires revision
// to already be a real branch or tag name and therefore can't check out a
// PR ref or an arbitrary SHA), this clones with --no-checkout, explicitly
// fetches the requested revision, and checks out FETCH_HEAD.
func Clone(opts CloneOptions) (string, error) {
	if err := checkNotPRURL(opts.URL); err != nil {
		return "", err
	}

	dir, err := os.MkdirTemp("", "k8s-gitops-ci-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// The clone destination is passed as a positional argument, so this
	// (unlike every other call below) doesn't need cmd.Dir set to dir.
	if err := runGit("", opts.Verbose, "clone", "--quiet", "--no-checkout", opts.URL, dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("git clone: %w", err)
	}

	if opts.Revision == "" {
		if err := runGit(dir, opts.Verbose, "checkout", "--quiet"); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("git checkout default branch: %w", err)
		}
		return dir, nil
	}

	if err := runGit(dir, opts.Verbose, "fetch", "--quiet", "origin", opts.Revision); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("git fetch %s: %w", opts.Revision, err)
	}
	if err := runGit(dir, opts.Verbose, "checkout", "--quiet", "FETCH_HEAD"); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("git checkout %s: %w", opts.Revision, err)
	}
	return dir, nil
}

// prURLPattern matches a pull/merge-request-style path segment followed by
// its numeric ID, e.g. ".../pull/582", ".../pulls/582" (some GitHub
// Enterprise variants use the plural form), or ".../merge_requests/582"
// (GitLab).
var prURLPattern = regexp.MustCompile(`/(?:pulls?|merge_requests)/(\d+)`)

// checkNotPRURL fails fast with an actionable error when url looks like a
// pull/merge-request URL (e.g. copy-pasted straight from a browser) rather
// than a bare repository URL. Passing a PR URL as the clone URL produces a
// raw, confusing "git clone: exit status 128" ("repository not found")
// failure with no indication of the actual mistake - the PR number belongs
// in a separate --pr flag, not the repository URL.
func checkNotPRURL(url string) error {
	m := prURLPattern.FindStringSubmatch(url)
	if m == nil {
		return nil
	}
	return fmt.Errorf(
		"--url must be the repository URL only (e.g. https://github.com/org/repo), not a pull-request URL — got %q; pass the PR number separately via --pr %s",
		url, m[1],
	)
}

// runGit runs a git subcommand with its working directory set to dir (an
// empty dir means the calling process's current directory, per os/exec's
// Cmd.Dir semantics - used for the initial "clone" invocation, which takes
// its destination as a positional argument instead).
func runGit(dir string, verbose bool, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// Cleanup removes a cloned temporary directory.
func Cleanup(dir string) error {
	return os.RemoveAll(dir)
}

// AddWorktree creates a detached git worktree of ref under a fresh temporary
// directory and returns its path plus a cleanup function. It lets a caller
// materialize another revision's working tree (e.g. a merge-base's config and
// templates) without mutating the caller's own checkout. cleanup is safe to
// call more than once and is intended for defer; it never reports an error,
// because failing to remove a temporary directory must not fail a run.
//
// ref may be any revision git accepts (a SHA, branch, or HEAD). The caller's
// repository must be a normal (non-bare) clone; Clone produces one.
func AddWorktree(ctx context.Context, ref string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "k8s-gitops-ci-wt-*")
	if err != nil {
		return "", nil, fmt.Errorf("create worktree dir: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--quiet", "--detach", dir, ref)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("git worktree add %s: %w: %s", ref, err, strings.TrimSpace(string(out)))
	}
	cleanup = func() {
		// context.Background, not the caller's ctx: cleanup must still run
		// (and remove the temp dir) even if that context was cancelled.
		_ = exec.CommandContext(context.Background(), "git", "worktree", "remove", "--force", dir).Run()
		_ = os.RemoveAll(dir)
	}
	return dir, cleanup, nil
}

// ShowRefPath returns the content of path at ref via git show.
func ShowRefPath(ctx context.Context, ref, path string) ([]byte, error) {
	return exec.CommandContext(ctx, "git", "show", fmt.Sprintf("%s:%s", ref, path)).Output()
}

// Diff returns the diff between base and head.
func Diff(ctx context.Context, base, head string) ([]byte, error) {
	if head == "" {
		return exec.CommandContext(ctx, "git", "diff", base).Output()
	}
	return exec.CommandContext(ctx, "git", "diff", fmt.Sprintf("%s...%s", base, head)).Output()
}

// MergeBase returns the merge-base of the current HEAD and ref.
func MergeBase(ctx context.Context, ref string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "merge-base", "HEAD", ref).Output()
	if err != nil {
		return "", err
	}
	// git's own stdout, not a filesystem path - filepath.Clean doesn't trim
	// whitespace (it only normalizes path separators/dots), so it silently
	// left the trailing newline `git merge-base` always outputs attached to
	// the returned SHA. That's harmless for a bare string comparison but
	// breaks the moment the SHA is fed into another git command as a
	// revision argument (e.g. `git show <sha>\n:<path>`, which git rejects
	// as an invalid revision) - exactly this package's own ShowRefPath.
	return strings.TrimSpace(string(out)), nil
}
