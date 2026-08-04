package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

func TestToIDSet_Empty(t *testing.T) {
	if got := toIDSet(nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
	if got := toIDSet([]string{}); got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

func TestToIDSet_Populated(t *testing.T) {
	set := toIDSet([]string{"sync-options", "golangci"})
	if !set["sync-options"] || !set["golangci"] {
		t.Fatalf("expected both ids present, got %v", set)
	}
	if set["kyverno"] {
		t.Errorf("expected kyverno absent")
	}
}

func TestStepEnabled_DefaultOnStepEnabledByDefault(t *testing.T) {
	if !stepEnabled("sync-options", nil, nil) {
		t.Errorf("default-on step should be enabled with no lists set")
	}
}

func TestStepEnabled_DefaultOnStepDisabledViaDisabledChecks(t *testing.T) {
	disabled := toIDSet([]string{"sync-options"})
	if stepEnabled("sync-options", disabled, nil) {
		t.Errorf("expected sync-options disabled")
	}
	// An unrelated ID must remain enabled.
	if !stepEnabled(stepGolangci, disabled, nil) {
		t.Errorf("expected golangci to remain enabled")
	}
}

func TestStepEnabled_KubeconformDefaultOnDisableableViaDisabledChecks(t *testing.T) {
	if !stepEnabled(stepKubeconform, nil, nil) {
		t.Errorf("kubeconform should default to enabled")
	}
	disabled := toIDSet([]string{stepKubeconform})
	if stepEnabled(stepKubeconform, disabled, nil) {
		t.Errorf("expected kubeconform disabled via DisabledChecks")
	}
}

func TestStepEnabled_DefaultOffStepDisabledByDefault(t *testing.T) {
	if stepEnabled(stepKyverno, nil, nil) {
		t.Errorf("kyverno should default to disabled")
	}
}

func TestStepEnabled_DefaultOffStepEnabledViaEnabledChecks(t *testing.T) {
	enabled := toIDSet([]string{stepKyverno})
	if !stepEnabled(stepKyverno, nil, enabled) {
		t.Errorf("expected kyverno enabled once listed in EnabledChecks")
	}
}

func TestStepEnabled_EnabledChecksHasNoEffectOnDefaultOnSteps(t *testing.T) {
	// Listing a default-on step in EnabledChecks is a no-op - it's already
	// enabled, and only DisabledChecks can turn it off.
	enabled := toIDSet([]string{stepGolangci})
	if !stepEnabled(stepGolangci, nil, enabled) {
		t.Errorf("expected golangci enabled")
	}
	disabled := toIDSet([]string{stepGolangci})
	if stepEnabled(stepGolangci, disabled, enabled) {
		t.Errorf("DisabledChecks must win over an unrelated EnabledChecks entry for a default-on step")
	}
}

// TestResolveEnabledChecks_ExplicitTakesPrecedence guards that a
// caller-supplied Options.EnabledChecks is always used as-is, even when
// DefaultEnabledChecks is also set - the explicit, per-run value must never
// be silently merged with or overridden by the package-level default.
func TestResolveEnabledChecks_ExplicitTakesPrecedence(t *testing.T) {
	old := DefaultEnabledChecks
	DefaultEnabledChecks = []string{"kyverno"}
	defer func() { DefaultEnabledChecks = old }()

	got := resolveEnabledChecks([]string{"other-check"})
	if len(got) != 1 || got[0] != "other-check" {
		t.Errorf("expected explicit EnabledChecks to win, got %v", got)
	}
}

// TestResolveEnabledChecks_FallsBackToDefault guards that an empty
// Options.EnabledChecks falls back to the DefaultEnabledChecks enablement
// seam (the org-injectable override point), rather than always being empty.
func TestResolveEnabledChecks_FallsBackToDefault(t *testing.T) {
	old := DefaultEnabledChecks
	DefaultEnabledChecks = []string{"kyverno"}
	defer func() { DefaultEnabledChecks = old }()

	got := resolveEnabledChecks(nil)
	if len(got) != 1 || got[0] != "kyverno" {
		t.Errorf("expected DefaultEnabledChecks fallback, got %v", got)
	}
}

// TestResolveEnabledChecks_NilDefault guards the zero-value/no-op case: with
// no explicit EnabledChecks and no DefaultEnabledChecks set, the result is
// nil - no behavior change for callers that never touch the seam.
func TestResolveEnabledChecks_NilDefault(t *testing.T) {
	old := DefaultEnabledChecks
	DefaultEnabledChecks = nil
	defer func() { DefaultEnabledChecks = old }()

	if got := resolveEnabledChecks(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// --- lint-tool missing-CLI hard failure -------------------------------------
//
// markdownlint, prettier, shellcheck, and golangci all hard-fail (rather
// than gracefully skip) when their underlying CLI isn't installed - see
// the comment above their runLintStep wiring in phases.go. Each test below
// mirrors the exec.LookPath+t.Skip pattern already used for CLI-wrapping
// tests elsewhere in this repo (e.g. pkg/kustomize/kustomize_test.go's
// TestCheckFix_MissingBinaryIsAHardError): it only runs for real in an
// environment where the tool genuinely isn't installed, skipping itself
// otherwise, since there's no supported way to force exec.LookPath to
// fail for an installed binary without editing PATH process-wide.

func TestRunAll_MarkdownlintMissingCLIBlocks(t *testing.T) {
	if _, err := exec.LookPath("markdownlint"); err == nil {
		t.Skip("markdownlint is installed in this environment; can't exercise the missing-binary path")
	}
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "README.md"), "# Title\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "prettier", "shellcheck", "golangci"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a missing markdownlint CLI to be surfaced as a logger failure")
	}
	if !res.Failed() {
		t.Error("expected a missing markdownlint CLI to make Result.Failed() report true")
	}
	var lint ReportSection
	for _, s := range res.Sections {
		if s.Name == "Linting" {
			lint = s
		}
	}
	if lint.Status != StatusError || !strings.Contains(lint.Body, "markdownlint not found in PATH") {
		t.Errorf("expected the Linting section to report the missing markdownlint CLI, got:\n%s", lint.Body)
	}
}

func TestRunAll_PrettierMissingCLIBlocks(t *testing.T) {
	if _, err := exec.LookPath("prettier"); err == nil {
		t.Skip("prettier is installed in this environment; can't exercise the missing-binary path")
	}
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "markdownlint", "shellcheck", "golangci"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a missing prettier CLI to be surfaced as a logger failure")
	}
	if !res.Failed() {
		t.Error("expected a missing prettier CLI to make Result.Failed() report true")
	}
	var lint ReportSection
	for _, s := range res.Sections {
		if s.Name == "Linting" {
			lint = s
		}
	}
	if lint.Status != StatusError || !strings.Contains(lint.Body, "prettier not found in PATH") {
		t.Errorf("expected the Linting section to report the missing prettier CLI, got:\n%s", lint.Body)
	}
}

