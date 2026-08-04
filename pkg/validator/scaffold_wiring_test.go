package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
)

// TestRunAll_ScaffoldSkippedWithoutConfig guards the fix for a real
// over-eager-validation bug found while wiring this up: an app with
// overlay changes but no .scafctl config at all (the common case for a
// generic-core consumer that doesn't use scafctl-based scaffolding) must
// never be treated as a scaffold execution failure just because
// HasScaffoldEnabled defaults to true with no test.sh.
func TestRunAll_ScaffoldSkippedWithoutConfig(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	// kustomize-fix is unrelated to what this test exercises, and this
	// minimal fixture isn't in kustomize's real canonical form, which
	// would otherwise always flag a spurious Kustomize Fix finding and
	// fail the HasFailures() assertion below. shellcheck is disabled for
	// the same reason: it now hard-fails when the CLI isn't installed,
	// and this dev/CI image doesn't have it.
	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "shellcheck"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected no failures for an app with no scaffold config at all")
	}
	var scaffoldSection ReportSection
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" {
			scaffoldSection = s
		}
	}
	if scaffoldSection.Status == StatusError {
		t.Errorf("expected the Scaffold Validation section to be clean, got:\n%s", scaffoldSection.Body)
	}
}

// TestRunAll_ScaffoldExecutionFailureBlocks exercises the real (not mocked)
// scafctl binary: it's installed in this dev/CI image but its "scaffold"
// subcommand doesn't exist here, so an app that DOES opt in (has a
// .scafctl config) with a changed overlay must surface that execution
// failure as a blocking error, distinct from the "no config, skip
// entirely" case above. App identity for scaffold config lookups is
// repo-root-relative (matching pkg/configdiff's own convention), so this
// chdirs into the temp repo root rather than using an absolute app path
// like the other end-to-end tests in this package.
func TestRunAll_ScaffoldExecutionFailureBlocks(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, ".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join(d, "myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	res, err := RunAll(Options{Dirs: []string{"myapp"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a scaffold execution failure to be surfaced as a logger failure")
	}
	var scaffoldSection ReportSection
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" {
			scaffoldSection = s
		}
	}
	if scaffoldSection.Status != StatusError {
		t.Errorf("expected the Scaffold Validation section to report the failure, got:\n%s", scaffoldSection.Body)
	}
	if !strings.Contains(scaffoldSection.Body, "Scaffold Exec") {
		t.Errorf("expected a Scaffold Exec entry, got:\n%s", scaffoldSection.Body)
	}
}

// TestRunAll_ScaffoldReadmeCheckDisabledByDefault guards the
// "scaffold-readme" step's default-off gating: a README scaffold-status
// table with a stale/missing row must NOT be reported unless the step is
// explicitly enabled - see docs/CI.md#scaffold-validation for why.
func TestRunAll_ScaffoldReadmeCheckDisabledByDefault(t *testing.T) {
	d := t.TempDir()
	// A stale row ("removed" has no on-disk overlay) would fail
	// CheckReadmeStatus if it ran.
	table := scaffold.GenerateScaffoldTable([]scaffold.StatusRow{{App: "myapp", Overlay: "removed", Status: "✅ ok"}})
	mustWrite(t, filepath.Join(d, "README.md"), "# Readme\n\n"+table)
	mustWrite(t, filepath.Join(d, "myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	// kustomize-fix is unrelated to what this test exercises, and this
	// minimal fixture isn't in kustomize's real canonical form, which
	// would otherwise always flag a spurious Kustomize Fix finding and
	// fail the HasFailures() assertion below. shellcheck is disabled for
	// the same reason: it now hard-fails when the CLI isn't installed,
	// and this dev/CI image doesn't have it.
	res, err := RunAll(Options{Dirs: []string{"myapp"}, DisabledChecks: []string{"kustomize-fix", "shellcheck"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected the scaffold-readme check to be skipped by default")
	}
	for _, s := range res.Sections {
		if s.Name == "Static Checks" && s.Status == StatusError {
			t.Errorf("expected a clean Static Checks section by default, got:\n%s", s.Body)
		}
	}
}

// TestRunAll_ScaffoldReadmeCheckEnabledViaEnabledChecks is the positive
// counterpart: once explicitly enabled, the same stale row must surface.
func TestRunAll_ScaffoldReadmeCheckEnabledViaEnabledChecks(t *testing.T) {
	d := t.TempDir()
	table := scaffold.GenerateScaffoldTable([]scaffold.StatusRow{{App: "myapp", Overlay: "removed", Status: "✅ ok"}})
	mustWrite(t, filepath.Join(d, "README.md"), "# Readme\n\n"+table)
	mustWrite(t, filepath.Join(d, "myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	res, err := RunAll(Options{Dirs: []string{"myapp"}, EnabledChecks: []string{"scaffold-readme"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected the scaffold-readme check to report the stale row once enabled")
	}
	found := false
	for _, s := range res.Sections {
		if s.Name == "Static Checks" && s.Status == StatusError && strings.Contains(s.Body, "Scaffold Table") {
			found = true
		}
	}
	if !found {
		t.Error("expected the Static Checks section to report a Scaffold Table error once enabled")
	}
}

// chdirTemp chdirs into a fresh temp dir, restoring the original working
// directory on cleanup - findUnprotectedApps (like scaffold.
// HasScaffoldEnabled/HasScaffoldConfig) resolves app/.scafctl paths
// relative to the process CWD, matching a real pipeline run's repo-root
// CWD.
func chdirTemp(t *testing.T) {
	t.Helper()
	d := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
}

// TestFindUnprotectedApps_NoTemplateIsNeverUnprotected guards that an app
// with overlay changes but no scaffold template at all (scaffold-drift
// detection isn't even available for it) is never flagged - there being
// nothing to protect against isn't the same as protection being disabled.
func TestFindUnprotectedApps_NoTemplateIsNeverUnprotected(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "test.sh"), "SCAFFOLD=false\n")

	got := findUnprotectedApps([]string{filepath.Join("myapp", "overlays", "prod", "kustomization.yaml")})
	if len(got) != 0 {
		t.Errorf("expected no unprotected apps without a scaffold template, got %v", got)
	}
}

// TestFindUnprotectedApps_DisabledWithTemplate is the positive case: an app
// with a scaffold template AND overlay changes AND SCAFFOLD=false is
// flagged as unprotected.
func TestFindUnprotectedApps_DisabledWithTemplate(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "test.sh"), "SCAFFOLD=false\n")
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "template.yaml"), "kind: Deployment\n")

	got := findUnprotectedApps([]string{filepath.Join("myapp", "overlays", "prod", "kustomization.yaml")})
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("expected [myapp] to be unprotected, got %v", got)
	}
}

