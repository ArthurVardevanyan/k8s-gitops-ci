package validator

import (
	"fmt"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
)

// RunAll runs the four validator phases.
func RunAll(opts Options) (*Result, error) {
	res := &Result{}

	changed, err := resolveChangeset(opts)
	if err != nil {
		return nil, fmt.Errorf("resolve changeset: %w", err)
	}

	if opts.LintOnly {
		runLintAndStaticChecks(changed, opts, res)
		return res, nil
	}

	runLintAndStaticChecks(changed, opts, res)
	runBuildAndPostBuild(changed, opts, res)

	res.Status = "ok"
	if res.Blocking {
		res.Status = "failed"
	}
	return res, nil
}

func resolveChangeset(opts Options) ([]string, error) {
	if len(opts.Dirs) > 0 {
		return changeset.GetFilesUnderDirs(opts.Dirs)
	}
	return changeset.GetChangedFiles(changeset.Options{
		RepoURL:          opts.RepoURL,
		PR:               opts.PR,
		BaseRef:          opts.BaseRef,
		IncludeDeletions: opts.IncludeDeletions,
	})
}
