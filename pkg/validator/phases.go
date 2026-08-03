package validator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/config"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/csv"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/kustomize"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/largefile"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/golangci"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/markdownlint"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/prettier"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/shellcheck"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/yamlsyntax"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// Step IDs for standalone (non-check-registry) lint/build steps that
// participate in the same generic enable/disable ID mechanism as
// check-registry checks. See stepEnabled and the Options doc comment.
const (
	stepGolangci       = "golangci"
	stepAVP            = "avp"
	stepKyverno        = "kyverno"
	stepScaffoldReadme = "scaffold-readme"
)

// defaultOffSteps lists step/check IDs that are disabled unless explicitly
// present in Options.EnabledChecks. Every other ID defaults to enabled and
// is only turned off via Options.DisabledChecks.
//
//   - kyverno defaults off because, unlike every other check in this repo,
//     it has no generic default policy set an arbitrary org could
//     reasonably run out of the box - an org must opt in and supply its
//     own policies (see pkg/lint/kyverno).
//   - scaffold-readme (scaffold.CheckReadmeStatus's README scaffold-status
//     table structural check, rendered as the "scaffold table" Static
//     Checks sub-check - see docs/CI.md#scaffold-validation) defaults off
//     because, like kyverno, this generic core can't know whether a given
//     repo's `<!-- scaffold-status -->` table actually matches the
//     one-row-per-app-per-overlay shape this check expects - an org
//     already maintaining that table in a different shape/grouping would
//     otherwise see this newly-real check start blocking PRs with false
//     positives. An org confirms compatibility once, then opts in.
var defaultOffSteps = map[string]bool{
	stepKyverno:        true,
	stepScaffoldReadme: true,
}

// stepEnabled reports whether the named step/check should run, given the
// resolved disabled/enabled ID sets (see toIDSet). Steps not present in
// defaultOffSteps run unless explicitly disabled; steps present in
// defaultOffSteps only run when explicitly enabled.
func stepEnabled(id string, disabled, enabled map[string]bool) bool {
	if defaultOffSteps[id] {
		return enabled[id]
	}
	return !disabled[id]
}

