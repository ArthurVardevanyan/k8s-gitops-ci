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

// Clone clones a repo to a temporary directory. Caller must call Cleanup(dir).
func Clone(opts CloneOptions) (string, error) {
	dir, err := os.MkdirTemp("", "k8s-gitops-ci-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	args := []string{"clone", "--quiet"}
	if opts.Revision != "" {
		args = append(args, "--depth", "1", "--branch", opts.Revision)
	}
	args = append(args, opts.URL, dir)
	cmd := exec.CommandContext(context.Background(), "git", args...)
	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("git clone: %w", err)
	}
	return dir, nil
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
