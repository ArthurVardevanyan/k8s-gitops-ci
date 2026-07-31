package largefile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// DefaultMaxSize is the default maximum file size (1 MiB).
const DefaultMaxSize int64 = 1 * 1024 * 1024

// DefaultIgnorePatterns lists filename glob patterns (matched by Check via
// isAllowed - basename or full-path suffix/glob) exempted from the
// large-file/binary check by default: generic, org-agnostic file types
// that legitimately balloon past DefaultMaxSize - compressed archives, web
// fonts, images/icons, and CustomResourceDefinition manifests (whose
// embedded OpenAPI schemas routinely run into several hundred KiB or more,
// with nothing wrong with the file). An org appends/replaces entries here
// rather than forking Check to get its own allowlist.
var DefaultIgnorePatterns = []string{
	"*.tar.gz", "*.woff", "*.woff2", "*.ttf", "*.eot", "*.png", "*.ico",
	"customresourcedefinition*.yaml",
}

// Violation records a binary or oversized file finding.
type Violation struct {
	File   string
	Reason string
}

func (v Violation) String() string { return fmt.Sprintf("%s: %s", v.File, v.Reason) }

// Check returns large-file violations for changed files.
func Check(changedFiles []string, maxSize int64, allowPatterns []string) []Violation {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	var out []Violation
	for _, f := range changedFiles {
		if isAllowed(f, allowPatterns) {
			continue
		}
		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Size() > maxSize {
			out = append(out, Violation{File: f, Reason: fmt.Sprintf("file too large (%s, max %s)", formatSize(info.Size()), formatSize(maxSize))})
			continue
		}
		if !isKnownText(f) && isBinary(f) {
			out = append(out, Violation{File: f, Reason: "binary file detected"})
		}
	}
	return out
}

func isAllowed(f string, patterns []string) bool {
	base := filepath.Base(f)
	for _, p := range patterns {
		if strings.HasPrefix(p, ".") {
			if strings.HasSuffix(base, p) || strings.HasSuffix(f, p) {
				return true
			}
		}
		if matched, _ := filepath.Match(p, base); matched {
			return true
		}
		if matched, _ := filepath.Match(p, f); matched {
			return true
		}
	}
	return false
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8*1024)
	n, err := f.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return false
	}
	if n == 0 {
		return false
	}
	data := buf[:n]
	if bytesContains(data, 0) {
		return true
	}
	return !utf8.Valid(data)
}

func bytesContains(b []byte, v byte) bool {
	for _, c := range b {
		if c == v {
			return true
		}
	}
	return false
}

func isKnownText(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if knownTextExtensions[ext] {
		return true
	}
	base := strings.ToLower(filepath.Base(path))
	for _, prefix := range knownTextNames {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	for _, name := range []string{".gitignore", ".dockerignore", ".editorconfig"} {
		if base == name {
			return true
		}
	}
	return false
}

var knownTextExtensions = map[string]bool{
	"yaml": true, "yml": true, "json": true, "toml": true, "md": true,
	"txt": true, "csv": true, "html": true, "xml": true, "go": true,
	"py": true, "js": true, "ts": true, "sh": true, "bash": true,
	"zsh": true, "env": true, "cfg": true, "conf": true, "ini": true,
	"properties": true, "sql": true, "graphql": true, "tf": true,
	"hcl": true, "rego": true,
}

var knownTextNames = []string{"makefile", "taskfile", "dockerfile", "containerfile"}

func formatSize(n int64) string {
	const (
		MiB = 1024 * 1024
		KiB = 1024
	)
	switch {
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/MiB)
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/KiB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
