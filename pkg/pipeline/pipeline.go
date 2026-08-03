package pipeline

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/git"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kyverno"
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
	TitleSuggestion  string // non-blocking (see github.PRTitleSuggestion) - never a failure reason.
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

	// opts.Providers.PipelineHeader() falls back to the generic default
	// ("GitOps CI Pipeline") when no Branding provider is wired, so an org
	// layer's custom header appears both here (the run's opening log line)
	// and in the unified PR-comment report (buildReport) instead of only
	// the latter.
	log.Header(opts.Providers.PipelineHeader())
	log.Info("%s", version.String())
	log.Info("URL: %s", opts.URL)
	log.Info("PR: %s", opts.PR)
	log.Info("Revision: %s", resolveRevision(opts.Revision, opts.PR))

	log.Header("Setup")
	setupStart := time.Now()
	cloneStart := time.Now()
	cleanup, err := setupWorkdir(opts)
	defer cleanup()
	tc.RecordStep("Setup", "clone", time.Since(cloneStart))
	if err != nil {
		tc.Record("Setup", time.Since(setupStart), false)
		log.Error("setup failed: %v", err)
		return fmt.Errorf("pipeline setup: %w", err)
	}

	// Prefetch schemas/policies once, up front, instead of paying for
	// their extraction lazily inside the concurrent Linting/Build+
	// Compliance phases on every run (see validator.Options.SchemaDir/
	// PolicyPath's doc comments). Schemas are always prefetched -
	// kubeconform always runs, and extraction is a cheap, pure
	// embedded-archive unpack. Policies are only prefetched when the
	// opt-in "kyverno" step (default off) is actually enabled, since
	// preparing them shells out to `kustomize build` - not worth paying
	// for on every run that never uses it.
	var schemaDir, policyPath string
	if schemasStart := time.Now(); true {
		if dir, schemaCleanup, schemaErr := kubeconform.ExtractSchemas(); schemaErr == nil {
			schemaDir = dir
			defer schemaCleanup()
		}
		tc.RecordStep("Setup", "schemas", time.Since(schemasStart))
	}
	if kyvernoEnabled(opts) {
		policiesStart := time.Now()
		if path, policyCleanup, policyErr := kyverno.PreparePolicies(); policyErr == nil {
			policyPath = path
			defer policyCleanup()
		}
		tc.RecordStep("Setup", "policies", time.Since(policiesStart))
	}

	tc.Record("Setup", time.Since(setupStart), false)
	log.Info("setup complete (%s)", time.Since(setupStart).Round(time.Millisecond))

	res := &Result{}
	if shouldRunPRChecks(opts) {
		prStart := time.Now()
		log.Header("PR Validation")
		client := github.NewClient(opts.URL, opts.PR)
		res.TitleErr = github.ValidatePRTitle(client)
		if res.TitleErr != nil {
			log.Error("PR title: %v", res.TitleErr)
		} else {
			log.Info("PR title: passed")
			// Only consulted once the required prefix has already passed -
			// see github.PRTitleSuggestion and ComposePRChecksSection/
			// prTitleChild's non-blocking rendering of this.
			res.TitleSuggestion = github.PRTitleSuggestion(client)
			if res.TitleSuggestion != "" {
				log.Warn("PR title suggestion: %s", res.TitleSuggestion)
			}
		}
		res.UnsignedErr = runUnsignedCheck(client)
		if res.UnsignedErr != nil {
			log.Error("unsigned commits: %v", res.UnsignedErr)
		} else {
			log.Info("unsigned commits check: passed")
		}
		if shouldRunChecklistCheck(opts) {
			res.ChecklistErr = github.ValidatePRChecklist(client)
			if res.ChecklistErr != nil {
				log.Warn("PR checklist: %v", res.ChecklistErr)
			} else {
				log.Info("PR checklist: passed")
			}
		}
		tc.Record("PR Validation", time.Since(prStart), false)
		// A single consolidated line - gated on the two BLOCKING checks
		// only (title/unsigned), matching downstream's behavior where this
		// still appears despite a non-blocking title-suggestion warning or
		// checklist warning - alongside the individual per-check lines
		// above, which stay so a failure's specific cause (title vs.
		// unsigned commits) remains visible; this aggregate only adds a
		// phase-level "how long did this take" summary.
		if res.TitleErr == nil && res.UnsignedErr == nil {
			log.Info("PR validation passed (%s)", time.Since(prStart).Round(time.Millisecond))
		}
	}
	res.PRValid = shouldRunPRChecks(opts) || opts.LintOnly

	vopts := toValidatorOptions(opts)
	vopts.Timing = tc
	vopts.SchemaDir = schemaDir
	vopts.PolicyPath = policyPath
	vr, verr := validator.RunAll(vopts)
	res.ValidatorResult = vr
	res.ValidationErr = verr

	res.ReproduceCommand = validator.ReproduceCommand(vopts)

	// Every phase (Linting, Static Checks, Build YAML, Post-Build
	// Validation, ...) composes a detail-bearing Section per check (the
	// actual file/message list behind a step's summary "N violation(s)" log
	// line), but res.Sections is otherwise only ever rendered into the PR
	// comment body - which is skipped whenever --comment isn't passed
	// (e.g. local/CLI-only runs). Print the failing sections' full detail
	// to the console here so
	// --verbose is self-sufficient without requiring --comment; this
	// applies uniformly to every section (linting, static checks, build,
	// scaffold, resource compliance), not just one of them.
	if opts.Verbose {
		printFailedSectionDetail(vr, log)
	}

	if vr != nil && vr.Logger != nil {
		if summary := tc.Summary(time.Since(start)); summary != "" {
			log.Raw(summary)
		}
		log.Raw(vr.Logger.Summary(len(vr.Sections), vr.FailedSectionCount()))
	}
	log.Info("pipeline completed in %s", time.Since(start).Round(time.Second))
	if res.ReproduceCommand != "" {
		log.Raw("")
		log.Info("Reproduce locally:")
		log.Info("  %s", res.ReproduceCommand)
	}

	// Comment posting happens after the local console summary/timing-table/
	// reproduce-locally output above (not before it, as it used to), so the
	// lowercase "pipeline completed in %s" line above measures core
	// validation work only, and the final "Pipeline completed in %s" line
	// below - a separate, later measurement of the same time.Since(start) -
	// genuinely differs from it whenever comment posting (a network call)
	// took measurable time, instead of both lines reporting the identical
	// value.
	if reason, skip := commentSkipReason(opts); skip {
		log.Info("comment posting skipped: %s", reason)
	} else if err := postComment(res, opts); err != nil {
		log.Warn("posting PR comment failed: %v", err)
	} else {
		log.Info("PR comment posted")
	}
	log.Info("Pipeline completed in %s", time.Since(start).Round(time.Second))

	if res.ValidationErr != nil || res.TitleErr != nil || res.UnsignedErr != nil || validatorResultFailed(vr) {
		// Not log.Error here: this exact message is returned as the error
		// below, which main() already prints once via fmt.Fprintln(os.Stderr,
		// err) - logging it too would print the identical line twice.
		return fmt.Errorf("pipeline completed with failures")
	}
	log.Info("All checks passed!")
	return nil
}

