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

	// Apply the org-level OpenShift default when the invocation didn't set
	// it (see DefaultAssumeOpenShift).
	opts.AssumeOpenShift = opts.AssumeOpenShift || DefaultAssumeOpenShift
	syncopts.AssumeOpenShift = opts.AssumeOpenShift
	configureClusterIdentityFromProviders(opts)

	// Thread pre-validation results/errors (from the pipeline layer) into
	// the unified report. When a PRValidationResult is supplied, prepend a
	// PR Checks section built from it; PreErrors are surfaced as a blocking
	// signal so the run fails even if every in-validator phase passed.
	if opts.PRValidation != nil {
		res.Sections = append(res.Sections, composePRChecksSectionFromResult(opts.PRValidation))
	}
	if len(opts.PreErrors) > 0 {
		for _, e := range opts.PreErrors {
			log.ErrorInSection("PreValidation", "%s", e)
		}
		res.Blocking = true
	}

	changed, err := resolveChangeset(opts)
	if err != nil {
		return nil, fmt.Errorf("resolve changeset: %w", err)
	}
	log.Info("files to validate: %d", len(changed))

	// Resolve test.sh files from non-app directories (e.g. okd/node-config/)
	// before either phase runs, so their EXEMPTIONS=(...) selectors are
	// available to the kubeconform lint step in runLintAndStaticChecks
	// and the resource-compliance checks in runBuildAndPostBuild.
	// These directories have no kustomize overlay structure and are therefore
	// never detected as apps by resolveAppHookConfigs; this early pass
	// covers them without requiring a full kustomize layout.
	hookSource := resolveHookSource(opts)
	nonAppCfgs := resolveNonAppHookConfigs(changed, hookSource)
	defer cleanupNonAppHookConfigs(nonAppCfgs)
	earlySelectors := nonAppExemptSelectors(nonAppCfgs)

	if opts.LintOnly {
		runLintAndStaticChecks(changed, opts, res, log, tc, earlySelectors)
		return res, nil
	}

	runLintAndStaticChecks(changed, opts, res, log, tc, earlySelectors)
	runBuildAndPostBuild(changed, opts, res, log, tc, earlySelectors)

	res.Status = "ok"
	if res.Blocking {
		res.Status = "failed"
	}
	return res, nil
}

func resolveChangeset(opts Options) ([]string, error) {
	var files []string
	var err error
	switch {
	case len(opts.Apps) > 0 || len(opts.Clusters) > 0:
		// Targeted ad-hoc validation (build-yaml --app/--cluster) takes
		// priority over Dirs/diff-based resolution - see
		// resolveTargetOverlays. Dirs is intentionally ignored here (it
		// neither scopes nor filters targeted overlays).
		files, err = resolveTargetOverlays(opts)
		return files, err
	case opts.FullScan:
		// Full-scan mode validates every file on disk, ignoring git
		// state entirely. Takes priority over Dirs (the user explicitly
		// asked for everything). Respects ExtraNonAppDirs and scaffold
		// template exclusions just like detectAppRoots does for overlay
		// discovery.
		files, err = getAllRepoFiles()
	case len(opts.Dirs) > 0:
		files, err = changeset.GetFilesUnderDirs(opts.Dirs)
	default:
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
	// When FullScan is set, Dirs is ignored (FullScan takes priority
	// as confirmed by the user) — the user explicitly asked for
	// everything, not a subset. Dirs is only applied as a filter for
	// non-FullScan modes.
	if opts.FullScan {
		return files, nil
	}
	return changeset.FilterByPrefixes(files, opts.Dirs), nil
}

// getAllRepoFiles delegates to changeset.GetAllFiles() (which respects
// .gitignore) and filters out scaffold templates and any ExtraNonAppDirs
// prefixes (the same exclusions detectAppRoots uses for overlay discovery).
// This is the changeset source for FullScan mode.
func getAllRepoFiles() ([]string, error) {
	files, err := changeset.GetAllFiles()
	if err != nil {
		return nil, err
	}
	var filtered []string
	for _, f := range files {
		if !isExtraNonAppPath(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}
