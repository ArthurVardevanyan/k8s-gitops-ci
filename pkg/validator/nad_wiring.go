package validator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/nad"
)

// runNADValidation writes every successfully-rendered overlay's YAML to a
// temp directory and runs NetworkAttachmentDefinition validation against the
// batch. That validation is advisory-only - the rules that block, including
// whether spec.config parses at all, are runtime checks in
// pkg/validator/runtime/k8scni - see pkg/validator/nad's package doc
// comment. This mirrors
// runKyvernoValidation's temp-file + remap pattern (kyverno_wiring.go) so a
// finding's File points at the overlay a reviewer can act on instead of an
// ephemeral temp path.
//
// Nothing here gates the run: the advisories are surfaced in the section (⚠️)
// only. The errs return is kept because reading a file can still fail, which
// is a failure of this wiring rather than a judgement about a NAD.
//
// The returned bool reports whether any NAD resource was actually present in
// the rendered-overlay chain. When it's false the caller omits the section
// entirely rather than rendering a "0 NADs, all good" stub: NAD validation is
// still always-on (never gated), but an empty section is pure noise on the
// (common) PRs that touch no NetworkAttachmentDefinition at all.
func runNADValidation(outputs []renderedOverlay, log *logger.Logger) (ReportSection, bool) {
	if len(outputs) == 0 {
		return ReportSection{}, false
	}

	dir, err := os.MkdirTemp("", "nad-resources-*")
	if err != nil {
		log.Warn("nad: creating temp dir: %v", err)
		return ReportSection{}, false
	}
	defer func() { _ = os.RemoveAll(dir) }()

	present := false
	files := make([]string, 0, len(outputs))
	srcOf := make(map[string]string, len(outputs))
	for i, o := range outputs {
		if nad.ContainsNAD(o.data) {
			present = true
		}
		f := filepath.Join(dir, fmt.Sprintf("resource-%d.yaml", i))
		if err := os.WriteFile(f, o.data, 0o600); err != nil {
			log.Warn("nad: writing %s: %v", f, err)
			continue
		}
		files = append(files, f)
		srcOf[f] = o.overlay
	}

	if !present {
		return ReportSection{}, false
	}

	errs, warns := nad.ValidateFiles(files)
	remap := func(fs []nad.ValidationError) {
		for i := range fs {
			if src, ok := srcOf[fs[i].File]; ok {
				fs[i].File = src
			}
		}
	}
	remap(errs)
	remap(warns)

	// errs no longer carries judgements about a NAD's contents - those are the
	// runtime checks' - so in practice it holds only read/decode failures,
	// which still have to gate rather than pass silently. Advisory warnings
	// are surfaced in the section (⚠️) but must not fail the pipeline
	// (Result.Failed keys off logged errors, not section status - see
	// validator.Result.Failed).
	if len(errs) > 0 {
		log.ErrorInSection("NAD", "%d NetworkAttachmentDefinition validation error(s)", len(errs))
	}
	return ComposeNADSection(errs, warns), true
}
