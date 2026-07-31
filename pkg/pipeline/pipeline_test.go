package pipeline

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsValidPR(t *testing.T) {
	if isValidPR("") {
		t.Error("empty PR invalid")
	}
	if isValidPR("{{ params.pr }}") {
		t.Error("placeholder PR invalid")
	}
	if !isValidPR("123") {
		t.Error("numeric PR valid")
	}
}

func TestResolveBaseRef(t *testing.T) {
	if got := resolveBaseRef("gh-readonly-queue/main/pr-1-abc"); got != "main" {
		t.Errorf("main base ref: %s", got)
	}
	if got := resolveBaseRef(""); got != "origin/main" {
		t.Errorf("default base ref: %s", got)
	}
}

func TestOptionsWorkers(t *testing.T) {
	o := Options{Concurrency: 4}
	if o.Workers() != 4 {
		t.Errorf("expected 4 workers")
	}
}

func TestToValidatorOptions_IncludePrefixes(t *testing.T) {
	opts := Options{IncludePrefixes: []string{"kubernetes/", "tekton/"}}
	vopts := toValidatorOptions(opts)
	if len(vopts.IncludePrefixes) != 2 {
		t.Fatalf("expected 2 include prefixes, got %v", vopts.IncludePrefixes)
	}
}

func TestShouldRunPRChecks_ValidPR(t *testing.T) {
	if !shouldRunPRChecks(Options{PR: "123"}) {
		t.Errorf("expected true for a valid PR")
	}
}

func TestShouldRunPRChecks_ValidPR_LintOnly(t *testing.T) {
	// Regression: title/signed-commit checks must still run in --lint-only
	// mode - they were previously (incorrectly) skipped entirely.
	if !shouldRunPRChecks(Options{PR: "123", LintOnly: true}) {
		t.Errorf("expected PR checks to run in lint-only mode for a valid PR")
	}
}

func TestShouldRunPRChecks_InvalidPR(t *testing.T) {
	if shouldRunPRChecks(Options{PR: ""}) {
		t.Errorf("expected false for an empty PR")
	}
	if shouldRunPRChecks(Options{PR: "{{ params.pr }}"}) {
		t.Errorf("expected false for a placeholder PR")
	}
}

func TestShouldRunPRChecks_MergeQueue(t *testing.T) {
	opts := Options{PR: "123", TargetBranch: "gh-readonly-queue/main/pr-1-abc"}
	if shouldRunPRChecks(opts) {
		t.Errorf("expected false in a merge-queue run")
	}
}

func TestShouldRunChecklistCheck_LintOnly(t *testing.T) {
	// The checklist check remains the one PR check that IS skipped in
	// lint-only mode.
	if shouldRunChecklistCheck(Options{PR: "123", LintOnly: true}) {
		t.Errorf("expected checklist check to be skipped in lint-only mode")
	}
}

func TestShouldRunChecklistCheck_NotLintOnly(t *testing.T) {
	if !shouldRunChecklistCheck(Options{PR: "123"}) {
		t.Errorf("expected checklist check to run outside lint-only mode")
	}
}

func TestShouldRunChecklistCheck_InvalidPR(t *testing.T) {
	if shouldRunChecklistCheck(Options{PR: ""}) {
		t.Errorf("expected false for an invalid PR regardless of lint-only")
	}
}

func TestCommentSkipReason_PostCommentOff(t *testing.T) {
	reason, skip := commentSkipReason(Options{PostComment: false, URL: "https://github.com/org/repo", PR: "1"})
	if !skip {
		t.Fatal("expected skip=true when PostComment is off")
	}
	if reason == "" {
		t.Error("expected a non-empty skip reason")
	}
}

func TestCommentSkipReason_NoRepoPRContext(t *testing.T) {
	reason, skip := commentSkipReason(Options{PostComment: true, URL: "", PR: ""})
	if !skip {
		t.Fatal("expected skip=true when no repo/PR context is available")
	}
	if reason == "" {
		t.Error("expected a non-empty skip reason")
	}
}

