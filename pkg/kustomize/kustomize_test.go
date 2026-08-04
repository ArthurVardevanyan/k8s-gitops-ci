package kustomize

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

// requireKustomizeCLI skips the calling test when the real kustomize
// binary isn't installed - every test in this file exercises the real
// CLI (see this package's doc comment for why it's used directly rather
// than reimplemented), matching the exec.LookPath+t.Skip pattern already
// used elsewhere in this repo for CLI-wrapping tests (e.g.
// pkg/lint/kyverno/policy_test.go).
func requireKustomizeCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	if _, err := exec.LookPath("prettier"); err != nil {
		t.Skip("prettier not installed")
	}
}

func TestRequireKustomize_MissingBinaryIsAHardError(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err == nil {
		t.Skip("kustomize is installed in this environment; can't exercise the missing-binary path")
	}
	if err := requireKustomize(); !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got %v", err)
	}
}

func TestCheckFix_MissingBinaryIsAHardError(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err == nil {
		t.Skip("kustomize is installed in this environment; can't exercise the missing-binary path")
	}
	_, err := CheckFix([]string{"anything/kustomization.yaml"})
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got %v", err)
	}
}

func TestFix_MissingBinaryIsAHardError(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err == nil {
		t.Skip("kustomize is installed in this environment; can't exercise the missing-binary path")
	}
	_, err := Fix(t.TempDir())
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got %v", err)
	}
}

// TestCheckFix_DetectsDeprecatedFieldConversion guards the actual feature
// this whole package exists to expose correctly: `kustomize edit fix
// --vars` converting deprecated fields (patchesStrategicMerge -> patches,
// commonLabels -> labels, vars -> replacements) - not just field
// reordering/formatting - see the --vars flag's own help text ("If
// specified, kustomize will attempt to convert vars to replacements").
func TestCheckFix_DetectsDeprecatedFieldConversion(t *testing.T) {
	requireKustomizeCLI(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "kustomization.yaml")
	content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
vars:
  - name: FOO
    objref:
      kind: ConfigMap
      name: my-configmap
      apiVersion: v1
    fieldref:
      fieldpath: data.foo
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	need, err := CheckFix([]string{f})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 1 {
		t.Fatalf("expected the deprecated vars: field to need a fix, got: %v", need)
	}

	// CheckFix must never mutate the file it's only checking.
	after, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != content {
		t.Error("expected CheckFix to leave the file on disk untouched")
	}
}

func TestCheckFix_AlreadyFixedFileNotFlagged(t *testing.T) {
	requireKustomizeCLI(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "kustomization.yaml")
	content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: my-app
resources:
  - deployment.yaml
patches:
  - path: patch.yaml
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bring it to the real fix pipeline's stable form first (kustomize
	// edit fix's own writer changes sequence-item indentation even when
	// no field actually needs converting - see this package's doc
	// comments) - a file we ourselves haven't run through Fix yet isn't
	// a reliable "already fixed" fixture.
	if _, err := Fix(dir); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	need, err := CheckFix([]string{f})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 0 {
		t.Errorf("expected an already-fixed file not to be flagged again, got: %v", need)
	}
}

