package validator

import (
	"path/filepath"
	"testing"
)

func TestDetectAppRoots(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)

	mustWrite(t, filepath.Join("app1", "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join("app1", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	got := detectAppRoots([]string{filepath.Join("app1", "base", "deployment.yaml")})
	if len(got) != 1 || got[0] != "app1" {
		t.Errorf("detectAppRoots = %v, want [app1]", got)
	}
}

func TestDetectAppRoots_ExtraNonAppDirsExcludesMatchingShape(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)

	// A vendored/example directory whose layout coincidentally matches an
	// app's shape (base/ + overlays/) but must never be treated as one.
	mustWrite(t, filepath.Join("vendor-example", "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join("vendor-example", "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("app1", "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join("app1", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	orig := ExtraNonAppDirs
	ExtraNonAppDirs = map[string]bool{"vendor-example": true}
	t.Cleanup(func() { ExtraNonAppDirs = orig })

	files := []string{
		filepath.Join("vendor-example", "base", "deployment.yaml"),
		filepath.Join("app1", "base", "deployment.yaml"),
	}
	got := detectAppRoots(files)
	if len(got) != 1 || got[0] != "app1" {
		t.Errorf("detectAppRoots with ExtraNonAppDirs = %v, want [app1]", got)
	}
}
