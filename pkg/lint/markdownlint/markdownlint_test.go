package markdownlint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilter(t *testing.T) {
	in := []string{"a.md", "b.yaml", "c.MD", "d.markdown"}
	out := Filter(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 md files, got %d", len(out))
	}
}

func TestFilterMarkdown(t *testing.T) {
	tmp := t.TempDir()
	md1 := filepath.Join(tmp, "README.md")
	md2 := filepath.Join(tmp, "guide.md")
	md3 := filepath.Join(tmp, "CHANGELOG.md")
	mustWriteFile(t, md1, "# Hi")
	mustWriteFile(t, md2, "# Guide")
	mustWriteFile(t, md3, "# Log")

	files := []string{
		md1,
		md2,
		filepath.Join(tmp, "main.go"),
		filepath.Join(tmp, "deploy.yaml"),
		md3,
	}

	got := FilterMarkdown(files)
	if len(got) != 3 {
		t.Errorf("expected 3 markdown files, got %d: %v", len(got), got)
	}
}

func TestFilterMarkdown_Empty(t *testing.T) {
	got := FilterMarkdown(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 for nil input, got %d", len(got))
	}
}

func TestFilterMarkdown_NoMatches(t *testing.T) {
	files := []string{"main.go", "deploy.yaml"}
	got := FilterMarkdown(files)
	if len(got) != 0 {
		t.Errorf("expected 0, got %d: %v", len(got), got)
	}
}

func TestFilterMarkdown_DeletedFiles(t *testing.T) {
	files := []string{"/nonexistent/path/deleted.md"}
	got := FilterMarkdown(files)
	if len(got) != 0 {
		t.Errorf("expected 0 for deleted files, got %d: %v", len(got), got)
	}
}

func TestFilterMarkdown_SkipsGitHubTemplates(t *testing.T) {
	tmp := t.TempDir()
	// Create real files so they pass the os.Stat check.
	legacyIssue := filepath.Join(tmp, "ISSUE_TEMPLATE.md")
	legacyPR := filepath.Join(tmp, "PULL_REQUEST_TEMPLATE.md")
	dirTemplate := filepath.Join(tmp, ".github", "ISSUE_TEMPLATE", "bug_report.md")
	regular := filepath.Join(tmp, "README.md")

	mustWriteFile(t, legacyIssue, "# Issue")
	mustWriteFile(t, legacyPR, "# PR")
	if err := os.MkdirAll(filepath.Dir(dirTemplate), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	mustWriteFile(t, dirTemplate, "# Bug")
	mustWriteFile(t, regular, "# Readme")

	files := []string{legacyIssue, legacyPR, dirTemplate, regular}
	got := FilterMarkdown(files)
	if len(got) != 1 {
		t.Errorf("expected 1 (only README.md), got %d: %v", len(got), got)
	}
	if len(got) > 0 && got[0] != regular {
		t.Errorf("expected %s, got %s", regular, got[0])
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