func TestRunAll_ShellcheckMissingCLIBlocks(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err == nil {
		t.Skip("shellcheck is installed in this environment; can't exercise the missing-binary path")
	}
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "markdownlint", "prettier", "golangci"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a missing shellcheck CLI to be surfaced as a logger failure")
	}
	if !res.Failed() {
		t.Error("expected a missing shellcheck CLI to make Result.Failed() report true")
	}
	var lint ReportSection
	for _, s := range res.Sections {
		if s.Name == "Linting" {
			lint = s
		}
	}
	if lint.Status != StatusError || !strings.Contains(lint.Body, "shellcheck not found in PATH") {
		t.Errorf("expected the Linting section to report the missing shellcheck CLI, got:\n%s", lint.Body)
	}
}

func TestRunAll_ShellcheckMissingCLISkipsWhenNoRelevantFiles(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err == nil {
		t.Skip("shellcheck is installed in this environment; can't exercise the missing-binary path")
	}
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "README.md"), "# Title\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "prettier", "golangci"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected a changeset with no shell-related files to skip shellcheck cleanly, even with the CLI missing")
	}
}

func TestRunAll_GolangciMissingCLIBlocks(t *testing.T) {
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		t.Skip("golangci-lint is installed in this environment; can't exercise the missing-binary path")
	}
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "main.go"), "package main\n\nfunc main() {}\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "markdownlint", "prettier", "shellcheck"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a missing golangci-lint CLI to be surfaced as a logger failure")
	}
	if !res.Failed() {
		t.Error("expected a missing golangci-lint CLI to make Result.Failed() report true")
	}
	var lint ReportSection
	for _, s := range res.Sections {
		if s.Name == "Linting" {
			lint = s
		}
	}
	if lint.Status != StatusError || !strings.Contains(lint.Body, "golangci-lint not found in PATH") {
		t.Errorf("expected the Linting section to report the missing golangci-lint CLI, got:\n%s", lint.Body)
	}
}

