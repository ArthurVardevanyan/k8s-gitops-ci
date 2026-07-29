package kustomize

import (
	"os"
	"path/filepath"
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
