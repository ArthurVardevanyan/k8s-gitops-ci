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
// batch (structural checks always; the OVN-Kubernetes-aware semantic tier
// additionally when assumeOpenshift is set - see pkg/validator/nad's package
// doc comment). This mirrors runKyvernoValidation's temp-file + remap
// pattern (kyverno_wiring.go) so a finding's File points at the overlay a
// reviewer can act on instead of an ephemeral temp path.
func runNADValidation(outputs []renderedOverlay, assumeOpenshift bool, log *logger.Logger) Section {
	if len(outputs) == 0 {
		return ComposeNADSection(nil, assumeOpenshift)
	}

	dir, err := os.MkdirTemp("", "nad-resources-*")
	if err != nil {
		log.Warn("nad: creating temp dir: %v", err)
		return ComposeNADSection(nil, assumeOpenshift)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	files := make([]string, 0, len(outputs))
	remap := make(map[string]string, len(outputs))
	for i, o := range outputs {
		f := filepath.Join(dir, fmt.Sprintf("resource-%d.yaml", i))
		if err := os.WriteFile(f, o.data, 0o600); err != nil {
			log.Warn("nad: writing %s: %v", f, err)
			continue
		}
		files = append(files, f)
		remap[f] = o.overlay
	}

	errs := nad.ValidateFiles(files, assumeOpenshift)
	for i := range errs {
		if src, ok := remap[errs[i].File]; ok {
			errs[i].File = src
		}
	}

	if len(errs) > 0 {
		log.ErrorInSection("NAD", "%d NetworkAttachmentDefinition validation error(s)", len(errs))
	}
	return ComposeNADSection(errs, assumeOpenshift)
}
