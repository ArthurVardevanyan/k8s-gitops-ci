package changeset

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

func TestExtractRepoFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/org/repo":     "org/repo",
		"https://github.com/org/repo.git": "org/repo",
		"git@github.com:org/repo.git":     "org/repo",
		"":                                "",
	}
	for in, want := range cases {
		if got := ExtractRepoFromURL(in); got != want {
			t.Errorf("ExtractRepoFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterByExtension(t *testing.T) {
	in := []string{"a.yaml", "b.go", "c.yml"}
	got := FilterByExtension(in, ".yaml", ".yml")
	if len(got) != 2 || !contains(got, "a.yaml") || !contains(got, "c.yml") {
		t.Errorf("unexpected filter result: %v", got)
	}
}

func TestExcludeByExtension(t *testing.T) {
	in := []string{"a.yaml", "b.go", "c.md"}
	got := ExcludeByExtension(in, ".go")
	if len(got) != 2 || contains(got, "b.go") {
		t.Errorf("unexpected exclude result: %v", got)
	}
}

func TestFilterByPrefix(t *testing.T) {
	got := FilterByPrefix([]string{"app/a.yaml", "base/b.yaml"}, "app/")
	if len(got) != 1 || got[0] != "app/a.yaml" {
		t.Errorf("unexpected prefix result: %v", got)
	}
}

func TestFilterByPrefixes(t *testing.T) {
	files := []string{"kubernetes/app/base.yaml", "okd/base.yaml", "tekton/base/task.yaml", ".tekton/pr.yaml", "ansible/playbook.yaml"}
	got := FilterByPrefixes(files, []string{"kubernetes/", "okd/", "tekton/", ".tekton/"})
	if len(got) != 4 {
		t.Errorf("expected 4 files, got %d: %v", len(got), got)
	}
	if contains(got, "ansible/playbook.yaml") {
		t.Errorf("expected ansible/ to be filtered out: %v", got)
	}
}

func TestFilterByPrefixes_Empty(t *testing.T) {
	files := []string{"a.yaml", "b.yaml"}
	got := FilterByPrefixes(files, nil)
	if len(got) != 2 {
		t.Errorf("expected no-op filter, got %v", got)
	}
}

func TestFilterByPrefixes_Dedup(t *testing.T) {
	// tekton/ and .tekton/ both match tekton/base/task.yaml? No -- ensure a file
	// matching multiple prefixes is only included once.
	files := []string{"tekton/base/task.yaml"}
	got := FilterByPrefixes(files, []string{"tekton/", "tekton/base"})
	if len(got) != 1 {
		t.Errorf("expected single de-duplicated match, got %v", got)
	}
}

func TestDetectApps_FilterByPrefix(t *testing.T) {
	files := []string{"app1/base.yaml", "app2/base.yaml", "app1/overlays/x.yaml"}
	got := FilterByApp(files, []string{"app1"})
	if len(got) != 2 || !contains(got, "app1/base.yaml") {
		t.Errorf("unexpected app filter: %v", got)
	}
}

func TestGetChangedFiles_LocalMode(t *testing.T) {
	_, err := GetChangedFiles(Options{})
	if err != nil {
		t.Skipf("git diff not available: %v", err)
	}
}

func TestIsPACPlaceholder(t *testing.T) {
	if !IsPACPlaceholder("{{ params.url }}") {
		t.Error("expected placeholder")
	}
	if IsPACPlaceholder("123") {
		t.Error("expected not placeholder")
	}
}

func TestWalkDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(dir, "a.txt", "x")
	writeFile(dir, filepath.Join("node_modules", "b.txt"), "y")
	files, err := walkDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	if !contains(names, "a.txt") || contains(names, "b.txt") {
		t.Errorf("unexpected walk result: %v", names)
	}
}

func writeFile(dir, rel, content string) {
	p := filepath.Join(dir, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}

// ── working-tree / local-mode diff regression tests ─────────────────────────
//
// These exercise the real `git diff`/`git diff --cached` behavior against a
// throwaway fixture repo, per parity-01 Step 8's explicit requirement to
// cover staged-only, unstaged-only, and staged+unstaged-mixed scenarios,
// plus the baseRef ref-diff path and the deletions on/off toggle.

// newGitFixture creates a temp git repo with one committed file
// ("committed.txt") on a branch named "main" and returns its path.
func newGitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "checkout", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(dir, "committed.txt", "v1\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func assertFiles(t *testing.T, got, want []string) {
	t.Helper()
	gotSorted := append([]string{}, got...)
	wantSorted := append([]string{}, want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range wantSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestGitDiffWorkingTree_UnstagedOnly(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "committed.txt", "v2\n")
	t.Chdir(dir)
	assertFiles(t, gitDiffWorkingTree("--diff-filter=d"), []string{"committed.txt"})
}

func TestGitDiffWorkingTree_StagedOnly(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "committed.txt", "v2\n")
	runGit(t, dir, "add", "committed.txt")
	t.Chdir(dir)
	assertFiles(t, gitDiffWorkingTree("--diff-filter=d"), []string{"committed.txt"})
}

func TestGitDiffWorkingTree_StagedAndUnstagedMixed_Deduped(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "second.txt", "v1\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-q", "-m", "add second")

	writeFile(dir, "committed.txt", "v2\n") // staged
	runGit(t, dir, "add", "committed.txt")
	writeFile(dir, "second.txt", "v2\n") // left unstaged

	t.Chdir(dir)
	assertFiles(t, gitDiffWorkingTree("--diff-filter=d"), []string{"committed.txt", "second.txt"})
}

func TestGitDiffWorkingTree_DeletionsExcludedByDefault(t *testing.T) {
	dir := newGitFixture(t)
	if err := os.Remove(filepath.Join(dir, "committed.txt")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got := gitDiffWorkingTree("--diff-filter=d")
	if len(got) != 0 {
		t.Errorf("expected deletions excluded, got %v", got)
	}
}

func TestGitDiffWorkingTree_DeletionsIncluded(t *testing.T) {
	dir := newGitFixture(t)
	if err := os.Remove(filepath.Join(dir, "committed.txt")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	assertFiles(t, gitDiffWorkingTree(""), []string{"committed.txt"})
}

func TestGitDiff_NoBaseRef_UsesWorkingTree(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "committed.txt", "v2\n")
	t.Chdir(dir)
	got, err := gitDiff("", false)
	if err != nil {
		t.Fatalf("gitDiff: %v", err)
	}
	assertFiles(t, got, []string{"committed.txt"})
}

func TestGitDiff_WithBaseRef_UsesRefDiff(t *testing.T) {
	dir := newGitFixture(t)
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(dir, "committed.txt", "v2\n")
	runGit(t, dir, "add", "committed.txt")
	runGit(t, dir, "commit", "-q", "-m", "change on feature")

	t.Chdir(dir)
	got, err := gitDiff("main", false)
	if err != nil {
		t.Fatalf("gitDiff: %v", err)
	}
	assertFiles(t, got, []string{"committed.txt"})
}

func TestGitDiff_WithBaseRef_ErrorsInsteadOfSilentFallback(t *testing.T) {
	// Regression: previously, a failing ref-diff (e.g. an unresolvable
	// baseRef) silently fell back to a plain unstaged-only diff instead of
	// surfacing the error - which could silently under-report the
	// changeset. It must now return a clear error.
	dir := newGitFixture(t)
	t.Chdir(dir)
	if _, err := gitDiff("does-not-exist-ref", false); err == nil {
		t.Error("expected an error for an unresolvable baseRef, got nil")
	}
}

func TestGitDiffAdded_StagedNewFile(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "new.txt", "hello\n")
	runGit(t, dir, "add", "new.txt")
	t.Chdir(dir)
	got, err := gitDiffAdded("")
	if err != nil {
		t.Fatalf("gitDiffAdded: %v", err)
	}
	assertFiles(t, got, []string{"new.txt"})
}

func TestGitDiffAdded_ModifiedFileNotIncluded(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "committed.txt", "v2\n")
	runGit(t, dir, "add", "committed.txt")
	t.Chdir(dir)
	got, err := gitDiffAdded("")
	if err != nil {
		t.Fatalf("gitDiffAdded: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no added files for a plain modification, got %v", got)
	}
}

