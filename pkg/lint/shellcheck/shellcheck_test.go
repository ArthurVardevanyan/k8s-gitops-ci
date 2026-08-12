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
	cases := []struct {
		line     string
		wantFile string
		wantLine int
		severity string
		sc       string
	}{
		{"file.sh:10:1: warning: message [SC1234]", "file.sh", 10, "warning", "[SC1234"},
		{"file.sh:11:2: style: msg [SC2250]", "file.sh", 11, "style", "[SC2250"},
		{"file.sh:12:3: info: msg [SC2312]", "file.sh", 12, "info", "[SC2312"},
		{"file.sh:13:4: error: msg [SC2148]", "file.sh", 13, "error", "[SC2148"},
		{"C:\\a\\b.sh:10:1: warning: msg [SC2086]", "C:\\a\\b.sh", 10, "warning", "[SC2086"},
	}
	for _, tc := range cases {
		v := parseGCC(tc.line)
		if len(v) != 1 {
			t.Fatalf("expected 1 violation for %q, got %+v", tc.line, v)
		}
		if v[0].File != tc.wantFile || v[0].Line != tc.wantLine {
			t.Errorf("expected file=%q line=%d, got file=%q line=%d", tc.wantFile, tc.wantLine, v[0].File, v[0].Line)
		}
		if v[0].Severity != tc.severity {
			t.Errorf("expected severity %q, got %q (%+v)", tc.severity, v[0].Severity, v[0])
		}
		if v[0].SC != tc.sc {
			t.Errorf("expected SC %q, got %q (%+v)", tc.sc, v[0].SC, v[0])
		}
	}
}

func TestFilterShell_Empty(t *testing.T) {
	if out := FilterShellScripts(nil); out != nil {
		t.Errorf("expected nil for nil input, got: %v", out)
	}
}

func TestFilterShell_NoMatches(t *testing.T) {
	d := t.TempDir()
	f, _ := os.Create(filepath.Join(d, "readme.md"))
	_ = f.Close()
	out := FilterShellScripts([]string{filepath.Join(d, "readme.md")})
	if len(out) != 0 {
		t.Errorf("expected no matches for a non-shell file, got: %v", out)
	}
}

func TestFilterShell_DeletedFiles(t *testing.T) {
	// A path that no longer exists on disk (e.g. deleted in the same
	// changeset) must not panic and must not be misclassified as a shell
	// script just because ReadFile failed.
	out := FilterShellScripts([]string{"/nonexistent/does-not-exist"})
	if len(out) != 0 {
		t.Errorf("expected no matches for a deleted/nonexistent file, got: %v", out)
	}
	// A deleted .sh file IS still classified as a shell script by
	// extension alone (isShellScript checks the extension before ever
	// touching the filesystem) - this is intentional: Run's own
	// FilterShellScripts -> exec.CommandContext call will simply fail to
	// find the file, which is the caller's problem to handle, not this
	// function's.
	out = FilterShellScripts([]string{"/nonexistent/does-not-exist.sh"})
	if len(out) != 1 {
		t.Errorf("expected a deleted .sh path to still match by extension, got: %v", out)
	}
}

func TestRun_EmptyInput(t *testing.T) {
	violations, out, err := Run(nil)
	if violations != nil || out != "" || err != nil {
		t.Errorf("expected a no-op for nil input, got: %v %q %v", violations, out, err)
	}
}

func TestRun_EmptySlice(t *testing.T) {
	violations, out, err := Run([]string{})
	if violations != nil || out != "" || err != nil {
		t.Errorf("expected a no-op for an empty slice, got: %v %q %v", violations, out, err)
	}
}