// TestFindUnprotectedApps_EnabledIsNeverFlagged guards the negative case:
// an app with a template but scaffold drift protection actually enabled
// (the default - no SCAFFOLD=false) is never flagged.
func TestFindUnprotectedApps_EnabledIsNeverFlagged(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "test.sh"), "echo hi\n")
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "template.yaml"), "kind: Deployment\n")

	got := findUnprotectedApps([]string{filepath.Join("myapp", "overlays", "prod", "kustomization.yaml")})
	if len(got) != 0 {
		t.Errorf("expected no unprotected apps when scaffold protection is enabled, got %v", got)
	}
}

// TestFindUnprotectedApps_TemplateOrConfigChangeAlsoAttributes guards that
// a change under .scafctl/templates/<app>/ or .scafctl/configs/<app>.yaml
// (not just <app>/overlays/...) also attributes to that app, matching
// runScaffoldValidation's own template/config change triggers.
func TestFindUnprotectedApps_TemplateOrConfigChangeAlsoAttributes(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "test.sh"), "SCAFFOLD=false\n")
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "template.yaml"), "kind: Deployment\n")

	got := findUnprotectedApps([]string{filepath.Join(".scafctl", "templates", "myapp", "template.yaml")})
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("expected a template-only change to still attribute to myapp, got %v", got)
	}

	got = findUnprotectedApps([]string{filepath.Join(".scafctl", "configs", "myapp.yaml")})
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("expected a config-only change to still attribute to myapp, got %v", got)
	}
}

func TestFlattenSkippedClusters_Empty(t *testing.T) {
	if got := flattenSkippedClusters(nil); got != nil {
		t.Errorf("expected nil for an empty/nil map, got %v", got)
	}
}

