package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveChangeset_DirsWithIncludePrefixes(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "kubernetes", "app", "base.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(d, "ansible", "playbook.yaml"), "- hosts: all\n")

	kubernetesDir := filepath.Join(d, "kubernetes")
	ansibleDir := filepath.Join(d, "ansible")

	files, err := resolveChangeset(Options{
		Dirs:            []string{kubernetesDir, ansibleDir},
		IncludePrefixes: []string{kubernetesDir},
	})
	if err != nil {
		t.Fatalf("resolveChangeset: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file after prefix filter, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "base.yaml" {
		t.Errorf("unexpected file: %s", files[0])
	}
}

func TestResolveChangeset_NoIncludePrefixes(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")

	files, err := resolveChangeset(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("resolveChangeset: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
