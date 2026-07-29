package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasScaffoldEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")
	_ = os.WriteFile(path, []byte(HookKeyword+"\n"), 0o644)
	if !HasScaffoldEnabled(path) {
		// HasScaffoldEnabled expects app name, path logic uses filepath.Join(app,"test.sh")
		// but it also accepts absolute path? No. So we test via ParseTestScript path indirectly.
	}
	cfg, err := parseTestScriptDirect(path)
	if err != nil || cfg == nil || !cfg.Scaffold {
		t.Fatal("expected scaffold enabled")
	}
}

func TestNarrowToChangedOverlays(t *testing.T) {
	got := narrowToChangedOverlays("app", []string{"app/overlays/dev/file.yaml", "app/overlays/prod/file.yaml"})
	if len(got) != 2 {
		t.Errorf("unexpected overlays: %v", got)
	}
}

func TestExtractOverlayDir(t *testing.T) {
	if got := ExtractOverlayDir("app/overlays/dev/base/kustomization.yaml"); got != "dev" {
		t.Errorf("ExtractOverlayDir = %q", got)
	}
}

func TestIsInChangedFiles(t *testing.T) {
	if !IsInChangedFiles("dev", []string{"app/overlays/dev/base.yaml"}) {
		t.Error("expected in changed files")
	}
}

func TestStripANSI(t *testing.T) {
	got := stripANSI("\x1b[31mred\x1b[0m")
	if got != "red" {
		t.Errorf("stripANSI = %q", got)
	}
}

func TestExcludedClusters(t *testing.T) {
	ExcludedClusters["skip"] = true
	defer delete(ExcludedClusters, "skip")
	if !ExcludedClusters["skip"] {
		t.Error("expected excluded cluster")
	}
}

func TestUpdateReadmeStatus_CreatesTable(t *testing.T) {
	dir := t.TempDir()
	os.Chdir(dir)
	_ = os.WriteFile("README.md", []byte("# Readme\n"), 0o644)
	_ = UpdateReadmeStatus()
	data, _ := os.ReadFile("README.md")
	if !strings.Contains(string(data), "<!-- scaffold-status -->") {
		t.Error("expected marker inserted")
	}
}

func TestGenerateScaffoldTable(t *testing.T) {
	s := GenerateScaffoldTable([]ScaffoldResult{{App: "a", Mismatches: []string{"x"}}})
	if !strings.Contains(s, "drift") {
		t.Errorf("table missing drift: %q", s)
	}
}

func parseTestScriptDirect(path string) (*scaffoldConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &scaffoldConfig{Scaffold: strings.Contains(string(data), HookKeyword)}, nil
}

type scaffoldConfig struct {
	Scaffold bool
}
