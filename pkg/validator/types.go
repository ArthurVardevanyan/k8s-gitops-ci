package validator

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// Options configures the validator orchestration.
type Options struct {
	RepoURL          string
	PR               string
	BaseRef          string
	Revision         string
	TriggerComment   string
	LintOnly         bool
	SkipAVP          bool
	SkipGolangci     bool
	NoComment        bool
	IncludeDeletions bool
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
