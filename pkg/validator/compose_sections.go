package validator

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/nad"
)

// titleCase uppercases the first letter of a string, safe for ASCII section names.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// ComposePRChecksSection renders PR-check results.
func ComposePRChecksSection(titleErr, signErr, checklistErr error) Section {
	var b strings.Builder
	var hasError bool
	if titleErr != nil {
		hasError = true
		fmt.Fprintf(&b, "- ❌ **PR Title** — %s\n", titleErr)
	} else {
		b.WriteString("- ✅ **PR Title**\n")
	}
	if signErr != nil {
		hasError = true
		fmt.Fprintf(&b, "- ❌ **Signed Commits** — %s\n", signErr)
	} else {
		b.WriteString("- ✅ **Signed Commits**\n")
	}
	if checklistErr != nil {
		hasError = true
		fmt.Fprintf(&b, "- ❌ **PR Checklist** — %s\n", checklistErr)
	} else {
		b.WriteString("- ✅ **PR Checklist**\n")
	}
	return Section{Name: "PR Checks", Body: b.String(), Error: hasError}
}

// summaryIndentUnit is the non-breaking-space prefix prepended once per
// nesting level to visually indent a <details> summary label. GitHub strips
// inline CSS and never indents <details> bodies, so the label itself is
// shifted instead.
const summaryIndentUnit = "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;"

// summaryIndent returns the &nbsp; prefix for a summary at the given
// structural depth (1 = a first-level sub-dropdown beneath a top-level
// "Expand:" section). Depth 0 (the top-level sections) returns no indent.
func summaryIndent(depth int) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat(summaryIndentUnit, depth)
}

// renderSubDropdown writes a ReportSection as a nested <details> dropdown at
// the given structural depth: an icon + name summary followed by the
// section's Body, or its Summary when the section passed (Body empty).
func renderSubDropdown(sb *strings.Builder, s ReportSection, depth int) {
	fmt.Fprintf(sb, "<details>\n<summary>%s%s %s</summary>\n\n", summaryIndent(depth), s.Status.Icon(), s.Name)
	if s.Body != "" {
		sb.WriteString(s.Body)
	} else {
		sb.WriteString(s.Summary)
	}
	sb.WriteString("\n\n</details>\n\n")
}

// RenderSubDropdown wraps a plain title/body pair in a single first-level
// (depth 1) nested dropdown, with no status icon - unchanged from its prior
// behavior. Kept as the simple string-in-string-out helper existing callers
// (ComposeKustomizeBuildSection, ComposeScaffoldValidationSection) already
// use; new call sites needing a full ReportSection (status icon, pass/skip
// summary, arbitrary depth) should use renderSubDropdown directly.
func RenderSubDropdown(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s%s</summary>\n\n%s\n\n</details>", summaryIndent(1), title, body)
}

// checkDisplayName maps raw check names (matching the keys phases.go's
// report maps and CheckOutcome.Name use) to their display label in the PR
// comment.
var checkDisplayName = map[string]string{
	"large-file":   "Large File",
	"YAML-syntax":  "YAML Syntax",
	"config-sort":  "Config Sort Order",
	"startingCSV":  "Starting CSV",
	"golangci":     "golangci-lint",
	"kubeconform":  "Kubeconform",
	"markdownlint": "Markdownlint",
	"prettier":     "Prettier",
	"shellcheck":   "Shellcheck",
}

// displayName returns the proper display name for a raw check name,
// falling back to titleCase(name) for anything not in checkDisplayName.
func displayName(name string) string {
	if d, ok := checkDisplayName[name]; ok {
		return d
	}
	return titleCase(name)
}

