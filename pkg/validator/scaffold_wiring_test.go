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

func TestRunScaffoldApps_EmptyJobsIsNoOp(t *testing.T) {
	called := false
	runScaffoldApps(nil, nil, nil, 4, func(string, *scaffold.Summary) { called = true })
	if called {
		t.Error("expected record to never be called with no jobs")
	}
}
