package pipeline

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
)

// TestPrintFailedSectionDetail_PrintsBodyOfErroredSections guards against a
// regression where a section's composed detail (the actual finding
// list/messages behind a "N violation(s)" summary line) is only ever
// rendered into the PR comment body, and gets silently discarded on
// --verbose CLI-only runs (no --comment) - see Run's call to
// printFailedSectionDetail.
func TestPrintFailedSectionDetail_PrintsBodyOfErroredSections(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "out.log")
	log := logger.NewLogger(true, logPath)
	defer log.Close()

	vr := &validator.Result{
		Sections: []validator.ReportSection{
			{Name: "Linting", Body: "linting all good", Status: validator.StatusPassed},
			{Name: "Resource Compliance", Body: "<details>\n<summary>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;❌ PSA Labels (1 finding(s))</summary>\n\n| Check | File | Message |\n| --- | --- | --- |\n| psa | ns.yaml | missing label |\n\n</details>\n", Status: validator.StatusError},
			{Name: "Kustomize Build", Body: "kustomize build apps/foo/overlays/bar: some error", Status: validator.StatusError},
		},
	}
	printFailedSectionDetail(vr, log)
	log.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "linting all good") {
		t.Errorf("did not expect non-error section body to be printed: %s", got)
	}
	for _, want := range []string{"Resource Compliance", "missing label", "Kustomize Build", "kustomize build apps/foo/overlays/bar", "❌ PSA Labels (1 finding(s)):"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected log output to contain %q, got: %s", want, got)
		}
	}
	// Guards the actual bug reported against this feature: raw GitHub-comment
	// markdown (<details>/<summary> dropdown tags, &nbsp; indentation) must
	// not leak into --verbose console/log output, which has no markdown
	// renderer - see SanitizeSectionBodyForConsole.
	for _, unwanted := range []string{"<details>", "</details>", "<summary>", "&nbsp;"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("did not expect raw markdown artifact %q in console output: %s", unwanted, got)
		}
	}
}

// TestPrintFailedSectionDetail_NilResult ensures a nil ValidatorResult (e.g.
// validator.RunAll itself failed) doesn't panic.
func TestPrintFailedSectionDetail_NilResult(t *testing.T) {
	log := logger.NewLogger(true, "")
	printFailedSectionDetail(nil, log)
}

// TestValidatorResultFailed_BlockingFindings guards a regression where
// Run's exit-code check only inspected res.ValidationErr (the Go error
// RunAll returns, which is nil for a completed-but-failing run) and never
// vr.Blocking/vr.Logger.HasFailures(), so a PR with blocking Resource
// Compliance findings (or any failed Linting/Static Checks/Build section)
// still exited 0 and printed "All checks passed!".
func TestValidatorResultFailed_BlockingFindings(t *testing.T) {
	vr := &validator.Result{Blocking: true, Logger: logger.NewLogger(false, "")}
	if !validatorResultFailed(vr) {
		t.Error("expected validatorResultFailed to be true when vr.Blocking is set")
	}
}

// TestValidatorResultFailed_LoggerHasFailures covers the non-Blocking case:
// a failed Linting/Static Checks/Build section (which never sets
// vr.Blocking - that's set only from Resource Compliance direct findings)
// still recorded an error via the validator's own Logger, and must still
// fail the run.
func TestValidatorResultFailed_LoggerHasFailures(t *testing.T) {
	log := logger.NewLogger(false, "")
	log.ErrorInSection("Linting", "shellcheck: 1 violation")
	vr := &validator.Result{Blocking: false, Logger: log}
	if !validatorResultFailed(vr) {
		t.Error("expected validatorResultFailed to be true when the validator logger recorded a failure")
	}
}

// TestValidatorResultFailed_PassingRun ensures a clean run doesn't get
// spuriously marked as failed.
func TestValidatorResultFailed_PassingRun(t *testing.T) {
	vr := &validator.Result{Blocking: false, Logger: logger.NewLogger(false, "")}
	if validatorResultFailed(vr) {
		t.Error("expected validatorResultFailed to be false for a clean run")
	}
}

