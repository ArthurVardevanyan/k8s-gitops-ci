package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kyverno"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

// renderedOverlay pairs a successfully-rendered overlay's YAML with the
// overlay path it came from, so a violation's temp resource file (see
// runKyvernoValidation) can be remapped back to something a reviewer can
// actually open, instead of the resource's bare Kind
// (pkg/lint/kyverno.Violation.File's previous, less useful value).
type renderedOverlay struct {
	overlay string
	data    []byte
}

// isKustomizationFile reports whether path is a kustomization root file
// (kustomization.yaml/.yml/Kustomization) rather than an actual resource
// manifest - these aren't real Kubernetes resources and would only add
// parse noise to a Kyverno run, so runKyvernoValidation excludes them from
// its raw-source-file input.
func isKustomizationFile(path string) bool {
	switch filepath.Base(path) {
	case "kustomization.yaml", "kustomization.yml", "Kustomization":
		return true
	default:
		return false
	}
}

// runKyvernoValidation runs the kyverno CLI, in one invocation, against
// both every successfully-rendered overlay's YAML (written to temp files -
// see buildOverlayWithHooks's rendered return value, collected during this
// phase's overlay build loop) and every raw changed YAML source file
// (sourceFiles, validated directly at their real repo-relative path - no
// temp write/remap needed since kyverno already reports their real path).
// The raw-source pass exists because a brand new component not yet
// referenced by any overlay's kustomization.yaml never appears in ANY
// rendered overlay output, so relying on rendered output alone would let
// policy violations in it go completely unnoticed until it's actually
// wired up; some overlap with the rendered-overlay pass is expected and
// harmless (findings are deduplicated below, and Kyverno is always a
// non-blocking advisory regardless of which pass a finding came from).
//
// This is always a non-blocking advisory (see kyverno.FormatComment's own
// doc comment) - a missing CLI, missing/unpreparable policies, or any
// per-file write failure all degrade to an empty "Kyverno Policies"
// section rather than failing the run; Kyverno support is opt-in
// (stepKyverno defaults off - see docs/CI.md#registered-checks) and
// best-effort once enabled.
func runKyvernoValidation(outputs []renderedOverlay, sourceFiles []string, log *logger.Logger) Section {
	if len(outputs) == 0 && len(sourceFiles) == 0 {
		return ComposeKyvernoSection("")
	}

	policyPath, cleanup, err := kyverno.PreparePolicies()
	if err != nil {
		log.Warn("kyverno: preparing policies: %v", err)
		return ComposeKyvernoSection("")
	}
	defer cleanup()

	files := make([]string, 0, len(outputs)+len(sourceFiles))
	// remap maps each temp resource file back to the overlay it was
	// rendered from, so Violation.File (kyverno.findResourceFile's
	// best-effort match against these temp filenames) reports the overlay
	// a reviewer can act on rather than an ephemeral temp path. Raw source
	// files need no remap entry - they're validated at their real path, so
	// Violation.File already reports something a reviewer can open.
	remap := make(map[string]string, len(outputs))

	if len(outputs) > 0 {
		dir, err := os.MkdirTemp("", "kyverno-resources-*")
		if err != nil {
			log.Warn("kyverno: creating temp dir: %v", err)
			return ComposeKyvernoSection("")
		}
		defer func() { _ = os.RemoveAll(dir) }()

		for i, o := range outputs {
			f := filepath.Join(dir, fmt.Sprintf("resource-%d.yaml", i))
			if err := os.WriteFile(f, o.data, 0o600); err != nil {
				log.Warn("kyverno: writing %s: %v", f, err)
				continue
			}
			files = append(files, f)
			remap[f] = o.overlay
		}
	}

	for _, f := range sourceFiles {
		if isKustomizationFile(f) {
			continue
		}
		files = append(files, f)
	}

	result, err := kyverno.ValidateFiles(files, policyPath)
	if err != nil {
		if errors.Is(err, kyverno.ErrCLINotFound) {
			log.Info("kyverno: skipped (%v)", err)
		} else {
			log.Warn("kyverno: %v", err)
		}
		return ComposeKyvernoSection("")
	}

	for i := range result.Violations {
		if src, ok := remap[result.Violations[i].File]; ok {
			result.Violations[i].File = src
		}
	}

	if len(result.Violations) == 0 {
		log.Info("kyverno: %s", result.Summary())
		return ComposeKyvernoSection("")
	}

	deduped := kyverno.Deduplicate(result.Violations, 3)
	log.Warn("kyverno: %d violation(s) across %d policy rule(s) (%s)", len(result.Violations), len(deduped), result.Summary())
	return ComposeKyvernoSection(kyverno.FormatComment(deduped))
}
