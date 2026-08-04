// Package schemas provides access to the kubeconform JSON schema archive.
//
// The embedded archive (schemas.tar.gz) is OPTIONAL and gated behind the
// `embedschemas` build tag. This keeps the package importable as a library
// module without requiring the (gitignored, build-time) archive to be present:
//
//   - Built WITHOUT `-tags embedschemas` (the default, and how downstream
//     consumers import this package): no archive is compiled in. Callers must
//     provide a pre-extracted schema directory via kubeconform.Options.SchemaDir
//     (see the package docs / docs/UPSTREAM.md). Extract returns ErrNoEmbeddedArchive.
//   - Built WITH `-tags embedschemas` (this project's own binary, after
//     scripts/pull-schemas.sh has produced schemas.tar.gz): the archive is
//     compiled in and Extract unpacks it to a temp directory.
package schemas

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrNoEmbeddedArchive is returned by Extract/EnsureArchive when the binary was
// built without the `embedschemas` build tag (so no archive is compiled in) and
// therefore cannot self-provision schemas. Consumers should supply a schema
// directory explicitly (kubeconform.Options.SchemaDir) instead.
var ErrNoEmbeddedArchive = errors.New(
	"no embedded kubeconform schema archive: build with -tags embedschemas " +
		"after running scripts/pull-schemas.sh, or provide a pre-extracted " +
		"schema directory via SchemaDir",
)

// maxExtractedFileSize bounds how much data is written per archive entry,
// as defense-in-depth against decompression-bomb style resource exhaustion
// even though the archive is embedded (trusted, build-time) content.
const maxExtractedFileSize = 512 << 20 // 512MiB

// Extract extracts the embedded schema archive to a temp directory. It returns
// ErrNoEmbeddedArchive when built without the `embedschemas` build tag.
func Extract() (dir string, cleanup func(), err error) {
	data, err := archiveBytes()
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

// EnsureArchive writes the embedded archive to path if it does not exist. It
// returns ErrNoEmbeddedArchive when built without the `embedschemas` build tag.
func EnsureArchive(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	data, err := archiveBytes()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, data, 0o644)
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
