// Package kustomize wraps the real `kustomize` CLI for detecting and
// applying `kustomize edit fix` normalization to kustomization.yaml files.
//
// This intentionally shells out to the real binary rather than
// reimplementing its field-ordering/deprecated-field-migration logic in
// Go: that logic (preserve existing top-level field order; only append
// brand-new or migrated fields - e.g. `vars:` -> `replacements:` - in one
// specific order; never touch nested map ordering at all) is intricate,
// undocumented as a stable public contract, and previously reimplemented
// here as a blanket alphabetical key sort that didn't match real
// kustomize behavior at all (it resorted every key, including nested
// ones, and forced apiVersion/kind to the top unconditionally - see
// docs/CI.md's "Kustomize Fix" section and this package's git history).
// Re-deriving it well enough to stay correct as kustomize's own CLI
// evolves isn't worth it when the real binary already does this
// correctly.
package kustomize

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/prettier"
)

// ErrCLINotFound is returned when the real `kustomize` binary isn't
// available in PATH. Unlike every pkg/lint/* wrapper's own ErrCLINotFound
// (which callers treat as "skip, don't fail" via errors.Is - see e.g.
// phases.go's markdownlint/prettier/shellcheck/golangci handling),
// callers here treat this as a hard failure: kustomize is a core,
// expected dependency for a kustomize-based GitOps CI pipeline, not an
// optional/best-effort tool an org may or may not have installed (unlike
// kyverno's org-supplied policies - the other check with a hard CLI
// dependency, and the reason it's opt-in). A run that can't actually
// verify/fix kustomization.yaml files should fail loudly, not silently
// report a clean bill of health it never checked.
var ErrCLINotFound = errors.New("kustomize not found in PATH")

// requireKustomize returns a wrapped ErrCLINotFound when the kustomize
// binary isn't on PATH.
func requireKustomize() error {
	if _, err := exec.LookPath("kustomize"); err != nil {
		return fmt.Errorf("%w: %s", ErrCLINotFound, err.Error())
	}
	return nil
}

// CheckFix reports which kustomization.yaml files (excluding scaffold
// templates - convention.ScaffoldTemplatesPrefix) the real fix pipeline
// (see Fix/runFixPipeline) would actually change, checked against a
// scratch copy of each file so this never mutates the working tree -
// unlike Fix, which applies the change for real. A per-file error (e.g. a
// kustomization.yaml the real kustomize CLI itself rejects) doesn't stop
// the rest from being checked; every such error is joined into the
// returned error so the caller still sees it.
func CheckFix(files []string) ([]string, error) {
	if err := requireKustomize(); err != nil {
		return nil, err
	}
	var need []string
	var errs []error
	for _, f := range files {
		if !strings.HasSuffix(f, "kustomization.yaml") {
			continue
		}
		if strings.Contains(f, convention.ScaffoldTemplatesPrefix()) {
			continue
		}
		changed, err := needsFix(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f, err))
			continue
		}
		if changed {
			need = append(need, f)
		}
	}
	return need, errors.Join(errs...)
}

// needsFix reports whether Fix would change path, without mutating path
// itself: the file is copied into a scratch temp directory, run through
// the real fix pipeline there, and compared byte-for-byte against the
// original.
func needsFix(path string) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	tmpDir, err := os.MkdirTemp("", "kustomize-fix-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpFile := filepath.Join(tmpDir, "kustomization.yaml")
	if err := os.WriteFile(tmpFile, original, 0o644); err != nil {
		return false, err
	}

	if err := runFixPipeline(tmpDir, tmpFile); err != nil {
		return false, err
	}

	fixed, err := os.ReadFile(tmpFile)
	if err != nil {
		return false, err
	}
	return !bytes.Equal(original, fixed), nil
}

// Fix runs the real fix pipeline on every kustomization.yaml file found
// under root, recursively (via filepath.WalkDir - so a root like
// "okd/okd-configuration/" finds nested overlays like
// "okd/okd-configuration/overlays/sandbox/kustomization.yaml" too).
// Returns the list of files actually changed (a file the pipeline leaves
// byte-for-byte identical is excluded from the result, even though it
// still ran against it). Files under a scaffold template directory
// (convention.ScaffoldTemplatesPrefix) are skipped, matching CheckFix's
// own exclusion - a template is deliberately not real, on-disk-app
// content.
//
// A per-file error aborts the walk immediately (via filepath.WalkDir's
// own early-return-on-error behavior) rather than silently skipping the
// offending file and continuing - unlike CheckFix (read-only, so
// collecting every error and still reporting the rest is safe), Fix
// mutates the working tree, so a real fix failure partway through should
// stop and surface loudly rather than leave a partially-fixed tree with
// no indication anything went wrong.
func Fix(root string) ([]string, error) {
	if err := requireKustomize(); err != nil {
		return nil, err
	}

	var fixed []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "kustomization.yaml" {
			return nil
		}
		if strings.Contains(path, convention.ScaffoldTemplatesPrefix()) {
			return nil
		}
		before, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := runFixPipeline(filepath.Dir(path), path); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !bytes.Equal(before, after) {
			fixed = append(fixed, path)
		}
		return nil
	})
	if err != nil {
		return fixed, err
	}
	return fixed, nil
}

// runFixPipeline runs `kustomize edit fix --vars` with dir as its working
// directory (kustomize edit fix takes no path argument - it always
// operates on the kustomization.yaml in the current directory), then
// `prettier --write` on file.
//
// --vars is always passed so a deprecated `vars:` block is converted to
// `replacements:` too, not just field/format normalization - kustomize's
// own `--vars` help text recommends only doing this in a clean git
// repository, since it's a bigger, semantic transformation rather than
// pure formatting; that's on the operator running `kustomize-fix`
// locally to heed, not something this wrapper can safely enforce itself.
//
// The prettier pass is required, not optional: kustomize's own YAML
// writer doesn't match this repo's prettier conventions (most visibly, it
// flattens sequence-item indentation instead of indenting list items
// under their parent key), so without it a freshly "fixed" file would
// immediately be re-flagged as needing a fix again - CheckFix/Fix would
// never agree with each other, and `kustomize-fix` would never converge.
// A missing prettier binary is therefore also a hard failure here, unlike
// prettier's own Run's graceful ErrCLINotFound-becomes-a-skip handling
// elsewhere (e.g. the Linting phase's prettier --check step) - this
// pipeline's correctness depends on it actually running.
func runFixPipeline(dir, file string) error {
	cmd := exec.CommandContext(context.Background(), "kustomize", "edit", "fix", "--vars")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("kustomize edit fix --vars: %s", msg)
		}
		return fmt.Errorf("kustomize edit fix --vars: %w", err)
	}

	if _, err := prettier.Write([]string{file}); err != nil {
		return fmt.Errorf("prettier --write: %w", err)
	}
	return nil
}

// FormatFixNeeded renders a human-readable message for kustomization files needing fix.
func FormatFixNeeded(files []string) string {
	if len(files) == 0 {
		return ""
	}
	apps := AppsFromFiles(files)
	var b strings.Builder
	b.WriteString("The following kustomization.yaml files need `kustomize edit fix --vars`:\n")
	for _, a := range apps {
		b.WriteString("  - ")
		b.WriteString(a)
		b.WriteByte('\n')
	}
	return b.String()
}

// AppsFromFiles maps kustomization paths to app names.
func AppsFromFiles(files []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		if len(parts) >= 2 {
			app := parts[len(parts)-2]
			if !seen[app] {
				seen[app] = true
				out = append(out, app)
			}
		}
	}
	return out
}
