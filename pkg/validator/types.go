package validator

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// Options configures the validator orchestration.
//
// Step/check enablement uses one generic ID-based mechanism instead of
// dedicated boolean flags per step: every gateable step (per-doc/per-overlay
// checks registered in pkg/validator/check, as well as standalone steps like
// "golangci" and "kyverno") has a string ID. Steps default to enabled unless
// they're explicitly opted out via DisabledChecks, EXCEPT steps registered as
// default-off (see defaultOffSteps in phases.go, e.g. "kyverno"), which stay
// disabled until their ID is explicitly listed in EnabledChecks. See
// stepEnabled in phases.go.
type Options struct {
	RepoURL        string
	PR             string
	BaseRef        string
	Revision       string
	TriggerComment string
	// HookSource is the raw hook-source signal (e.g. "main"/"pr"/"local",
	// wired straight through from pipeline.Options.HookSource / the CLI's
	// --hook-source flag) that hook.ResolveSource normalizes - fail-closed
	// to the base/target branch's test.sh - before every app's hooks and
	// EXEMPTIONS are resolved. See resolveHookSource in hook_wiring.go.
	HookSource       string
	LintOnly         bool
	Verbose          bool
	IncludeDeletions bool
	AssumeOpenShift  bool     // treat OpenShift/OKD-only API groups as exempt from the sync-options check; see syncopts.AssumeOpenShift
	DisabledChecks   []string // IDs to disable entirely (e.g. "sync-options", "golangci", "avp"); only affects steps that default to enabled
	EnabledChecks    []string // IDs to explicitly enable; only affects steps that default to disabled (e.g. "kyverno")
	Concurrency      int
	Apps, Clusters   []string
	Dirs             []string // explicit subdirectories to validate; bypasses git diff
	IncludePrefixes  []string // restrict the resolved changeset (git diff or PR files) to files under these path prefixes; empty means no restriction
	Providers        provider.Providers
	// Timing allows an external TimingCollector to be passed in (e.g. from
	// pkg/pipeline, which needs to record its own setup/PR-validation phases
	// alongside the validator's). When nil, RunAll constructs its own.
	Timing *TimingCollector
	// SchemaDir, when set, is a pre-extracted kubeconform schema directory
	// (see kubeconform.ExtractSchemas) that the kubeconform lint step reuses
	// instead of extracting its own copy - set by pkg/pipeline's Setup phase,
	// which prefetches schemas once up front (see docs/DEVELOPMENT.md's
	// timing-table section) rather than paying the extraction cost lazily,
	// inside the concurrent Linting phase, on every run. Left empty by
	// callers that don't prefetch (e.g. test-all/build-yaml/scan-all), in
	// which case the kubeconform step falls back to its own lazy extraction
	// exactly as before this field existed.
	SchemaDir string
	// PolicyPath, when set, is a pre-prepared Kyverno policy file/dir path
	// (see kyverno.PreparePolicies) that runKyvernoValidation reuses instead
	// of preparing its own copy - same prefetch rationale as SchemaDir, but
	// only populated when the opt-in "kyverno" step (default off) is
	// actually enabled, since preparing policies shells out to `kustomize
	// build` and shouldn't be paid for runs that never use it.
	PolicyPath string
}

// Result carries per-section findings.
type Result struct {
	Sections   []Section
	Status     string
	Blocking   bool
	Check      check.Result
	ReportBody string
	Logger     *logger.Logger
	Timing     *TimingCollector
}

// Section is a named report section.
type Section struct {
	Name, Body string
	Error      bool
}

// HasErrorSection reports whether any error-section exists.
func (r *Result) HasErrorSection() bool {
	for _, s := range r.Sections {
		if s.Error {
			return true
		}
	}
	return false
}

// FailedSectionCount returns how many Sections have Error set. Used
// alongside len(r.Sections) to feed Logger.Summary(totalSections,
// failedSections int)'s "Sections: N passed, M failed" line - kept as a
// method here (rather than a loop inlined at each call site) since
// pkg/logger can't import pkg/validator to compute this itself (validator
// already imports logger).
func (r *Result) FailedSectionCount() int {
	n := 0
	for _, s := range r.Sections {
		if s.Error {
			n++
		}
	}
	return n
}

// Workers returns concurrency.
func (o *Options) Workers() int {
	// import in engine.go
	return 0
}
