package validator

import (
	"fmt"
	"runtime"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/syncopts"
)

// RunAll runs the four validator phases.
func RunAll(opts Options) (*Result, error) {
	log := logger.NewLogger(opts.Verbose, "")
	tc := opts.Timing
	if tc == nil {
		tc = NewTimingCollector()
	}
	tc.SetConcurrency(runtime.NumCPU(), Workers(opts))
	res := &Result{Logger: log, Timing: tc}

	syncopts.AssumeOpenShift = opts.AssumeOpenShift
	configureClusterIdentityFromProviders(opts)

	changed, err := resolveChangeset(opts)
	if err != nil {
		return nil, fmt.Errorf("resolve changeset: %w", err)
	}
	log.Info("files to validate: %d", len(changed))

	if opts.LintOnly {
		runLintAndStaticChecks(changed, opts, res, log, tc)
		return res, nil
	}

	runLintAndStaticChecks(changed, opts, res, log, tc)
	runBuildAndPostBuild(changed, opts, res, log, tc)

	res.Status = "ok"
	if res.Blocking {
		res.Status = "failed"
	}
	return res, nil
}

func resolveChangeset(opts Options) ([]string, error) {
	var files []string
	var err error
	if len(opts.Dirs) > 0 {
		files, err = changeset.GetFilesUnderDirs(opts.Dirs)
	} else {
		files, err = changeset.GetChangedFiles(changeset.Options{
			RepoURL:          opts.RepoURL,
			PR:               opts.PR,
			BaseRef:          opts.BaseRef,
			IncludeDeletions: opts.IncludeDeletions,
		})
	}
	if err != nil {
		return nil, err
	}
	return changeset.FilterByPrefixes(files, opts.IncludePrefixes), nil
}
