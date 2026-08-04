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
	Sections   []ReportSection
	Status     string
	Blocking   bool
	Check      check.Result
	ReportBody string
	Logger     *logger.Logger
	Timing     *TimingCollector
}

// HasErrorSection reports whether any error-status section exists. Only
// StatusError counts (matching pre-ReportSection-unification behavior) -
// StatusWarning/StatusInfo sections are "worth a look" but don't count as
// a hard failure here, same distinction FailedSectionCount below makes.
func (r *Result) HasErrorSection() bool {
	for _, s := range r.Sections {
		if s.Status == StatusError {
			return true
		}
	}
	return false
}

// FailedSectionCount returns how many Sections have StatusError. Used
// alongside len(r.Sections) to feed Logger.Summary(totalSections,
// failedSections int)'s "Sections: N passed, M failed" line - kept as a
// method here (rather than a loop inlined at each call site) since
// pkg/logger can't import pkg/validator to compute this itself (validator
// already imports logger).
func (r *Result) FailedSectionCount() int {
	n := 0
	for _, s := range r.Sections {
		if s.Status == StatusError {
			n++
		}
	}
	return n
}

// Failed reports whether this run should be treated as a failure: either
// Blocking is set (Resource Compliance direct findings, or a blocking
// ghost patch - see phases.go), or the run's own Logger recorded any
// Error/ErrorInSection call across any phase (Linting, Static Checks,
// Kustomize Build, ...). This is the single source of truth every CLI
// entry point's exit-code decision should be based on - a check that
// renders ❌ in the report but never calls log.ErrorInSection (a real bug
// found in Kustomize Fix findings: they rendered as StatusError yet never
// set Blocking or logged an error, so "pipeline" returned success despite
// the report showing a hard failure) would otherwise silently never fail
// the run. nil-safe so callers don't need their own nil check first (a
// nil Result - e.g. RunAll itself hard-failed before producing one - is
// never itself a reason to report failure; the caller's own error return
// already covers that case).
func (r *Result) Failed() bool {
	if r == nil {
		return false
	}
	return r.Blocking || (r.Logger != nil && r.Logger.HasFailures())
}

// Workers returns concurrency.
func (o *Options) Workers() int {
	// import in engine.go
	return 0
}
