package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

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
	return filepath.Clean(string(out)), nil
}