// validatorResultFailed reports whether the validator's own run found any
// blocking/error-level condition. res.ValidationErr is only ever non-nil for
// a hard setup failure inside validator.RunAll (e.g. failing to resolve the
// changeset) - RunAll always returns a nil error for a run that completed
// but found blocking/error-level findings, communicating that instead via
// vr.Blocking (Resource Compliance direct findings) and vr.Logger's
// recorded errors/failed sections (see vr.Logger.Summary()'s
// "Errors"/"Failed sections", the same counters HasFailures() reads).
// Previously Run's failure check only looked at res.ValidationErr, so a PR
// with blocking Resource Compliance findings, or any failed Linting/Static
// Checks/Build section, still exited 0 and printed "All checks passed!".
func validatorResultFailed(vr *validator.Result) bool {
	if vr == nil {
		return false
	}
	return vr.Blocking || (vr.Logger != nil && vr.Logger.HasFailures())
}

// printFailedSectionDetail logs the full Body of every errored section in
// vr.Sections to the console. This is the console-output analog of what
// composeSections/postComment already does for the PR comment - see the
// comment at its call site in Run for why this is needed independently of
// comment posting. Uses log.SubHeader for each section's header so this
// shares the same "----\n Title\n----" banner family as the phase headers
// (log.Header/log.SubHeader) and cmd/k8s-gitops-ci's test-all/scan-all/
// build-yaml failed-section rendering, instead of inventing its own style.
func printFailedSectionDetail(vr *validator.Result, log *logger.Logger) {
	if vr == nil {
		return
	}
	for _, s := range vr.Sections {
		if !s.Error || strings.TrimSpace(s.Body) == "" {
			continue
		}
		log.Raw("")
		log.SubHeader(s.Name)
		log.Raw(SanitizeSectionBodyForConsole(s.Body))
	}
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

// kyvernoEnabled reports whether the opt-in "kyverno" step (default off -
// see phases.go's defaultOffSteps) is enabled for this run, so Run's Setup
// phase knows whether it's worth prefetching Kyverno policies (which shells
// out to `kustomize build`) up front versus never touching them at all.
func kyvernoEnabled(opts Options) bool {
	for _, id := range opts.EnabledChecks {
		if id == "kyverno" {
			return true
		}
	}
	return false
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
		TriggerComment:  opts.TriggerComment,
		HookSource:      opts.HookSource,
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

// buildReport constructs the unified PR-comment report from the run result,
// pulling Title/Header/Marker from the org-injectable provider.Providers
// seam (opts.Providers) rather than hardcoding generic-sounding defaults, so
// an org's Branding provider actually takes effect on every rendered field.
func buildReport(res *Result, opts Options) validator.Report {
	marker := opts.Providers.ReportMarker()
	if marker == "" {
		marker = "<!-- gitops-ci-report -->"
	}
	return validator.Report{
		Marker:   marker,
		Title:    opts.Providers.ReportTitle(),
		Header:   opts.Providers.PipelineHeader(),
		Sections: composeSections(res, opts),
		Body:     "```bash\n" + res.ReproduceCommand + "\n```",
	}
}

func postComment(res *Result, opts Options) error {
	client := github.NewClient(opts.URL, opts.PR)
	if !client.IsAvailable() {
		return nil
	}
	report := buildReport(res, opts)
	if err := github.UpsertComment(client, report.Marker, report.Render()); err != nil {
		return err
	}
	if err := github.DeleteComments(client, validator.LegacyMarkers()...); err != nil {
		return err
	}
	// Prune unwanted third-party bot comments as configured by the
	// CommentPolicy provider (e.g. an org-specific bot comment). No-op when
	// no provider is wired, since ForeignMarkers() returns nil by default.
	return github.DeleteComments(client, opts.Providers.ForeignMarkers()...)
}

// validatorSectionOrFallback looks up a named section in vr.Sections (the
// fully-realized, nested-dropdown Sections phases.go already builds -
// Linting/Static Checks/Kustomize Build/Scaffold Validation/Resource
// Compliance), reusing it verbatim instead of re-deriving it from an
// already-rendered Body string and composing it a second time (which used
// to double-nest the markdown). Falls back to a plain "No results." stub
// when vr is nil (e.g. the validation phase never ran) or the named section
// wasn't produced (e.g. --lint-only mode, which skips the build phase
// entirely and so never produces "Kustomize Build" et al).
func validatorSectionOrFallback(vr *validator.Result, name string) validator.Section {
	if s, ok := validatorSection(vr, name); ok {
		return s
	}
	return validator.Section{Name: name, Body: "No results."}
}

// validatorSection looks up a named section in vr.Sections, reporting
// whether it was found - unlike validatorSectionOrFallback, callers for
// whom "not produced at all" (e.g. an opt-in phase that never ran) means
// something different than "produced, found nothing" use this to omit the
// section entirely instead of rendering a fallback stub.
func validatorSection(vr *validator.Result, name string) (validator.Section, bool) {
	if vr == nil {
		return validator.Section{}, false
	}
	for _, s := range vr.Sections {
		if s.Name == name {
			return s, true
		}
	}
	return validator.Section{}, false
}

func composeSections(res *Result, opts Options) []validator.Section {
	sections := make([]validator.Section, 0, 7)

	// 1. PR Checks
	sections = append(sections, validator.ComposePRChecksSection(res.TitleErr, res.UnsignedErr, res.ChecklistErr, res.TitleSuggestion))

	// 2–7. Linting, Static Checks, Kustomize Build, Scaffold Validation,
	// Scaffold Drift Protection, and Resource Compliance are all fully
	// composed by phases.go during validator.RunAll - reuse them by name
	// rather than recomposing. These are unconditional (--lint-only mode,
	// which skips the build phase entirely, is the only reason one'd be
	// missing - hence the "No results." fallback).
	names := []string{
		"Linting", "Static Checks", "Kustomize Build", "Scaffold Validation",
		"Scaffold Drift Protection", "Resource Compliance",
	}
	for _, name := range names {
		sections = append(sections, validatorSectionOrFallback(res.ValidatorResult, name))
	}

	// 8–9. NetworkAttachmentDefinition Validation and Kyverno Policies are
	// both omit-when-absent: phases.go only produces the NAD section when a
	// NAD is actually present in the rendered chain, and the Kyverno section
	// only when the opt-in "kyverno" step is enabled (default off - see
	// docs/CI.md#registered-checks). validatorSectionOrFallback's "No
	// results." stub for a name phases.go never produced would be misleading
	// here (it'd read as "checked, found nothing" rather than "no NAD in this
	// change"/"not run"), so each is appended only when actually present.
	for _, name := range []string{"NetworkAttachmentDefinition Validation", "Kyverno Policies"} {
		if s, ok := validatorSection(res.ValidatorResult, name); ok {
			sections = append(sections, s)
		}
	}

	// 9. CI Notes
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
