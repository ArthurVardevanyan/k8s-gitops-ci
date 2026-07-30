package validator

import (
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
	RepoURL          string
	PR               string
	BaseRef          string
	Revision         string
	TriggerComment   string
	LintOnly         bool
	NoComment        bool
	IncludeDeletions bool
	AssumeOpenShift  bool     // treat OpenShift/OKD-only API groups as exempt from the sync-options check; see syncopts.AssumeOpenShift
	DisabledChecks   []string // IDs to disable entirely (e.g. "sync-options", "golangci", "avp"); only affects steps that default to enabled
	EnabledChecks    []string // IDs to explicitly enable; only affects steps that default to disabled (e.g. "kyverno")
	Concurrency      int
	Apps, Clusters   []string
	Dirs             []string // explicit subdirectories to validate; bypasses git diff
	IncludePrefixes  []string // restrict the resolved changeset (git diff or PR files) to files under these path prefixes; empty means no restriction
	Providers        provider.Providers
}

// Result carries per-section findings.
type Result struct {
	Sections   []Section
	Status     string
	Blocking   bool
	Check      check.Result
	ReportBody string
}

// Section is a named report section.
type Section struct {
	Name, Body string
	Error      bool
}

// ErrorInSection reports whether any error-section exists.
func (r *Result) ErrorInSection() bool {
	for _, s := range r.Sections {
		if s.Error {
			return true
		}
	}
	return false
}

// Workers returns concurrency.
func (o *Options) Workers() int {
	// import in engine.go
	return 0
}
