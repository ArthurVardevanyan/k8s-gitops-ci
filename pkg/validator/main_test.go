package validator

import (
	"os"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
)

// TestMain extracts the embedded kubeconform schema archive exactly once for
// this entire test binary, instead of once per RunAll call.
//
// Every RunAll invocation that reaches the kubeconform phase without an
// Options.SchemaDir already set calls kubeconform.ExtractSchemas()
// (phases.go) - and this package alone has ~30 such tests. Left alone, that
// unpacks the embedded archive (108MB across 3105 files, per
// scripts/pull-schemas.sh) dozens of times per test run: real CI timing
// showed 19 separate extractions totaling 227s of kubeconform time,
// dwarfing everything else in the package's test suite. Overriding the
// exported kubeconform.ExtractSchemas seam (the same seam
// kubeconform.TestExtractSchemas_IsOverridable guards, and the one an
// org/consumer layer would replace to pull schemas from elsewhere) to
// return one already-extracted, shared, read-only directory turns 19+
// extractions into 1.
//
// This doesn't reduce coverage of the real extraction path: that's
// exercised directly (and independently of this package) by
// pkg/lint/kubeconform's own TestExtractSchemas_ContainsExpectedSubdirs
// (embedschemas-tagged) and TestExtractSchemas_IsOverridable.
func TestMain(m *testing.M) {
	orig := kubeconform.ExtractSchemas
	dir, cleanup, err := orig()
	if err == nil {
		// Built with -tags embedschemas (as both `task test` and
		// `task test:cover` always do): share the one extraction.
		kubeconform.ExtractSchemas = func() (string, func(), error) {
			return dir, func() {}, nil
		}
	}
	// err != nil (e.g. a plain `go test ./...` without -tags
	// embedschemas) means every RunAll call's own ExtractSchemas() call
	// fails the same fast way it always did - nothing real to share.

	code := m.Run()

	kubeconform.ExtractSchemas = orig
	if cleanup != nil {
		cleanup()
	}
	os.Exit(code)
}