// TestValidatorResultFailed_Nil ensures a nil ValidatorResult (e.g.
// RunAll itself hard-failed, so res.ValidationErr already covers it)
// doesn't panic and doesn't spuriously report failure on its own.
func TestValidatorResultFailed_Nil(t *testing.T) {
	if validatorResultFailed(nil) {
		t.Error("expected validatorResultFailed(nil) to be false")
	}
}

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

func TestToValidatorOptions_Dirs(t *testing.T) {
	opts := Options{Dirs: []string{"kubernetes/", "tekton/"}}
	vopts := toValidatorOptions(opts)
	if len(vopts.Dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %v", vopts.Dirs)
	}
}

// TestToValidatorOptions_HookSourceAndTriggerComment guards a real wiring
// bug: pipeline.Options.TriggerComment/HookSource (populated from the CLI's
// --trigger-comment/--hook-source flags) previously weren't copied into
// validator.Options at all, so hook.ResolveSource - which validator's
// build phase now calls - could never see the actual PaC trigger signal.
func TestToValidatorOptions_HookSourceAndTriggerComment(t *testing.T) {
	opts := Options{TriggerComment: "/hook-test", HookSource: "pr"}
	vopts := toValidatorOptions(opts)
	if vopts.TriggerComment != "/hook-test" {
		t.Errorf("TriggerComment = %q, want %q", vopts.TriggerComment, "/hook-test")
	}
	if vopts.HookSource != "pr" {
		t.Errorf("HookSource = %q, want %q", vopts.HookSource, "pr")
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
	if sections[0].Status != validator.StatusError {
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
			Sections: []validator.ReportSection{
				{Name: "Linting", Body: rendered},
				{Name: "Static Checks", Body: "static body"},
				{Name: "Kustomize Build", Body: "should be ignored here"},
			},
		},
	}
	sections := composeSections(res, Options{})

	var lintSection *validator.ReportSection
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

// TestComposeSections_ReusesKustomizeBuildAndResourceCompliance guards a
// second instance of the same double-composition bug (Kustomize Build and
// Resource Compliance were also being recomposed a second time from stub/
// empty args in composeSections, even though phases.go already built full
// versions of both during runBuildAndPostBuild).
func TestComposeSections_ReusesKustomizeBuildAndResourceCompliance(t *testing.T) {
	res := &Result{
		ValidatorResult: &validator.Result{
			Sections: []validator.ReportSection{
				{Name: "Kustomize Build", Body: "real kustomize build body", Status: validator.StatusError},
				{Name: "Scaffold Validation", Body: "real scaffold body"},
				{Name: "Resource Compliance", Body: "real resource compliance body", Status: validator.StatusError},
			},
		},
	}
	sections := composeSections(res, Options{})

	byName := map[string]validator.ReportSection{}
	for _, s := range sections {
		byName[s.Name] = s
	}
	if byName["Kustomize Build"].Body != "real kustomize build body" {
		t.Errorf("expected Kustomize Build to be reused verbatim, got:\n%s", byName["Kustomize Build"].Body)
	}
	if byName["Resource Compliance"].Body != "real resource compliance body" {
		t.Errorf("expected Resource Compliance to be reused verbatim, got:\n%s", byName["Resource Compliance"].Body)
	}
}

// TestComposeSections_OmitsKyvernoWhenNotProduced guards the "kyverno" step's
// default-off gating: unlike Kustomize Build/Scaffold Validation/Resource
// Compliance (which always fall back to a "No results." stub), a missing
// Kyverno Policies section must be omitted entirely - phases.go only ever
// produces it when the step is actually enabled, and a fallback stub here
// would misleadingly read as "ran, found nothing" rather than "didn't run".
func TestComposeSections_OmitsKyvernoWhenNotProduced(t *testing.T) {
	res := &Result{ValidatorResult: &validator.Result{}}
	sections := composeSections(res, Options{})
	for _, s := range sections {
		if s.Name == "Kyverno Policies" {
			t.Errorf("expected no Kyverno Policies section when phases.go never produced one, got: %+v", s)
		}
	}
}

// TestComposeSections_ReusesKyvernoWhenProduced is the positive
// counterpart: when phases.go did produce a "Kyverno Policies" section
// (the "kyverno" step was enabled), composeSections must reuse it verbatim.
func TestComposeSections_ReusesKyvernoWhenProduced(t *testing.T) {
	res := &Result{
		ValidatorResult: &validator.Result{
			Sections: []validator.ReportSection{
				{Name: "Kyverno Policies", Body: "real kyverno body", Status: validator.StatusError},
			},
		},
	}
	sections := composeSections(res, Options{})
	var found *validator.ReportSection
	for i := range sections {
		if sections[i].Name == "Kyverno Policies" {
			found = &sections[i]
		}
	}
	if found == nil {
		t.Fatal("expected a Kyverno Policies section to be present")
	}
	if found.Body != "real kyverno body" {
		t.Errorf("expected the Kyverno Policies section to be reused verbatim, got:\n%s", found.Body)
	}
}

// TestComposeSections_FallsBackWhenValidatorResultMissingSections guards
// the --lint-only path: runBuildAndPostBuild never runs, so
// ValidatorResult.Sections only has "Linting"/"Static Checks" - composeSections
// must still return every section name (with a "No results." fallback body)
// rather than omitting them or panicking.
func TestComposeSections_FallsBackWhenValidatorResultMissingSections(t *testing.T) {
	res := &Result{
		ValidatorResult: &validator.Result{
			Sections: []validator.ReportSection{
				{Name: "Linting", Body: "lint body"},
				{Name: "Static Checks", Body: "static body"},
			},
		},
	}
	sections := composeSections(res, Options{})

	byName := map[string]validator.ReportSection{}
	for _, s := range sections {
		byName[s.Name] = s
	}
	for _, name := range []string{"Kustomize Build", "Scaffold Validation", "Resource Compliance"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("expected a %q section to be present even in --lint-only mode", name)
			continue
		}
		if s.Body != "No results." {
			t.Errorf("expected %q to fall back to %q, got %q", name, "No results.", s.Body)
		}
	}
	// NAD is now omit-when-absent (like Kyverno Policies): with no NAD
	// section produced upstream, composeSections must not synthesize a
	// "No results." stub for it.
	if _, ok := byName["NetworkAttachmentDefinition Validation"]; ok {
		t.Error("expected no NetworkAttachmentDefinition Validation section when none was produced upstream")
	}
}