// TestRunAll_MarkdownlintDisabledSkipsCheck guards the opt-out half of the
// hard-fail change above: --disable-checks markdownlint must still work as
// an explicit escape hatch, rendering a "Disabled." child instead of
// running the check at all (and never contributing a failure), exactly
// like kustomize-fix's own disable path.
func TestRunAll_MarkdownlintDisabledSkipsCheck(t *testing.T) {
	d := t.TempDir()
	// Deliberately malformed markdown (a bare URL, which markdownlint's
	// default ruleset flags) - if this ran, it would fail; disabling the
	// check must mean it never runs at all.
	mustWrite(t, filepath.Join(d, "README.md"), "# Title\n\nhttps://example.com\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "markdownlint", "prettier", "shellcheck", "golangci"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected --disable-checks markdownlint to skip the check entirely, not fail")
	}
	var lint ReportSection
	for _, s := range res.Sections {
		if s.Name == "Linting" {
			lint = s
		}
	}
	if !strings.Contains(lint.Body, "Disabled.") {
		t.Errorf("expected the Linting section to show markdownlint as Disabled, got:\n%s", lint.Body)
	}
}

// TestRunAll_KubeconformDisabledSkipsCheck guards that kubeconform, like
// every other lint step, actually honors --disable-checks kubeconform.
// kubeconform previously had no stepEnabled gate at all in phases.go - the
// step ran unconditionally regardless of DisabledChecks/EnabledChecks,
// silently making "kubeconform" (despite being a documented, exemptable
// step ID - see exempt.RegisterExemptable("kubeconform")) impossible to
// turn off wholesale, only per-file via EXEMPTIONS=(check=kubeconform,...).
func TestRunAll_KubeconformDisabledSkipsCheck(t *testing.T) {
	d := t.TempDir()
	// A non-Kubernetes YAML file (no kind/apiVersion) - kubeconform would
	// report this as an error ("missing 'kind' key") if the step ran at
	// all; disabling the check must mean it never runs.
	mustWrite(t, filepath.Join(d, "config.yaml"), "foo: bar\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "markdownlint", "prettier", "shellcheck", "golangci", "kubeconform"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected --disable-checks kubeconform to skip the check entirely, not fail")
	}
	var lint ReportSection
	for _, s := range res.Sections {
		if s.Name == "Linting" {
			lint = s
		}
	}
	if !strings.Contains(lint.Body, "Disabled.") {
		t.Errorf("expected the Linting section to show kubeconform as Disabled, got:\n%s", lint.Body)
	}
}

// TestRunAll_KubeconformSkipsKnownNonManifestFiles guards that well-known
// non-Kubernetes tooling config files (Taskfile.yml, .golangci.yml, ...)
// never trip kubeconform's "missing 'kind' key" error, even though
// kubeconform itself remains enabled and still validates a real
// Kubernetes manifest changed in the same run.
func TestRunAll_KubeconformSkipsKnownNonManifestFiles(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "Taskfile.yml"), "version: '3'\n")
	mustWrite(t, filepath.Join(d, ".golangci.yml"), "run:\n  timeout: 5m\n")
	mustWrite(t, filepath.Join(d, ".goreleaser.yaml"), "version: 2\n")
	mustWrite(t, filepath.Join(d, ".pre-commit-config.yaml"), "repos: []\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "markdownlint", "prettier", "shellcheck", "golangci"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Errorf("expected known non-manifest tooling config files to be skipped by kubeconform, not fail")
	}
}

// --- shellcheck extraction wiring (PR-7) -----------------------------------

func hasShellcheckBinary() bool {
	_, err := exec.LookPath("shellcheck")
	return err == nil
}

