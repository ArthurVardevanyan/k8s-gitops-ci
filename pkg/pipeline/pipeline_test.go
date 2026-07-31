package pipeline

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
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

// TestComposeSections_ReusesAlreadyRenderedLintingSection guards against a
// regression of the double-composition bug: composeSections used to
// re-derive "Linting"/"Static Checks" section bodies from the *already-
// rendered* validator.ValidatorResult.Sections body strings and re-compose
// them a second time via ComposeLintingSection/ComposeStaticChecksSection,
// producing double-nested markdown. It must now just reuse
// res.ValidatorResult.Sections by name unchanged.
func TestComposeSections_ReusesAlreadyRenderedLintingSection(t *testing.T) {
	const rendered = "<details>\n<summary>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;✅ Markdownlint</summary>\n\nPassed.\n\n</details>\n\n"
	res := &Result{
		ValidatorResult: &validator.Result{
			Sections: []validator.Section{
				{Name: "Linting", Body: rendered},
				{Name: "Static Checks", Body: "static body"},
				{Name: "Kustomize Build", Body: "should be ignored here"},
			},
		},
	}
	sections := composeSections(res, Options{})

	var lintSection *validator.Section
	for i := range sections {
		if sections[i].Name == "Linting" {
			lintSection = &sections[i]
		}
	}
	if lintSection == nil {
		t.Fatal("expected a Linting section in composeSections output")
	}
	if lintSection.Body != rendered {
		t.Errorf("expected the Linting section body to be reused verbatim (not re-composed/double-nested), got:\n%s", lintSection.Body)
	}
	// The double-composition bug wrapped the already-rendered body in a
	// second layer of <details>; guard against that by counting exactly one
	// "<details>" open tag in the reused section.
	if n := strings.Count(lintSection.Body, "<details>"); n != 1 {
		t.Errorf("expected exactly 1 <details> tag (no double-nesting), got %d in:\n%s", n, lintSection.Body)
	}
}

// TestBuildReport_UsesProvidersForTitleAndHeader guards against
// buildReport falling back to hardcoded "GitOps CI Results"/"GitOps CI
// Pipeline" strings instead of the org-injectable provider.Providers seam
// (opts.Providers.ReportTitle()/PipelineHeader()) - the same seam already
// correctly used for ReportMarker() two lines above in the original code.
func TestBuildReport_UsesProvidersForTitleAndHeader(t *testing.T) {
	res := &Result{ReproduceCommand: "go run ./cmd/k8s-gitops-ci pipeline"}
	opts := Options{Providers: provider.Providers{Branding: fakeBranding{}}}

	report := buildReport(res, opts)

	if report.Title != "CUSTOM TITLE" {
		t.Errorf("Title = %q, want the Branding provider's ReportTitle()", report.Title)
	}
	if report.Header != "CUSTOM HEADER" {
		t.Errorf("Header = %q, want the Branding provider's PipelineHeader()", report.Header)
	}
	if report.Marker != "<!-- custom-marker -->" {
		t.Errorf("Marker = %q, want the Branding provider's ReportMarker()", report.Marker)
	}
}

// TestBuildReport_DefaultsWhenNoProviders guards the generic (no org
// Branding wired) fallback path still working post-refactor.
func TestBuildReport_DefaultsWhenNoProviders(t *testing.T) {
	report := buildReport(&Result{}, Options{})
	if report.Title != "GitOps CI Results" {
		t.Errorf("Title = %q, want the generic default", report.Title)
	}
	if report.Header != "GitOps CI Pipeline" {
		t.Errorf("Header = %q, want the generic default", report.Header)
	}
}

type fakeBranding struct{}

func (fakeBranding) ReportMarker() string   { return "<!-- custom-marker -->" }
func (fakeBranding) ReportTitle() string    { return "CUSTOM TITLE" }
func (fakeBranding) PipelineHeader() string { return "CUSTOM HEADER" }

type fakeCommentPolicy struct{ markers []string }

func (f fakeCommentPolicy) ForeignMarkers() []string { return f.markers }

// installFakeGH writes an executable "gh" shim to a temp dir, prepends it
// to PATH for the test's duration, and returns the path to a log file every
// invocation's args are appended to. Used to assert postComment actually
// queries/deletes by a given marker without hitting the real GitHub API.
func installFakeGH(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocations.log")
	script := fmt.Sprintf(`#!/bin/sh
{
  printf 'ARGS:'
  for a in "$@"; do printf ' %%s' "$a"; done
  printf '\n'
} >> %q
exit 0
`, logPath)
	shPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(shPath, []byte(script), 0o755); err != nil { //nolint:gosec // test-only executable shim
		t.Fatalf("writing fake gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// TestPostComment_QueriesForeignMarkersFromCommentPolicy guards against
// opts.Providers.ForeignMarkers() being defined but never actually called:
// postComment must query for (and attempt to delete) comments matching
// every marker the CommentPolicy provider returns, not just the built-in
// LegacyMarkers() set.
func TestPostComment_QueriesForeignMarkersFromCommentPolicy(t *testing.T) {
	logPath := installFakeGH(t)
	opts := Options{
		URL: "https://github.com/example-org/example-repo",
		PR:  "42",
		Providers: provider.Providers{
			CommentPolicy: fakeCommentPolicy{markers: []string{"<!-- some-foreign-bot -->"}},
		},
	}
	res := &Result{ReproduceCommand: "go run ./cmd/k8s-gitops-ci pipeline"}

	if err := postComment(res, opts); err != nil {
		t.Fatalf("postComment: %v", err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading invocation log: %v", err)
	}
	if !strings.Contains(string(log), "some-foreign-bot") {
		t.Errorf("expected postComment to query for the CommentPolicy's foreign marker, got log:\n%s", log)
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