func TestGetAllFiles_IncludesUntrackedNonIgnored(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "second.txt", "v1\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-q", "-m", "add second")
	writeFile(dir, "untracked.txt", "nope\n") // untracked but not ignored - should appear

	t.Chdir(dir)
	got, err := GetAllFiles()
	if err != nil {
		t.Fatalf("GetAllFiles: %v", err)
	}
	assertFiles(t, got, []string{"committed.txt", "second.txt", "untracked.txt"})
}

func TestGetAllFiles_ExcludesIgnoredFiles(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "second.txt", "v1\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-q", "-m", "add second")
	// Create a .gitignore that ignores build artifacts and .log files.
	writeFile(dir, ".gitignore", "build/\n*.log\n")
	// Stage the .gitignore so it's tracked.
	runGit(t, dir, "add", ".gitignore")
	runGit(t, dir, "commit", "-q", "-m", "add gitignore")
	// Now create ignored + non-ignored untracked files.
	writeFile(dir, "build/output.txt", "secret\n")
	writeFile(dir, "debug.log", "trace\n")
	writeFile(dir, "kept.txt", "safe\n") // not ignored

	t.Chdir(dir)
	got, err := GetAllFiles()
	if err != nil {
		t.Fatalf("GetAllFiles: %v", err)
	}
	// .gitignore is tracked (committed), build/* and *.log are ignored.
	assertFiles(t, got, []string{".gitignore", "committed.txt", "kept.txt", "second.txt"})
}

func TestGetFilesUnderDirs_IncludesUntrackedNonIgnored(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "second.txt", "v1\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-q", "-m", "add second")

	// Create a subdirectory with untracked files.
	writeFile(dir, "myapp/base/kustomization.yaml", "apiVersion: kustomize.config.k8s.io/v1beta1\n")
	writeFile(dir, "myapp/overlays/prod/kustomization.yaml", "apiVersion: kustomize.config.k8s.io/v1beta1\n")

	t.Chdir(dir)
	got, err := GetFilesUnderDirs([]string{"myapp/base", "myapp/overlays/prod"})
	if err != nil {
		t.Fatalf("GetFilesUnderDirs: %v", err)
	}
	assertFiles(t, got, []string{"myapp/base/kustomization.yaml", "myapp/overlays/prod/kustomization.yaml"})
}

