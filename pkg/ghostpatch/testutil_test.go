package ghostpatch

import (
	"os"
	"path/filepath"
	"testing"
)

func makeOverlay(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(content), 0o644)
	return dir
}
