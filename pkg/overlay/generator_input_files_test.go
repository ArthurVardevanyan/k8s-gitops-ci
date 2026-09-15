package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratorInputFiles(t *testing.T) {
	// Create a temp app root with base, components, and overlays that
	// declare configMapGenerator and secretGenerator blocks.
	d := t.TempDir()

	// ── base ───────────────────────────────────────────────────────
	baseDir := filepath.Join(d, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseK := filepath.Join(baseDir, "kustomization.yaml")
	if err := os.WriteFile(baseK, []byte(`
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
configMapGenerator:
- name: app-config
  files:
  - app.conf
  envs:
  - .env
secretGenerator:
- name: app-secret
  files:
  - secret-data.txt
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// ── component ──────────────────────────────────────────────────
	compDir := filepath.Join(d, "components", "extra")
	if err := os.MkdirAll(compDir, 0o755); err != nil {
		t.Fatal(err)
	}
	compK := filepath.Join(compDir, "kustomization.yaml")
	if err := os.WriteFile(compK, []byte(`
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Component
secretGenerator:
- name: component-secret
  files:
  - extra-credentials.txt
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// ── overlay ────────────────────────────────────────────────────
	ovDir := filepath.Join(d, "overlays", "prod")
	if err := os.MkdirAll(ovDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ovK := filepath.Join(ovDir, "kustomization.yaml")
	if err := os.WriteFile(ovK, []byte(`
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../base
components:
- ../components/extra
configMapGenerator:
- name: overlay-config
  files:
  - overlay.conf
`), 0o644); err != nil {
		t.Fatal(err)
	}

	inputs := GeneratorInputFiles(d)

	// Build a set for easy assertions.
	set := make(map[string]bool, len(inputs))
	for _, f := range inputs {
		set[filepath.Base(f)] = true
	}

	// We expect: app.conf, .env, secret-data.txt, extra-credentials.txt,
	// overlay.conf — five files total (one from base configMapGenerator
	// files, one from base configMapGenerator envs, one from base
	// secretGenerator, one from component secretGenerator, one from
	// overlay configMapGenerator).
	expected := []string{"app.conf", ".env", "secret-data.txt", "extra-credentials.txt", "overlay.conf"}
	for _, name := range expected {
		if !set[name] {
			t.Errorf("expected %q in generator inputs, got: %v", name, inputs)
		}
	}

	if len(inputs) != len(expected) {
		t.Errorf("expected %d generator inputs, got %d: %v", len(expected), len(inputs), inputs)
	}
}

func TestGeneratorInputFiles_EmptyAppRoot(t *testing.T) {
	d := t.TempDir()
	inputs := GeneratorInputFiles(d)
	if len(inputs) != 0 {
		t.Fatalf("expected no inputs for empty app root, got: %v", inputs)
	}
}

func TestGeneratorInputFiles_MalformedYAML(t *testing.T) {
	d := t.TempDir()
	baseDir := filepath.Join(d, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write deliberately malformed YAML.
	if err := os.WriteFile(filepath.Join(baseDir, "kustomization.yaml"),
		[]byte("::: not valid yaml {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Should not panic or error; should just return no inputs.
	inputs := GeneratorInputFiles(d)
	if len(inputs) != 0 {
		t.Errorf("expected no inputs for malformed YAML, got: %v", inputs)
	}
}

func TestGeneratorInputFiles_KustomizationFileNames(t *testing.T) {
	// Verify that the legacy "Kustomization" filename (capital K) is
	// also discovered.
	d := t.TempDir()
	baseDir := filepath.Join(d, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "Kustomization"), []byte(`
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
secretGenerator:
- name: legacy
  files:
  - legacy.txt
`), 0o644); err != nil {
		t.Fatal(err)
	}
	inputs := GeneratorInputFiles(d)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input for Kustomization filename, got %d: %v", len(inputs), inputs)
	}
}

func TestGeneratorInputFiles_NestedComponents(t *testing.T) {
	d := t.TempDir()
	// components/x/kustomization.yaml
	compDir := filepath.Join(d, "components", "x")
	if err := os.MkdirAll(compDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(compDir, "kustomization.yaml"), []byte(`
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Component
configMapGenerator:
- name: nested
  files:
  - nested.conf
`), 0o644); err != nil {
		t.Fatal(err)
	}
	inputs := GeneratorInputFiles(d)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input from nested component, got %d: %v", len(inputs), inputs)
	}
	if base := filepath.Base(inputs[0]); base != "nested.conf" {
		t.Errorf("expected nested.conf, got %s", base)
	}
}

// TestGeneratorInputFiles_NonAdjacentDuplicate verifies that files
// referenced by two different generator blocks in the same
// kustomization.yaml are deduplicated even when they appear at
// non-adjacent positions in the raw traversal order. This is a
// regression guard against the dedupSorted order-dependency:
// without sorting the input, non-adjacent duplicates survive.
func TestGeneratorInputFiles_NonAdjacentDuplicate(t *testing.T) {
	t.Parallel()

	d := t.TempDir()
	baseDir := filepath.Join(d, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Two generator blocks; "dup.conf" appears in both but
	// separated by "keep.conf" → non-adjacent in raw results.
	if err := os.WriteFile(filepath.Join(baseDir, "kustomization.yaml"), []byte(`
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
configMapGenerator:
- name: a
  files:
  - dup.conf
  - keep.conf
- name: b
  files:
  - dup.conf
`), 0o644); err != nil {
		t.Fatal(err)
	}

	inputs := GeneratorInputFiles(d)
	if len(inputs) != 2 {
		t.Fatalf("expected 2 unique inputs, got %d: %v", len(inputs), inputs)
	}
	if base := filepath.Base(inputs[0]); base != "dup.conf" {
		t.Errorf("expected dup.conf first, got %s", base)
	}
}
