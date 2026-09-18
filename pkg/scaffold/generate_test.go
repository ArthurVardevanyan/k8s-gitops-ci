package scaffold

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestDiffOverlay(t *testing.T) {
	cases := []struct {
		name       string
		base, head *GenerateResult
		overlay    string
		wantDiffer bool
		wantFiles  []string
	}{
		{"both missing", nil, nil, "dev", false, nil},
		{
			"equal", &GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h1"}}},
			&GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h1"}}},
			"dev", false, nil,
		},
		{
			"differing hash", &GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h1"}}},
			&GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h2"}}},
			"dev", true,
			[]string{"a.yaml"},
		},
		{
			"file only in head", &GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h1"}}},
			&GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h1", "b.yaml": "h2"}}},
			"dev", true,
			[]string{"b.yaml"},
		},
		{
			"overlay only at base", &GenerateResult{Overlays: map[string]map[string]string{"dev": {"a.yaml": "h1"}}},
			&GenerateResult{Overlays: map[string]map[string]string{}},
			"dev", true,
			[]string{"a.yaml"},
		},
		{
			"empty overlay on both sides", &GenerateResult{Overlays: map[string]map[string]string{"dev": {}}},
			&GenerateResult{Overlays: map[string]map[string]string{"dev": {}}},
			"dev", false, nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			differ, files := DiffOverlay(c.base, c.head, c.overlay)
			if differ != c.wantDiffer {
				t.Errorf("differ = %v, want %v", differ, c.wantDiffer)
			}
			if len(files) != len(c.wantFiles) {
				t.Fatalf("files = %v, want %v", files, c.wantFiles)
			}
			for i := range files {
				if files[i] != c.wantFiles[i] {
					t.Errorf("files = %v, want %v", files, c.wantFiles)
					break
				}
			}
		})
	}
}

func TestHashOverlayTrees(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "dev", "kustomization.yaml"), "one\n")
	mustWrite(t, filepath.Join(root, "dev", "sub", "patch.yaml"), "two\n")
	mustWrite(t, filepath.Join(root, "prod", "kustomization.yaml"), "three\n")
	// A stray regular file at the root is not an overlay directory.
	mustWrite(t, filepath.Join(root, "notes.txt"), "ignore me\n")

	res, err := hashOverlayTrees(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Overlays) != 2 {
		t.Fatalf("expected 2 overlays, got %v", res.Overlays)
	}
	if got := res.Overlays["dev"]["kustomization.yaml"]; got != sha("one\n") {
		t.Errorf("dev/kustomization.yaml hash = %q", got)
	}
	if got := res.Overlays["dev"]["sub/patch.yaml"]; got != sha("two\n") {
		t.Errorf("dev/sub/patch.yaml hash = %q", got)
	}
	if got := res.Overlays["prod"]["kustomization.yaml"]; got != sha("three\n") {
		t.Errorf("prod/kustomization.yaml hash = %q", got)
	}
}

// TestGenerate_DryRunParseInPlace exercises the real write-mode path: the
// fake tool writes new content into the worktree's overlay directory and
// Generate hashes the result.
func TestGenerate_DryRunParseInPlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell tool not supported on windows")
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "myapp", "overlays", "dev", "kustomization.yaml"), "committed\n")

	bin := filepath.Join(t.TempDir(), "faketool")
	script := "#!/bin/sh\nprintf 'generated\\n' > myapp/overlays/dev/kustomization.yaml\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // test-only fake tool
		t.Fatal(err)
	}

	origBin, origMode, origArgs := Binary, DriftMode, GenerateArgs
	Binary = bin
	DriftMode = DryRunParse
	GenerateArgs = func(string) []string { return []string{"scaffold"} }
	t.Cleanup(func() { Binary, DriftMode, GenerateArgs = origBin, origMode, origArgs })

	res, err := Generate(GenerateOptions{App: "myapp", WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Overlays["dev"]["kustomization.yaml"]; got != sha("generated\n") {
		t.Errorf("generated hash = %q, want the tool-written content's hash", got)
	}
}

func TestGenerate_DryRunParseRequiresGenerateArgs(t *testing.T) {
	origMode, origArgs := DriftMode, GenerateArgs
	DriftMode = DryRunParse
	GenerateArgs = nil
	t.Cleanup(func() { DriftMode, GenerateArgs = origMode, origArgs })

	if _, err := Generate(GenerateOptions{App: "myapp", WorkDir: t.TempDir()}); err == nil {
		t.Error("expected an error when GenerateArgs is unset in DryRunParse mode")
	}
}
