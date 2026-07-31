package policies

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureArchive_WritesWellFormedPlaceholderWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "placeholder.tar.gz")

	if err := EnsureArchive(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening placeholder archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("placeholder archive is not valid gzip (regression: gzip writer closed before tar writer flushed its data): %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	h, err := tr.Next()
	if err != nil {
		t.Fatalf("placeholder archive has no tar entries (regression: close-ordering truncated the archive): %v", err)
	}
	if h.Name != "kyverno-policies/" || h.Typeflag != tar.TypeDir {
		t.Errorf("unexpected placeholder entry: %+v", h)
	}
}

func TestEnsureArchive_NoOpWhenArchiveAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "placeholder.tar.gz")
	if err := os.WriteFile(path, []byte("existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureArchive(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing content" {
		t.Errorf("expected existing archive left untouched, got: %s", data)
	}
}
