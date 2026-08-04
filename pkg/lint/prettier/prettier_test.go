package prettier

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilter(t *testing.T) {
	in := []string{"a.yaml", "b.go", "c.json", "d.md"}
	out := Filter(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 prettier files, got %d", len(out))
	}
}

func TestWrite_NoFilterableFiles(t *testing.T) {
	out, err := Write([]string{"a.go", "b.bin"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output for a file list with nothing prettier can format, got: %q", out)
	}
}

// TestWrite_ActuallyRewritesFile guards the core behavior pkg/kustomize.Fix
// depends on: Write must actually rewrite the file in place (unlike Run,
// which only checks).
func TestWrite_ActuallyRewritesFile(t *testing.T) {
	if _, err := exec.LookPath("prettier"); err != nil {
		t.Skip("prettier not installed")
	}
	f := filepath.Join(t.TempDir(), "kustomization.yaml")
	unformatted := "resources:\n- a.yaml\n- b.yaml\n"
	if err := os.WriteFile(f, []byte(unformatted), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Write([]string{f}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	after, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) == unformatted {
		t.Error("expected the file to actually be rewritten by prettier --write")
	}
	if !strings.Contains(string(after), "  - a.yaml") {
		t.Errorf("expected prettier's indented sequence-item style, got: %s", after)
	}
}
