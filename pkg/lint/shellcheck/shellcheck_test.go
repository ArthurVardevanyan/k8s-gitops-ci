package shellcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterShellScripts(t *testing.T) {
	d := t.TempDir()
	f1, _ := os.Create(filepath.Join(d, "a.sh"))
	_ = f1.Close()
	err := os.WriteFile(filepath.Join(d, "b"), []byte("#!/bin/bash\necho hi"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	f3, _ := os.Create(filepath.Join(d, "c.go"))
	_ = f3.Close()
	files := []string{filepath.Join(d, "a.sh"), filepath.Join(d, "b"), filepath.Join(d, "c.go")}
	out := FilterShellScripts(files)
	if len(out) != 2 {
		t.Fatalf("expected 2 shell scripts, got %d", len(out))
	}
}

func TestParseGCC(t *testing.T) {
	v := parseGCC("file.sh:10:1: warning: message [SC1234]")
	if len(v) != 1 || v[0].File != "file.sh" || v[0].Line != 10 {
		t.Errorf("unexpected parse: %+v", v)
	}
}
