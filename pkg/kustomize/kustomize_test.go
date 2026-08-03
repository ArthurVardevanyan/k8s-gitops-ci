package kustomize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

func TestNormalizeYAML(t *testing.T) {
	in := `zzz: 1
aaa: 2
`
	out, err := NormalizeYAML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "aaa: 2\nzzz: 1\n"
	if string(out) != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestNormalizeYAML_InvalidInput(t *testing.T) {
	_, err := NormalizeYAML([]byte("[bad"))
	if err == nil {
		t.Error("expected error")
	}
}

func TestNormalizeYAML_EmptyInput(t *testing.T) {
	out, err := NormalizeYAML([]byte(""))
	if err != nil || string(out) != "" {
		t.Errorf("empty should remain empty: %q err %v", out, err)
	}
}

func TestNormalizeYAML_LeadingDocumentMarkerPreserved(t *testing.T) {
	in := `---
zzz: 1
aaa: 2
`
	out, err := NormalizeYAML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\naaa: 2\nzzz: 1\n"
	if string(out) != want {
		t.Errorf("got %q want %q", out, want)
	}

	// Re-normalizing the already-normalized output must be a no-op
	// (idempotent), otherwise CheckFix would report a false positive.
	out2, err := NormalizeYAML(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(out2) != string(out) {
		t.Errorf("normalization not idempotent: got %q want %q", out2, out)
	}
}

func TestNormalizeYAML_NoLeadingDocumentMarker(t *testing.T) {
	in := "zzz: 1\naaa: 2\n"
	out, err := NormalizeYAML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(string(out), "---") {
		t.Errorf("did not expect a leading document marker to be introduced: %q", out)
	}
}

func TestNormalizeYAML_MultiDocumentWithLeadingMarker(t *testing.T) {
	in := `---
zzz: 1
aaa: 2
---
bbb: 2
ccc: 1
`
	out, err := NormalizeYAML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\naaa: 2\nzzz: 1\n---\nbbb: 2\nccc: 1\n"
	if string(out) != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestCheckFix_LeadingDocumentMarkerNotFlagged(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "kustomization.yaml")
	content := `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: homelab
resources:
  - repo.yaml
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	need, err := CheckFix([]string{f})
	if err != nil {
		t.Fatal(err)
	}
	if len(need) != 0 {
		t.Errorf("file with only a leading document marker and already-sorted keys should not need a fix: %v", need)
	}
}

func TestCheckFix_TemplateFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()

	tmpl := filepath.Join(dir, "templates", "kustomization.yaml")
	_ = os.MkdirAll(filepath.Dir(tmpl), 0o755)
	_ = os.WriteFile(tmpl, []byte("zzz: 1\naaa: 2\n"), 0o644)
	need, _ := CheckFix([]string{tmpl})
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

// TestFix_RecursivelyNormalizesNestedKustomizationFiles guards the actual
// bug reported against the "kustomize-fix" CLI command: Fix used to only
// os.ReadDir a single directory (no recursion), so running it against an
// app root like "okd/okd-configuration/" would never reach a nested
// overlay's kustomization.yaml (e.g.
// "okd/okd-configuration/overlays/sandbox/kustomization.yaml") at all.
func TestFix_RecursivelyNormalizesNestedKustomizationFiles(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "overlays", "sandbox", "kustomization.yaml")
	unsorted := "resources:\n  - resource.yaml\nkind: Kustomization\napiVersion: kustomize.config.k8s.io/v1beta1\n"
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte(unsorted), 0o644); err != nil {
		t.Fatal(err)
	}

	need, err := CheckFix([]string{nested})
	if err != nil {
		t.Fatal(err)
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

	need, err = CheckFix([]string{nested})
	if err != nil {
		t.Fatal(err)
	}
	if len(need) != 0 {
		t.Errorf("expected the nested file to no longer need a fix after Fix wrote it back, got: %v", need)
	}
}

// TestFix_LeavesAlreadyNormalizedFilesUntouched guards that an
// already-normalized file isn't reported as fixed (and, implicitly, isn't
// rewritten) - Fix should be idempotent/a no-op on a clean tree.
func TestFix_LeavesAlreadyNormalizedFilesUntouched(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "kustomization.yaml")
	sorted := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - resource.yaml\n"
	if err := os.WriteFile(f, []byte(sorted), 0o644); err != nil {
		t.Fatal(err)
	}

	fixed, err := Fix(root)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(fixed) != 0 {
		t.Errorf("expected no files reported as fixed for an already-normalized tree, got: %v", fixed)
	}
}

// TestFix_SkipsScaffoldTemplates mirrors TestCheckFix_TemplateFilesSkipped
// for Fix: a scaffold template's kustomization.yaml is deliberately not
// real, on-disk app content, so it must never be rewritten.
func TestFix_SkipsScaffoldTemplates(t *testing.T) {
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()

	tmpl := filepath.Join(dir, "templates", "kustomization.yaml")
	if err := os.MkdirAll(filepath.Dir(tmpl), 0o755); err != nil {
		t.Fatal(err)
	}
	unsorted := []byte("zzz: 1\naaa: 2\n")
	if err := os.WriteFile(tmpl, unsorted, 0o644); err != nil {
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
	if string(after) != string(unsorted) {
		t.Errorf("expected the scaffold template to be left untouched, got: %q", after)
	}
}

// TestFix_NonexistentRoot guards that a bad root path surfaces a real
// error instead of silently reporting "nothing to fix".
func TestFix_NonexistentRoot(t *testing.T) {
	_, err := Fix(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Error("expected an error for a nonexistent root")
	}
}