func TestFlattenSkippedClusters_SortsAppsAndClusters(t *testing.T) {
	got := flattenSkippedClusters(map[string][]string{
		"zapp": {"staging", "dev"},
		"aapp": {"prod"},
	})
	want := []string{"aapp/prod", "zapp/dev", "zapp/staging"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestIsOverlayRelatedToChangedFiles(t *testing.T) {
	cases := []struct {
		name    string
		cluster string
		changed []string
		want    bool
	}{
		{"overlay itself changed", "prod", []string{"myapp/overlays/prod/kustomization.yaml"}, true},
		{"base changed (flows into every overlay)", "prod", []string{"myapp/base/deployment.yaml"}, true},
		{"components changed (flows into every overlay)", "prod", []string{"myapp/components/foo/patch.yaml"}, true},
		{"a different overlay changed", "prod", []string{"myapp/overlays/dev/kustomization.yaml"}, false},
		{"an unrelated app changed", "prod", []string{"otherapp/overlays/prod/kustomization.yaml"}, false},
		{"nothing changed", "prod", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isOverlayRelatedToChangedFiles("myapp", c.cluster, c.changed); got != c.want {
				t.Errorf("isOverlayRelatedToChangedFiles(%q, %v) = %v, want %v", c.cluster, c.changed, got, c.want)
			}
		})
	}
}

func TestComputeBaselineMismatches_EmptyBaseRefSkipsEntirely(t *testing.T) {
	// A local test-all run against a live working tree always has an
	// empty BaseRef (see gitDiff's own doc comment) - this must be an
	// instant no-op, never attempting a git call or touching any file,
	// regardless of whether the CWD is even a git repo.
	log := logger.NewLogger(false, "")
	got := computeBaselineMismatches(Options{}, "myapp", log)
	if len(got) != 0 {
		t.Errorf("expected an empty baseline set, got %v", got)
	}
}

// runGitForTest runs a git command in dir, failing the test on error - used
// to build a small real repo so computeBaselineMismatches's merge-base +
// backup/restore machinery can be exercised end to end.
func runGitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestComputeBaselineMismatches_RestoresFilesRegardlessOfOutcome is the
// safety-critical guard for computeBaselineMismatches: whatever happens
// during the merge-base re-run (scafctl not being configured for the
// baseline content will itself typically error, which is fine - the
// function must still degrade gracefully, never panic), the app's on-disk
// template/config files must end up back at their pre-call (HEAD/PR)
// content, never left sitting at the merge-base content it temporarily
// swapped in.
func TestComputeBaselineMismatches_RestoresFilesRegardlessOfOutcome(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	runGitForTest(t, dir, "init", "-q")
	runGitForTest(t, dir, "config", "user.email", "test@example.com")
	runGitForTest(t, dir, "config", "user.name", "Test")

	configPath := filepath.Join(".scafctl", "configs", "myapp.yaml")
	templatePath := filepath.Join(".scafctl", "templates", "myapp", "template.yaml")
	mustWrite(t, configPath, "v1\n")
	mustWrite(t, templatePath, "v1\n")
	runGitForTest(t, dir, "add", "-A")
	runGitForTest(t, dir, "commit", "-q", "-m", "base")
	runGitForTest(t, dir, "branch", "old-main") // simulates the PR's target branch

	// The "PR's own commit": bump both files past the merge-base content.
	mustWrite(t, configPath, "v2 (PR content)\n")
	mustWrite(t, templatePath, "v2 (PR content)\n")
	runGitForTest(t, dir, "add", "-A")
	runGitForTest(t, dir, "commit", "-q", "-m", "pr change")

	log := logger.NewLogger(false, "")
	// Does not assert on the returned set's contents - scafctl isn't
	// configured with a real solution for "v1"/"v2" content, so the
	// re-run itself is expected to error out (Summary.Errors, no
	// MismatchFiles) - only that it never panics and always restores.
	_ = computeBaselineMismatches(Options{BaseRef: "old-main"}, "myapp", log)

	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config after call: %v", err)
	}
	if string(gotConfig) != "v2 (PR content)\n" {
		t.Errorf("config file left at %q, want restored to the PR content %q", gotConfig, "v2 (PR content)\n")
	}

	gotTemplate, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("reading template after call: %v", err)
	}
	if string(gotTemplate) != "v2 (PR content)\n" {
		t.Errorf("template file left at %q, want restored to the PR content %q", gotTemplate, "v2 (PR content)\n")
	}
}

// TestComputeBaselineMismatches_NewFileNotAtBaselineIsRemovedAfterRestore
// guards the "file didn't exist at merge-base" branch: computeBaseline-
// Mismatches only overwrites a template file with baseline content when
// `git show` actually finds it there; for this test that means the
// template file is never touched at all (it's the config file's baseline-
// absence path that's meaningfully exercised elsewhere), but a brand new
// template file added only in the PR's own commit must still exist,
// untouched, after the call.
func TestComputeBaselineMismatches_NewFileNotAtBaselineIsRemovedAfterRestore(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	runGitForTest(t, dir, "init", "-q")
	runGitForTest(t, dir, "config", "user.email", "test@example.com")
	runGitForTest(t, dir, "config", "user.name", "Test")

	mustWrite(t, "README.md", "placeholder\n")
	runGitForTest(t, dir, "add", "-A")
	runGitForTest(t, dir, "commit", "-q", "-m", "base")
	runGitForTest(t, dir, "branch", "old-main")

	// The app (config + template) is introduced entirely in the PR - it
	// doesn't exist at all at the merge-base.
	configPath := filepath.Join(".scafctl", "configs", "myapp.yaml")
	templatePath := filepath.Join(".scafctl", "templates", "myapp", "template.yaml")
	mustWrite(t, configPath, "new app\n")
	mustWrite(t, templatePath, "new app\n")
	runGitForTest(t, dir, "add", "-A")
	runGitForTest(t, dir, "commit", "-q", "-m", "add myapp")

	log := logger.NewLogger(false, "")
	_ = computeBaselineMismatches(Options{BaseRef: "old-main"}, "myapp", log)

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected the new app's config to still exist after the call, got: %v", err)
	}
	if string(got) != "new app\n" {
		t.Errorf("config file = %q, want unchanged %q", got, "new app\n")
	}
}

func TestRunScaffoldApps_EmptyJobsIsNoOp(t *testing.T) {
	called := false
	runScaffoldApps(nil, nil, nil, 4, func(string, *scaffold.Summary) { called = true })
	if called {
		t.Error("expected record to never be called with no jobs")
	}
}
