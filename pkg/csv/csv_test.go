package csv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckStartingCSVFolderMatch_NoFiles(t *testing.T) {
	m, err := CheckStartingCSVFolderMatch([]string{"app/base/kustomization.yaml"})
	if err != nil || len(m) != 0 {
		t.Fatalf("expected no matches: %v err %v", m, err)
	}
}

func TestCheckStartingCSVFolderMatch_Match(t *testing.T) {
	dir := writeComponent(t, "v2.12.1", "foo-operator.v2.12.1")
	m, err := CheckStartingCSVFolderMatch([]string{dir})
	if err != nil || len(m) != 0 {
		t.Fatalf("expected match: %v err %v", m, err)
	}
}

func TestCheckStartingCSVFolderMatch_Mismatch(t *testing.T) {
	dir := writeComponent(t, "v2.12.0", "foo-operator.v2.12.1")
	m, err := CheckStartingCSVFolderMatch([]string{dir})
	if err != nil || len(m) != 1 {
		t.Fatalf("expected mismatch: %v err %v", m, err)
	}
	if !strings.Contains(FormatMismatches(m), "startingCSV version") {
		t.Errorf("format missing header: %q", FormatMismatches(m))
	}
}

func writeComponent(t *testing.T, folder, csv string) string {
	t.Helper()
	dir := t.TempDir()
	comp := filepath.Join(dir, "components", folder, "kustomization.yaml")
	_ = os.MkdirAll(filepath.Dir(comp), 0o755)
	content := `- op: add
  path: /spec/startingCSV
  value: ` + csv + `
`
	_ = os.WriteFile(comp, []byte(content), 0o644)
	return comp
}

func TestExtractVersionFolder(t *testing.T) {
	tests := []struct {
		file   string
		folder string
	}{
		{"foo/components/v1.0.0/kustomization.yaml", "v1.0.0"},
		{"foo/components/1.0/kustomization.yaml", "1.0"},
		{"foo/components/other/kustomization.yaml", "other"},
	}
	for _, tc := range tests {
		if got := folderForFile(tc.file); got != tc.folder {
			t.Errorf("folderForFile(%q) = %q", tc.file, got)
		}
	}
}
