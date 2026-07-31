package pipeline

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/git"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
)

// Options configures the pipeline run.
//
// Step/check enablement uses one generic ID-based mechanism instead of
// dedicated boolean flags per step - see the doc comment on
// validator.Options for the full explanation. DisabledChecks/EnabledChecks
// are passed straight through to validator.Options unchanged.
type Options struct {
	URL            string
	PR             string
	Revision       string
	TargetBranch   string
	HookSource     string
	TriggerComment string
	LintOnly       bool
	// PostComment controls whether a PR-comment summary is posted after the
	// run. Off by default; the CLI's --comment flag opts in. Comment
	// posting is also independently gated by github.Client.IsAvailable()
	// (repo/PR context present), so a local/no-PR run skips commenting
	// regardless of this flag.
	PostComment     bool
	Verbose         bool
	AssumeOpenShift bool     // treat OpenShift/OKD-only API groups as exempt from the sync-options check
	DisabledChecks  []string // IDs to disable entirely (e.g. "sync-options", "golangci", "avp"); only affects steps that default to enabled
	EnabledChecks   []string // IDs to explicitly enable; only affects steps that default to disabled (e.g. "kyverno")
	Concurrency     int
	Apps            []string
	Clusters        []string
	IncludePrefixes []string // restrict the changeset to files under these path prefixes (e.g. "kubernetes/", "tekton/"); empty means no restriction
	Providers       provider.Providers
}

// Result captures the pipeline outcome.
type Result struct {
	PRValid          bool
	TitleErr         error
	UnsignedErr      error
	ChecklistErr     error
	ValidatorResult  *validator.Result
	ValidationErr    error
	ReproduceCommand string
}

// Run executes the pipeline phases. When opts.URL is set (the normal
// Tekton-invoked path - see setupWorkdir), it clones the repo to a temp
// directory, checks out the resolved revision, and chdirs into it for the
// duration of the run before restoring the original working directory and
// removing the clone.
func Run(opts Options) error {
	start := time.Now()
	log := logger.NewLogger(opts.Verbose, "")
	tc := validator.NewTimingCollector()

	log.Header("GitOps CI Pipeline")
	log.Info(fmt.Sprintf("URL: %s", opts.URL))
	log.Info(fmt.Sprintf("PR: %s", opts.PR))
	log.Info(fmt.Sprintf("Revision: %s", resolveRevision(opts.Revision, opts.PR)))

	setupStart := time.Now()
	cleanup, err := setupWorkdir(opts)
	defer cleanup()
	tc.Record("Setup", time.Since(setupStart))
	if err != nil {
		log.Errorf("setup failed: %v", err)
		return fmt.Errorf("pipeline setup: %w", err)
	}
	log.Info(fmt.Sprintf("setup complete (%s)", time.Since(setupStart).Round(time.Millisecond)))

	res := &Result{}
	if shouldRunPRChecks(opts) {
		prStart := time.Now()
		log.Header("PR Validation")
		client := github.NewClient(opts.URL, opts.PR)
		res.TitleErr = github.ValidatePRTitle(client)
		if res.TitleErr != nil {
			log.Errorf("PR title: %v", res.TitleErr)
		} else {
			log.Info("PR title: passed")
		}
		res.UnsignedErr = runUnsignedCheck(client)
		if res.UnsignedErr != nil {
			log.Errorf("unsigned commits: %v", res.UnsignedErr)
		} else {
			log.Info("unsigned commits check: passed")
		}
		if shouldRunChecklistCheck(opts) {
			res.ChecklistErr = github.ValidatePRChecklist(client)
			if res.ChecklistErr != nil {
				log.Warn(fmt.Sprintf("PR checklist: %v", res.ChecklistErr))
			} else {
				log.Info("PR checklist: passed")
			}
		}
		tc.Record("PR Validation", time.Since(prStart))
	}
	res.PRValid = shouldRunPRChecks(opts) || opts.LintOnly

	vopts := toValidatorOptions(opts)
	vopts.Timing = tc
	vr, verr := validator.RunAll(vopts)
	res.ValidatorResult = vr
	res.ValidationErr = verr

	res.ReproduceCommand = validator.ReproduceCommand(vopts)

	if reason, skip := commentSkipReason(opts); skip {
		log.Info("comment posting skipped: " + reason)
	} else if err := postComment(res, opts); err != nil {
		log.Warn(fmt.Sprintf("posting PR comment failed: %v", err))
	} else {
		log.Info("PR comment posted")
	}

	if vr != nil && vr.Logger != nil {
		log.Info(vr.Logger.Summary())
	}
	log.Info(fmt.Sprintf("pipeline completed in %s", time.Since(start).Round(time.Second)))
	if res.ReproduceCommand != "" {
		log.Info("")
		log.Info("Reproduce locally:")
		log.Info("  " + res.ReproduceCommand)
	}

	if res.ValidationErr != nil || res.TitleErr != nil || res.UnsignedErr != nil {
		log.Error("pipeline completed with failures")
		return fmt.Errorf("pipeline completed with failures")
	}
	log.Info("All checks passed!")
	return nil
}

