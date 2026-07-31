package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected no failures for an app with no scaffold config at all")
	}
	var scaffoldSection Section
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" {
			scaffoldSection = s
		}
	}
	if scaffoldSection.Error {
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
	var scaffoldSection Section
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" {
			scaffoldSection = s
		}
	}
	if !scaffoldSection.Error {
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

	res, err := RunAll(Options{Dirs: []string{"myapp"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected the scaffold-readme check to be skipped by default")
	}
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" && s.Error {
			t.Errorf("expected a clean Scaffold Validation section by default, got:\n%s", s.Body)
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
		if s.Name == "Scaffold Validation" && s.Error {
			found = true
		}
	}
	if !found {
		t.Error("expected the Scaffold Validation section to report an error once enabled")
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

func TestRunScaffoldApps_EmptyJobsIsNoOp(t *testing.T) {
	called := false
	runScaffoldApps(nil, nil, nil, 4, func(string, *scaffold.Summary) { called = true })
	if called {
		t.Error("expected record to never be called with no jobs")
	}
}