// composeCheckChild builds a nested-dropdown ReportSection for a single
// named check. A non-empty failure report (reports[rawName]) takes
// precedence over the recorded outcome, and includes any fix-command hint
// fixHints can generate for that check. A missing outcome (the check didn't
// run at all, e.g. disabled) renders as a non-failing "Not run" child
// instead of silently vanishing from the report.
func composeCheckChild(rawName string, outcomes map[string]CheckOutcome, reports map[string]string) ReportSection {
	display := displayName(rawName)
	if report := reports[rawName]; report != "" {
		var sb strings.Builder
		fmt.Fprintf(&sb, "```\n%s\n```\n", strings.TrimSpace(truncateDetails(report, 4000)))
		if hints := fixHints([]LintFinding{{Check: rawName}}); len(hints) > 0 {
			sb.WriteString("\n**Fix command:**\n")
			for _, h := range hints {
				fmt.Fprintf(&sb, "- `%s`\n", h)
			}
		}
		return ReportSection{Name: display, Status: StatusError, Body: sb.String()}
	}
	o, ok := outcomes[rawName]
	if !ok {
		return ReportSection{Name: display, Status: StatusPassed, Summary: "Not run."}
	}
	summary := o.Note
	if summary == "" {
		summary = "Passed."
	}
	return ReportSection{Name: display, Status: o.Status, Summary: summary}
}

// composeParentFromChildren renders children as nested sub-dropdowns (depth
// 1) and returns a parent Section whose Error is set when any child's
// status is StatusError. The parent always has a Body, so the full
// sub-check breakdown is visible even when every child passed.
func composeParentFromChildren(name string, children []ReportSection) Section {
	hasError := false
	var sb strings.Builder
	for _, c := range children {
		if c.Status == StatusError {
			hasError = true
		}
		renderSubDropdown(&sb, c, 1)
	}
	return Section{Name: name, Body: sb.String(), Error: hasError}
}

// ComposeLintingSection renders the Linting section. Every linter
// (markdownlint, prettier, shellcheck, golangci, kubeconform) is always
// rendered as its own nested sub-dropdown showing its pass/skip/fail state
// (driven by outcomes), so the full breakdown is visible even when
// everything passed - not just a flat bullet list that disappears once a
// check is clean.
func ComposeLintingSection(outcomes []CheckOutcome, reports map[string]string) Section {
	byName := make(map[string]CheckOutcome, len(outcomes))
	for _, o := range outcomes {
		byName[o.Name] = o
	}
	order := []string{"markdownlint", "prettier", "shellcheck", "golangci", "kubeconform"}
	children := make([]ReportSection, 0, len(order))
	for _, name := range order {
		children = append(children, composeCheckChild(name, byName, reports))
	}
	return composeParentFromChildren("Linting", children)
}

// ComposeStaticChecksSection renders the Static Checks section the same way
// ComposeLintingSection does: every check always shown as its own nested
// sub-dropdown, driven by outcomes.
func ComposeStaticChecksSection(outcomes []CheckOutcome, reports map[string]string) Section {
	byName := make(map[string]CheckOutcome, len(outcomes))
	for _, o := range outcomes {
		byName[o.Name] = o
	}
	order := []string{"large-file", "YAML-syntax", "config-sort", "startingCSV"}
	children := make([]ReportSection, 0, len(order))
	for _, name := range order {
		children = append(children, composeCheckChild(name, byName, reports))
	}
	return composeParentFromChildren("Static Checks", children)
}

