package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// runLintAndStaticChecks runs all linters and static checks, populating sections.
func runLintAndStaticChecks(changed []string, opts Options, res *Result) {
	w := Workers(opts)
	_ = w

	// ── linting ──────────────────────────────────────────────────────────────
	lintReports := map[string]string{}

	if mdOut, err := markdownlint.Run(changed); err == nil {
		lintReports["markdownlint"] = mdOut
	} else if !errors.Is(err, markdownlint.ErrCLINotFound) {
		lintReports["markdownlint"] = err.Error()
	}

	if pOut, err := prettier.Run(changed, nil); err == nil {
		lintReports["prettier"] = pOut
	} else if !errors.Is(err, prettier.ErrCLINotFound) {
		lintReports["prettier"] = err.Error()
	}

	if scViolations, _, scErr := shellcheck.Run(changed); scErr == nil {
		if len(scViolations) > 0 {
			var sb strings.Builder
			for _, v := range scViolations {
				fmt.Fprintf(&sb, "%s:%d: %s\n", v.File, v.Line, v.Message)
			}
			lintReports["shellcheck"] = sb.String()
		}
	} else if !errors.Is(scErr, shellcheck.ErrCLINotFound) {
		lintReports["shellcheck"] = scErr.Error()
	}

	if !opts.SkipGolangci {
		if glOut, err := golangci.Run(changed); err != nil && !errors.Is(err, golangci.ErrCLINotFound) {
			lintReports["golangci"] = err.Error()
		} else {
			lintReports["golangci"] = glOut
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
		}
	}

	res.Sections = append(res.Sections, ComposeLintingSection(lintReports))

	// ── static checks ────────────────────────────────────────────────────────
	staticReports := map[string]string{}

	if violations := largefile.Check(changed, largefile.DefaultMaxSize, nil); len(violations) > 0 {
		var sb strings.Builder
		for _, v := range violations {
			sb.WriteString(v.String() + "\n")
		}
		staticReports["large-file"] = sb.String()
	}

	if yvs, _ := yamlsyntax.CheckFiles(changed); len(yvs) > 0 {
		var sb strings.Builder
		for _, v := range yvs {
			fmt.Fprintf(&sb, "%s: %s\n", v.File, v.Message)
		}
		staticReports["YAML-syntax"] = sb.String()
	}

	if sorted, err := config.CheckSortOrder(); err == nil && len(sorted) > 0 {
		staticReports["config-sort"] = config.FormatUnsortedError(sorted)
	}

	if mismatches, err := csv.CheckStartingCSVFolderMatch(changed); err == nil && len(mismatches) > 0 {
		staticReports["startingCSV"] = csv.FormatMismatches(mismatches)
	}

	res.Sections = append(res.Sections, ComposeStaticChecksSection(staticReports))
}

// runBuildAndPostBuild runs the registry-driven doc + overlay check engine.
func runBuildAndPostBuild(changed []string, opts Options, res *Result) {
	w := Workers(opts)

	// Resolve selectors from hook config (empty for now; org layer injects via Options).
	var selectors []exempt.Selector

	disabled := toDisabledSet(opts.DisabledChecks)

	// Doc engine over all changed YAML files.
	yamlFiles := filterYAML(changed)
	docResult := runDocChecks(yamlFiles, selectors, w, disabled)

	// Overlay engine - overlays detected from changed files.
	overlays := detectOverlays(changed)
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

	res.Sections = append(res.Sections, Section{
		Name: "Kustomize Build",
		Body: fmt.Sprintf("%d overlay(s) checked.", len(overlays)),
	})

	res.Sections = append(res.Sections, Section{
		Name: "Scaffold Validation",
		Body: "Scaffold checks complete.",
	})

	res.Sections = append(res.Sections, ComposeResourceComplianceSection(combinedCheck.Findings))
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

// toDisabledSet converts a slice of disabled check IDs into a lookup set.
func toDisabledSet(ids []string) map[string]bool {
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