// TestComposeSections_ReusesNADSection guards against the "produced but
// never surfaced" bug: phases.go's runBuildAndPostBuild appends a
// "NetworkAttachmentDefinition Validation" section to
// validator.Result.Sections whenever a NAD is present in the rendered chain
// (see nad_wiring.go's runNADValidation), but composeSections used to only
// relay a hardcoded name whitelist that never included it - so the actual PR
// comment silently never showed NAD findings even though validator.RunAll
// (and thus the build-yaml/test-all CLI commands, which print every
// res.Sections entry directly) produced them. When present it must be
// relayed verbatim, like Kustomize Build/Scaffold Validation/Resource
// Compliance.
func TestComposeSections_ReusesNADSection(t *testing.T) {
	res := &Result{
		ValidatorResult: &validator.Result{
			Sections: []validator.ReportSection{
				{Name: "NetworkAttachmentDefinition Validation", Body: "real nad body", Status: validator.StatusError},
			},
		},
	}
	sections := composeSections(res, Options{})
	var found *validator.ReportSection
	for i := range sections {
		if sections[i].Name == "NetworkAttachmentDefinition Validation" {
			found = &sections[i]
		}
	}
	if found == nil {
		t.Fatal("expected a NetworkAttachmentDefinition Validation section to be present")
	}
	if found.Body != "real nad body" {
		t.Errorf("expected the NAD section to be reused verbatim, got:\n%s", found.Body)
	}
	if found.Status != validator.StatusError {
		t.Error("expected the NAD section's error status to be preserved")
	}
}

