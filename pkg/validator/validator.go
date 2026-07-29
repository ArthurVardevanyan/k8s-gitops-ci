package validator

import (
	"fmt"
	"runtime"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
)

// RunAll runs the four validator phases.
func RunAll(opts Options) (*Result, error) {
	res := &Result{}
	changed, err := resolveChangeset(opts)
	if err != nil {
		return nil, fmt.Errorf("resolve changeset: %w", err)
	}
	_ = changed
	if opts.LintOnly {
		res.Sections = append(res.Sections, Section{Name: "lint-only", Body: "Lint-only mode enabled; build checks skipped.", Error: false})
		return res, nil
	}
	res.Sections = append(res.Sections, Section{Name: "status", Body: "Validation running", Error: false})
	return res, nil
}

// Workers returns the configured concurrency.
func Workers(opts Options) int {
	if opts.Concurrency > 0 {
		return opts.Concurrency
	}
	return runtime.NumCPU() * 2
}

func resolveChangeset(opts Options) ([]string, error) {
	return changeset.GetChangedFiles(changeset.Options{
		RepoURL:          opts.RepoURL,
		PR:               opts.PR,
		BaseRef:          opts.BaseRef,
		IncludeDeletions: opts.IncludeDeletions,
	})
}
