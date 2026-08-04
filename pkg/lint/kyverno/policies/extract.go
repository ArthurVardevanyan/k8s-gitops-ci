// Package policies provides access to the Kyverno policy archive.
//
// The embedded archive (policies.tar.gz) is OPTIONAL and gated behind the
// `embedschemas` build tag, mirroring pkg/lint/kubeconform/schemas. This keeps
// the package importable as a library module without the (gitignored,
// build-time) archive:
//
//   - Built WITHOUT `-tags embedschemas` (default; how downstream consumers
//     import this package): no archive is compiled in. Callers must provide a
//     pre-prepared policy path via kubeconform/validator Options.PolicyPath.
//     Extract returns ErrNoEmbeddedArchive.
//   - Built WITH `-tags embedschemas` (this project's own binary, after
//     scripts/pull-policies.sh): the archive is compiled in and Extract unpacks it.
package policies

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoEmbeddedArchive is returned by Extract when the binary was built without
// the `embedschemas` build tag (so no archive is compiled in). Consumers should
// supply a policy path explicitly (Options.PolicyPath) instead.
var ErrNoEmbeddedArchive = errors.New(
	"no embedded kyverno policy archive: build with -tags embedschemas " +
		"after running scripts/pull-policies.sh, or provide a pre-prepared " +
		"policy path via PolicyPath",
)

// maxExtractedFileSize bounds how much data is written per archive entry,
// as defense-in-depth against decompression-bomb style resource exhaustion
// even though the archive is embedded (trusted, build-time) content.
const maxExtractedFileSize = 512 << 20 // 512MiB

// Extract extracts the embedded policy archive to a temp directory. It returns
// ErrNoEmbeddedArchive when built without the `embedschemas` build tag.
func Extract() (dir string, cleanup func(), err error) {
	data, err := archiveBytes()
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

// EnsureArchive creates a placeholder archive if none exists. It is
// independent of the embedded archive (and the `embedschemas` build tag): it
// only writes a well-formed empty tar.gz so //go:embed directives compiling
// the source tree have a file to embed. The tar and gzip writers must be
// closed in that order (tar first, to flush its end-of-archive padding into
// the gzip stream; gzip second, to flush its own trailer) *before* the
// buffer's bytes are read.
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
