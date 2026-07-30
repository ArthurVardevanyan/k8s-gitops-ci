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

func TestFilterByPrefixes(t *testing.T) {
	files := []string{"kubernetes/app/base.yaml", "okd/base.yaml", "tekton/base/task.yaml", ".tekton/pr.yaml", "ansible/playbook.yaml"}
	got := FilterByPrefixes(files, []string{"kubernetes/", "okd/", "tekton/", ".tekton/"})
	if len(got) != 4 {
		t.Errorf("expected 4 files, got %d: %v", len(got), got)
	}
	if contains(got, "ansible/playbook.yaml") {
		t.Errorf("expected ansible/ to be filtered out: %v", got)
	}
}

func TestFilterByPrefixes_Empty(t *testing.T) {
	files := []string{"a.yaml", "b.yaml"}
	got := FilterByPrefixes(files, nil)
	if len(got) != 2 {
		t.Errorf("expected no-op filter, got %v", got)
	}
}

func TestFilterByPrefixes_Dedup(t *testing.T) {
	// tekton/ and .tekton/ both match tekton/base/task.yaml? No -- ensure a file
	// matching multiple prefixes is only included once.
	files := []string{"tekton/base/task.yaml"}
	got := FilterByPrefixes(files, []string{"tekton/", "tekton/base"})
	if len(got) != 1 {
		t.Errorf("expected single de-duplicated match, got %v", got)
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
	writeFile(dir, "a.txt", "x")
	writeFile(dir, filepath.Join("node_modules", "b.txt"), "y")
	files, err := walkDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	if !contains(names, "a.txt") || contains(names, "b.txt") {
		t.Errorf("unexpected walk result: %v", names)
	}
}

func writeFile(dir, rel, content string) {
	p := filepath.Join(dir, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
