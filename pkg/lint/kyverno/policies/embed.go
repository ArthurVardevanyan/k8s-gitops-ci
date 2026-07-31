package policies

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed policies.tar.gz
var policiesArchive embed.FS

// maxExtractedFileSize bounds how much data is written per archive entry,
// as defense-in-depth against decompression-bomb style resource exhaustion
// even though the archive is embedded (trusted, build-time) content.
const maxExtractedFileSize = 512 << 20 // 512MiB

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
		if errors.Is(err, io.EOF) {
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
		n, err := io.Copy(f, io.LimitReader(tr, maxExtractedFileSize+1))
		if err != nil {
			f.Close()
			cleanup()
			return "", nil, err
		}
		if n > maxExtractedFileSize {
			f.Close()
			cleanup()
			return "", nil, fmt.Errorf("archive entry %s exceeds max extracted size", h.Name)
		}
		_ = f.Close()
	}
	return dir, cleanup, nil
}

// EnsureArchive creates a placeholder archive if none exists. The tar and
// gzip writers must be closed in that order (tar first, to flush its
// end-of-archive padding into the gzip stream; gzip second, to flush its
// own trailer) *before* the buffer's bytes are read - closing gzip first
// (or relying on deferred closes that run after the buffer is already read)
// silently produces a truncated/empty-looking archive, since gzip.Close
// finalizes the compressed stream and any tar data written afterward never
// reaches the buffer.
func EnsureArchive(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "kyverno-policies/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		return fmt.Errorf("writing placeholder archive header: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing placeholder tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing placeholder gzip writer: %w", err)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
