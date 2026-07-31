package kustomize

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestResolveRefs_NoKustomization(t *testing.T) {
	dir := t.TempDir()
	if got := ResolveRefs(dir); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestResolveRefs_ResourcesAndBases(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	writeFile(t, filepath.Join(base, "kustomization.yaml"), "resources:\n  - namespace.yaml\n  - deployment.yaml\n")
	writeFile(t, filepath.Join(base, "namespace.yaml"), "kind: Namespace\n")
	writeFile(t, filepath.Join(base, "deployment.yaml"), "kind: Deployment\n")

	overlay := filepath.Join(root, "overlays", "dev")
	writeFile(t, filepath.Join(overlay, "kustomization.yaml"), "resources:\n  - ../../base\n")

	refs := ResolveRefs(overlay)
	wantBase, _ := filepath.Abs(base)
	found := false
	for _, r := range refs {
		abs, _ := filepath.Abs(r)
		if abs == wantBase {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected refs to include base dir %s, got %v", base, refs)
	}
	// Recursion into the base dir should surface its own resources too.
	wantNS, _ := filepath.Abs(filepath.Join(base, "namespace.yaml"))
	nsFound := false
	for _, r := range refs {
		abs, _ := filepath.Abs(r)
		if abs == wantNS {
			nsFound = true
		}
	}
	if !nsFound {
		t.Errorf("expected refs to recurse into base's namespace.yaml, got %v", refs)
	}
}

func TestResolveRefs_Components(t *testing.T) {
	root := t.TempDir()
	comp := filepath.Join(root, "components", "widget", "1.0.0")
	writeFile(t, filepath.Join(comp, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	writeFile(t, filepath.Join(comp, "deployment.yaml"), "kind: Deployment\n")

	overlay := filepath.Join(root, "overlays", "dev")
	writeFile(t, filepath.Join(overlay, "kustomization.yaml"), "components:\n  - ../../components/widget/1.0.0\n")

	refs := ResolveRefs(overlay)
	wantComp, _ := filepath.Abs(comp)
	found := false
	for _, r := range refs {
		abs, _ := filepath.Abs(r)
		if abs == wantComp {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected refs to include component dir %s, got %v", comp, refs)
	}
}

// TestResolveRefs_CycleProtection ensures a self-referential (or mutually
// referential) kustomization graph terminates instead of recursing forever.
func TestResolveRefs_CycleProtection(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	writeFile(t, filepath.Join(a, "kustomization.yaml"), "resources:\n  - ../b\n")
	writeFile(t, filepath.Join(b, "kustomization.yaml"), "resources:\n  - ../a\n")

	done := make(chan []string, 1)
	go func() { done <- ResolveRefs(a) }()
	select {
	case refs := <-done:
		sort.Strings(refs)
		if len(refs) == 0 {
			t.Errorf("expected some refs before the cycle was cut, got none")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ResolveRefs did not terminate on a cyclic kustomization graph")
	}
}
