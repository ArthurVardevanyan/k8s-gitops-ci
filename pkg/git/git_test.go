package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	sha, err := MergeBase(context.Background(), "origin/main")
	if err != nil {
		t.Skip("not in a git repo or no origin/main")
	}
	// git merge-base always terminates its stdout with a trailing newline;
	// the returned SHA must have it (and any other whitespace) trimmed, or
	// passing it straight into another git revision argument (e.g.
	// ShowRefPath) breaks with an "invalid revision" error.
	if sha != strings.TrimSpace(sha) {
		t.Errorf("MergeBase result has untrimmed whitespace: %q", sha)
	}
	if sha == "" {
		t.Error("expected a non-empty merge-base SHA")
	}
}

// TestMergeBase_ResultUsableAsRevisionArgument guards the actual bug found
// while wiring MergeBase into scaffold baseline diffing: a SHA with a
// trailing newline is a valid string but an invalid git revision argument -
// ShowRefPath(ctx, sha, "go.mod") must succeed with MergeBase's result.
func TestMergeBase_ResultUsableAsRevisionArgument(t *testing.T) {
	sha, err := MergeBase(context.Background(), "origin/main")
	if err != nil {
		t.Skip("not in a git repo or no origin/main")
	}
	if _, err := ShowRefPath(context.Background(), sha, "go.mod"); err != nil {
		t.Errorf("expected ShowRefPath to accept MergeBase's result as a valid revision, got: %v", err)
	}
}

func TestDiff(t *testing.T) {
	_, err := Diff(context.Background(), "HEAD", "")
	if err != nil {
		t.Skipf("diff not available: %v", err)
	}
}

// ── Clone against a real local fixture repo ──────────────────────────────────
//
// newCloneFixture builds a source repo with a "main" branch (2 commits),
// a "feature" branch, and a synthetic PR ref (refs/pull/1/head, mirroring
// what GitHub exposes for real PRs) pointing at the feature commit. Clone
// treats the fixture's path as a plain local "URL", exercising the same
// clone/fetch/checkout code path as a real remote.

type cloneFixture struct {
	repoPath   string
	mainSHA    string
	featureSHA string
}

func newCloneFixture(t *testing.T) cloneFixture {
	t.Helper()
	dir := t.TempDir()
	runGitCmd(t, dir, "init", "-q")
	runGitCmd(t, dir, "checkout", "-q", "-b", "main")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "Test")
	writeTestFile(t, dir, "README.md", "on main\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-q", "-m", "init")
	mainSHA := gitOutput(t, dir, "rev-parse", "HEAD")

	runGitCmd(t, dir, "checkout", "-q", "-b", "feature")
	writeTestFile(t, dir, "feature.txt", "on feature\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-q", "-m", "feature work")
	featureSHA := gitOutput(t, dir, "rev-parse", "HEAD")
	runGitCmd(t, dir, "update-ref", "refs/pull/1/head", featureSHA)

	// Leave HEAD on main so the "default branch" checkout tests see main.
	runGitCmd(t, dir, "checkout", "-q", "main")

	return cloneFixture{repoPath: dir, mainSHA: mainSHA, featureSHA: featureSHA}
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClone_ByBranch(t *testing.T) {
	fx := newCloneFixture(t)
	dir, err := Clone(CloneOptions{URL: fx.repoPath, Revision: "feature"})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	defer func() { _ = Cleanup(dir) }()
	assertFileExists(t, dir, "feature.txt")
}

func TestClone_BySHA(t *testing.T) {
	fx := newCloneFixture(t)
	dir, err := Clone(CloneOptions{URL: fx.repoPath, Revision: fx.mainSHA})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	defer func() { _ = Cleanup(dir) }()
	assertFileExists(t, dir, "README.md")
	assertFileMissing(t, dir, "feature.txt")
}

func TestClone_ByPRRef(t *testing.T) {
	// The whole point of this rewrite: a ref that is neither a branch nor a
	// tag (like a PR head) must still be clonable. A plain
	// `git clone --branch refs/pull/1/head` would fail here.
	fx := newCloneFixture(t)
	dir, err := Clone(CloneOptions{URL: fx.repoPath, Revision: "refs/pull/1/head"})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	defer func() { _ = Cleanup(dir) }()
	assertFileExists(t, dir, "feature.txt")
}

func TestClone_EmptyRevision_UsesDefaultBranch(t *testing.T) {
	fx := newCloneFixture(t)
	dir, err := Clone(CloneOptions{URL: fx.repoPath})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	defer func() { _ = Cleanup(dir) }()
	assertFileExists(t, dir, "README.md")
	assertFileMissing(t, dir, "feature.txt")
}

func TestClone_UnresolvableRevision_Errors(t *testing.T) {
	fx := newCloneFixture(t)
	dir, err := Clone(CloneOptions{URL: fx.repoPath, Revision: "refs/pull/999/head"})
	if err == nil {
		_ = Cleanup(dir)
		t.Fatal("expected an error for a revision that doesn't exist")
	}
}

func TestClone_PRURL_GitHub_RejectedBeforeClone(t *testing.T) {
	before := countCloneTempDirs(t)
	_, err := Clone(CloneOptions{URL: "https://github.com/ArthurVardevanyan/HomeLab/pull/582"})
	if err == nil {
		t.Fatal("expected an error for a PR URL passed as --url")
	}
	if !strings.Contains(err.Error(), "--pr 582") {
		t.Errorf("expected error to suggest --pr 582, got: %v", err)
	}
	if strings.Contains(err.Error(), "exit status 128") {
		t.Errorf("expected the friendly pre-check error, not a raw git-clone failure: %v", err)
	}
	after := countCloneTempDirs(t)
	if after > before {
		t.Errorf("expected no temp dir created when rejecting before clone (before=%d, after=%d)", before, after)
	}
}

func TestClone_PRURL_GitHubEnterprisePluralForm_Rejected(t *testing.T) {
	_, err := Clone(CloneOptions{URL: "https://github.example.com/org/repo/pulls/42"})
	if err == nil {
		t.Fatal("expected an error for a /pulls/ URL passed as --url")
	}
	if !strings.Contains(err.Error(), "--pr 42") {
		t.Errorf("expected error to suggest --pr 42, got: %v", err)
	}
}

func TestClone_PRURL_GitLabMergeRequest_Rejected(t *testing.T) {
	_, err := Clone(CloneOptions{URL: "https://gitlab.com/org/repo/-/merge_requests/7"})
	if err == nil {
		t.Fatal("expected an error for a /merge_requests/ URL passed as --url")
	}
	if !strings.Contains(err.Error(), "--pr 7") {
		t.Errorf("expected error to suggest --pr 7, got: %v", err)
	}
}

func TestClone_InvalidURL_CleansUpTempDir(t *testing.T) {
	before := countCloneTempDirs(t)
	_, err := Clone(CloneOptions{URL: filepath.Join(t.TempDir(), "does-not-exist")})
	if err == nil {
		t.Fatal("expected an error for a nonexistent source path")
	}
	after := countCloneTempDirs(t)
	if after > before {
		t.Errorf("expected no leftover k8s-gitops-ci-* temp dirs (before=%d, after=%d)", before, after)
	}
}

func countCloneTempDirs(t *testing.T) int {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(os.TempDir(), "k8s-gitops-ci-*"))
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func assertFileExists(t *testing.T, dir, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Errorf("expected %s to exist in %s: %v", name, dir, err)
	}
}

func assertFileMissing(t *testing.T, dir, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		t.Errorf("expected %s to NOT exist in %s", name, dir)
	}
}
