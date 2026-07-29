package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAllOverlays_NoDir(t *testing.T) {
	if got := FindAllOverlays("/tmp/not-an-app-lkj"); got != nil {
		t.Errorf("expected nil: %v", got)
	}
}

func TestIsExcluded(t *testing.T) {
	if !IsExcluded("app/overlays/excluded", map[string]bool{"excluded": true}) {
		t.Error("expected excluded")
	}
}

func TestGetOverlaysToTest_NoChanges(t *testing.T) {
	dir := makeApp(t)
	ov, full, _ := GetOverlaysToTest(dir, []string{}, false)
	if len(ov) != 0 || full {
		t.Errorf("expected no overlays: %v full=%v", ov, full)
	}
}

func TestGetOverlaysToTest_BaseChange(t *testing.T) {
	dir := makeApp(t)
	ov, full, _ := GetOverlaysToTest(dir, []string{dir + "/base/kustomization.yaml"}, false)
	if !full || len(ov) != 2 {
		t.Errorf("expected full test: %v full=%v", ov, full)
	}
}

func TestGetOverlaysToTest_SpecificOverlay(t *testing.T) {
	dir := makeApp(t)
	ov, full, _ := GetOverlaysToTest(dir, []string{dir + "/overlays/dev/kustomization.yaml"}, false)
	if full || len(ov) != 1 {
		t.Errorf("expected one overlay: %v full=%v", ov, full)
	}
}

func TestRunBuildLoop_Empty(t *testing.T) {
	res := RunBuildLoop(BuildOptions{})
	if len(res) != 0 {
		t.Errorf("expected empty: %v", res)
	}
}

func TestRunBuildLoop_KustomizeBuild(t *testing.T) {
	dir := makeApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	res := RunBuildLoop(BuildOptions{Overlays: []string{ov}, Strategy: StrategyKustomize, OutputDir: t.TempDir()})
	if len(res) != 1 {
		t.Fatalf("expected one result: %v", res)
	}
	// kustomize may or may not be installed; either outcome is acceptable for smoke test
	_ = res[0].Err
}

func makeApp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, ov := range []string{"dev", "prod"} {
		p := filepath.Join(dir, "overlays", ov)
		_ = os.MkdirAll(p, 0o755)
		_ = os.WriteFile(filepath.Join(p, "kustomization.yaml"), []byte("resources:\n- ../../base\n"), 0o644)
	}
	_ = os.MkdirAll(filepath.Join(dir, "base"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "base", "kustomization.yaml"), []byte("resources: []\n"), 0o644)
	return dir
}