func TestGetFilesUnderDirs_ExcludesIgnoredFiles(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "second.txt", "v1\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-q", "-m", "add second")

	// Create a .gitignore that ignores build artifacts and .log files.
	writeFile(dir, ".gitignore", "build/\n*.log\n")
	runGit(t, dir, "add", ".gitignore")
	runGit(t, dir, "commit", "-q", "-m", "add gitignore")

	// Create a subdirectory with mixed files.
	writeFile(dir, "myapp/base/kustomization.yaml", "apiVersion: kustomize.config.k8s.io/v1beta1\n")
	writeFile(dir, "myapp/base/build/output.txt", "secret\n")  // ignored by build/
	writeFile(dir, "myapp/overlays/prod/debug.log", "trace\n") // ignored by *.log

	t.Chdir(dir)
	got, err := GetFilesUnderDirs([]string{"myapp/base", "myapp/overlays/prod"})
	if err != nil {
		t.Fatalf("GetFilesUnderDirs: %v", err)
	}
	// Should include tracked + non-ignored untracked, but not ignored files.
	assertFiles(t, got, []string{"myapp/base/kustomization.yaml"})
}

func TestGetFilesUnderDirs_DeduplicatesAcrossDirs(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(dir, "second.txt", "v1\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-q", "-m", "add second")

	// Create files under a shared directory.
	writeFile(dir, "shared/file.txt", "data\n")
	writeFile(dir, "shared/nested/file2.txt", "data2\n")

	t.Chdir(dir)
	got, err := GetFilesUnderDirs([]string{"shared", "shared/nested"})
	if err != nil {
		t.Fatalf("GetFilesUnderDirs: %v", err)
	}
	// shared/nested/file2.txt appears under both dirs, should be deduplicated.
	assertFiles(t, got, []string{"shared/file.txt", "shared/nested/file2.txt"})
	if len(got) != 2 {
		t.Errorf("expected 2 files after deduplication, got %d: %v", len(got), got)
	}
}

func TestGetFilesUnderDirs_EmptyDirs(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := GetFilesUnderDirs([]string{})
	if err != nil {
		t.Fatalf("GetFilesUnderDirs: %v", err)
	}
	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestGetFilesUnderDirs_GitFallback(t *testing.T) {
	// Create a temp directory that is NOT a git repo.
	dir := t.TempDir()
	writeFile(dir, "file.txt", "data\n")
	writeFile(dir, "subdir/file2.txt", "data2\n")

	// Change to the non-git directory.
	t.Chdir(dir)

	// GetFilesUnderDirs should fall back to walkDir when git is unavailable.
	got, err := GetFilesUnderDirs([]string{"."})
	if err != nil {
		t.Fatalf("GetFilesUnderDirs: %v", err)
	}
	// Should contain both files.
	if len(got) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(got), got)
	}
}

func TestHasDirPrefix(t *testing.T) {
	tests := []struct {
		file, dir string
		want      bool
	}{
		{"base/kustomization.yaml", "base", true},
		{"base/overlay.yaml", "base", true},
		{"overlays/prod/kustomization.yaml", "overlays/prod", true},
		{"other/file.txt", "base", false},
		{"basement/file.txt", "base", false},
		{"base", "base", true},
		{"./base/file.txt", "base", true},
		{"base/file.txt", "./base", true},
		{"./base/file.txt", "./base", true},
	}

	for _, tt := range tests {
		got := hasDirPrefix(tt.file, tt.dir)
		if got != tt.want {
			t.Errorf("hasDirPrefix(%q, %q) = %v, want %v", tt.file, tt.dir, got, tt.want)
		}
	}
}

func TestGhResponseHint(t *testing.T) {
	if hint := ghResponseHint([]byte(`{"filename":"a"}`)); hint != "" {
		t.Errorf("expected no hint for JSON object, got %q", hint)
	}
	if hint := ghResponseHint([]byte(`[{"filename":"a"}]`)); hint != "" {
		t.Errorf("expected no hint for JSON array, got %q", hint)
	}
	if hint := ghResponseHint(nil); hint != "" {
		t.Errorf("expected no hint for empty response, got %q", hint)
	}
	if hint := ghResponseHint([]byte("<html>Not Found</html>")); hint == "" {
		t.Error("expected a hint for a non-JSON (HTML) response")
	}
}

func TestFetchPRFiles_RepoExtractionFailure(t *testing.T) {
	// A URL that doesn't parse into an owner/repo slug must fail fast with
	// a clear error, without ever invoking gh.
	_, err := fetchPRFiles(Options{RepoURL: "not-a-url", PR: "1"})
	if err == nil {
		t.Error("expected an error when repo cannot be extracted from RepoURL")
	}
}
