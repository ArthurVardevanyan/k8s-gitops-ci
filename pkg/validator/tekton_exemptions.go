package validator

import (
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// TektonPACDir is the top-level repository directory convention used by
// Tekton Pipelines-as-code for PipelineRun manifests it manages directly:
// PAC's webhook/controller creates, applies, and prunes these resources
// itself in response to Git events, rather than relying on Argo CD's sync
// lifecycle. Because of that, PipelineRun resources found there don't need
// - and can't always satisfy - the sync-options ("SkipDryRunOnMissingResource")
// or namespace ("metadata.namespace" present) requirements the rest of this
// repo's checks enforce for Argo CD-synced resources.
//
// Set to "" (e.g. once at process startup, before running the pipeline) to
// disable this default exemption entirely and have PipelineRun manifests
// under this directory checked like any other resource.
var TektonPACDir = ".tekton"

// builtinExemptSelectors returns the process's built-in exemption
// selectors - currently just the Tekton Pipelines-as-code default above -
// to be merged ahead of any hook-provided EXEMPTIONS selectors.
func builtinExemptSelectors() []exempt.Selector {
	if TektonPACDir == "" {
		return nil
	}
	dir := strings.Trim(TektonPACDir, "/")
	if dir == "" {
		return nil
	}
	return []exempt.Selector{
		{Check: "sync-options", Kind: "PipelineRun", Dir: dir},
		{Check: "namespace", Kind: "PipelineRun", Dir: dir},
	}
}
