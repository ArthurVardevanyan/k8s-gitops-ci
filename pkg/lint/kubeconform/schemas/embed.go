package schemas

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed schemas.tar.gz
var schemasArchive embed.FS

// Extract extracts the embedded schema archive to a temp directory.
func Extract() (dir string, cleanup func(), err error) {
	data, err := schemasArchive.ReadFile("schemas.tar.gz")
	if err != nil {
		return "", nil, err
	}
	dir, err = os.MkdirTemp("", "kubeconform-schemas-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return "", nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		path := filepath.Join(dir, filepath.Clean(h.Name))
		if !stringsHasPrefix(path, dir) {
			cleanup()
			return "", nil, fmt.Errorf("path traversal: %s", h.Name)
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		f, err := os.Create(path)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			cleanup()
			return "", nil, err
		}
		_ = f.Close()
	}
	return dir, cleanup, nil
}

// EnsureArchive writes the embedded archive to path if it does not exist.
func EnsureArchive(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	data, err := schemasArchive.ReadFile("schemas.tar.gz")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, data, 0o644)
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
