package validator

import (
	"path/filepath"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/psa"
)

// psaMissingLabelsExtraKey is the check.Finding.Extra key psaCheck.CheckDoc
// (register_checks.go) uses to carry a psa-labels finding's raw,
// comma-separated MissingLabels, so filterCommentedPSAFindings can check
// each missing label individually against psa.FindCommentedNamespaces
// without re-parsing the rendered Message string.
const psaMissingLabelsExtraKey = "missing_labels"

// filterCommentedPSAFindings drops psa-labels findings whose every missing
// label is present, commented-out, in the finding's app's base/ directory -
// e.g. an operator temporarily commented out a PSA label while
// troubleshooting, intending to restore it, rather than never having set
// one at all. psa.FindCommentedNamespaces has existed (and been tested)
// since PSA support was added, but was never actually wired into the
// psa-labels check's results before this, so this suppression never took
// effect.
//
// Only findings whose MissingLabels entries match verbatim are suppressed:
// an "invalid value" entry (e.g.
// "pod-security.kubernetes.io/enforce (invalid value \"foo\")") never
// matches a bare commented-out key, so a label that's present but has the
// wrong value is never silently suppressed by this filter - only a label
// that's genuinely absent, and only when a comment shows it was
// intentionally disabled rather than never configured.
func filterCommentedPSAFindings(findings []check.Finding) []check.Finding {
	commentedByAppRoot := make(map[string]map[string]map[string]bool)
	out := make([]check.Finding, 0, len(findings))
	for _, f := range findings {
		if f.CheckID != "psa-labels" {
			out = append(out, f)
			continue
		}
		appRoot, ok := appRootFromBaseFile(f.File)
		if !ok {
			out = append(out, f)
			continue
		}
		commented, ok := commentedByAppRoot[appRoot]
		if !ok {
			commented = psa.FindCommentedNamespaces(appRoot)
			commentedByAppRoot[appRoot] = commented
		}
		if allLabelsCommented(f.Get(psaMissingLabelsExtraKey), commented[f.Name]) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// allLabelsCommented reports whether every label in missingLabelsCSV (a
// comma-separated list, see psaMissingLabelsExtraKey) is present in
// commented. Returns false for an empty list or nil map, so a finding is
// never suppressed in the absence of positive evidence every one of its
// missing labels was intentionally commented out.
func allLabelsCommented(missingLabelsCSV string, commented map[string]bool) bool {
	if missingLabelsCSV == "" || len(commented) == 0 {
		return false
	}
	for _, l := range strings.Split(missingLabelsCSV, ",") {
		if !commented[l] {
			return false
		}
	}
	return true
}

// appRootFromBaseFile returns the app root directory (the parent of a
// "base" path segment) for a file living under <approot>/base/..., e.g.
// "myapp/base/namespace.yaml" -> "myapp". Returns false if file contains no
// "base" segment, since psa.FindCommentedNamespaces only ever looks under
// <dir>/base and there's no meaningful app root to check otherwise.
func appRootFromBaseFile(file string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i, p := range parts {
		if p == "base" {
			return filepath.FromSlash(strings.Join(parts[:i], "/")), true
		}
	}
	return "", false
}
