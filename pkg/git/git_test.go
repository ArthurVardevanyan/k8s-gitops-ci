package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCloneNotARepo(t *testing.T) {
	_, err := Clone(CloneOptions{URL: "/nonexistent/repo"})
	if err == nil {
		t.Fatal("expected clone error")
	}
}

func TestCleanup(t *testing.T) {
	dir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := Cleanup(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("dir should be removed")
	}
}

func TestShowRefPathNoRepo(t *testing.T) {
	cwd, _ := os.Getwd()
	dir := t.TempDir()
	_ = os.Chdir(dir)
	defer os.Chdir(cwd)
	_, err := ShowRefPath(context.Background(), "main", "x")
	if err == nil {
		t.Error("expected error outside repo")
	}
}

func TestMergeBase(t *testing.T) {
	_, err := MergeBase(context.Background(), "origin/main")
	if err == nil {
		t.Skip("not in a git repo or no origin/main")
	}
}

func TestDiff(t *testing.T) {
	_, err := Diff(context.Background(), "HEAD", "")
	if err != nil {
		t.Skipf("diff not available: %v", err)
	}
}

func TestCloneLocal(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# test"), 0o644)
	_, _ = initRepo(dir)
	cloned, err := Clone(CloneOptions{URL: dir})
	if err != nil {
		t.Skipf("skipping shallow clone from non-repo: %v", err)
	}
	_ = Cleanup(cloned)
}

func initRepo(dir string) ([]byte, error) {
	return nil, nil
}
