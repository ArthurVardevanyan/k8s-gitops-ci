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
//     table structural check - see docs/CI.md#scaffold-validation)
//     defaults off because, like kyverno, this generic core can't know
//     whether a given repo's `<!-- scaffold-status -->` table actually
//     matches the one-row-per-app-per-overlay shape this check expects -
//     an org already maintaining that table in a different shape/grouping
//     would otherwise see this newly-real check start blocking PRs with
//     false positives. An org confirms compatibility once, then opts in.
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
func runLintAndStaticChecks(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector) {
	phaseStart := time.Now()

	disabled := toIDSet(opts.DisabledChecks)
	enabled := toIDSet(opts.EnabledChecks)

	log.Header("Linting")

	// ── linting ──────────────────────────────────────────────────────────────
	lintReports := map[string]string{}
	var lintOutcomes []CheckOutcome
	var lintMu sync.Mutex
	var lintWg sync.WaitGroup

	// lintStepResult is what each linter closure returns: a failure report
	// (empty when it passed/was skipped) plus enough to build a CheckOutcome
	// so the Linting section can always render a full sub-check breakdown,
	// not just a flattened bullet that disappears once a check is clean.
	type lintStepResult struct {
		report  string
		status  SectionStatus
		skipped bool
		note    string
	}

	runLintStep := func(name string, fn func(sl *logger.ScopedLogger) lintStepResult) {
		lintWg.Add(1)
		go func() {
			defer lintWg.Done()
			start := time.Now()
			sl := log.Scope()
			r := fn(sl)
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

	runLintStep("markdownlint", func(sl *logger.ScopedLogger) lintStepResult {
		if mdOut, err := markdownlint.Run(changed); err == nil {
			sl.Info("markdownlint: passed")
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

	runLintStep("prettier", func(sl *logger.ScopedLogger) lintStepResult {
		if pOut, err := prettier.Run(changed, nil); err == nil {
			sl.Info("prettier: passed")
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

	runLintStep("shellcheck", func(sl *logger.ScopedLogger) lintStepResult {
		if _, err := exec.LookPath("shellcheck"); err != nil {
			sl.Debug("shellcheck: not found in PATH, skipping")
			return lintStepResult{status: StatusPassed, skipped: true, note: "shellcheck not found in PATH."}
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
		// findings in the Build+Compliance phase (a base/component
		// change ripples to every overlay that depends on it, and an
		// issue in a file the author never touched shouldn't block
		// their PR).
		yamlChanged := filterYAML(changed)
		blocking += writeShellcheckExtractionReport(&sb, "", yamlChanged)

		external := externalOverlayYAMLFiles(changed)
		warning += writeShellcheckExtractionReport(&sb, " (external)", external)

		if blocking > 0 {
			sl.ErrorInSection("Shellcheck", "%d shellcheck violation(s)", blocking)
			return lintStepResult{report: sb.String(), status: StatusError}
		}
		if warning > 0 {
			sl.Info("shellcheck: passed (%d external/non-blocking warning(s))", warning)
			return lintStepResult{report: sb.String(), status: StatusPassed, note: fmt.Sprintf("%d external warning(s) in overlay files not directly changed (non-blocking).", warning)}
		}
		sl.Info("shellcheck: passed")
		return lintStepResult{status: StatusPassed}
	})

	if stepEnabled(stepGolangci, disabled, enabled) {
		runLintStep("golangci", func(sl *logger.ScopedLogger) lintStepResult {
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
			sl.Info("golangci-lint: passed")
			return lintStepResult{status: StatusPassed}
		})
	} else {
		lintMu.Lock()
		lintOutcomes = append(lintOutcomes, CheckOutcome{Name: "golangci", Status: StatusPassed, Skipped: true, Note: "Disabled."})
		lintMu.Unlock()
	}

	runLintStep("kubeconform", func(sl *logger.ScopedLogger) lintStepResult {
		yamlFiles := changeset.FilterByExtension(changed, ".yaml", ".yml")
		kcOpts := kubeconform.DefaultOptions()
		if schemaDir, cleanup, err := kubeconform.ExtractSchemas(); err == nil {
			kcOpts.SchemaDir = schemaDir
			defer cleanup()
		}
		if kcRes, err := validateWithRenderedOverlays(yamlFiles, kcOpts); err == nil && kcRes != nil {
			if kcRes.Invalid > 0 || kcRes.Errors > 0 {
				sl.ErrorInSection("Kubeconform", "%s", kcRes.Summary())
				return lintStepResult{report: kcRes.Summary(), status: StatusError}
			}
			sl.Info("kubeconform: passed")
		}
		return lintStepResult{status: StatusPassed}
	})

	lintWg.Wait()

	res.Sections = append(res.Sections, ComposeLintingSection(lintOutcomes, lintReports))
	tc.Record("Linting", time.Since(phaseStart), true)

	// ── static checks ────────────────────────────────────────────────────────
	staticStart := time.Now()
	log.Header("Static Checks")
	staticReports := map[string]string{}
	var staticOutcomes []CheckOutcome
	var staticMu sync.Mutex
	var staticWg sync.WaitGroup

	runStaticStep := func(name string, fn func(sl *logger.ScopedLogger) lintStepResult) {
		staticWg.Add(1)
		go func() {
			defer staticWg.Done()
			start := time.Now()
			sl := log.Scope()
			r := fn(sl)
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

	runStaticStep("large-file", func(sl *logger.ScopedLogger) lintStepResult {
		if violations := largefile.Check(changed, largefile.DefaultMaxSize, nil); len(violations) > 0 {
			var sb strings.Builder
			for _, v := range violations {
				sb.WriteString(v.String() + "\n")
			}
			sl.ErrorInSection("LargeFile", "%d large file violation(s)", len(violations))
			return lintStepResult{report: sb.String(), status: StatusError}
		}
		sl.Info("large-file check: passed")
		return lintStepResult{status: StatusPassed}
	})

	runStaticStep("YAML-syntax", func(sl *logger.ScopedLogger) lintStepResult {
		if yvs, _ := yamlsyntax.CheckFiles(changed); len(yvs) > 0 {
			var sb strings.Builder
			for _, v := range yvs {
				fmt.Fprintf(&sb, "%s: %s\n", v.File, v.Message)
			}
			sl.ErrorInSection("YAMLSyntax", "%d YAML syntax error(s)", len(yvs))
			return lintStepResult{report: sb.String(), status: StatusError}
		}
		sl.Info("YAML-syntax check: passed")
		return lintStepResult{status: StatusPassed}
	})

	runStaticStep("config-sort", func(sl *logger.ScopedLogger) lintStepResult {
		if sorted, err := config.CheckSortOrder(); err == nil && len(sorted) > 0 {
			sl.ErrorInSection("ConfigSort", "%d unsorted config file(s)", len(sorted))
			return lintStepResult{report: config.FormatUnsortedError(sorted), status: StatusError}
		}
		sl.Info("config-sort check: passed")
		return lintStepResult{status: StatusPassed}
	})

	runStaticStep("startingCSV", func(sl *logger.ScopedLogger) lintStepResult {
		if mismatches, err := csv.CheckStartingCSVFolderMatch(changed); err == nil && len(mismatches) > 0 {
			sl.ErrorInSection("StartingCSV", "%d startingCSV mismatch(es)", len(mismatches))
			return lintStepResult{report: csv.FormatMismatches(mismatches), status: StatusError}
		}
		sl.Info("startingCSV check: passed")
		return lintStepResult{status: StatusPassed}
	})

	staticWg.Wait()

	res.Sections = append(res.Sections, ComposeStaticChecksSection(staticOutcomes, staticReports))
	tc.Record("Static Checks", time.Since(staticStart), true)
}

// runBuildAndPostBuild runs the registry-driven doc + overlay check engine.
func runBuildAndPostBuild(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector) {
	buildStart := time.Now()
	log.Header("Build + Compliance")
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

	// Doc engine over all changed YAML files.
	yamlFiles := filterYAML(changed)
	log.Info("running doc checks over %d YAML file(s)...", len(yamlFiles))
	docResult := runDocChecks(yamlFiles, selectors, w, disabled)

	// Overlay engine - overlays detected from changed files. Each overlay is
	// independent (its own checks over its own path/cluster), so fan them out
	// across a bounded worker pool instead of one-at-a-time; this mirrors the
	// job-queue pattern runDocChecks/runOverlayChecks already use internally
	// for per-file/per-overlay checks, one level up.
	log.Info("running overlay checks over %d overlay(s)...", len(overlays))
	kyvernoEnabled := stepEnabled(stepKyverno, disabled, enabled)
	var overlayResult check.Result
	buildErrs := make([]string, 0, len(overlays))
	hookResults := make(map[string]*appHookResult, len(apps))
	for _, app := range apps {
		hookResults[app] = &appHookResult{}
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
					tc.RecordStep("Build+Compliance", ov.path, time.Since(ovStart))
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

	if kyvernoEnabled {
		res.Sections = append(res.Sections, runKyvernoValidation(renderedOverlays, log))
	}

	// NetworkAttachmentDefinition validation over every successfully-rendered
	// overlay. Structural checks always run (default-on, like every other
	// check in this phase); the OVN-Kubernetes-aware semantic tier is
	// additionally applied when Options.AssumeOpenShift is set, since an
	// OpenShift/OKD cluster's default CNI is OVN-Kubernetes - the same
	// assumption AssumeOpenShift already makes for the sync-options check.
	res.Sections = append(res.Sections, runNADValidation(renderedOverlays, opts.AssumeOpenShift, log))

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
	res.Blocking = len(direct) > 0

	if res.Blocking {
		log.ErrorInSection("ResourceCompliance", "%d blocking finding(s)", len(direct))
	} else {
		log.Info("resource compliance: passed")
	}

	fixNeeded, _ := kustomize.CheckFix(changed)
	hookTable := buildHookTable(apps, hookCfgs, hookResults)
	ghostTable := buildGhostTable(apps)
	res.Sections = append(res.Sections, ComposeKustomizeBuildSection(len(overlays), buildErrs, hookTable, fixNeeded, ghostTable))

	scaffoldResult := runScaffoldValidation(opts, apps, changed, log)
	driftLines := scaffoldResult.DriftLines
	if stepEnabled(stepScaffoldReadme, disabled, enabled) {
		if readmeCurrent, readmeDiff := scaffold.CheckReadmeStatus(); !readmeCurrent {
			driftLines = append(driftLines, readmeDiff)
			log.ErrorInSection("Scaffold", "%s", readmeDiff)
		}
	}
	res.Sections = append(res.Sections, ComposeScaffoldValidationSection(strings.Join(driftLines, "\n"), scaffoldResult.ExecErrors, nil))

	res.Sections = append(res.Sections, ComposeResourceComplianceSection(direct, indirect, combinedCheck.Exempted))
	tc.Record("Build+Compliance", time.Since(buildStart), len(overlays) > 1)
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