// ComposeResourceComplianceSection renders resource-compliance findings
// grouped by CheckID into per-check nested <details> sub-sections (rather
// than one flat table for every finding regardless of check type), plus an
// "Accepted Exceptions" audit block listing applied exemptions.
//
// blocking findings are in files this PR directly modifies (must be fixed
// before merge, per finalizeCompliance); warning findings are pre-existing
// (surfaced for visibility, non-blocking). A check's sub-section renders
// with a ❌ icon (and rolls the parent section's Error up) when it has any
// blocking finding, otherwise ⚠️. Check IDs are sorted for deterministic
// output - this generic core has no fixed, org-defined check ordering
// (unlike an org layer's own `complianceCheckOrder`, which is exactly the
// kind of policy decision that doesn't belong here).
func ComposeResourceComplianceSection(blocking, warning []check.Finding, exempted []exempt.Applied) Section {
	hasFindings := len(blocking) > 0 || len(warning) > 0
	hasExemptions := len(exempted) > 0
	if !hasFindings && !hasExemptions {
		return Section{Name: "Resource Compliance", Body: "No compliance findings."}
	}

	byCheck := map[string][]check.Finding{}
	isBlocking := map[string]bool{}
	for _, f := range blocking {
		byCheck[f.CheckID] = append(byCheck[f.CheckID], f)
		isBlocking[f.CheckID] = true
	}
	for _, f := range warning {
		byCheck[f.CheckID] = append(byCheck[f.CheckID], f)
	}
	ids := make([]string, 0, len(byCheck))
	for id := range byCheck {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	if hasFindings {
		b.WriteString("If the affected resource is being modified in this PR, these issues **must** be corrected.\n")
		b.WriteString("Otherwise, these are non-blocking warnings for pre-existing issues.\n\n")
	}
	for _, id := range ids {
		findings := byCheck[id]
		icon := "⚠️"
		if isBlocking[id] {
			icon = "❌"
		}
		fmt.Fprintf(&b, "<details>\n<summary>%s%s %s (%d finding(s))</summary>\n\n", summaryIndent(1), icon, displayName(id), len(findings))
		b.WriteString("| File | Message |\n| --- | --- |\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "| %s | %s |\n", f.File, f.Message)
		}
		b.WriteString("\n</details>\n\n")
	}

	if hasExemptions {
		renderAcceptedExceptions(&b, exempted)
	}

	return Section{Name: "Resource Compliance", Body: b.String(), Error: len(blocking) > 0}
}

// renderAcceptedExceptions writes the "Accepted Exceptions" audit sub-block
// from the applied exemptions (check.Result.Exempted / exempt.Applied),
// distinguishing exemptions applied to a directly-modified resource
// (e.Direct) from pre-existing ones. This data already existed
// (exempt.Applied.Direct) but was never rendered anywhere before this.
func renderAcceptedExceptions(b *strings.Builder, exemptions []exempt.Applied) {
	var haveDirect bool
	for _, e := range exemptions {
		if e.Direct {
			haveDirect = true
			break
		}
	}
	label := "Accepted Exceptions"
	if !haveDirect {
		label += " (pre-existing)"
	}
	fmt.Fprintf(b, "<details>\n<summary>%sℹ️ %s (%d)</summary>\n\n", summaryIndent(1), label, len(exemptions))
	b.WriteString("| Resource | Value | Scope |\n| --- | --- | --- |\n")
	for _, e := range exemptions {
		resource := e.Name
		if e.Kind != "" {
			resource = fmt.Sprintf("%s `%s`", e.Kind, e.Name)
		}
		scope := "pre-existing"
		if e.Direct {
			scope = "directly modified"
		}
		fmt.Fprintf(b, "| %s | `%s` | %s |\n", resource, e.Value, scope)
	}
	b.WriteString("\n</details>\n\n")
}

// ComposeKustomizeBuildSection renders the Kustomize Build section: overlay
// build pass/fail (grouped by root cause via groupBuildErrors/
// formatBuildErrors so N overlays sharing one underlying error don't repeat
// it N times), a hooks matrix, files needing `kustomize edit fix`, and any
// ghost-patch findings.
//
// buildErrs are raw error strings in this repo's real overlay-build format
// ("kustomize build <overlay>: <cause>", see pkg/overlay/overlay.go and
// comments.go's groupBuildErrors doc comment). hookTable and ghostTable are
// pre-rendered markdown (typically a table) built by the caller from
// pkg/hook and pkg/ghostpatch data respectively; empty means "nothing to
// show" (not "not checked").
func ComposeKustomizeBuildSection(overlayCount int, buildErrs []string, hookTable string, fixNeeded []string, ghostTable string) Section {
	var b strings.Builder
	hasError := false

	// Overlay build summary
	groups, other := groupBuildErrors(buildErrs)
	if len(groups) == 0 && len(other) == 0 {
		fmt.Fprintf(&b, "- ✅ **Overlay Build** — %d overlay(s) built successfully\n", overlayCount)
	} else {
		hasError = true
		b.WriteString("- ❌ **Overlay Build**\n\n")
		if len(groups) > 0 {
			formatBuildErrors(&b, groups)
		}
		for _, e := range other {
			fmt.Fprintf(&b, "> - %s\n", e)
		}
		b.WriteString("\n")
	}

	// Hook results
	if hookTable != "" {
		b.WriteString(RenderSubDropdown("Hook Results", hookTable))
		b.WriteString("\n")
	} else {
		b.WriteString("- ✅ **Hooks** — no hooks defined\n")
	}

	// Kustomize fix
	if len(fixNeeded) > 0 {
		hasError = true
		b.WriteString("- ❌ **Kustomize Fix** — the following files need `kustomize edit fix`:\n")
		for _, f := range fixNeeded {
			fmt.Fprintf(&b, "  - `%s`\n", f)
		}
	} else {
		b.WriteString("- ✅ **Kustomize Fix** — all kustomization.yaml files are up to date\n")
	}

	// Ghost patches
	if ghostTable != "" {
		hasError = true
		b.WriteString(RenderSubDropdown("Ghost Patches", ghostTable))
		b.WriteString("\n")
	} else {
		b.WriteString("- ✅ **Ghost Patches** — none detected\n")
	}

	return Section{Name: "Kustomize Build", Body: b.String(), Error: hasError}
}

// ComposeScaffoldValidationSection renders scaffold validation results.
func ComposeScaffoldValidationSection(driftSummary string, execErrors, missingClusters []string) Section {
	var b strings.Builder
	hasError := false

	// Drift
	if driftSummary == "" {
		b.WriteString("- ✅ **Scaffold Drift** — no drift detected\n")
	} else {
		hasError = true
		b.WriteString("- ❌ **Scaffold Drift**\n\n")
		b.WriteString(RenderSubDropdown("Drift Details", driftSummary))
		b.WriteString("\n")
	}

	// Exec errors
	if len(execErrors) == 0 {
		b.WriteString("- ✅ **Scaffold Exec** — all scaffold runs succeeded\n")
	} else {
		hasError = true
		b.WriteString("- ❌ **Scaffold Exec**\n")
		for _, e := range execErrors {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}

	// Missing clusters
	if len(missingClusters) == 0 {
		b.WriteString("- ✅ **Cluster Coverage** — all clusters accounted for\n")
	} else {
		hasError = true
		b.WriteString("- ❌ **Missing Clusters**\n")
		for _, c := range missingClusters {
			fmt.Fprintf(&b, "  - `%s`\n", c)
		}
	}

	return Section{Name: "Scaffold Validation", Body: b.String(), Error: hasError}
}

// ComposeKyvernoSection renders the Kyverno subsection.
func ComposeKyvernoSection(body string) Section {
	if body == "" {
		return Section{Name: "Kyverno Policies", Body: "No Kyverno findings."}
	}
	return Section{Name: "Kyverno Policies", Body: body, Error: true}
}

// ComposeCINotesSection renders CI notes.
func ComposeCINotesSection(body string) Section {
	return Section{Name: "CI Notes", Body: body}
}

// ComposeNADSection renders NetworkAttachmentDefinition validation results.
// tier reflects which rule set actually ran: "structural" (the always-on
// default) or "OVN-aware" (Options.AssumeOpenShift's additional semantic
// tier - see pkg/validator/nad's package doc comment).
func ComposeNADSection(nadErrors []nad.ValidationError, assumeOpenshift bool) Section {
	tier := "structural"
	if assumeOpenshift {
		tier = "OVN-aware"
	}
	if len(nadErrors) == 0 {
		return Section{Name: "NetworkAttachmentDefinition Validation", Body: fmt.Sprintf("All NetworkAttachmentDefinition resources passed %s validation.", tier)}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%d invalid NetworkAttachmentDefinition(s)** (%s validation):\n\n", len(nadErrors), tier)
	b.WriteString("| File | Error |\n| --- | --- |\n")
	for _, e := range nadErrors {
		fmt.Fprintf(&b, "| %s | %s |\n", e.File, strings.ReplaceAll(e.Message, "|", "\\|"))
	}

	return Section{Name: "NetworkAttachmentDefinition Validation", Body: b.String(), Error: true}
}