// setupWorkdir clones opts.URL (when set) to a temp directory, checks out
// the resolved revision, and chdirs into it, returning a cleanup function
// the caller must defer regardless of whether an error is also returned
// (cleanup is always safe to call and never itself errors). When opts.URL
// is empty - a local run against the current working directory, as used by
// the test-all/build-yaml/scan-all subcommands, or a bare `pipeline`
// invocation with no --url - this is a no-op: cleanup does nothing and no
// chdir happens, so Run behaves exactly as it did before this function
// existed.
func setupWorkdir(opts Options) (cleanup func(), err error) {
	noop := func() {}
	if opts.URL == "" {
		return noop, nil
	}

	revision := resolveRevision(opts.Revision, opts.PR)
	dir, err := git.Clone(git.CloneOptions{URL: opts.URL, Revision: revision, Verbose: opts.Verbose})
	if err != nil {
		return noop, fmt.Errorf("cloning %s: %w", opts.URL, err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		_ = git.Cleanup(dir)
		return noop, fmt.Errorf("getting working directory: %w", err)
	}
	if err := os.Chdir(dir); err != nil {
		_ = git.Cleanup(dir)
		return noop, fmt.Errorf("entering cloned repo %s: %w", dir, err)
	}

	return func() {
		_ = os.Chdir(origWD)
		_ = git.Cleanup(dir)
	}, nil
}

// resolveRevision determines the git revision to check out. An explicit
// raw revision always wins. Otherwise, a valid PR number resolves to that
// PR's head ref (refs/pull/<pr>/head) so PR runs check out the PR's actual
// commits instead of falling through to the target repo's default branch -
// which would silently validate the wrong code. With neither set, "HEAD"
// requests the clone's default branch.
func resolveRevision(raw, pr string) string {
	if raw != "" {
		return raw
	}
	if isValidPR(pr) {
		return fmt.Sprintf("refs/pull/%s/head", pr)
	}
	return "HEAD"
}

func isValidPR(pr string) bool {
	if pr == "" {
		return false
	}
	matched, _ := regexp.MatchString(`\{\{.*\}\}`, pr)
	return !matched
}

func (o *Options) isMergeQueue() bool {
	return strings.Contains(o.TargetBranch, "gh-readonly-queue/")
}

// shouldRunPRChecks reports whether the blocking PR-title and signed-commit
// checks should run. These run whenever there's a valid, non-merge-queue PR
// - including in --lint-only mode, since they're cheap, GitHub-API-only
// checks unrelated to the (skipped) build/validation phases.
func shouldRunPRChecks(opts Options) bool {
	return isValidPR(opts.PR) && !opts.isMergeQueue()
}

// shouldRunChecklistCheck reports whether the non-blocking PR-checklist
// check should run. Unlike shouldRunPRChecks, this is explicitly skipped in
// --lint-only mode: the checklist covers build/validation-adjacent items
// that don't make sense to ask about when the build phase itself is skipped.
func shouldRunChecklistCheck(opts Options) bool {
	return shouldRunPRChecks(opts) && !opts.LintOnly
}

func (o *Options) Workers() int {
	if o.Concurrency > 0 {
		return o.Concurrency
	}
	return runtime.NumCPU() * 2
}

func toValidatorOptions(opts Options) validator.Options {
	return validator.Options{
		RepoURL:         opts.URL,
		PR:              opts.PR,
		BaseRef:         resolveBaseRef(opts.TargetBranch),
		Revision:        resolveRevision(opts.Revision, opts.PR),
		LintOnly:        opts.LintOnly,
		Verbose:         opts.Verbose,
		AssumeOpenShift: opts.AssumeOpenShift,
		DisabledChecks:  opts.DisabledChecks,
		EnabledChecks:   opts.EnabledChecks,
		Concurrency:     opts.Workers(),
		Apps:            opts.Apps,
		Clusters:        opts.Clusters,
		IncludePrefixes: opts.IncludePrefixes,
		Providers:       opts.Providers,
	}
}

func resolveBaseRef(targetBranch string) string {
	if strings.HasPrefix(targetBranch, "gh-readonly-queue/") {
		parts := strings.Split(targetBranch, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	if targetBranch != "" {
		return targetBranch
	}
	return "origin/main"
}

func runUnsignedCheck(c *github.Client) error {
	commits, err := github.GetUnsignedCommits(c)
	if err != nil {
		return err
	}
	if len(commits) > 0 {
		return fmt.Errorf("%d unsigned commits detected", len(commits))
	}
	return nil
}

// commentSkipReason reports whether posting a PR comment should be skipped,
// and if so, why. Comment posting requires both an explicit opt-in
// (opts.PostComment, set from the CLI's --comment flag - off by default)
// and repo/PR context being available (github.Client.IsAvailable()); a
// local/no-PR run skips commenting regardless of the flag.
func commentSkipReason(opts Options) (reason string, skip bool) {
	if !opts.PostComment {
		return "--comment not passed", true
	}
	if !github.NewClient(opts.URL, opts.PR).IsAvailable() {
		return "no repo/PR context available", true
	}
	return "", false
}

func postComment(res *Result, opts Options) error {
	client := github.NewClient(opts.URL, opts.PR)
	if !client.IsAvailable() {
		return nil
	}
	marker := opts.Providers.ReportMarker()
	if marker == "" {
		marker = "<!-- gitops-ci-report -->"
	}
	report := validator.Report{
		Marker:   marker,
		Title:    "GitOps CI Results",
		Header:   "GitOps CI Pipeline",
		Sections: composeSections(res, opts),
		Body:     "```bash\n" + res.ReproduceCommand + "\n```",
	}
	if err := github.UpsertComment(client, marker, report.Render()); err != nil {
		return err
	}
	return github.DeleteComments(client, validator.LegacyMarkers()...)
}

func composeSections(res *Result, opts Options) []validator.Section {
	var sections []validator.Section

	// 1. PR Checks
	sections = append(sections, validator.ComposePRChecksSection(res.TitleErr, res.UnsignedErr, res.ChecklistErr))

	// 2–3. Linting + Static Checks pulled from ValidatorResult sections if present.
	lintReports := map[string]string{}
	staticReports := map[string]string{}
	if res.ValidatorResult != nil {
		for _, s := range res.ValidatorResult.Sections {
			switch s.Name {
			case "Linting":
				lintReports["summary"] = s.Body
			case "Static Checks":
				staticReports["summary"] = s.Body
			}
		}
	}
	sections = append(sections, validator.ComposeLintingSection(lintReports))
	sections = append(sections, validator.ComposeStaticChecksSection(staticReports))

	// 4. Kustomize Build
	sections = append(sections, validator.ComposeKustomizeBuildSection(0, nil, nil, nil))

	// 5. Scaffold Validation
	sections = append(sections, validator.ComposeScaffoldValidationSection("", nil, nil))

	// 6. Resource Compliance
	if res.ValidatorResult != nil {
		sections = append(sections, validator.ComposeResourceComplianceSection(res.ValidatorResult.Check.Findings))
	} else {
		sections = append(sections, validator.Section{Name: "Resource Compliance", Body: "No results."})
	}

	// 7. CI Notes
	_ = opts
	sections = append(sections, validator.ComposeCINotesSection("Pipeline completed."))
	return sections
}

// EnvOptions loads options from environment.
func EnvOptions() Options {
	return Options{
		URL:            os.Getenv("PARAMS_URL"),
		PR:             os.Getenv("PARAMS_PR"),
		Revision:       os.Getenv("PARAMS_REVISION"),
		TargetBranch:   os.Getenv("PARAMS_TARGET_BRANCH"),
		TriggerComment: os.Getenv("PARAMS_TRIGGER_COMMENT"),
	}
}