func TestWriteShellcheckExtractionReport_TektonAndEmbedded(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	d := t.TempDir()
	taskFile := filepath.Join(d, "task.yaml")
	mustWrite(t, taskFile, `kind: Task
metadata:
  name: build
spec:
  steps:
  - name: build
    script: |
      #!/usr/bin/env bash
      for f in $(ls); do
        echo $f
      done
`)
	deployFile := filepath.Join(d, "deploy.yaml")
	mustWrite(t, deployFile, `kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      containers:
      - name: main
        command:
        - bash
        - -c
        - |
          #!/usr/bin/env bash
          for f in $(ls); do
            echo $f
          done
`)
	var sb strings.Builder
	total := writeShellcheckExtractionReport(&sb, "", []string{taskFile, deployFile})
	if total != 2 {
		t.Fatalf("expected 2 total violations (1 Tekton + 1 embedded), got %d: %s", total, sb.String())
	}
	if !strings.Contains(sb.String(), "[Tekton build/build]") {
		t.Errorf("expected a Tekton-labeled report line, got: %s", sb.String())
	}
	if !strings.Contains(sb.String(), "[Deployment/app main]") {
		t.Errorf("expected an embedded-labeled report line, got: %s", sb.String())
	}
}

func TestWriteShellcheckExtractionReport_LabelSuffix(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	d := t.TempDir()
	taskFile := filepath.Join(d, "task.yaml")
	mustWrite(t, taskFile, `kind: Task
metadata:
  name: build
spec:
  steps:
  - name: build
    script: |
      #!/usr/bin/env bash
      for f in $(ls); do
        echo $f
      done
`)
	var sb strings.Builder
	total := writeShellcheckExtractionReport(&sb, " (external)", []string{taskFile})
	if total != 1 {
		t.Fatalf("expected 1 violation, got %d", total)
	}
	if !strings.Contains(sb.String(), "(external)") {
		t.Errorf("expected the label suffix to appear in the report, got: %s", sb.String())
	}
}

func TestWriteShellcheckExtractionReport_NoFindings(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "cm.yaml")
	mustWrite(t, f, "kind: ConfigMap\nmetadata:\n  name: cm\n")
	var sb strings.Builder
	total := writeShellcheckExtractionReport(&sb, "", []string{f})
	if total != 0 || sb.String() != "" {
		t.Errorf("expected no findings/report for a plain ConfigMap, got total=%d report=%q", total, sb.String())
	}
}

// TestExternalOverlayYAMLFiles_BaseChangeExposesUnchangedOverlayFile
// exercises the direct-vs-external classification's file-set half: a
// change to a shared base file must resolve to the overlay that depends
// on it, and that overlay's own (unchanged) files must show up as
// "external" candidates - while the base file that WAS changed must be
// excluded (it's already covered as "direct").
func TestExternalOverlayYAMLFiles_BaseChangeExposesUnchangedOverlayFile(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - task.yaml\n")
	mustWrite(t, filepath.Join(app, "base", "task.yaml"), "kind: Task\nmetadata:\n  name: build\n")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources:\n  - ../../base\n")

	changed := []string{filepath.Join(app, "base", "kustomization.yaml")}
	external := externalOverlayYAMLFiles(changed)

	foundOverlayKustomization := false
	for _, f := range external {
		if filepath.Clean(f) == filepath.Clean(filepath.Join(app, "overlays", "dev", "kustomization.yaml")) {
			foundOverlayKustomization = true
		}
		if filepath.Clean(f) == filepath.Clean(changed[0]) {
			t.Errorf("expected the directly-changed file to be excluded from external files, got it in: %v", external)
		}
	}
	if !foundOverlayKustomization {
		t.Errorf("expected the affected overlay's own (unchanged) YAML file to be listed as external, got: %v", external)
	}
}

func TestExternalOverlayYAMLFiles_NoOverlaysAffected(t *testing.T) {
	external := externalOverlayYAMLFiles([]string{"README.md"})
	if len(external) != 0 {
		t.Errorf("expected no external files for a change with no affected overlays, got: %v", external)
	}
}