func TestCheckFix_TemplateFilesSkipped(t *testing.T) {
	requireKustomizeCLI(t)
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()

	tmpl := filepath.Join(dir, "templates", "kustomization.yaml")
	if err := os.MkdirAll(filepath.Dir(tmpl), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpl, []byte("vars:\n  - name: FOO\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	need, err := CheckFix([]string{tmpl})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 0 {
		t.Errorf("template should be skipped: %v", need)
	}
}

func TestAppsFromFiles(t *testing.T) {
	apps := AppsFromFiles([]string{"app1/overlays/dev/kustomization.yaml", "app2/base/kustomization.yaml"})
	if len(apps) != 2 || apps[0] != "dev" {
		t.Errorf("unexpected apps: %v", apps)
	}
}

func TestFormatFixNeeded(t *testing.T) {
	s := FormatFixNeeded([]string{})
	if s != "" {
		t.Errorf("expected empty, got %q", s)
	}
}

// TestFix_RecursivelyFixesNestedKustomizationFiles guards the actual bug
// reported against the "kustomize-fix" CLI command: Fix used to only
// os.ReadDir a single directory (no recursion), so running it against an
// app root like "okd/okd-configuration/" would never reach a nested
// overlay's kustomization.yaml (e.g.
// "okd/okd-configuration/overlays/sandbox/kustomization.yaml") at all.
// Also guards that Fix actually converts vars: to replacements: (--vars),
// and that a follow-up CheckFix agrees the fixed file is now clean.
func TestFix_RecursivelyFixesNestedKustomizationFiles(t *testing.T) {
	requireKustomizeCLI(t)
	root := t.TempDir()
	nested := filepath.Join(root, "overlays", "sandbox", "kustomization.yaml")
	content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
vars:
  - name: FOO
    objref:
      kind: ConfigMap
      name: my-configmap
      apiVersion: v1
    fieldref:
      fieldpath: data.foo
`
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	need, err := CheckFix([]string{nested})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 1 {
		t.Fatalf("expected the nested file to need a fix before calling Fix, got: %v", need)
	}

	fixed, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(fixed) != 1 || fixed[0] != nested {
		t.Errorf("expected exactly the nested file to be reported fixed, got: %v", fixed)
	}

	after, err := os.ReadFile(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "replacements:") || strings.Contains(string(after), "vars:") {
		t.Errorf("expected vars: to actually be converted to replacements: (--vars), got:\n%s", after)
	}

	need, err = CheckFix([]string{nested})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 0 {
		t.Errorf("expected the nested file to no longer need a fix after Fix wrote it back, got: %v", need)
	}
}

// TestFix_LeavesAlreadyFixedFilesUntouched guards that Fix is idempotent
// on a clean tree: a file already in the real fix pipeline's stable form
// isn't reported as fixed (and, implicitly, isn't rewritten again).
func TestFix_LeavesAlreadyFixedFilesUntouched(t *testing.T) {
	requireKustomizeCLI(t)
	root := t.TempDir()
	f := filepath.Join(root, "kustomization.yaml")
	content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Fix(root); err != nil {
		t.Fatalf("Fix (first pass): %v", err)
	}
	stable, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	fixed, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix (second pass): %v", err)
	}
	if len(fixed) != 0 {
		t.Errorf("expected no files reported as fixed on an already-stable tree, got: %v", fixed)
	}
	after, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(stable) {
		t.Error("expected the file to be unchanged by a second Fix pass")
	}
}

// TestFix_SkipsScaffoldTemplates mirrors TestCheckFix_TemplateFilesSkipped
// for Fix: a scaffold template's kustomization.yaml is deliberately not
// real, on-disk app content, so it must never be rewritten.
func TestFix_SkipsScaffoldTemplates(t *testing.T) {
	requireKustomizeCLI(t)
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()

	tmpl := filepath.Join(dir, "templates", "kustomization.yaml")
	if err := os.MkdirAll(filepath.Dir(tmpl), 0o755); err != nil {
		t.Fatal(err)
	}
	unfixed := []byte("vars:\n  - name: FOO\n")
	if err := os.WriteFile(tmpl, unfixed, 0o644); err != nil {
		t.Fatal(err)
	}

	fixed, err := Fix(dir)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(fixed) != 0 {
		t.Errorf("expected the scaffold template to be skipped, got: %v", fixed)
	}
	after, err := os.ReadFile(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(unfixed) {
		t.Errorf("expected the scaffold template to be left untouched, got: %q", after)
	}
}

// TestFix_NonexistentRoot guards that a bad root path surfaces a real
// error instead of silently reporting "nothing to fix".
func TestFix_NonexistentRoot(t *testing.T) {
	requireKustomizeCLI(t)
	_, err := Fix(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Error("expected an error for a nonexistent root")
	}
}

// TestFix_PreservesLeadingDocumentSeparator guards against a real
// regression: `kustomize edit fix`'s own writer silently drops a leading
// "---" YAML document-start marker, and prettier's --write pass (which
// always runs right after) doesn't restore it either - so without
// runFixPipeline's explicit restore step, `kustomize-fix` would strip a
// marker the operator had in their file on disk.
func TestFix_PreservesLeadingDocumentSeparator(t *testing.T) {
	requireKustomizeCLI(t)
	root := t.TempDir()
	f := filepath.Join(root, "kustomization.yaml")
	content := `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
vars:
  - name: FOO
    objref:
      kind: ConfigMap
      name: my-configmap
      apiVersion: v1
    fieldref:
      fieldpath: data.foo
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	fixed, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(fixed) != 1 {
		t.Fatalf("expected the file to be reported fixed (vars: -> replacements:), got: %v", fixed)
	}

	after, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), "---\n") {
		t.Errorf("expected the leading --- document separator to be preserved, got:\n%s", after)
	}
	if !strings.Contains(string(after), "replacements:") || strings.Contains(string(after), "vars:") {
		t.Errorf("expected vars: to still be converted to replacements:, got:\n%s", after)
	}

	// CheckFix must agree the file is now clean - otherwise the
	// preserved "---" would make Fix/CheckFix disagree forever.
	need, err := CheckFix([]string{f})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 0 {
		t.Errorf("expected the fixed file not to be flagged again, got: %v", need)
	}
}

// TestFix_NoLeadingDocumentSeparatorNotAdded guards the other direction:
// a file that never had a leading "---" must not gain one.
func TestFix_NoLeadingDocumentSeparatorNotAdded(t *testing.T) {
	requireKustomizeCLI(t)
	root := t.TempDir()
	f := filepath.Join(root, "kustomization.yaml")
	content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
vars:
  - name: FOO
    objref:
      kind: ConfigMap
      name: my-configmap
      apiVersion: v1
    fieldref:
      fieldpath: data.foo
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Fix(root); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	after, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(string(after), "---") {
		t.Errorf("expected no leading --- to be added, got:\n%s", after)
	}
}

// TestCheckFix_AlreadyFixedFileWithLeadingSeparatorNotFlagged mirrors
// TestCheckFix_AlreadyFixedFileNotFlagged specifically for a file with a
// leading "---": the restore step in runFixPipeline must be idempotent,
// or CheckFix/Fix would never converge for such a file.
func TestCheckFix_AlreadyFixedFileWithLeadingSeparatorNotFlagged(t *testing.T) {
	requireKustomizeCLI(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "kustomization.yaml")
	content := `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: my-app
resources:
  - deployment.yaml
patches:
  - path: patch.yaml
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bring it to the real fix pipeline's stable form first, same as
	// TestCheckFix_AlreadyFixedFileNotFlagged.
	if _, err := Fix(dir); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	need, err := CheckFix([]string{f})
	if err != nil {
		t.Fatalf("CheckFix: %v", err)
	}
	if len(need) != 0 {
		t.Errorf("expected an already-fixed file (with a leading ---) not to be flagged again, got: %v", need)
	}
}