// runLintAndStaticChecks runs all linters and static checks concurrently
// (each linter/check is independent - different tools, different file
// filters, no shared mutable state besides the mutex-guarded report maps
// below), populating sections and per-step timing.
func runLintAndStaticChecks(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector, earlySelectors []exempt.Selector) {
	disabled := toIDSet(opts.DisabledChecks)
	enabled := toIDSet(opts.EnabledChecks)

	// lintStepResult is what each linter/static-check closure returns: a
	// failure report (empty when it passed/was skipped) plus enough to
	// build a CheckOutcome so the Linting/Static Checks sections can always
	// render a full sub-check breakdown, not just a flattened bullet that
	// disappears once a check is clean.
	type lintStepResult struct {
		report  string
		status  SectionStatus
		skipped bool
		note    string
	}

	// staticReports/staticOutcomes accumulate across all of Large File
	// Check, YAML Syntax, and the Static Checks phase below - even though
	// large-file/YAML-syntax now get their own standalone console
	// log.Header banners (and their own top-level TimingCollector entries,
	// matching a downstream fork's equivalent phase breakdown), they still
	// feed the same "Static Checks" Section/PR-comment rendering
	// (ComposeStaticChecksSection's fixed 5-check order) as before this
	// split - only the live console/timing-table grouping changed. No mutex
	// needed for the two sequential appends below (Large File Check/YAML
	// Syntax run before any goroutine touches these slices); the
	// concurrent Static Checks group further down still guards its own
	// appends with staticMu.
	staticReports := map[string]string{}
	var staticOutcomes []CheckOutcome

	// ── large file check (standalone phase, sequential) ─────────────────────
	largeFileStart := time.Now()
	log.Header("Large File Check")
	if violations := largefile.Check(changed, largefile.DefaultMaxSize, largefile.DefaultIgnorePatterns); len(violations) > 0 {
		var sb strings.Builder
		for _, v := range violations {
			sb.WriteString(v.String() + "\n")
		}
		log.ErrorInSection("LargeFile", "%d large file violation(s)", len(violations))
		staticReports["large-file"] = sb.String()
		staticOutcomes = append(staticOutcomes, CheckOutcome{Name: "large-file", Status: StatusError})
	} else {
		log.Info("large-file check: passed (%s)", time.Since(largeFileStart).Round(time.Millisecond))
		staticOutcomes = append(staticOutcomes, CheckOutcome{Name: "large-file", Status: StatusPassed})
	}
	tc.Record("Large File Check", time.Since(largeFileStart), false)

	// ── YAML syntax (standalone phase, sequential) ──────────────────────────
	yamlSyntaxStart := time.Now()
	log.Header("YAML Syntax")
	log.Info("checking %d YAML file(s) for syntax errors...", len(filterYAML(changed)))
	if yvs, _ := yamlsyntax.CheckFiles(changed); len(yvs) > 0 {
		var sb strings.Builder
		for _, v := range yvs {
			fmt.Fprintf(&sb, "%s: %s\n", v.File, v.Message)
		}
		log.ErrorInSection("YAMLSyntax", "%d YAML syntax error(s)", len(yvs))
		staticReports["YAML-syntax"] = sb.String()
		staticOutcomes = append(staticOutcomes, CheckOutcome{Name: "YAML-syntax", Status: StatusError})
	} else {
		log.Info("YAML-syntax check: passed (%s)", time.Since(yamlSyntaxStart).Round(time.Millisecond))
		staticOutcomes = append(staticOutcomes, CheckOutcome{Name: "YAML-syntax", Status: StatusPassed})
	}
	tc.Record("YAML Syntax", time.Since(yamlSyntaxStart), false)

	// ── linting ──────────────────────────────────────────────────────────────
	lintStart := time.Now()
	log.Header("Linting")

	lintReports := map[string]string{}
	var lintOutcomes []CheckOutcome
	var lintMu sync.Mutex
	var lintWg sync.WaitGroup

	runLintStep := func(name string, fn func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult) {
		lintWg.Add(1)
		go func() {
			defer lintWg.Done()
			start := time.Now()
			elapsed := func() time.Duration { return time.Since(start) }
			sl := log.Scope()
			r := fn(sl, elapsed)
			sl.Flush()
			tc.RecordStep("Linting", name, time.Since(start))
			lintMu.Lock()
			if r.report != "" {
				lintReports[name] = r.report
			}
			lintOutcomes = append(lintOutcomes, CheckOutcome{Name: name, Status: r.status, Skipped: r.skipped, Note: r.note})
			lintMu.Unlock()
		}()
	}

	runLintStep("markdownlint", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
		if len(markdownlint.Filter(changed)) == 0 {
			sl.Info("markdownlint: no markdown files changed")
			return lintStepResult{status: StatusPassed, skipped: true, note: "No markdown files changed."}
		}
		if mdOut, err := markdownlint.Run(changed); err == nil {
			sl.Info("markdownlint: passed (%s)", elapsed().Round(time.Millisecond))
			return lintStepResult{status: StatusPassed}
		} else if !errors.Is(err, markdownlint.ErrCLINotFound) {
			sl.ErrorInSection("Markdownlint", "markdownlint: %s", err)
			detail := mdOut
			if detail == "" {
				detail = err.Error()
			}
			return lintStepResult{report: detail, status: StatusError}
		}
		sl.Debug("markdownlint: not found in PATH, skipping")
		return lintStepResult{status: StatusPassed, skipped: true, note: "markdownlint not found in PATH."}
	})

	runLintStep("prettier", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
		if pOut, err := prettier.Run(changed, nil); err == nil {
			sl.Info("prettier: passed (%s)", elapsed().Round(time.Millisecond))
			return lintStepResult{status: StatusPassed}
		} else if !errors.Is(err, prettier.ErrCLINotFound) {
			sl.ErrorInSection("Prettier", "prettier: %s", err)
			detail := pOut
			if detail == "" {
				detail = err.Error()
			}
			return lintStepResult{report: detail, status: StatusError}
		}
		sl.Debug("prettier: not found in PATH, skipping")
		return lintStepResult{status: StatusPassed, skipped: true, note: "prettier not found in PATH."}
	})

	runLintStep("shellcheck", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
		if _, err := exec.LookPath("shellcheck"); err != nil {
			sl.Debug("shellcheck: not found in PATH, skipping")
			return lintStepResult{status: StatusPassed, skipped: true, note: "shellcheck not found in PATH."}
		}

		// yamlChanged feeds both the "any relevant files at all" short-
		// circuit below and the direct/blocking Tekton+embedded-script
		// extraction pass further down - computed once and reused rather
		// than calling filterYAML(changed) twice.
		yamlChanged := filterYAML(changed)
		if len(shellcheck.FilterShellScripts(changed)) == 0 && len(yamlChanged) == 0 {
			sl.Info("shellcheck: no shell files changed")
			return lintStepResult{status: StatusPassed, skipped: true, note: "No shell files changed."}
		}

		var sb strings.Builder
		blocking, warning := 0, 0

		// Raw shell script files: always direct/blocking - they're
		// literally files in this changeset's diff, so any finding here
		// is the author's own responsibility to fix.
		if scViolations, _, scErr := shellcheck.Run(changed); scErr == nil {
			for _, v := range scViolations {
				fmt.Fprintf(&sb, "%s:%d: %s\n", v.File, v.Line, v.Message)
			}
			blocking += len(scViolations)
		} else if !errors.Is(scErr, shellcheck.ErrCLINotFound) {
			sl.ErrorInSection("Shellcheck", "shellcheck: %s", scErr)
			return lintStepResult{report: scErr.Error(), status: StatusError}
		}

		// Tekton Task step scripts and embedded container-command/
		// ConfigMap scripts: classified direct (blocking) vs. external
		// (warning-only) by whether the script's source YAML file was
		// itself changed in this diff, or only pulled in because the
		// overlay it lives in was affected by an unrelated base/
		// component change elsewhere - the same distinction
		// finalizeCompliance already draws for doc/overlay check
		// findings in the Post-Build Validation phase (a base/component
		// change ripples to every overlay that depends on it, and an
		// issue in a file the author never touched shouldn't block
		// their PR).
		blocking += writeShellcheckExtractionReport(&sb, "", yamlChanged)

		external := externalOverlayYAMLFiles(changed)
		warning += writeShellcheckExtractionReport(&sb, " (external)", external)

		if blocking > 0 {
			sl.ErrorInSection("Shellcheck", "%d shellcheck violation(s)", blocking)
			return lintStepResult{report: sb.String(), status: StatusError}
		}
		if warning > 0 {
			sl.Info("shellcheck: passed (%d external/non-blocking warning(s)) (%s)", warning, elapsed().Round(time.Millisecond))
			return lintStepResult{report: sb.String(), status: StatusPassed, note: fmt.Sprintf("%d external warning(s) in overlay files not directly changed (non-blocking).", warning)}
		}
		sl.Info("shellcheck: passed (%s)", elapsed().Round(time.Millisecond))
		return lintStepResult{status: StatusPassed}
	})

	if stepEnabled(stepGolangci, disabled, enabled) {
		runLintStep("golangci", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			if len(golangci.FilterGo(changed)) == 0 {
				sl.Info("golangci: no Go files changed")
				return lintStepResult{status: StatusPassed, skipped: true, note: "No Go files changed."}
			}
			glOut, err := golangci.Run(changed)
			if err != nil && !errors.Is(err, golangci.ErrCLINotFound) {
				sl.ErrorInSection("Golangci", "golangci: %s", err)
				detail := glOut
				if detail == "" {
					detail = err.Error()
				}
				return lintStepResult{report: detail, status: StatusError}
			}
			if err != nil {
				sl.Debug("golangci: not found in PATH, skipping")
				return lintStepResult{status: StatusPassed, skipped: true, note: "golangci-lint not found in PATH."}
			}
			sl.Info("golangci-lint: passed (%s)", elapsed().Round(time.Millisecond))
			return lintStepResult{status: StatusPassed}
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "golangci", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	runLintStep("kubeconform", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
		yamlFiles := changeset.FilterByExtension(changed, ".yaml", ".yml")
		yamlFiles = filterKubeconformExemptions(yamlFiles, earlySelectors)
		kcOpts := kubeconform.DefaultOptions()
		if opts.SchemaDir != "" {
			// Already extracted once, up front, by pkg/pipeline's Setup
			// phase (see validator.Options.SchemaDir's doc comment) -
			// callers that don't prefetch (test-all/build-yaml/scan-all)
			// leave this empty and fall through to the lazy extraction
			// below, exactly as before this field existed.
			kcOpts.SchemaDir = opts.SchemaDir
		} else if schemaDir, cleanup, err := kubeconform.ExtractSchemas(); err == nil {
			kcOpts.SchemaDir = schemaDir
			defer cleanup()
		}
		if kcRes, err := validateWithRenderedOverlays(yamlFiles, kcOpts); err == nil && kcRes != nil {
			if kcRes.Invalid > 0 || kcRes.Errors > 0 {
				sl.ErrorInSection("Kubeconform", "%s", kcRes.Summary())
				return lintStepResult{report: kcRes.Summary(), status: StatusError}
			}
			sl.Info("kubeconform: passed (%s)", elapsed().Round(time.Millisecond))
		}
		return lintStepResult{status: StatusPassed}
	})

	lintWg.Wait()

	res.Sections = append(res.Sections, ComposeLintingSection(lintOutcomes, lintReports))
	tc.Record("Linting", time.Since(lintStart), true)

	// ── static checks ────────────────────────────────────────────────────────
	// large-file/YAML-syntax outcomes were already appended above (their own
	// standalone phases); this group only covers the remaining checks that
	// still share one concurrent "Static Checks" console phase.
	staticStart := time.Now()
	log.Header("Static Checks")
	var staticMu sync.Mutex
	var staticWg sync.WaitGroup

	runStaticStep := func(name string, fn func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult) {
		staticWg.Add(1)
		go func() {
			defer staticWg.Done()
			start := time.Now()
			elapsed := func() time.Duration { return time.Since(start) }
			sl := log.Scope()
			r := fn(sl, elapsed)
			sl.Flush()
			tc.RecordStep("Static Checks", name, time.Since(start))
			staticMu.Lock()
			if r.report != "" {
				staticReports[name] = r.report
			}
			staticOutcomes = append(staticOutcomes, CheckOutcome{Name: name, Status: r.status, Skipped: r.skipped, Note: r.note})
			staticMu.Unlock()
		}()
	}

	runStaticStep("config-sort", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
		if sorted, err := config.CheckSortOrder(); err == nil && len(sorted) > 0 {
			sl.ErrorInSection("ConfigSort", "%d unsorted config file(s)", len(sorted))
			return lintStepResult{report: config.FormatUnsortedError(sorted), status: StatusError}
		}
		sl.Info("config-sort check: passed (%s)", elapsed().Round(time.Millisecond))
		return lintStepResult{status: StatusPassed}
	})

	runStaticStep("startingCSV", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
		if mismatches, err := csv.CheckStartingCSVFolderMatch(changed); err == nil && len(mismatches) > 0 {
			sl.ErrorInSection("StartingCSV", "%d startingCSV mismatch(es)", len(mismatches))
			return lintStepResult{report: csv.FormatMismatches(mismatches), status: StatusError}
		}
		sl.Info("startingCSV check: passed (%s)", elapsed().Round(time.Millisecond))
		return lintStepResult{status: StatusPassed}
	})

	// scaffold table - a cheap, structural, per-PR check of the README's
	// <!-- scaffold-status --> table: does it list exactly the (app,
	// overlay) pairs that exist on disk today, with no missing/stale rows?
	// Named "scaffold table" (matching hintByCheck's key in comments.go) so
	// a failure automatically gets the "k8s-gitops-ci update-scaffold-
	// status" fix-command hint composeCheckChild already generates for any
	// named check with a registered hint. Gated behind stepScaffoldReadme
	// (default off - see defaultOffSteps' doc comment above): this generic
	// core can't know whether a given repo's table actually matches the
	// one-row-per-app-per-overlay shape this check expects.
	if stepEnabled(stepScaffoldReadme, disabled, enabled) {
		runStaticStep("scaffold table", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			if readmeCurrent, readmeDiff := scaffold.CheckReadmeStatus(); !readmeCurrent {
				sl.ErrorInSection("Scaffold", "%s", readmeDiff)
				return lintStepResult{report: readmeDiff, status: StatusError}
			}
			sl.Info("scaffold table check: passed (%s)", elapsed().Round(time.Millisecond))
			return lintStepResult{status: StatusPassed}
		})
	} else {
		staticMu.Lock()
		staticOutcomes = append(staticOutcomes, CheckOutcome{Name: "scaffold table", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		staticMu.Unlock()
	}

	staticWg.Wait()

	res.Sections = append(res.Sections, ComposeStaticChecksSection(staticOutcomes, staticReports))
	tc.Record("Static Checks", time.Since(staticStart), true)
}

// runBuildAndPostBuild runs the registry-driven doc + overlay check engine,
// split into two console phases - "Build YAML" (kustomize builds, hooks,
// scaffold validation) and "Post-Build Validation" (doc/overlay compliance
// checks, Kyverno, NAD) - matching a downstream fork's equivalent phase
// breakdown. The split is safe because neither phase's work depends on the
// other's *findings* as input: the doc engine (runDocChecks) only reads raw
// changed YAML files, not build output, and the overlay-check pass
// (runOverlayChecks) is run in the same worker-pool loop as the actual
// build (buildOverlayWithHooks) since both need the same per-overlay
// worker-pool parallelism and neither result feeds the other within a
// single iteration - see runOverlayChecks/buildOverlayWithHooks's
// respective doc comments.
func runBuildAndPostBuild(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector) {
	w := Workers(opts)

	// Overlays/apps are resolved up front (rather than after the doc/
	// overlay check loops, as before) because both this run's exemption
	// selectors and its hook execution need the app list first: each app's
	// test.sh is resolved exactly once here, its EXEMPTIONS=(...) merged
	// into selectors below, and the same *hook.Config reused for every one
	// of that app's overlay builds plus its single POST_VALIDATE_HOOK call.
	overlays := detectOverlaysForChanges(changed)
	apps := uniqueApps(overlays)

	disabled := toIDSet(opts.DisabledChecks)
	enabled := toIDSet(opts.EnabledChecks)

	hookSource := resolveHookSource(opts)
	hookCfgs := resolveAppHookConfigs(apps, hookSource)
	defer cleanupAppHookConfigs(hookCfgs)

	// AVP ("avp" step, default-on - see stepAVP/stepEnabled and Options'
	// DisabledChecks doc comment) auto-detects, per app, whether its
	// overlays need to be piped through argocd-vault-plugin secret
	// resolution before the rest of this phase's checks run against them.
	avpEnabled := stepEnabled(stepAVP, disabled, enabled)
	appStrategies := resolveAppBuildStrategies(apps, avpEnabled, hookCfgs)

	// Built-in selectors (e.g. the Tekton Pipelines-as-code .tekton/
	// default, see tekton_exemptions.go) plus every app's hook-provided
	// EXEMPTIONS=(...) selectors (see docs/HOOKS.md). A malformed
	// EXEMPTIONS entry exempts nothing (fail-closed) and is surfaced as a
	// blocking build error below rather than silently dropped.
	hookSelectors, hookExemptErrs := hookExemptSelectorsAndErrors(hookCfgs)
	selectors := append(builtinExemptSelectors(), hookSelectors...)

	// ── Build YAML ───────────────────────────────────────────────────────────
	buildStart := time.Now()
	log.Header("Build YAML")

	// Scaffold Validation runs first, before the per-app builds below - it's
	// a drift check of each app's scaffold template against disk, entirely
	// independent of this run's own overlay-build/compliance-check output,
	// matching a downstream fork's ordering (scaffold's drift check
	// precedes the actual per-app kustomize builds in its "Build YAML"
	// phase). The README scaffold-status table's own structural staleness
	// check ("scaffold table") runs as a Static Checks sub-check instead
	// (see runLintAndStaticChecks) - it's a per-PR structural check, not a
	// per-app drift re-run, so it belongs with large-file/YAML-syntax/
	// config-sort/startingCSV rather than folded into this section's drift
	// summary.
	scaffoldResult := runScaffoldValidation(opts, apps, changed, log)

	log.Info("running overlay checks over %d overlay(s)...", len(overlays))
	kyvernoEnabled := stepEnabled(stepKyverno, disabled, enabled)
	var overlayResult check.Result
	buildErrs := make([]string, 0, len(overlays))
	hookResults := make(map[string]*appHookResult, len(apps))
	for _, app := range apps {
		hookResults[app] = &appHookResult{}
	}
	// overlaysPerApp counts each app's overlays (preserving overlays'
	// global sorted-by-path order - see detectOverlaysForChanges), so the
	// per-app "Building: <name>" summary banner printed after the loop
	// below can report each app's overlay count without needing to
	// serialize the underlying worker-pool build itself (see
	// appFromOverlayPath) - i.e. builds still run fully in parallel across
	// every overlay of every app; the banner is a post-hoc summary, not a
	// live per-app buffered stream.
	overlaysPerApp := make(map[string]int, len(apps))
	for _, ov := range overlays {
		overlaysPerApp[appFromOverlayPath(ov.path)]++
	}
	var renderedOverlays []renderedOverlay
	if len(overlays) > 0 {
		overlayWorkers := w
		if overlayWorkers > len(overlays) {
			overlayWorkers = len(overlays)
		}
		if overlayWorkers < 1 {
			overlayWorkers = 1
		}
		jobs := make(chan overlayRef, len(overlays))
		var overlayMu sync.Mutex
		var overlayWg sync.WaitGroup
		for i := 0; i < overlayWorkers; i++ {
			overlayWg.Add(1)
			go func() {
				defer overlayWg.Done()
				for ov := range jobs {
					ovStart := time.Now()
					r := runOverlayChecks([]string{ov.path}, ov.cluster, selectors, 1, disabled)
					app := appFromOverlayPath(ov.path)
					buildErr, pre, post, rendered := buildOverlayWithHooks(ov, hookCfgs[app], appStrategies[app])
					tc.RecordStep("Build YAML", ov.path, time.Since(ovStart))
					overlayMu.Lock()
					overlayResult.Findings = append(overlayResult.Findings, r.Findings...)
					overlayResult.Exempted = append(overlayResult.Exempted, r.Exempted...)
					if buildErr != "" {
						buildErrs = append(buildErrs, buildErr)
						log.ErrorInSection("Hooks", "%s", buildErr)
					} else if len(rendered) > 0 {
						// Collected unconditionally (not just when Kyverno is
						// enabled) since NAD validation below also consumes
						// every successfully-rendered overlay.
						renderedOverlays = append(renderedOverlays, renderedOverlay{overlay: ov.path, data: rendered})
					}
					if hr := hookResults[app]; hr != nil {
						hr.PreBuild = mergeHookOutcome(hr.PreBuild, pre)
						hr.PostBuild = mergeHookOutcome(hr.PostBuild, post)
					}
					overlayMu.Unlock()
				}
			}()
		}
		for _, ov := range overlays {
			jobs <- ov
		}
		close(jobs)
		overlayWg.Wait()
	}

	for _, err := range runAppPostValidateHooks(apps, hookCfgs, hookResults) {
		buildErrs = append(buildErrs, err)
		log.ErrorInSection("Hooks", "%s", err)
	}
	for _, err := range hookExemptErrs {
		buildErrs = append(buildErrs, err)
		log.ErrorInSection("Hooks", "%s", err)
	}

	// Per-app "Building: <name>" summary banner - printed once per app,
	// after every overlay of every app has already finished building
	// (fully in parallel, above), reporting that app's overlay count and
	// detected hook activity. This is a post-hoc summary rather than a
	// live per-overlay buffered stream, so it doesn't require serializing
	// the worker-pool build to get a contiguous per-app output block.
	for _, app := range apps {
		log.SubHeader("Building: " + app)
		log.Info("overlays: %d", overlaysPerApp[app])
		if hr := hookResults[app]; hr != nil {
			if hr.PreBuild == hookRan {
				log.Info("hook: PRE_BUILD_HOOK detected")
			}
			if hr.PostBuild == hookRan {
				log.Info("hook: POST_BUILD_HOOK detected")
			}
		}
	}

	fixNeeded, _ := kustomize.CheckFix(changed)
	hookTable := buildHookTable(apps, hookCfgs, hookResults)
	// addedFiles feeds ghostpatch.ClassifyOverlay's "is this
	// kustomization.yaml itself new" check (a ghost patch on a brand-new
	// overlay is a warning, not this PR's fault to have introduced against
	// existing history) - an error resolving it (e.g. no git history
	// available) degrades to "no added files", matching this phase's
	// existing tolerant-of-git-failure pattern (kustomize.CheckFix above).
	addedFiles, _ := changeset.GetAddedFiles(changeset.Options{BaseRef: opts.BaseRef, PR: opts.PR, RepoURL: opts.RepoURL})
	ghostTable, ghostBlockingCount := buildGhostTable(apps, addedFiles)
	res.Sections = append(res.Sections, ComposeKustomizeBuildSection(len(overlays), buildErrs, hookTable, fixNeeded, ghostTable))
	if ghostBlockingCount > 0 {
		log.ErrorInSection("KustomizeBuild", "%d blocking ghost patch(es)", ghostBlockingCount)
	}

	res.Sections = append(res.Sections, ComposeScaffoldValidationSection(
		strings.Join(scaffoldResult.DriftLines, "\n"),
		scaffoldResult.ExecErrors,
		flattenSkippedClusters(scaffoldResult.SkippedClusters),
		strings.Join(scaffoldResult.PreExistingDriftLines, "\n"),
	))
	res.Sections = append(res.Sections, ComposeDriftProtectionSection(findUnprotectedApps(changed)))
	tc.Record("Build YAML", time.Since(buildStart), len(overlays) > 1)

	// ── Post-Build Validation ────────────────────────────────────────────────
	postBuildStart := time.Now()
	log.Header("Post-Build Validation")

	// Doc engine over all changed YAML files. kyverno-test.yaml fixture
	// directories are excluded from compliance doc-checks (their paired
	// resources are deliberately non-compliant CLI test data, not real
	// workloads) - this doesn't affect kubeconform/Kyverno validation,
	// which run over `changed`/rendered overlays through their own paths.
	yamlFiles := filterKyvernoTestFixtureDirs(filterYAML(changed))
	log.Info("running doc checks over %d YAML file(s)...", len(yamlFiles))
	docResult := runDocChecks(yamlFiles, selectors, w, disabled)
	// Drop psa-labels findings whose missing labels are commented out
	// (rather than genuinely absent) in the app's base/ - see
	// filterCommentedPSAFindings for why this is scoped to exact,
	// verbatim-missing-label matches only.
	docResult.Findings = filterCommentedPSAFindings(docResult.Findings)

	if kyvernoEnabled {
		res.Sections = append(res.Sections, runKyvernoValidation(renderedOverlays, yamlFiles, opts.PolicyPath, log))
	}

	// NetworkAttachmentDefinition validation over every successfully-rendered
	// overlay. Structural checks always run (default-on, like every other
	// check in this phase); the OVN-Kubernetes-aware semantic tier is
	// additionally applied when Options.AssumeOpenShift is set, since an
	// OpenShift/OKD cluster's default CNI is OVN-Kubernetes - the same
	// assumption AssumeOpenShift already makes for the sync-options check.
	// The section is only emitted when a NAD is actually present in the
	// rendered chain (like the opt-in Kyverno section above); a changeset
	// with no NAD gets no section rather than an empty "all good" stub.
	if nadSection, present := runNADValidation(renderedOverlays, opts.AssumeOpenShift, log); present {
		res.Sections = append(res.Sections, nadSection)
	}

	// Merge and classify.
	allFindings := make([]check.Finding, 0, len(docResult.Findings)+len(overlayResult.Findings))
	allFindings = append(allFindings, docResult.Findings...)
	allFindings = append(allFindings, overlayResult.Findings...)
	changedSet := detectSourceFiles(changed)
	direct, indirect := finalizeCompliance(allFindings, changedSet)

	combinedCheck := check.Result{
		Findings: append(direct, indirect...),
		Exempted: append(docResult.Exempted, overlayResult.Exempted...),
	}
	res.Check = combinedCheck

	res.Blocking = len(direct) > 0 || ghostBlockingCount > 0

	if len(direct) > 0 {
		log.ErrorInSection("ResourceCompliance", "%d blocking finding(s)", len(direct))
	} else {
		log.Info("resource compliance: passed")
	}

	res.Sections = append(res.Sections, ComposeResourceComplianceSection(direct, indirect, combinedCheck.Exempted))
	tc.Record("Post-Build Validation", time.Since(postBuildStart), true)
}

// overlayRef pairs an overlay path with its cluster name.
type overlayRef struct {
	path, cluster string
}

// toIDSet converts a slice of IDs (from DisabledChecks or EnabledChecks)
// into a lookup set. Reading a missing key from a nil map is a safe
// zero-value (false) in Go, so callers can use the result directly without
// a nil check.
func toIDSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func filterYAML(files []string) []string {
	var out []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".yaml" || ext == ".yml" {
			if _, err := os.Stat(f); err == nil {
				out = append(out, f)
			}
		}
	}
	return out
}

