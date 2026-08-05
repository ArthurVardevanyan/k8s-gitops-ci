package validator

import (
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
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

// A nested (multi-segment) ExtraNonAppDirs prefix must exclude a subtree
// while leaving a sibling real app discoverable.
func TestDetectAppRoots_ExtraNonAppDirsNestedPrefix(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)

	mustWrite(t, filepath.Join("tooling", "examples", "app", "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join("tooling", "examples", "app", "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("app1", "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join("app1", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	orig := ExtraNonAppDirs
	ExtraNonAppDirs = map[string]bool{"tooling/examples": true}
	t.Cleanup(func() { ExtraNonAppDirs = orig })

	files := []string{
		filepath.Join("tooling", "examples", "app", "base", "deployment.yaml"),
		filepath.Join("app1", "base", "deployment.yaml"),
	}
	got := detectAppRoots(files)
	if len(got) != 1 || got[0] != "app1" {
		t.Errorf("detectAppRoots with nested ExtraNonAppDirs = %v, want [app1]", got)
	}
}

// Scaffold templates (<ScaffoldDir>/templates/, e.g. .scafctl/templates/)
// are Go-template source that renders into real overlays; they must never
// be treated as buildable kustomize app roots even though their layout
// contains base/ and overlays/ segments.
func TestDetectAppRoots_ScaffoldTemplatesExcluded(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)

	origDir := convention.ScaffoldDir
	convention.ScaffoldDir = ".scafctl"
	t.Cleanup(func() { convention.ScaffoldDir = origDir })

	// A scaffold template overlay directory that lacks a kustomization.yaml
	// at the "env" overlay path - exactly the shape that previously caused a
	// spurious "unable to find kustomization.yaml" build failure.
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "overlays", "env", "role-binding.yaml"), "kind: RoleBinding\n")
	mustWrite(t, filepath.Join("myapp", "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dv01", "kustomization.yaml"), "resources: []\n")

	files := []string{
		filepath.Join(".scafctl", "templates", "myapp", "overlays", "env", "role-binding.yaml"),
		filepath.Join("myapp", "overlays", "dv01", "kustomization.yaml"),
	}
	got := detectAppRoots(files)
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("detectAppRoots with scaffold template = %v, want [myapp]", got)
	}
}
