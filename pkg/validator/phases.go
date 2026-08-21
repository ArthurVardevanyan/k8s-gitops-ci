package validator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/config"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
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
	stepMarkdownlint   = "markdownlint"
	stepPrettier       = "prettier"
	stepShellcheck     = "shellcheck"
	stepGolangci       = "golangci"
	stepKubeconform    = "kubeconform"
	stepAVP            = "avp"
	stepKyverno        = "kyverno"
	stepScaffoldReadme = "scaffold-readme"
	stepKustomizeFix   = "kustomize-fix"
)

// StepKyverno is the exported alias for the internal kyverno step ID, so
// external callers can reference validator.StepKyverno.
const StepKyverno = stepKyverno

// DefaultEnabledChecks is an enablement seam: when Options.EnabledChecks is
// empty, this slice is used as the default enabled set in the
// phase-enablement logic. Defaults to nil (no behavior change unless set).
var DefaultEnabledChecks []string

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
// resolveEnabledChecks returns the effective enabled-check ID list: the
// caller-supplied Options.EnabledChecks when non-empty, otherwise the
// DefaultEnabledChecks enablement seam (nil by default).
func resolveEnabledChecks(enabled []string) []string {
	if len(enabled) == 0 {
		return DefaultEnabledChecks
	}
	return enabled
}

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
	enabled := toIDSet(resolveEnabledChecks(opts.EnabledChecks))

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
	filtered := filterLargeFileExemptions(changed, earlySelectors)
	if violations := largefile.Check(filtered, largefile.DefaultMaxSize, largefile.DefaultIgnorePatterns); len(violations) > 0 {
		var sb strings.Builder
		for _, v := range violations {
			sb.WriteString(v.String() + "\n")
		}
		detail := strings.TrimRight(sb.String(), "\n")
		log.ErrorInSection("LargeFile", "%d large file violation(s)\n%s", len(violations), detail)
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
	if yvs, _ := yamlsyntax.CheckFiles(filterYAML(changed)); len(yvs) > 0 {
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

	// markdownlint, prettier, shellcheck, and golangci all hard-fail (a
	// blocking StatusError, not a graceful skip) when their underlying CLI
	// isn't installed - a missing lint tool is a broken toolchain, not a
	// "nothing to check" outcome, and silently passing CI in that state
	// hides real findings a properly-provisioned run would have caught
	// (see docs/CI.md's Linting phase section). Each is individually
	// gated behind its own step ID so an environment that genuinely can't
	// provision a given tool can opt out explicitly via
	// --disable-checks <name> instead of always failing.
	//
	// runCLILintOutcome is shared by every step below whose only two
	// failure modes are "the CLI errored" and "the CLI is missing" (i.e.
	// every wrapper here except shellcheck, whose relevance-check/PATH-
	// check ordering and multi-source (raw + extracted) reporting don't
	// fit this shape) - it turns run's result into a lintStepResult,
	// treating a missing CLI (errors.Is(err, notFoundErr)) as a
	// StatusError carrying notFoundNote rather than the raw ErrCLINotFound
	// text.
	runCLILintOutcome := func(sl *logger.ScopedLogger, elapsed func() time.Duration, displayName, passLabel string, run func() (string, error), notFoundErr error, notFoundNote string) lintStepResult {
		out, err := run()
		if err == nil {
			sl.Info("%s: passed (%s)", passLabel, elapsed().Round(time.Millisecond))
			return lintStepResult{status: StatusPassed}
		}
		if errors.Is(err, notFoundErr) {
			sl.ErrorInSection(displayName, "%s: %s", passLabel, err)
			return lintStepResult{status: StatusError, note: notFoundNote}
		}
		detail := out
		if detail == "" {
			detail = err.Error()
			sl.ErrorInSection(displayName, "%s: %s", passLabel, err)
		} else {
			// Surface the CLI's own per-file/line output immediately, not
			// just a bare error - otherwise this detail (which files/what
			// broke) only ever reaches the console via
			// pipeline.printFailedSectionDetail's single end-of-run pass,
			// long after a slow Build YAML/Post-Build Validation phase
			// finishes on a large changeset. Matches kubeconform's existing
			// ErrorInSection call, which already logs its full multi-line
			// Summary() immediately rather than just a count.
			sl.ErrorInSection(displayName, "%s: %s\n%s", passLabel, err, strings.TrimRight(detail, "\n"))
		}
		return lintStepResult{report: detail, status: StatusError}
	}

	if stepEnabled(stepMarkdownlint, disabled, enabled) {
		runLintStep("markdownlint", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			// FilterMarkdown so GitHub issue/PR templates are
			// excluded: they don't follow markdownlint's heading conventions
			// (the first heading may be any level, not a single top-level "#").
			md := markdownlint.FilterMarkdown(changed)
			if len(md) == 0 {
				sl.Info("markdownlint: no markdown files changed")
				return lintStepResult{status: StatusPassed, skipped: true, note: "No markdown files changed."}
			}
			return runCLILintOutcome(sl, elapsed, "Markdownlint", "markdownlint", func() (string, error) {
				return markdownlint.Run(md)
			}, markdownlint.ErrCLINotFound, "markdownlint not found in PATH.")
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "markdownlint", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	if stepEnabled(stepPrettier, disabled, enabled) {
		runLintStep("prettier", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			return runCLILintOutcome(sl, elapsed, "Prettier", "prettier", func() (string, error) {
				return prettier.Run(changed, nil)
			}, prettier.ErrCLINotFound, "prettier not found in PATH.")
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "prettier", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	if stepEnabled(stepShellcheck, disabled, enabled) {
		runLintStep("shellcheck", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			// yamlChanged feeds both the "any relevant files at all" short-
			// circuit below and the direct/blocking Tekton+embedded-script
			// extraction pass further down - computed once and reused
			// rather than calling filterYAML(changed) twice. The relevance
			// check runs before the PATH check (unlike a naive ordering)
			// so a changeset with no shell-related content at all still
			// skips cleanly even when shellcheck isn't installed.
			yamlChanged := filterYAML(changed)
			shellChanged := excludeInvalidTestdata(changed)
			if len(shellcheck.FilterShellScripts(shellChanged)) == 0 && len(yamlChanged) == 0 {
				sl.Info("shellcheck: no shell files changed")
				return lintStepResult{status: StatusPassed, skipped: true, note: "No shell files changed."}
			}

			if _, err := exec.LookPath("shellcheck"); err != nil {
				sl.ErrorInSection("Shellcheck", "shellcheck: %s", err)
				return lintStepResult{status: StatusError, note: "shellcheck not found in PATH."}
			}

			var sb strings.Builder
			blocking, warning := 0, 0

			// Raw shell script files: always direct/blocking - they're
			// literally files in this changeset's diff, so any finding here
			// is the author's own responsibility to fix.
			scViolations, _, scErr := shellcheck.Run(shellChanged)
			if scErr != nil {
				sl.ErrorInSection("Shellcheck", "shellcheck: %s", scErr)
				return lintStepResult{report: scErr.Error(), status: StatusError}
			}
			for _, v := range scViolations {
				fmt.Fprintf(&sb, "%s:%d: %s\n", v.File, v.Line, v.Message)
			}
			blocking += len(scViolations)

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
				sl.ErrorInSection("Shellcheck", "%d shellcheck violation(s)\n%s", blocking, strings.TrimRight(sb.String(), "\n"))
				return lintStepResult{report: sb.String(), status: StatusError}
			}
			if warning > 0 {
				sl.Info("shellcheck: passed (%d external/non-blocking warning(s)) (%s)", warning, elapsed().Round(time.Millisecond))
				return lintStepResult{report: sb.String(), status: StatusPassed, note: fmt.Sprintf("%d external warning(s) in overlay files not directly changed (non-blocking).", warning)}
			}
			sl.Info("shellcheck: passed (%s)", elapsed().Round(time.Millisecond))
			return lintStepResult{status: StatusPassed}
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "shellcheck", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	if stepEnabled(stepGolangci, disabled, enabled) {
		runLintStep("golangci", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			if len(golangci.FilterGo(changed)) == 0 {
				sl.Info("golangci: no Go files changed")
				return lintStepResult{status: StatusPassed, skipped: true, note: "No Go files changed."}
			}
			return runCLILintOutcome(sl, elapsed, "Golangci", "golangci-lint", func() (string, error) {
				return golangci.Run(changed)
			}, golangci.ErrCLINotFound, "golangci-lint not found in PATH.")
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "golangci", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	if stepEnabled(stepKubeconform, disabled, enabled) {
		runLintStep("kubeconform", func(sl *logger.ScopedLogger, elapsed func() time.Duration) lintStepResult {
			yamlFiles := changeset.FilterByExtension(changed, ".yaml", ".yml")
			// Scaffold configs/templates are the scaffolding CLI's own input
			// files, not Kubernetes manifests (no apiVersion/kind), so exclude
			// them from schema validation - otherwise every changed
			// <ScaffoldDir>/configs/*.yaml trips a "missing 'kind' key" error.
			yamlFiles = excludeScaffoldArtifacts(yamlFiles)
			yamlFiles = excludeInvalidTestdata(yamlFiles)
			yamlFiles = excludeKnownNonManifestFiles(yamlFiles)
			yamlFiles = filterKubeconformExemptions(yamlFiles, earlySelectors)
			kcOpts, cleanup := kubeconformSchemaOpts(opts)
			defer cleanup()
			// Changed files that participate in a scoped overlay's build chain
			// are schema-validated from the authoritative rendered output in the
			// post-build "Kubeconform (Rendered)" pass (see
			// runBuildAndPostBuild). Excluding them here keeps each changed
			// manifest validated by exactly one pass, and avoids a misleading
			// raw pass tripping over unresolved AVP placeholders that the
			// rendered output resolves. This exclusion only applies when the
			// rendered pass will actually run: under --lint-only the Build/
			// Post-Build phase (and therefore the rendered pass) is skipped, so
			// removing these files here would drop their kubeconform coverage
			// entirely. In lint-only mode every changed manifest file is
			// validated raw instead.
			if !opts.LintOnly {
				scoped := detectOverlaysForChanges(changed)
				yamlFiles = filesNotCovered(yamlFiles, coverByScopedOverlays(scoped, yamlFiles))
			}
			if kcRes, err := kubeconform.ValidateFiles(yamlFiles, kcOpts); err == nil && kcRes != nil {
				if kcRes.Invalid > 0 || kcRes.Errors > 0 {
					sl.ErrorInSection("Kubeconform", "%s", kcRes.Summary())
					return lintStepResult{report: kcRes.Summary(), status: StatusError}
				}
				if len(kcRes.SkippedNonManifest) > 0 {
					note := formatSkippedNonManifest(kcRes.SkippedNonManifest)
					sl.Info("kubeconform: passed (%s); %s", elapsed().Round(time.Millisecond), note)
					return lintStepResult{status: StatusInfo, note: note}
				}
				sl.Info("kubeconform: passed (%s)", elapsed().Round(time.Millisecond))
			}
			return lintStepResult{status: StatusPassed}
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "kubeconform", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	lintWg.Wait()

	res.Sections = append(res.Sections, ComposeLintingSection(lintOutcomes, lintReports, opts.Providers.BinaryName()))
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
			return lintStepResult{report: config.FormatUnsortedError(sorted, opts.Providers.BinaryName()), status: StatusError}
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

	res.Sections = append(res.Sections, ComposeStaticChecksSection(staticOutcomes, staticReports, opts.Providers.BinaryName()))
	tc.Record("Static Checks", time.Since(staticStart), true)
}

// runBuildAndPostBuild runs the registry-driven doc + overlay check engine,
// split into two console phases - "Build YAML" (kustomize builds, hooks,
// scaffold validation) and "Post-Build Validation" (doc/overlay compliance
// checks, Kyverno, NAD) - matching a downstream fork's equivalent phase
// breakdown. The split is safe because neither phase's work depends on the
// other's *findings* as input: the doc engine's raw pass (runDocChecks)
// reads raw changed YAML files, its rendered pass (runDocChecksRendered)
// consumes the same rendered overlays already collected by the build loop
// (renderedOverlays) - so it must run after Build YAML, but takes no build
// *findings* as input - and the overlay-check pass (runOverlayChecks) is run
// in the same worker-pool loop as the actual build (buildOverlayWithHooks)
// since both need the same per-overlay worker-pool parallelism and neither
// result feeds the other within a single iteration - see runOverlayChecks/
// buildOverlayWithHooks's respective doc comments.
func runBuildAndPostBuild(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector, earlySelectors []exempt.Selector) {
	w := Workers(opts)

	// Overlays/apps are resolved up front (rather than after the doc/
	// overlay check loops, as before) because both this run's exemption
	// selectors and its hook execution need the app list first: each app's
	// test.sh is resolved exactly once here, its EXEMPTIONS=(...) merged
	// into selectors below, and the same *hook.Config reused for every one
	// of that app's overlay builds plus its single POST_VALIDATE_HOOK call.
	var overlays []overlayRef
	if opts.FullScan {
		overlays = detectAllOverlays()
	} else {
		overlays = detectOverlaysForChanges(changed)
	}
	apps := uniqueApps(overlays)

	disabled := toIDSet(opts.DisabledChecks)
	enabled := toIDSet(resolveEnabledChecks(opts.EnabledChecks))

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
	selectors := append(append(builtinExemptSelectors(), hookSelectors...), earlySelectors...)

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
	scaffoldStart := time.Now()
	scaffoldResult := runScaffoldValidation(opts, apps, changed, log)
	scaffoldDur := time.Since(scaffoldStart)
	tc.RecordStep("Build YAML", "scaffold-validation", scaffoldDur)
	log.Debug("scaffold-validation: %s", scaffoldDur.Round(time.Millisecond))

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
					buildErr, pre, post, rendered := buildOverlayWithHooks(ov, hookCfgs[app], appStrategies[app], log)
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

	for _, err := range runAppPostValidateHooks(apps, hookCfgs, hookResults, log) {
		buildErrs = append(buildErrs, err)
		log.ErrorInSection("Hooks", "%s", err)
	}
	for _, err := range hookExemptErrs {
		buildErrs = append(buildErrs, err)
		log.ErrorInSection("Hooks", "%s", err)
	}
	for _, err := range hookMisdeclaredErrors(hookCfgs) {
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

	// kustomize.CheckFix shells out to the real `kustomize` CLI (see
	// pkg/kustomize's package doc comment for why) - unlike every
	// pkg/lint/* wrapper's own missing-binary handling (skip gracefully),
	// a missing kustomize binary is treated as a hard failure here: it's
	// a core, expected dependency for this pipeline, not an optional
	// best-effort tool, and a run that couldn't actually check
	// kustomization.yaml files should never silently report a clean bill
	// of health it never verified. composeKustomizeFixChild renders
	// either outcome (findings, or the check itself failing) as a
	// StatusError sub-dropdown, so it must also be logged as a real
	// failure here, otherwise a run with nothing else wrong exits
	// 0/green despite the report showing a hard ❌ (see Result.Failed's
	// doc comment).
	//
	// Gated behind stepKustomizeFix (default-on, like every check with no
	// org-specific compatibility concern - unlike kyverno/scaffold-readme)
	// purely so tests exercising unrelated behavior with a deliberately
	// minimal/non-canonical kustomization.yaml fixture (this repo has
	// many) can opt out via --disable-checks kustomize-fix instead of
	// needing every such fixture to exactly match the real kustomize
	// binary's canonical output - production runs are unaffected, since
	// nothing disables it by default.
	var fixNeeded []string
	var fixCheckErr error
	kustomizeFixEnabled := stepEnabled(stepKustomizeFix, disabled, enabled)
	if kustomizeFixEnabled {
		fixStart := time.Now()
		fixNeeded, fixCheckErr = kustomize.CheckFix(changed)
		fixDur := time.Since(fixStart)
		tc.RecordStep("Build YAML", "kustomize-fix", fixDur)
		log.Debug("kustomize-fix: %s", fixDur.Round(time.Millisecond))
		switch {
		case fixCheckErr != nil:
			log.ErrorInSection("KustomizeBuild", "kustomize fix check: %v", fixCheckErr)
		case len(fixNeeded) > 0:
			log.ErrorInSection("KustomizeBuild", "%d file(s) need `kustomize edit fix --vars`", len(fixNeeded))
		}
	}
	hookTable := buildHookTable(apps, hookCfgs, hookResults)
	hookFailed := anyHookFailed(hookResults)
	// addedFiles feeds ghostpatch.ClassifyOverlay's "is this
	// kustomization.yaml itself new" check (a ghost patch on a brand-new
	// overlay is a warning, not this PR's fault to have introduced against
	// existing history) - an error resolving it (e.g. no git history
	// available) degrades to "no added files", matching this phase's
	// existing tolerant-of-git-failure pattern for changeset-resolution
	// errors specifically (unlike kustomize.CheckFix above, which is a
	// hard failure, not tolerated).
	addedFilesStart := time.Now()
	addedFiles, _ := changeset.GetAddedFiles(changeset.Options{BaseRef: opts.BaseRef, PR: opts.PR, RepoURL: opts.RepoURL})
	addedFilesDur := time.Since(addedFilesStart)
	tc.RecordStep("Build YAML", "added-files", addedFilesDur)
	log.Debug("added-files: %s", addedFilesDur.Round(time.Millisecond))
	ghostStart := time.Now()
	ghostTable, ghostBlockingCount := buildGhostTable(renderedOverlays, changed, addedFiles)
	ghostDur := time.Since(ghostStart)
	tc.RecordStep("Build YAML", "ghost-patch", ghostDur)
	log.Debug("ghost-patch: %s", ghostDur.Round(time.Millisecond))
	res.Sections = append(res.Sections, ComposeKustomizeBuildSection(len(overlays), buildErrs, hookTable, hookFailed, fixNeeded, fixCheckErr, kustomizeFixEnabled, ghostTable, ghostBlockingCount, opts.Providers.BinaryName()))
	if ghostBlockingCount > 0 {
		log.ErrorInSection("KustomizeBuild", "%d blocking ghost patch(es)", ghostBlockingCount)
	}

	res.Sections = append(res.Sections, ComposeScaffoldValidationSection(
		strings.Join(scaffoldResult.DriftLines, "\n"),
		scaffoldResult.ExecErrors,
		flattenSkippedClusters(scaffoldResult.SkippedClusters),
		strings.Join(scaffoldResult.PreExistingDriftLines, "\n"),
		flattenDisabledClusters(scaffoldResult.DisabledClusters),
	))
	res.Sections = append(res.Sections, ComposeDriftProtectionSection(findUnprotectedApps(changed)))
	buildDur := time.Since(buildStart)
	tc.Record("Build YAML", buildDur, len(overlays) > 1)
	// Phase-summary line matching Setup/PR Validation/YAML Syntax's own
	// "... passed (<duration>)" console line (see those phases' Info calls
	// above/below) - Build YAML previously had no such line, so its
	// duration was only visible in the end-of-run tc.Summary table.
	log.Info("build: %d overlay(s) rendered (%s)", len(overlays), buildDur.Round(time.Millisecond))

	// ── Post-Build Validation ────────────────────────────────────────────────
	postBuildStart := time.Now()
	log.Header("Post-Build Validation")

	// Doc engine over all changed YAML files. kyverno-test.yaml fixture
	// directories are excluded from compliance doc-checks (their paired
	// resources are deliberately non-compliant CLI test data, not real
	// workloads) - this doesn't affect kubeconform/Kyverno validation,
	// which run over `changed`/rendered overlays through their own paths.
	// Well-known non-manifest tooling configs (Taskfile.yml, .golangci.yml,
	// etc.) are also excluded - they never carry Kubernetes manifests, so
	// sentinel words in their descriptions (e.g. "placeholder") would
	// otherwise produce false-positive findings.
	yamlFiles := filterKyvernoTestFixtureDirs(
		excludeScaffoldArtifacts(
			excludeKnownNonManifestFiles(filterYAML(changed)),
		),
	)
	log.Info("running doc checks over %d YAML file(s)...", len(yamlFiles))

	// Dual-pass compliance (raw + rendered). Render-sensitive checks (see
	// check.RenderSensitive) are authoritative on the kustomize/AVP-rendered
	// overlay stream, so a value injected/replaced by a base+overlay+component
	// merge (e.g. `image: <PATCHED_BY_KUSTOMIZE>` replaced by an overlay
	// `images:`/JSON-patch) is judged on its final rendered result rather
	// than the intermediate raw fragment. renderedFiles is the set of raw
	// source files that participate in at least one successfully-rendered
	// overlay: runDocChecks skips render-sensitive checks for those (they're
	// covered by the rendered pass) but still runs them over any file NOT in
	// a rendered overlay (a brand-new component not yet wired into any
	// kustomization.yaml), so nothing is silently skipped.
	renderedFiles := filesCoveredByRenderedOverlays(renderedOverlays, yamlFiles)
	docResult := runDocChecks(yamlFiles, renderedFiles, selectors, w, disabled)
	renderedResult := runDocChecksRendered(renderedOverlays, selectors, w, disabled)
	docResult.Findings = append(docResult.Findings, renderedResult.Findings...)
	docResult.Exempted = append(docResult.Exempted, renderedResult.Exempted...)
	// Drop psa-labels findings whose missing labels are commented out
	// (rather than genuinely absent) in the app's base/ - see
	// filterCommentedPSAFindings for why this is scoped to exact,
	// verbatim-missing-label matches only.
	docResult.Findings = filterCommentedPSAFindings(docResult.Findings)

	if kyvernoEnabled {
		res.Sections = append(res.Sections, runKyvernoValidation(renderedOverlays, yamlFiles, opts.PolicyPath, log))
	}

	// Schema-validation over the AVP/Helm-rendered overlay output (the same
	// bytes Kyverno/NAD consume above). This is the authoritative pass for
	// changed manifests that live inside an overlay - the raw Linting
	// "Kubeconform" sub-check excludes those files (see runLintAndStaticChecks)
	// so each manifest is schema-validated exactly once, and here we validate
	// what actually deploys (AVP placeholders resolved, Helm charts rendered)
	// rather than raw source. Only emitted when at least one overlay rendered.
	if len(renderedOverlays) > 0 {
		kcOpts, kcCleanup := kubeconformSchemaOpts(opts)
		renderedKc := validateRenderedOverlays(renderedOverlays, kcOpts, Workers(opts))
		kcCleanup()
		// Always visible on the console (even on a clean pass - the report
		// section for this pass is only printed when it fails), mirroring how
		// the raw kubeconform "passed" line is always shown.
		log.Info("kubeconform (rendered): %s", renderedKc.Summary())
		res.Sections = append(res.Sections, ComposeKubeconformRenderedSection(renderedKc))
	}

	// NetworkAttachmentDefinition validation over every successfully-rendered
	// overlay. Validation dispatches on each NAD's declared CNI type: OVN
	// NADs (type ovn-k8s-cni-overlay) get OVN-Kubernetes' semantic rules,
	// non-OVN NADs (macvlan, bridge, SR-IOV, ...) get structural gates plus
	// advisory-only warnings. This is independent of Options.AssumeOpenShift -
	// the type field is self-describing, so no global OpenShift assumption is
	// needed (that flag still governs the sync-options check). The section is
	// only emitted when a NAD is actually present in the rendered chain (like
	// the opt-in Kyverno section above); a changeset with no NAD gets no
	// section rather than an empty "all good" stub.
	if nadSection, present := runNADValidation(renderedOverlays, log); present {
		res.Sections = append(res.Sections, nadSection)
	}

	// Merge and classify.
	allFindings := make([]check.Finding, 0, len(docResult.Findings)+len(overlayResult.Findings))
	allFindings = append(allFindings, docResult.Findings...)
	allFindings = append(allFindings, overlayResult.Findings...)

	// Resource-level blocking/warning classification. Uses the attribution
	// context (built from the PR's changed files) so a finding is blocking
	// only when its specific resource (Kind/Name) was directly modified in a
	// source file that feeds this overlay - not when an entirely unrelated
	// overlay kustomization.yaml was touched (see compliance_attribution.go).
	attrCtx := buildAttributionCtx(changed, apps)
	blockingByCheck, nonblockingByCheck := classifyResourceCompliance(allFindings, attrCtx)

	var directTotal, indirectTotal int
	combinedBlocking := make([]check.Finding, 0, len(allFindings))
	combinedNonblocking := make([]check.Finding, 0, len(allFindings))
	for _, id := range complianceCheckOrder {
		if d, ok := blockingByCheck[id]; ok {
			combinedBlocking = append(combinedBlocking, d...)
			directTotal += len(d)
		}
		if nd, ok := nonblockingByCheck[id]; ok {
			combinedNonblocking = append(combinedNonblocking, nd...)
			indirectTotal += len(nd)
		}
	}
	// combinedBlocking / combinedNonblocking are ALREADY the final
	// resource-level split from classifyResourceCompliance (which honors
	// ForcedDirect, the resource-level attribution model, and the
	// no-TableSpec fallback). Re-running them through the file-based
	// finalizeCompliance here used to UNDO that: a resource-level-blocking
	// finding whose File is an overlay dir (not a literal changed file) was
	// silently demoted back to a warning. Keep the split, only promoting any
	// stray ForcedDirect finding that slipped into the non-blocking set.
	direct := combinedBlocking
	var indirect []check.Finding
	for _, f := range combinedNonblocking {
		if f.ForcedDirect {
			direct = append(direct, f)
		} else {
			indirect = append(indirect, f)
		}
	}
	// Re-sort by complianceCheckOrder.
	sort.SliceStable(direct, func(i, j int) bool {
		return indexOfComplianceCheck(direct[i].CheckID) < indexOfComplianceCheck(direct[j].CheckID)
	})
	sort.SliceStable(indirect, func(i, j int) bool {
		return indexOfComplianceCheck(indirect[i].CheckID) < indexOfComplianceCheck(indirect[j].CheckID)
	})

	combinedCheck := check.Result{
		Findings: append(direct, indirect...),
		Exempted: append(docResult.Exempted, overlayResult.Exempted...),
	}
	res.Check = combinedCheck

	res.Blocking = len(direct) > 0 || ghostBlockingCount > 0

	// Per-check console lines: blocking
	// sub-checks log an error (fail the run), non-blocking sub-checks log a
	// warning (surfaced inline and counted in the summary's "Warnings: N"
	// line, but non-blocking). Counts are DEDUPED to match the PR-comment
	// sub-section headings - a base/component resource flagged across many
	// overlays is one finding, not one per overlay.
	unionByCheck := make(map[string][]check.Finding, len(blockingByCheck)+len(nonblockingByCheck))
	for id, fs := range blockingByCheck {
		unionByCheck[id] = append(unionByCheck[id], fs...)
	}
	for id, fs := range nonblockingByCheck {
		unionByCheck[id] = append(unionByCheck[id], fs...)
	}
	anyFinding := false
	for _, id := range orderedComplianceIDs(unionByCheck) {
		if b := blockingByCheck[id]; len(b) > 0 {
			anyFinding = true
			log.ErrorInSection("ResourceCompliance", "%s: %d finding(s) in directly changed file(s)",
				complianceTitle(id), complianceRowCount(id, b))
		}
		if w := nonblockingByCheck[id]; len(w) > 0 {
			anyFinding = true
			log.Warn("%s: %d finding(s) (non-blocking, pre-existing)",
				complianceTitle(id), complianceRowCount(id, w))
		}
	}
	if !anyFinding {
		log.Info("resource compliance: passed")
	}

	res.Sections = append(res.Sections, ComposeResourceComplianceSection(direct, indirect, combinedCheck.Exempted, attrCtx.changedKeys))
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

// excludeScaffoldArtifacts drops scaffold-tool config/template files (see
// convention.IsScaffoldArtifact) from a file list - they are not Kubernetes
// manifests, so manifest validators must not attempt to schema-check them.
func excludeScaffoldArtifacts(files []string) []string {
	var out []string
	for _, f := range files {
		if convention.IsScaffoldArtifact(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// isInvalidTestdata reports whether f lives under a `testdata/invalid/`
// directory. By repo convention, deliberately-malformed fixtures (inputs a
// linter/validator is expected to reject) live in a `testdata/invalid/`
// subfolder; valid/"good" fixtures sit directly under `testdata/`. Only the
// `invalid/` subfolder is excluded from linting - e.g. a vendored Go module
// checked into the repo carries the upstream's bad-by-design shellcheck /
// kubeconform fixtures there. `testdata` itself is a Go-toolchain-reserved
// directory name (ignored by the go tool).
func isInvalidTestdata(f string) bool {
	s := filepath.ToSlash(f)
	return strings.HasPrefix(s, "testdata/invalid/") || strings.Contains(s, "/testdata/invalid/")
}

// excludeInvalidTestdata drops files under any `testdata/invalid/` directory
// (see isInvalidTestdata).
func excludeInvalidTestdata(files []string) []string {
	var out []string
	for _, f := range files {
		if isInvalidTestdata(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// excludeKnownNonManifestFiles drops well-known non-Kubernetes tooling
// config files (Taskfile.yml, .golangci.yml, ...) from kubeconform's input
// set - see convention.KnownNonManifestFiles's doc comment for why this
// is a permanent, structural exclusion rather than a per-file
// EXEMPTIONS=(...) entry.
func excludeKnownNonManifestFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if convention.IsKnownNonManifestFile(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// maxSkippedNonManifestListed caps how many file paths
// formatSkippedNonManifest lists inline before summarizing the remainder as
// "and N more", keeping the non-blocking ℹ️ sub-check note a single concise
// line in the unified PR comment.
const maxSkippedNonManifestListed = 5

// formatSkippedNonManifest renders the non-blocking note describing files
// the kubeconform step skipped because they carry no root apiVersion/kind
// (see kubeconform.IsManifestYAML) - e.g. an Ansible inventory or NMState
// config in a flat, non-app directory. Surfaced (never silent) so a
// genuinely header-less manifest that was skipped stays visible for a human
// to catch.
func formatSkippedNonManifest(files []string) string {
	listed := files
	extra := ""
	if len(listed) > maxSkippedNonManifestListed {
		extra = fmt.Sprintf(", and %d more", len(listed)-maxSkippedNonManifestListed)
		listed = listed[:maxSkippedNonManifestListed]
	}
	return fmt.Sprintf("Skipped %d non-manifest YAML file(s) (no apiVersion/kind): %s%s", len(files), strings.Join(listed, ", "), extra)
}

func filterYAML(files []string) []string {
	var out []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		// Scaffold templates are Go-templated source, frequently not valid
		// standalone YAML (unresolved {{ ... }}); never syntax/manifest-check
		// them as raw files.
		if convention.IsScaffoldTemplate(f) {
			continue
		}
		// Deliberately-invalid test fixtures (testdata/invalid/) are meant to
		// be rejected by a validator; never lint them.
		if isInvalidTestdata(f) {
			continue
		}
		if _, err := os.Stat(f); err == nil {
			out = append(out, f)
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

// filterLargeFileExemptions drops files that match any check=large-file
// selector from selectors, reusing the same exempt.Evaluate path that doc
// and overlay checks use. A file is excluded when at least one selector
// matches — the caller logs nothing for exempted files; the exemption is
// silently skipped in the large-file step.
func filterLargeFileExemptions(files []string, selectors []exempt.Selector) []string {
	if len(selectors) == 0 {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		scalar := exempt.Scalar{File: f}
		if ok, _ := exempt.Evaluate(exempt.IDLargeFile, scalar, nil, selectors); ok {
			continue
		}
		out = append(out, f)
	}
	return out
}