func TestCommentSkipReason_PostsWhenOptedInWithContext(t *testing.T) {
	_, skip := commentSkipReason(Options{PostComment: true, URL: "https://github.com/org/repo", PR: "1"})
	if skip {
		t.Fatal("expected skip=false when PostComment is on and repo/PR context is available")
	}
}

func TestComposeSections(t *testing.T) {
	res := &Result{TitleErr: errors.New("bad title")}
	sections := composeSections(res, Options{})
	if len(sections) == 0 {
		t.Errorf("expected sections")
	}
	if !sections[0].Error {
		t.Errorf("expected PR checks error")
	}
}

// ── resolveRevision ───────────────────────────────────────────────────────

func TestResolveRevision_ExplicitWins(t *testing.T) {
	if got := resolveRevision("v1.2.3", "42"); got != "v1.2.3" {
		t.Errorf("resolveRevision = %q, want %q", got, "v1.2.3")
	}
}

func TestResolveRevision_PRFallsBackToRefsPullHead(t *testing.T) {
	// This is the correctness fix: a PR run with no explicit --revision
	// must check out the PR's own commits, not the target repo's default
	// branch - otherwise the pipeline would silently validate the wrong code.
	if got := resolveRevision("", "42"); got != "refs/pull/42/head" {
		t.Errorf("resolveRevision = %q, want %q", got, "refs/pull/42/head")
	}
}

func TestResolveRevision_NoRevisionNoPR_DefaultsToHEAD(t *testing.T) {
	if got := resolveRevision("", ""); got != "HEAD" {
		t.Errorf("resolveRevision = %q, want %q", got, "HEAD")
	}
}

func TestResolveRevision_InvalidPR_DefaultsToHEAD(t *testing.T) {
	// A placeholder/invalid PR value must not be templated into the ref.
	if got := resolveRevision("", "{{ params.pr }}"); got != "HEAD" {
		t.Errorf("resolveRevision = %q, want %q", got, "HEAD")
	}
}

// ── setupWorkdir ──────────────────────────────────────────────────────────

func newPipelineFixture(t *testing.T) (repoPath string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("checkout", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return dir
}

func TestSetupWorkdir_NoURL_IsNoOp(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := setupWorkdir(Options{})
	if err != nil {
		t.Fatalf("setupWorkdir: %v", err)
	}
	defer cleanup()

	gotWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if gotWD != origWD {
		t.Errorf("expected cwd unchanged for empty URL, got %q, want %q", gotWD, origWD)
	}
}

func TestSetupWorkdir_ClonesChdirsAndRestores(t *testing.T) {
	fixture := newPipelineFixture(t)
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cleanup, err := setupWorkdir(Options{URL: fixture, Revision: "main"})
	if err != nil {
		t.Fatalf("setupWorkdir: %v", err)
	}

	clonedWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if clonedWD == origWD {
		t.Fatal("expected cwd to change into the cloned repo")
	}
	if _, err := os.Stat(filepath.Join(clonedWD, "marker.txt")); err != nil {
		t.Errorf("expected marker.txt in cloned repo: %v", err)
	}

	cleanup()

	restoredWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if restoredWD != origWD {
		t.Errorf("expected cwd restored to %q, got %q", origWD, restoredWD)
	}
	if _, err := os.Stat(clonedWD); !os.IsNotExist(err) {
		t.Errorf("expected cloned dir %q removed after cleanup", clonedWD)
	}
}

func TestSetupWorkdir_InvalidURL_ReturnsError(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := setupWorkdir(Options{URL: filepath.Join(t.TempDir(), "does-not-exist")})
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error for a nonexistent repo URL")
	}
	gotWD, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatal(wdErr)
	}
	if gotWD != origWD {
		t.Errorf("expected cwd unchanged on clone failure, got %q, want %q", gotWD, origWD)
	}
}
