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
