package prettier

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrCLINotFound returned when prettier is not available.
var ErrCLINotFound = errors.New("prettier not found in PATH")

// DefaultArgs are passed to prettier.
var DefaultArgs = []string{"--check", "--ignore-unknown"}

// Filter returns files prettier can format (non-binary, known extensions).
func Filter(files []string) []string {
	var out []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		switch ext {
		case ".yaml", ".yml", ".json", ".md", ".markdown", ".ts", ".tsx", ".js", ".jsx", ".html", ".css", ".scss":
			out = append(out, f)
		}
	}
	return out
}

// Run executes prettier --check on files.
func Run(files, extraArgs []string) (string, error) {
	files = Filter(files)
	if len(files) == 0 {
		return "", nil
	}
	if _, err := exec.LookPath("prettier"); err != nil {
		return "", ErrCLINotFound
	}
	args := append([]string{}, DefaultArgs...)
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}
	args = append(args, files...)
	cmd := exec.CommandContext(context.Background(), "prettier", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("prettier check failed: %w", err)
	}
	return string(out), nil
}

// WriteArgs are passed to prettier when actually rewriting files in place
// (as opposed to Run's --check-only mode).
var WriteArgs = []string{"--write", "--ignore-unknown"}

// Write runs prettier --write on files, rewriting them in place. Used by
// pkg/kustomize.Fix as a follow-up formatting pass after `kustomize edit
// fix`: kustomize's own YAML writer doesn't match this repo's prettier
// conventions (e.g. it flattens sequence-item indentation instead of
// indenting list items under their parent key), so without this pass a
// "fixed" kustomization.yaml would immediately be re-flagged as needing a
// fix again - see pkg/kustomize's own doc comments for why CheckFix/Fix's
// consistency depends on this actually running.
func Write(files []string) (string, error) {
	files = Filter(files)
	if len(files) == 0 {
		return "", nil
	}
	if _, err := exec.LookPath("prettier"); err != nil {
		return "", ErrCLINotFound
	}
	args := append([]string{}, WriteArgs...)
	args = append(args, files...)
	cmd := exec.CommandContext(context.Background(), "prettier", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("prettier write failed: %w", err)
	}
	return string(out), nil
}