// TestComposeSections_ReusesScaffoldDriftProtectionSection guards that the
// unconditional "Scaffold Drift Protection" section phases.go produces
// (findUnprotectedApps/ComposeDriftProtectionSection in scaffold_wiring.go/
// compose_sections.go) is relayed into the actual PR comment, the same way
// Kustomize Build/Scaffold Validation/Resource Compliance/NAD are.
func TestComposeSections_ReusesScaffoldDriftProtectionSection(t *testing.T) {
	res := &Result{
		ValidatorResult: &validator.Result{
			Sections: []validator.ReportSection{
				{Name: "Scaffold Drift Protection", Body: "real drift protection body"},
			},
		},
	}
	sections := composeSections(res, Options{})
	var found *validator.ReportSection
	for i := range sections {
		if sections[i].Name == "Scaffold Drift Protection" {
			found = &sections[i]
		}
	}
	if found == nil {
		t.Fatal("expected a Scaffold Drift Protection section to be present")
	}
	if found.Body != "real drift protection body" {
		t.Errorf("expected the section to be reused verbatim, got:\n%s", found.Body)
	}
}

// TestBuildReport_UsesProvidersForTitleAndHeader guards against
// buildReport falling back to hardcoded "GitOps CI Results"/"GitOps CI
// Pipeline" strings instead of the org-injectable provider.Providers seam
// (opts.Providers.ReportTitle()/PipelineHeader()) - the same seam already
// correctly used for ReportMarker() two lines above in the original code.
func TestBuildReport_UsesProvidersForTitleAndHeader(t *testing.T) {
	res := &Result{ReproduceCommand: "k8s-gitops-ci pipeline"}
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
func (fakeBranding) BinaryName() string     { return "custom-ci" }

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
	res := &Result{ReproduceCommand: "k8s-gitops-ci pipeline"}

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
		cmd := exec.CommandContext(t.Context(), "git", args...)
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

// TestRun_UsesProviderPipelineHeaderForLogHeader guards that Run's
// run-start log header comes from opts.Providers.PipelineHeader() rather
// than a hardcoded string, so an org's Branding provider takes effect on
// the console banner as well as the PR-comment report (buildReport,
// already covered by TestBuildReport_UsesProvidersForTitleAndHeader).
// pkg/logger writes directly to os.Stdout with no injectable io.Writer, so
// this captures it via a redirected os.Stdout pipe; the given URL is a
// nonexistent local path so setupWorkdir fails fast right after the
// header/URL/PR/Revision lines are logged, with no network access needed.
func TestRun_UsesProviderPipelineHeaderForLogHeader(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := Run(Options{
		URL:       filepath.Join(t.TempDir(), "does-not-exist"),
		Providers: provider.Providers{Branding: fakeBranding{}},
	})

	_ = w.Close()
	os.Stdout = origStdout
	if runErr == nil {
		t.Fatal("expected an error for a nonexistent repo URL")
	}
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("reading captured stdout: %v", readErr)
	}
	if !strings.Contains(string(out), "CUSTOM HEADER") {
		t.Errorf("expected the run-start log header to use the Branding provider's PipelineHeader(), got:\n%s", out)
	}
	if strings.Contains(string(out), "GitOps CI Pipeline") {
		t.Errorf("expected the generic default header to NOT appear when a Branding provider is wired, got:\n%s", out)
	}
}

// TestRun_PrintsVersionLineAndSetupHeader guards two additions to Run's
// console output: (1) the version.String() line (previously only printed
// by the standalone "version" CLI subcommand, never at the start of an
// actual pipeline run, unlike a downstream fork's equivalent output), and
// (2) a "Setup" log.Header banner around setupWorkdir/clone (previously
// setup only fed the timing collector - tc.Record("Setup", ...) - with no
// visible banner, unlike every other phase). Uses the same nonexistent-URL/
// captured-stdout technique as TestRun_UsesProviderPipelineHeaderForLogHeader
// to fail fast right after these lines are logged.
func TestRun_PrintsVersionLineAndSetupHeader(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := Run(Options{URL: filepath.Join(t.TempDir(), "does-not-exist")})

	_ = w.Close()
	os.Stdout = origStdout
	if runErr == nil {
		t.Fatal("expected an error for a nonexistent repo URL")
	}
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("reading captured stdout: %v", readErr)
	}
	got := string(out)
	if !strings.Contains(got, version.String()) {
		t.Errorf("expected the version line to be printed at run start, got:\n%s", got)
	}
	if !strings.Contains(got, "Setup") {
		t.Errorf("expected a \"Setup\" header banner around setupWorkdir, got:\n%s", got)
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