// writeShellcheckExtractionReport runs both the Tekton-step and embedded-
// script shellcheck extractors over files, appends a human-readable report
// line per violation to sb (labelSuffix distinguishes direct vs. external
// findings in that report), and returns the total violation count.
func writeShellcheckExtractionReport(sb *strings.Builder, labelSuffix string, files []string) int {
	total := 0
	tektonResults, _ := shellcheck.RunTekton(files)
	for _, r := range tektonResults {
		for _, v := range r.Violations {
			fmt.Fprintf(sb, "%s:%d: [Tekton %s/%s]%s %s\n", v.File, v.Line, r.TaskName, r.StepName, labelSuffix, v.Message)
		}
		total += len(r.Violations)
	}
	embeddedResults, _ := shellcheck.RunEmbedded(files)
	for _, r := range embeddedResults {
		for _, v := range r.Violations {
			fmt.Fprintf(sb, "%s:%d: [%s/%s %s]%s %s\n", v.File, v.Line, r.ResourceKind, r.ResourceName, r.ContainerName, labelSuffix, v.Message)
		}
		total += len(r.Violations)
	}
	return total
}

// externalOverlayYAMLFiles returns every YAML file under every overlay
// affected by changed (per detectOverlaysForChanges) that was NOT itself
// part of changed - i.e. files pulled in only because a shared base/
// component the overlay depends on changed elsewhere. Findings extracted
// from these files are reported as non-blocking warnings (see the
// shellcheck lint step above), mirroring finalizeCompliance's identical
// direct/indirect split for doc/overlay check findings.
func externalOverlayYAMLFiles(changed []string) []string {
	changedSet := detectSourceFiles(changed)
	var out []string
	seen := map[string]bool{}
	for _, ov := range detectOverlaysForChanges(changed) {
		_ = filepath.WalkDir(ov.path, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr // filepath.WalkDir convention: skip entry, keep walking
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}
			clean := filepath.Clean(path)
			if changedSet[clean] || seen[clean] {
				return nil
			}
			seen[clean] = true
			out = append(out, path)
			return nil
		})
	}
	return out
}

// Workers returns the effective concurrency.
func Workers(opts Options) int {
	if opts.Concurrency > 0 {
		return opts.Concurrency
	}
	return runtime.NumCPU() * 2
}

// filterKubeconformExemptions drops files that match any check=kubeconform
// selector from selectors, reusing the same exempt.Evaluate path that doc
// and overlay checks use. A file is excluded when at least one selector
// matches — the caller logs nothing for exempted files; the exemption is
// intentionally silent in the kubeconform step (matching how
// IgnoreMissingSchemas-skipped resources appear in kubeconform output).
func filterKubeconformExemptions(files []string, selectors []exempt.Selector) []string {
	if len(selectors) == 0 {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		scalar := exempt.Scalar{File: f}
		if ok, _ := exempt.Evaluate("kubeconform", scalar, nil, selectors); ok {
			continue
		}
		out = append(out, f)
	}
	return out
}
