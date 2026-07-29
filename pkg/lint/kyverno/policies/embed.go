package policies

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed policies.tar.gz
var policiesArchive embed.FS

// Extract extracts the embedded policy archive to a temp directory.
func Extract() (dir string, cleanup func(), err error) {
	data, err := policiesArchive.ReadFile("policies.tar.gz")
	if err != nil {
		return "", nil, err
	}
	dir, err = os.MkdirTemp("", "kyverno-policies-*")
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
		if !strings.HasPrefix(path, dir) {
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

// EnsureArchive creates a placeholder archive if none exists.
func EnsureArchive(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	_ = tw.WriteHeader(&tar.Header{Name: "kyverno-policies/", Mode: 0o755, Typeflag: tar.TypeDir})
	gw.Close()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