// TestRunLintAndStaticChecks_LogsDurationSuffix guards a regression where
// each check's "<name>: passed" console line gave no indication of how long
// that check took - unlike the fully-built (but previously unprinted, see
// pkg/validator/timing.go) per-step timing already recorded into the
// TimingCollector for the same checks. Every runLintStep/runStaticStep
// closure now also prints a "(<duration>)" suffix on its terminal pass/fail
// line, using the same time.Since(start) already computed for
// tc.RecordStep - this exercises a real check (config-sort, cheap and
// dependency-free) end-to-end through the logger's file-backed writer to
// assert the suffix actually reaches the printed line, not just the
// TimingCollector.
func TestRunLintAndStaticChecks_LogsDurationSuffix(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")
	logPath := filepath.Join(d, "out.log")
	log := logger.NewLogger(true, logPath)
	defer log.Close()

	res := &Result{}
	tc := NewTimingCollector()
	runLintAndStaticChecks([]string{filepath.Join(d, "a.yaml")}, Options{}, res, log, tc, nil)
	log.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	durationSuffixRe := regexp.MustCompile(`config-sort check: passed \([0-9µm.a-zµ]*s\)`)
	if !durationSuffixRe.MatchString(got) {
		t.Errorf("expected a '(<duration>)' suffix on the config-sort check line, got:\n%s", got)
	}
}

func TestExcludeScaffoldArtifacts(t *testing.T) {
	in := []string{
		"myapp/overlays/dev/kustomization.yaml",
		".scafctl/configs/myapp.yaml",
		".scafctl/templates/myapp/overlays/kustomization.yaml",
		"other/deployment.yaml",
	}
	got := excludeScaffoldArtifacts(in)
	want := []string{
		"myapp/overlays/dev/kustomization.yaml",
		"other/deployment.yaml",
	}
	if len(got) != len(want) {
		t.Fatalf("excludeScaffoldArtifacts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilterYAML_ExcludesScaffoldTemplates(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "app", "overlays", "dev", "kustomization.yaml")
	tpl := filepath.Join(dir, ".scafctl", "templates", "app", "overlays", "k.yaml")
	cfg := filepath.Join(dir, ".scafctl", "configs", "app.yaml")
	for _, p := range []string{manifest, tpl, cfg} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("resources: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := filterYAML([]string{manifest, tpl, cfg})
	// Templates are excluded; configs are valid YAML so filterYAML keeps
	// them (manifest validators exclude configs separately via
	// excludeScaffoldArtifacts).
	for _, f := range got {
		if convention.IsScaffoldTemplate(f) {
			t.Errorf("filterYAML must exclude scaffold templates, got %q", f)
		}
	}
	if len(got) != 2 {
		t.Errorf("expected manifest + config (2), got %v", got)
	}
}

func TestIsInvalidTestdata(t *testing.T) {
	cases := map[string]bool{
		"testdata/invalid/bad.yaml":                      true,
		"pkg/lint/shellcheck/testdata/invalid/c.yaml":    true,
		"a/b/testdata/invalid/bad.yaml":                  true,
		"testdata/good.yaml":                             false,
		"pkg/lint/shellcheck/testdata/cronjob-bash.yaml": false,
		"app/overlays/dev/kustomization.yaml":            false,
		"my-invalid/x.yaml":                              false,
	}
	for in, want := range cases {
		if got := isInvalidTestdata(in); got != want {
			t.Errorf("isInvalidTestdata(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestExcludeInvalidTestdata(t *testing.T) {
	in := []string{
		"pkg/overlay/overlay.go",
		"pkg/lint/shellcheck/testdata/invalid/cronjob-bash.yaml",
		"pkg/validator/syncopts/testdata/malformed.yaml", // good/top-level fixture, kept
		"app/overlays/dev/kustomization.yaml",
	}
	got := excludeInvalidTestdata(in)
	want := []string{
		"pkg/overlay/overlay.go",
		"pkg/validator/syncopts/testdata/malformed.yaml",
		"app/overlays/dev/kustomization.yaml",
	}
	if len(got) != len(want) {
		t.Fatalf("excludeInvalidTestdata = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilterYAML_ExcludesInvalidTestdata(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "app", "overlays", "dev", "kustomization.yaml")
	good := filepath.Join(dir, "pkg", "lint", "shellcheck", "testdata", "job-bash.yaml")
	bad := filepath.Join(dir, "pkg", "lint", "shellcheck", "testdata", "invalid", "cronjob-bash.yaml")
	for _, p := range []string{manifest, good, bad} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("resources: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := filterYAML([]string{manifest, good, bad})
	// Only the testdata/invalid/ fixture is dropped; the top-level "good"
	// fixture is kept.
	if len(got) != 2 {
		t.Errorf("filterYAML must drop only testdata/invalid/ fixtures, got %v", got)
	}
	for _, f := range got {
		if isInvalidTestdata(f) {
			t.Errorf("filterYAML leaked an invalid fixture: %q", f)
		}
	}
}
