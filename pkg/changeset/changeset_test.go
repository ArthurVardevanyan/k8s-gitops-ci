package changeset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRepoFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/org/repo":     "org/repo",
		"https://github.com/org/repo.git": "org/repo",
		"git@github.com:org/repo.git":     "org/repo",
		"":                                "",
	}
	for in, want := range cases {
		if got := ExtractRepoFromURL(in); got != want {
			t.Errorf("ExtractRepoFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterByExtension(t *testing.T) {
	in := []string{"a.yaml", "b.go", "c.yml"}
	got := FilterByExtension(in, ".yaml", ".yml")
	if len(got) != 2 || !contains(got, "a.yaml") || !contains(got, "c.yml") {
		t.Errorf("unexpected filter result: %v", got)
	}
}

func TestExcludeByExtension(t *testing.T) {
	in := []string{"a.yaml", "b.go", "c.md"}
	got := ExcludeByExtension(in, ".go")
	if len(got) != 2 || contains(got, "b.go") {
		t.Errorf("unexpected exclude result: %v", got)
	}
}

func TestFilterByPrefix(t *testing.T) {
	got := FilterByPrefix([]string{"app/a.yaml", "base/b.yaml"}, "app/")
	if len(got) != 1 || got[0] != "app/a.yaml" {
		t.Errorf("unexpected prefix result: %v", got)
	}
}

func TestDetectApps_FilterByPrefix(t *testing.T) {
	files := []string{"app1/base.yaml", "app2/base.yaml", "app1/overlays/x.yaml"}
	got := FilterByApp(files, []string{"app1"})
	if len(got) != 2 || !contains(got, "app1/base.yaml") {
		t.Errorf("unexpected app filter: %v", got)
	}
}

func TestGetChangedFiles_LocalMode(t *testing.T) {
	_, err := GetChangedFiles(Options{})
	if err != nil {
		t.Skipf("git diff not available: %v", err)
	}
}

func TestIsPACPlaceholder(t *testing.T) {
	if !IsPACPlaceholder("{{ params.url }}") {
		t.Error("expected placeholder")
	}
	if IsPACPlaceholder("123") {
		t.Error("expected not placeholder")
	}
}

func TestWalkDir(t *testing.T) {
	dir := t.TempDir()
	_ = writeFile(dir, "a.txt", "x")
	_ = writeFile(dir, filepath.Join("node_modules", "b.txt"), "y")
	files, err := walkDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	if !contains(names, "a.txt") || contains(names, "b.txt") {
		t.Errorf("unexpected walk result: %v", names)
	}
}

func writeFile(dir, rel, content string) string {
	p := filepath.Join(dir, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
	return p
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
