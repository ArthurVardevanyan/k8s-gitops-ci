package validator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

func TestRunKyvernoValidation_NoOutputsSkipsEntirely(t *testing.T) {
	log := logger.NewLogger(false, "")
	s := runKyvernoValidation(nil, nil, "", log)
	if s.Name != "Kyverno Policies" {
		t.Errorf("Name = %q, want %q", s.Name, "Kyverno Policies")
	}
	if s.Error {
		t.Error("expected no error with no rendered overlays or source files to validate")
	}
	if s.Body != "No Kyverno findings." {
		t.Errorf("Body = %q, want the no-findings stub", s.Body)
	}
}

// TestRunKyvernoValidation_SourceFilesOnlyStillRuns guards that Kyverno
// validates raw changed YAML source files even with zero successfully-
// rendered overlays (e.g. a brand new component not yet referenced by any
// overlay's kustomization.yaml, which never appears in any rendered
// output) - this is the whole point of the sourceFiles parameter, not just
// an edge case of the rendered-overlay pass.
func TestRunKyvernoValidation_SourceFilesOnlyStillRuns(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "deployment.yaml")
	mustWrite(t, f, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")

	log := logger.NewLogger(false, "")
	s := runKyvernoValidation(nil, []string{f}, "", log)
	if s.Name != "Kyverno Policies" {
		t.Errorf("Name = %q, want %q", s.Name, "Kyverno Policies")
	}
	if log.HasFailures() {
		t.Error("expected a missing kyverno CLI to never mark the logger as failed")
	}
}

// TestRunKyvernoValidation_ExcludesKustomizationFiles guards that
// kustomization.yaml/.yml/Kustomization files - not real resource
// manifests - are filtered out of the raw-source-file input rather than
// passed to the kyverno CLI as noise.
func TestRunKyvernoValidation_ExcludesKustomizationFiles(t *testing.T) {
	if got := isKustomizationFile("myapp/overlays/prod/kustomization.yaml"); !got {
		t.Error("expected kustomization.yaml to be excluded")
	}
	if got := isKustomizationFile("myapp/overlays/prod/kustomization.yml"); !got {
		t.Error("expected kustomization.yml to be excluded")
	}
	if got := isKustomizationFile("myapp/base/deployment.yaml"); got {
		t.Error("expected a real resource manifest to not be excluded")
	}
}

// TestRunKyvernoValidation_MissingCLIDegradesGracefully guards that Kyverno
// support is best-effort once enabled (see docs/CI.md): a missing kyverno
// CLI must never fail the run, only skip validation - and, critically,
// must never call anything that marks the Logger as failed (kyverno
// findings/setup issues are a non-blocking advisory, not a build error).
func TestRunKyvernoValidation_MissingCLIDegradesGracefully(t *testing.T) {
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n")},
	}
	s := runKyvernoValidation(outputs, nil, "", log)
	if s.Error {
		t.Error("expected no error when the kyverno CLI is unavailable")
	}
	if log.HasFailures() {
		t.Error("expected a missing kyverno CLI to never mark the logger as failed")
	}
}

// TestRunAll_KyvernoSectionOnlyPresentWhenEnabled guards the "kyverno"
// step's default-off gating end to end: RunAll must not produce a
// "Kyverno Policies" section at all unless the step is explicitly enabled
// (see stepKyverno/defaultOffSteps) - not even an empty one.
func TestRunAll_KyvernoSectionOnlyPresentWhenEnabled(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	for _, s := range res.Sections {
		if s.Name == "Kyverno Policies" {
			t.Errorf("expected no Kyverno Policies section when the step isn't enabled, got: %+v", s)
		}
	}
}

func TestRunAll_KyvernoSectionPresentWhenEnabled(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}, EnabledChecks: []string{"kyverno"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	found := false
	for _, s := range res.Sections {
		if s.Name == "Kyverno Policies" {
			found = true
		}
	}
	if !found {
		t.Error("expected a Kyverno Policies section once the step is enabled")
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected enabling kyverno (with no CLI installed) to never fail the run")
	}
}

func TestRunAll_KyvernoDisabledEvenIfExplicitlyDisabled(t *testing.T) {
	// Redundant with the default-off test above, but guards the explicit
	// --disable-checks kyverno path too (a no-op today since it's already
	// off by default, but should stay a no-op if that default ever flips).
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kyverno"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	for _, s := range res.Sections {
		if s.Name == "Kyverno Policies" {
			t.Errorf("expected no Kyverno Policies section when explicitly disabled, got: %+v", s)
		}
	}
}

func TestRunKyvernoValidation_WritesEachOutputAsASeparateFile(t *testing.T) {
	// Not directly observable from the Section alone (no CLI installed in
	// the test environment to report violations back), but at minimum this
	// must not panic across a multi-output batch, exercising the
	// resource-N.yaml naming/remap bookkeeping.
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/dev", data: []byte("kind: ConfigMap\n")},
		{overlay: "myapp/overlays/prod", data: []byte("kind: ConfigMap\n")},
	}
	s := runKyvernoValidation(outputs, nil, "", log)
	if !strings.Contains(s.Name, "Kyverno") {
		t.Errorf("unexpected section: %+v", s)
	}
}
