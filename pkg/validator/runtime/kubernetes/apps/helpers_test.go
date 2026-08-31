package apps

import (
	"slices"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// runKindChecks runs every registered check in this package that applies to
// kind, mirroring what the pipeline does: the adapter selects checks by
// Kinds(), then runs them.
//
// Tests use this instead of a hand-maintained per-kind check list, so a new
// check added to allChecks is exercised automatically and the integration
// tests cannot drift from what is actually registered.
func runKindChecks(data []byte, kind string) []runtime.Finding {
	const source = "test.yaml"

	findings := []runtime.Finding{}
	for _, c := range allChecks() {
		if !slices.Contains(c.Kinds(), kind) {
			continue
		}
		findings = append(findings, c.Run(data, source)...)
	}

	return findings
}
