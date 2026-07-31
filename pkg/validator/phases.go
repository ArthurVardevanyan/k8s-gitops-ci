package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/config"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/csv"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/largefile"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/golangci"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/markdownlint"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/prettier"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/shellcheck"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/yamlsyntax"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// Step IDs for standalone (non-check-registry) lint/build steps that
// participate in the same generic enable/disable ID mechanism as
// check-registry checks. See stepEnabled and the Options doc comment.
const (
	stepGolangci = "golangci"
	stepAVP      = "avp"
	stepKyverno  = "kyverno"
)

// defaultOffSteps lists step/check IDs that are disabled unless explicitly
// present in Options.EnabledChecks. Every other ID defaults to enabled and
// is only turned off via Options.DisabledChecks. Kyverno defaults off
// because, unlike every other check in this repo, it has no generic default
// policy set an arbitrary org could reasonably run out of the box - an org
// must opt in and supply its own policies (see pkg/lint/kyverno).
var defaultOffSteps = map[string]bool{
	stepKyverno: true,
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

// runLintAndStaticChecks runs all linters and static checks, populating sections.
func runLintAndStaticChecks(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector) {
	phaseStart := time.Now()
	w := Workers(opts)
	_ = w

	disabled := toIDSet(opts.DisabledChecks)
	enabled := toIDSet(opts.EnabledChecks)

	log.Header("Linting")

	// ── linting ──────────────────────────────────────────────────────────────
	lintReports := map[string]string{}

	if mdOut, err := markdownlint.Run(changed); err == nil {
		lintReports["markdownlint"] = mdOut
		log.Info("markdownlint: passed")
	} else if !errors.Is(err, markdownlint.ErrCLINotFound) {
		lintReports["markdownlint"] = err.Error()
		log.ErrorInSection("Markdownlint", "markdownlint: %s", err)
	}

	if pOut, err := prettier.Run(changed, nil); err == nil {
		lintReports["prettier"] = pOut
		log.Info("prettier: passed")
	} else if !errors.Is(err, prettier.ErrCLINotFound) {
		lintReports["prettier"] = err.Error()
		log.ErrorInSection("Prettier", "prettier: %s", err)
	}

	if scViolations, _, scErr := shellcheck.Run(changed); scErr == nil {
		if len(scViolations) > 0 {
			var sb strings.Builder
			for _, v := range scViolations {
				fmt.Fprintf(&sb, "%s:%d: %s\n", v.File, v.Line, v.Message)
			}
			lintReports["shellcheck"] = sb.String()
			log.ErrorInSection("Shellcheck", "%d shellcheck violation(s)", len(scViolations))
		} else {
			log.Info("shellcheck: passed")
		}
	} else if !errors.Is(scErr, shellcheck.ErrCLINotFound) {
		lintReports["shellcheck"] = scErr.Error()
		log.ErrorInSection("Shellcheck", "shellcheck: %s", scErr)
	}

	if stepEnabled(stepGolangci, disabled, enabled) {
		if glOut, err := golangci.Run(changed); err != nil && !errors.Is(err, golangci.ErrCLINotFound) {
			lintReports["golangci"] = err.Error()
			log.ErrorInSection("Golangci", "golangci: %s", err)
		} else {
			lintReports["golangci"] = glOut
			log.Info("golangci-lint: passed")
		}
	}

	yamlFiles := changeset.FilterByExtension(changed, ".yaml", ".yml")
	kcOpts := kubeconform.DefaultOptions()
	if schemaDir, cleanup, err := kubeconform.ExtractSchemas(); err == nil {
		kcOpts.SchemaDir = schemaDir
		defer cleanup()
	}
	if kcRes, err := validateWithRenderedOverlays(yamlFiles, kcOpts); err == nil && kcRes != nil {
		if kcRes.Invalid > 0 || kcRes.Errors > 0 {
			lintReports["kubeconform"] = kcRes.Summary()
			log.ErrorInSection("Kubeconform", "%s", kcRes.Summary())
		} else {
			log.Info("kubeconform: passed")
		}
	}

	res.Sections = append(res.Sections, ComposeLintingSection(lintReports))
	tc.Record("Linting", time.Since(phaseStart))

	// ── static checks ────────────────────────────────────────────────────────
	staticStart := time.Now()
	log.Header("Static Checks")
	staticReports := map[string]string{}

	if violations := largefile.Check(changed, largefile.DefaultMaxSize, nil); len(violations) > 0 {
		var sb strings.Builder
		for _, v := range violations {
			sb.WriteString(v.String() + "\n")
		}
		staticReports["large-file"] = sb.String()
		log.ErrorInSection("LargeFile", "%d large file violation(s)", len(violations))
	} else {
		log.Info("large-file check: passed")
	}

	if yvs, _ := yamlsyntax.CheckFiles(changed); len(yvs) > 0 {
		var sb strings.Builder
		for _, v := range yvs {
			fmt.Fprintf(&sb, "%s: %s\n", v.File, v.Message)
		}
		staticReports["YAML-syntax"] = sb.String()
		log.ErrorInSection("YAMLSyntax", "%d YAML syntax error(s)", len(yvs))
	} else {
		log.Info("YAML-syntax check: passed")
	}

	if sorted, err := config.CheckSortOrder(); err == nil && len(sorted) > 0 {
		staticReports["config-sort"] = config.FormatUnsortedError(sorted)
		log.ErrorInSection("ConfigSort", "%d unsorted config file(s)", len(sorted))
	} else {
		log.Info("config-sort check: passed")
	}

	if mismatches, err := csv.CheckStartingCSVFolderMatch(changed); err == nil && len(mismatches) > 0 {
		staticReports["startingCSV"] = csv.FormatMismatches(mismatches)
		log.ErrorInSection("StartingCSV", "%d startingCSV mismatch(es)", len(mismatches))
	} else {
		log.Info("startingCSV check: passed")
	}

	res.Sections = append(res.Sections, ComposeStaticChecksSection(staticReports))
	tc.Record("Static Checks", time.Since(staticStart))
}

// runBuildAndPostBuild runs the registry-driven doc + overlay check engine.
func runBuildAndPostBuild(changed []string, opts Options, res *Result, log *logger.Logger, tc *TimingCollector) {
	buildStart := time.Now()
	log.Header("Build + Compliance")
	w := Workers(opts)

	// Resolve selectors from hook config (empty for now; org layer injects via Options).
	var selectors []exempt.Selector

	disabled := toIDSet(opts.DisabledChecks)

	// Doc engine over all changed YAML files.
	yamlFiles := filterYAML(changed)
	log.Info("running doc checks over %d YAML file(s)...", len(yamlFiles))
	docResult := runDocChecks(yamlFiles, selectors, w, disabled)

	// Overlay engine - overlays detected from changed files.
	overlays := detectOverlays(changed)
	log.Info("running overlay checks over %d overlay(s)...", len(overlays))
	var overlayResult check.Result
	if len(overlays) > 0 {
		for _, ov := range overlays {
			r := runOverlayChecks([]string{ov.path}, ov.cluster, selectors, w, disabled)
			overlayResult.Findings = append(overlayResult.Findings, r.Findings...)
			overlayResult.Exempted = append(overlayResult.Exempted, r.Exempted...)
		}
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
	res.Blocking = len(direct) > 0

	if res.Blocking {
		log.ErrorInSection("ResourceCompliance", "%d blocking finding(s)", len(direct))
	} else {
		log.Info("resource compliance: passed")
	}

	res.Sections = append(res.Sections, Section{
		Name: "Kustomize Build",
		Body: fmt.Sprintf("%d overlay(s) checked.", len(overlays)),
	})

	res.Sections = append(res.Sections, Section{
		Name: "Scaffold Validation",
		Body: "Scaffold checks complete.",
	})

	res.Sections = append(res.Sections, ComposeResourceComplianceSection(combinedCheck.Findings))
	tc.Record("Build+Compliance", time.Since(buildStart))
}

// overlayRef pairs an overlay path with its cluster name.
type overlayRef struct {
	path, cluster string
}

// detectOverlays heuristically finds overlay dirs from changed files.
// An "overlay" is any directory two levels under an "overlays/" segment
// (e.g. apps/myapp/overlays/mycluster).
func detectOverlays(files []string) []overlayRef {
	seen := map[string]bool{}
	var refs []overlayRef
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		for i, p := range parts {
			if p == "overlays" && i+1 < len(parts) {
				ovDir := filepath.Join(parts[:i+2]...)
				cluster := parts[i+1]
				if !seen[ovDir] {
					seen[ovDir] = true
					refs = append(refs, overlayRef{path: ovDir, cluster: cluster})
				}
				break
			}
		}
	}
	return refs
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

// Workers returns the effective concurrency.
func Workers(opts Options) int {
	if opts.Concurrency > 0 {
		return opts.Concurrency
	}
	return runtime.NumCPU() * 2
}
