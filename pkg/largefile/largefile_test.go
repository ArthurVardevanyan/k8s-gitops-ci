package largefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck_NormalFiles(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "small.yaml")
	_ = os.WriteFile(f, []byte("hello"), 0o644)
	v := Check([]string{f}, 0, nil)
	if len(v) != 0 {
		t.Fatalf("unexpected violations: %v", v)
	}
}

func TestCheck_LargeFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "big.bin")
	_ = os.WriteFile(f, make([]byte, DefaultMaxSize+1), 0o644)
	v := Check([]string{f}, 0, nil)
	if len(v) != 1 || !strings.Contains(v[0].Reason, "file too large") {
		t.Fatalf("expected large violation: %v", v)
	}
}

func TestCheck_BinaryFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "data.bin")
	_ = os.WriteFile(f, []byte{0, 1, 2, 3}, 0o644)
	v := Check([]string{f}, 0, nil)
	if len(v) != 1 || !strings.Contains(v[0].Reason, "binary") {
		t.Fatalf("expected binary violation: %v", v)
	}
}

func TestCheck_AllowPatterns(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".keep")
	_ = os.WriteFile(f, []byte{0, 1}, 0o644)
	v := Check([]string{f}, 0, []string{".keep"})
	if len(v) != 0 {
		t.Fatalf("allow pattern should skip: %v", v)
	}
}

func TestCheck_DefaultIgnorePatterns_LargeCRDAllowed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "customresourcedefinition-widgets.yaml")
	_ = os.WriteFile(f, make([]byte, DefaultMaxSize+1), 0o644)
	v := Check([]string{f}, 0, DefaultIgnorePatterns)
	if len(v) != 0 {
		t.Fatalf("expected a large CRD manifest to be ignored by default, got: %v", v)
	}
}

func TestCheck_DefaultIgnorePatterns_LargeCRDAliasedAllowed(t *testing.T) {
	dir := t.TempDir()
	tests := []string{
		"crd.yaml",
		"crd.yml",
		"crds.yaml",
		"crds.yml",
		"crd-someapp.yaml",
		"crd-someapp.yml",
		"crd-controllers.yaml",
	}
	for _, name := range tests {
		f := filepath.Join(dir, name)
		_ = os.WriteFile(f, make([]byte, DefaultMaxSize+1), 0o644)
		v := Check([]string{f}, 0, DefaultIgnorePatterns)
		if len(v) != 0 {
			t.Fatalf("expected large %s to be ignored by default, got: %v", name, v)
		}
	}
}

func TestCheck_DefaultIgnorePatterns_LargeArchiveAllowed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "schemas.tar.gz")
	// Binary-looking content too, so this also proves the archive glob
	// suppresses both the size *and* binary-detection violation paths.
	_ = os.WriteFile(f, append(make([]byte, DefaultMaxSize+1), 0, 1, 2), 0o644)
	v := Check([]string{f}, 0, DefaultIgnorePatterns)
	if len(v) != 0 {
		t.Fatalf("expected a large tar.gz archive to be ignored by default, got: %v", v)
	}
}

func TestCheck_DefaultIgnorePatterns_UnrelatedLargeFileStillFlagged(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "not-a-crd.yaml")
	_ = os.WriteFile(f, make([]byte, DefaultMaxSize+1), 0o644)
	v := Check([]string{f}, 0, DefaultIgnorePatterns)
	if len(v) != 1 {
		t.Fatalf("expected an unrelated large file to still be flagged, got: %v", v)
	}
}

func TestCheck_DeletedFile(t *testing.T) {
	v := Check([]string{"/tmp/does-not-exist-lkj"}, 0, nil)
	if len(v) != 0 {
		t.Fatalf("expected none for missing file: %v", v)
	}
}

func TestCheck_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty.yaml")
	_ = os.WriteFile(f, []byte{}, 0o644)
	v := Check([]string{f}, 0, nil)
	if len(v) != 0 {
		t.Fatalf("expected none for empty: %v", v)
	}
}

func TestKnownTextWithNonUTF8(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "text.yaml")
	_ = os.WriteFile(f, []byte{0xff, 0xfe}, 0o644)
	if isKnownText(f) && len(Check([]string{f}, 0, nil)) != 0 {
		t.Error("known-text extension with non-utf8 should not be flagged")
	}
}

func TestIsBinary(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bin")
	_ = os.WriteFile(f, []byte{0x00, 0x01}, 0o644)
	if !isBinary(f) {
		t.Error("expected binary")
	}
}

func TestIsKnownText(t *testing.T) {
	if !isKnownText("/foo/bar.yaml") {
		t.Error("yaml should be known text")
	}
	if isKnownText("/foo/bar.jpg") {
		t.Error("jpg should not be known text")
	}
}
