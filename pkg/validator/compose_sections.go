package validator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
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

// ComposePRChecksSection renders PR-check results. Each of the three checks
// (title convention, commit signing, checklist) is its own independent
// collapsible sub-dropdown with its own ✅/❌ status - via the same
// composeParentFromChildren/ReportSection machinery ComposeLintingSection/
// ComposeStaticChecksSection already use - rather than a single flat bullet
// list, so a reader can tell at a glance which specific check(s) failed
// without reading prose, and each failure's detail is tucked away until
// expanded instead of always showing inline. titleSuggestion, when
// non-empty, is a non-blocking note (see github.TitleSuggestion/
// github.PRTitleSuggestion) folded into an otherwise-passing "PR Title"
// child as a ⚠️ warning rather than promoted to a failure - an org's
// optional title convention (e.g. a ticket-reference suffix) never blocks
// the pipeline, unlike the required Conventional-Commits prefix (titleErr).
func ComposePRChecksSection(titleErr, signErr, checklistErr error, titleSuggestion string) ReportSection {
	children := []ReportSection{
		prTitleChild(titleErr, titleSuggestion),
		prCheckChild("Signed Commits", signErr),
		prCheckChild("PR Checklist", checklistErr),
	}
	return composeParentFromChildren("PR Checks", children)
}

// composePRChecksSectionFromResult builds the "PR Checks" section from a
// PRValidationResult (the struct form threaded through validator.Options),
// mirroring ComposePRChecksSection's error-based rendering.
func composePRChecksSectionFromResult(r *PRValidationResult) ReportSection {
	titleStatus := StatusPassed
	titleBody := "Passed."
	if !r.TitlePassed {
		titleStatus = StatusWarning
		if r.TitleBlocking {
			titleStatus = StatusError
		}
		if r.TitleMsg != "" {
			titleBody = r.TitleMsg
		} else {
			titleBody = "PR title check failed."
		}
	}

	signStatus := StatusPassed
	signBody := "Passed."
	if !r.CommitsPassed {
		signStatus = StatusError
		signBody = fmt.Sprintf("%d of %d commit(s) unsigned.", r.UnsignedCount, r.TotalCommits)
	}

	checklistStatus := StatusPassed
	checklistBody := "Passed."
	if !r.ChecklistPassed {
		checklistStatus = StatusError
		if r.ChecklistMsg != "" {
			checklistBody = r.ChecklistMsg
		} else {
			checklistBody = "PR checklist incomplete."
		}
	}

	children := []ReportSection{
		{Name: "PR Title", Status: titleStatus, Body: titleBody},
		{Name: "Signed Commits", Status: signStatus, Body: signBody},
		{Name: "PR Checklist", Status: checklistStatus, Body: checklistBody},
	}
	return composeParentFromChildren("PR Checks", children)
}

// prCheckChild renders one PR-level check's outcome as a ReportSection: a
// passed check gets a plain "Passed." summary, a failed one carries err's
// message as its expandable body.
func prCheckChild(name string, err error) ReportSection {
	if err != nil {
		return ReportSection{Name: name, Status: StatusError, Body: err.Error()}
	}
	return ReportSection{Name: name, Status: StatusPassed, Summary: "Passed."}
}

// prTitleChild renders the "PR Title" check outcome. A non-empty
// titleSuggestion is only ever consulted when titleErr is nil (the
// required prefix already passed - see github.PRTitleSuggestion, which
// itself withholds a suggestion until the required check passes), so a
// hard title failure is never diluted by also showing a suggestion.
func prTitleChild(titleErr error, titleSuggestion string) ReportSection {
	if titleErr != nil {
		return ReportSection{Name: "PR Title", Status: StatusError, Body: titleErr.Error()}
	}
	if titleSuggestion != "" {
		return ReportSection{Name: "PR Title", Status: StatusWarning, Body: "Passed. Suggestion: " + titleSuggestion}
	}
	return ReportSection{Name: "PR Title", Status: StatusPassed, Summary: "Passed."}
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
	"large-file":     "Large File",
	"YAML-syntax":    "YAML Syntax",
	"config-sort":    "Config Sort Order",
	"startingCSV":    "Starting CSV",
	"golangci":       "golangci-lint",
	"kubeconform":    "Kubeconform",
	"markdownlint":   "Markdownlint",
	"prettier":       "Prettier",
	"shellcheck":     "Shellcheck",
	"scaffold table": "Scaffold Table",
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
func composeCheckChild(rawName string, outcomes map[string]CheckOutcome, reports map[string]string, binaryName string) ReportSection {
	display := displayName(rawName)
	if report := reports[rawName]; report != "" {
		var sb strings.Builder
		fmt.Fprintf(&sb, "```\n%s\n```\n", strings.TrimSpace(truncateDetails(report, 4000)))
		if hints := fixHints([]LintFinding{{Check: rawName}}, binaryName); len(hints) > 0 {
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
// 1) and returns a parent ReportSection whose Status is the most severe
// status among its children (StatusError > StatusWarning > StatusInfo >
// StatusPassed, matching SectionStatus's declared iota order) - so a
// parent's own icon correctly inherits the worst thing inside it instead of
// only ever collapsing to a binary ✅/❌ and silently hiding an internal
// ⚠️/ℹ️ child (e.g. a single non-blocking StatusWarning child, with no
// StatusError sibling, now rolls the parent up to ⚠️ rather than staying a
// misleadingly-plain ✅). The parent always has a Body, so the full
// sub-check breakdown is visible even when every child passed.
func composeParentFromChildren(name string, children []ReportSection) ReportSection {
	status := StatusPassed
	var sb strings.Builder
	for _, c := range children {
		if c.Status > status {
			status = c.Status
		}
		renderSubDropdown(&sb, c, 1)
	}
	return ReportSection{Name: name, Status: status, Body: sb.String()}
}

// ComposeLintingSection renders the Linting section. Every linter
// (markdownlint, prettier, shellcheck, golangci, kubeconform) is always
// rendered as its own nested sub-dropdown showing its pass/skip/fail state
// (driven by outcomes), so the full breakdown is visible even when
// everything passed - not just a flat bullet list that disappears once a
// check is clean.
func ComposeLintingSection(outcomes []CheckOutcome, reports map[string]string, binaryName string) ReportSection {
	byName := make(map[string]CheckOutcome, len(outcomes))
	for _, o := range outcomes {
		byName[o.Name] = o
	}
	order := []string{"markdownlint", "prettier", "shellcheck", "golangci", "kubeconform"}
	children := make([]ReportSection, 0, len(order))
	for _, name := range order {
		children = append(children, composeCheckChild(name, byName, reports, binaryName))
	}
	return composeParentFromChildren("Linting", children)
}

// ComposeStaticChecksSection renders the Static Checks section the same way
// ComposeLintingSection does: every check always shown as its own nested
// sub-dropdown, driven by outcomes.
func ComposeStaticChecksSection(outcomes []CheckOutcome, reports map[string]string, binaryName string) ReportSection {
	byName := make(map[string]CheckOutcome, len(outcomes))
	for _, o := range outcomes {
		byName[o.Name] = o
	}
	order := []string{"large-file", "YAML-syntax", "config-sort", "startingCSV", "scaffold table"}
	children := make([]ReportSection, 0, len(order))
	for _, name := range order {
		children = append(children, composeCheckChild(name, byName, reports, binaryName))
	}
	return composeParentFromChildren("Static Checks", children)
}

// ComposeRuntimeValidationSection renders runtime validation findings grouped
// by family, then by category, into nested <details> sub-sections.
//
// Runtime checks return Section() == "runtime-validation" and are always
// blocking, so this section never shows ⚠️ or StatusWarning, only
// ❌/StatusError or ✅/StatusPassed. Keys are sorted for deterministic output.
//
// The family level exists because the strength of "the cluster rejects this"
// varies by upstream: an API-server rule always holds, whereas a rule owned by
// an operator only holds where that operator is installed. Grouping by family
// puts that qualifier in the report's structure instead of leaving it to prose
// a reviewer may not read. It is omitted entirely when only one family is
// present, so a single-family run renders exactly as it did before families
// existed rather than growing a wrapper around one child.
func ComposeRuntimeValidationSection(findings []check.Finding) ReportSection {
	// Group by category ("core", "batch", ...), not CheckID. CheckID is the
	// rule ID, so grouping on it would render one <details> block per rule -
	// dozens of single-finding sub-sections. Category keeps the section
	// readable while CheckID remains the finding's true identity for
	// dispatch and registry lookups.
	byFamily := map[string]map[string][]check.Finding{}
	for _, f := range findings {
		key := f.Extra["category"]
		if key == "" {
			key = f.CheckID
		}
		fam := f.Extra["family"]
		if byFamily[fam] == nil {
			byFamily[fam] = map[string][]check.Finding{}
		}
		byFamily[fam][key] = append(byFamily[fam][key], f)
	}
	if len(byFamily) == 0 {
		return ReportSection{Name: "Runtime Validation", Status: StatusPassed, Body: "No runtime validation findings."}
	}

	var b strings.Builder
	b.WriteString("These are structural/runtime validation rules enforced by the cluster. Findings here indicate manifests that the cluster would reject.\n\n")

	families := make([]string, 0, len(byFamily))
	for fam := range byFamily {
		families = append(families, fam)
	}
	sort.Strings(families)
	nested := len(families) > 1

	for _, fam := range families {
		byCheck := byFamily[fam]
		depth := 1
		if nested {
			total := 0
			for _, fs := range byCheck {
				total += len(fs)
			}
			fmt.Fprintf(&b, "<details open>\n<summary>%s❌ %s (%d finding(s))</summary>\n\n",
				summaryIndent(1), runtimeFamilyTitle(fam), total)
			depth = 2
		}
		for _, id := range orderedComplianceIDs(byCheck) {
			findings := byCheck[id]
			count, body := renderRuntimeSub(findings)
			fmt.Fprintf(&b, "<details>\n<summary>%s❌ %s (%d finding(s))</summary>\n\n", summaryIndent(depth), complianceTitle(id), count)
			b.WriteString(body)
			b.WriteString("\n</details>\n\n")
		}
		if nested {
			b.WriteString("</details>\n\n")
		}
	}

	return ReportSection{Name: "Runtime Validation", Body: b.String(), Status: StatusError}
}

// runtimeFamilyTitle renders a family key as a report heading, including what
// its findings actually guarantee - the qualifier that distinguishes the
// families and the only reason the family level is rendered at all.
func runtimeFamilyTitle(family string) string {
	switch family {
	case "":
		return "Runtime"
	case "kubernetes":
		return "Kubernetes — rejected by the API server"
	default:
		return displayName(family)
	}
}

// renderRuntimeSub renders one category's runtime findings.
//
// The generic compliance renderer cannot be reused here. It looks up a
// TableSpec by the sub-section's ID, and these are grouped by category, which
// has no spec - so it fell back to a File/Message table that dropped the
// resource, the field path, the rule that fired and the upstream citation,
// and skipped deduplication, printing the same finding once per overlay it
// was rendered from.
//
// The citation is the whole claim this family makes: that a finding
// corresponds to a specific function in the API server rather than to this
// tool's opinion. It is listed once per rule under the table instead of
// repeated on every row, which keeps the rows narrow enough to read.
func renderRuntimeSub(findings []check.Finding) (count int, body string) {
	rows := dedupFindingsForTable(findings)

	cell := func(s string) string {
		if s == "" {
			return "—"
		}
		// sanitizeCell escapes pipes and folds newlines. A resource name or
		// field value is copied from the manifest, so it can contain either;
		// an unescaped newline splits one row into several and corrupts the
		// rest of the table.
		return sanitizeCell(s)
	}

	var b strings.Builder
	b.WriteString("| Rule | Resource | File | Field | Message |\n| --- | --- | --- | --- | --- |\n")

	refs := map[string]string{}
	order := []string{}
	for _, f := range rows {
		resource := f.Kind
		if f.Name != "" {
			resource += "/" + f.Name
		}
		rule := f.Extra["ruleId"]
		if rule == "" {
			rule = f.CheckID
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n",
			cell(rule), cell(resource), cell(f.File), cell(f.Path), cell(f.Message))

		if ref := f.Extra["upstreamRef"]; ref != "" {
			if _, seen := refs[rule]; !seen {
				order = append(order, rule)
			}
			refs[rule] = ref
		}
	}

	if len(order) > 0 {
		b.WriteString("\nUpstream Kubernetes validation these rules are ported from:\n\n")
		for _, rule := range order {
			fmt.Fprintf(&b, "- `%s` — `%s`\n", rule, refs[rule])
		}
	}

	return len(rows), b.String()
}

// ComposeResourceComplianceSection renders resource-compliance findings
// grouped by CheckID into per-check nested <details> sub-sections (rather
// than one flat table for every finding regardless of check type), plus an
// "Accepted Exemptions" audit block listing applied exemptions.
//
// blocking findings are in files this PR directly modifies (must be fixed
// before merge, per finalizeCompliance); warning findings are pre-existing
// (surfaced for visibility, non-blocking). A check's sub-section renders
// with a ❌ icon (and rolls the parent section's Status up to StatusError)
// when it has any blocking finding, otherwise ⚠️ (rolling the parent up to
// StatusWarning at most). Exemptions alone (no findings at all) roll the
// parent up to StatusInfo instead - "nothing wrong, but here's an audit
// trail of what was excused" is worth a glance, but shouldn't look like an
// active warning. Check IDs are sorted for deterministic output - this
// generic core has no fixed, org-defined check ordering (unlike an org
// layer's own `complianceCheckOrder`, which is exactly the kind of policy
// decision that doesn't belong here).
func ComposeResourceComplianceSection(blocking, warning []check.Finding, exempted []exempt.Applied, sources map[string][]string) ReportSection {
	hasFindings := len(blocking) > 0 || len(warning) > 0
	hasExemptions := len(exempted) > 0
	if !hasFindings && !hasExemptions {
		return ReportSection{Name: "Resource Compliance", Status: StatusPassed, Body: "No compliance findings."}
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

	var b strings.Builder
	if hasFindings {
		b.WriteString("If the affected resource is being modified in this PR, these issues **must** be corrected.\n")
		b.WriteString("Otherwise, these are non-blocking warnings for pre-existing issues.\n\n")
	}
	for _, id := range orderedComplianceIDs(byCheck) {
		findings := byCheck[id]
		blockingCheck := isBlocking[id]
		icon := "⚠️"
		if blockingCheck {
			icon = "❌"
		}
		// count is the number of DEDUPED rows (unique resource issues), not the
		// raw per-overlay fan-out - a single base/component resource flagged
		// across 55 overlays is one finding, with the 55-overlay spread shown
		// in the Overlays column (see renderResourceComplianceTable).
		count, body := renderComplianceSub(id, findings, blockingCheck, sources)
		fmt.Fprintf(&b, "<details>\n<summary>%s%s %s (%d finding(s))</summary>\n\n", summaryIndent(1), icon, complianceTitle(id), count)
		b.WriteString(body)
		b.WriteString("\n</details>\n\n")
	}

	if hasExemptions {
		renderAcceptedExemptions(&b, exempted)
	}

	status := StatusPassed
	switch {
	case len(blocking) > 0:
		status = StatusError
	case len(warning) > 0:
		status = StatusWarning
	case hasExemptions:
		status = StatusInfo
	}

	return ReportSection{Name: "Resource Compliance", Body: b.String(), Status: status}
}

// orderedComplianceIDs returns the check IDs present in byCheck, ordered by the
// fixed complianceCheckOrder first, then any remaining IDs (e.g. placeholder,
// cluster-identity) sorted alphabetically for deterministic output.
func orderedComplianceIDs(byCheck map[string][]check.Finding) []string {
	var ids []string
	seen := map[string]bool{}
	for _, id := range complianceCheckOrder {
		if len(byCheck[id]) > 0 {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	var rest []string
	for id := range byCheck {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	return append(ids, rest...)
}

// complianceTitle returns a check's registered TableSpec.Title (a fuller,
// more descriptive heading - e.g. "Image Digest Pinning" rather than just
// "Image-checksum") when one is registered, falling back to displayName(id)
// for any check id without one (e.g. a newly-added check that hasn't been
// given a TableSpec entry yet - see register_tables.go).
func complianceTitle(id string) string {
	if ts, ok := TableSpecForCheck(id); ok && ts.Title != "" {
		return ts.Title
	}
	return displayName(id)
}

// complianceRowCount returns the deduped row count for a check's findings -
// the same number rendered in the sub-section heading (renderComplianceSub) -
// so the console per-check warning/error lines agree with the PR comment.
func complianceRowCount(id string, findings []check.Finding) int {
	spec, ok := TableSpecForCheck(id)
	if !ok {
		return len(findings)
	}
	if spec.ResourceKey == nil {
		return len(dedupFindingsForTable(findings))
	}
	return len(dedupComplianceRows(findings))
}

// renderComplianceSub renders one check's deduped findings and returns the
// deduped row count (for the sub-section heading) plus the rendered table body.
//
// Three rendering paths:
//   - Resource-based checks (TableSpec with a ResourceKey - podspec, psa, rbac,
//     image-checksum, etc.): deduped by resource identity, rendered with an
//     "Overlays" column (a count when the same issue spans multiple overlays, or
//     the single built-file when just one) plus, for blocking sub-sections, a
//     "Source File(s)" column naming the changed source that made it blocking.
//   - File-based checks (TableSpec without a ResourceKey - placeholder,
//     cluster-identity): the legacy file-list dedup (a File cell listing every
//     distinct location).
//   - No TableSpec: a flat File/Message dump.
func renderComplianceSub(id string, findings []check.Finding, blocking bool, sources map[string][]string) (count int, body string) {
	spec, ok := TableSpecForCheck(id)
	if !ok {
		var b strings.Builder
		b.WriteString("| File | Message |\n| --- | --- |\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "| %s | %s |\n", f.File, f.Message)
		}
		return len(findings), b.String()
	}
	if spec.ResourceKey == nil {
		rows := dedupFindingsForTable(findings)
		return len(rows), RenderColumnedTable(rows, id)
	}
	rows := dedupComplianceRows(findings)
	return len(rows), renderResourceComplianceTable(spec, rows, blocking, sources)
}

// compRow is one deduplicated resource-compliance row: a representative finding
// carrying the cell data, plus the distinct overlay/build files it was seen in
// (count = len(files)).
type compRow struct {
	rep   check.Finding
	files []string
}

// dedupComplianceRows groups findings by resource identity (findingDedupKey),
// accumulating the distinct files each was seen in, preserving first-seen order.
func dedupComplianceRows(findings []check.Finding) []compRow {
	idx := map[string]int{}
	seen := map[string]map[string]bool{}
	var rows []compRow
	for _, f := range findings {
		k := findingDedupKey(f)
		i, ok := idx[k]
		if !ok {
			i = len(rows)
			idx[k] = i
			seen[k] = map[string]bool{}
			rows = append(rows, compRow{rep: f})
		}
		if f.File != "" && !seen[k][f.File] {
			seen[k][f.File] = true
			rows[i].files = append(rows[i].files, f.File)
		}
	}
	for i := range rows {
		sort.Strings(rows[i].files)
	}
	return rows
}

// renderResourceComplianceTable renders resource-compliance rows with the
// TableSpec's own columns plus an "Overlays" column (and, when blocking, a
// "Source File(s)" column). A base/component
// resource flagged across many overlays is one row with an overlay count,
// instead of the same row repeated per overlay or a giant path list.
func renderResourceComplianceTable(spec check.TableSpec, rows []compRow, blocking bool, sources map[string][]string) string {
	var b strings.Builder
	if spec.Preamble != "" {
		fmt.Fprintf(&b, "%s\n\n", spec.Preamble)
	}
	headers := make([]string, 0, len(spec.Columns)+2)
	for _, c := range spec.Columns {
		headers = append(headers, c.Header)
	}
	if blocking {
		headers = append(headers, "Source File(s)")
	}
	headers = append(headers, "Overlays")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Fprintf(&b, "| %s |\n| %s |\n", strings.Join(headers, " | "), strings.Join(seps, " | "))
	for _, row := range rows {
		cells := make([]string, 0, len(headers))
		for _, c := range spec.Columns {
			cells = append(cells, sanitizeCell(c.Cell(row.rep)))
		}
		if blocking {
			kind, name := "", ""
			if spec.SourceKey != nil {
				kind, name = spec.SourceKey(row.rep)
			}
			cells = append(cells, sanitizeCell(sourceInfo(sources, kind, name, len(row.files))))
		}
		cells = append(cells, overlaysCell(row, blocking))
		fmt.Fprintf(&b, "| %s |\n", strings.Join(cells, " | "))
	}
	return b.String()
}

// overlaysCell renders the "Overlays" column value: for a non-blocking row seen
// in exactly one overlay, the single built-file label (e.g. `app/pd1010.yaml`);
// otherwise the count of distinct overlays the issue spans.
func overlaysCell(row compRow, blocking bool) string {
	if !blocking && len(row.files) == 1 {
		return fmt.Sprintf("`%s`", builtFileLabel(row.files[0]))
	}
	return fmt.Sprintf("%d", len(row.files))
}

// builtFileLabel normalizes a rendered finding's overlay-dir path
// (app/overlays/<cluster>) to the built-file style label
// (app/<cluster>.yaml), so single-overlay rows show something a reviewer can
// map to the built output rather than a bare directory.
func builtFileLabel(overlayPath string) string {
	slash := filepath.ToSlash(overlayPath)
	if i := strings.Index(slash, "/overlays/"); i >= 0 {
		app := slash[:i]
		cluster := slash[i+len("/overlays/"):]
		if j := strings.IndexByte(cluster, '/'); j >= 0 {
			cluster = cluster[:j]
		}
		return app + "/" + cluster + ".yaml"
	}
	return overlayPath
}

// sourceInfo renders the "Source File(s)" cell for a blocking row: the changed
// source file(s) (from the PR's changedResourceKeys) that define the resource
// and made this finding blocking. Falls back to an overlay count when no source
// mapping is available.
func sourceInfo(sources map[string][]string, kind, name string, count int) string {
	files := sources[kind+"/"+name]
	if len(files) == 0 {
		if count == 1 {
			return "1 overlay"
		}
		return fmt.Sprintf("%d overlays", count)
	}
	if len(files) == 1 {
		return fmt.Sprintf("`%s`", files[0])
	}
	if len(files) <= 3 {
		quoted := make([]string, len(files))
		for i, f := range files {
			quoted[i] = "`" + f + "`"
		}
		return strings.Join(quoted, ", ")
	}
	return fmt.Sprintf("%d files", len(files))
}

// dedupFindingsForTable collapses findings that describe the same
// underlying resource/issue but were fanned out across multiple overlays or
// build locations (every field but File and CheckID identical - CheckID is
// already fixed by the caller's per-check grouping, and File is exactly the
// field fan-out varies on; see runDocChecks/evaluateDoc in engine.go) into a
// single representative row per group, preserving first-seen order. The
// representative's File becomes the sorted, backtick-quoted, comma-joined
// list of every distinct file the finding occurred in, and its Count
// records how many locations were merged (informational; no current
// TableSpec column reads it, but see check.CountMode/Finding.Count).
//
// This applies one generic key to every check rather than requiring each
// check to supply its own TableSpec.DedupKey - the fan-out this addresses
// is structural to the engine, not check-specific.
func dedupFindingsForTable(findings []check.Finding) []check.Finding {
	type group struct {
		rep   check.Finding
		files []string
		seen  map[string]bool
	}
	order := make([]string, 0, len(findings))
	groups := make(map[string]*group, len(findings))
	for _, f := range findings {
		key := findingDedupKey(f)
		g, ok := groups[key]
		if !ok {
			g = &group{rep: f, seen: map[string]bool{}}
			groups[key] = g
			order = append(order, key)
		}
		if f.File != "" && !g.seen[f.File] {
			g.seen[f.File] = true
			g.files = append(g.files, f.File)
		}
	}
	out := make([]check.Finding, 0, len(order))
	for _, key := range order {
		g := groups[key]
		rep := g.rep
		rep.Count = len(g.files)
		if len(g.files) > 0 {
			sort.Strings(g.files)
			rep.File = joinBackticked(g.files)
		}
		out = append(out, rep)
	}
	return out
}

// findingDedupKey is the generic (File- and CheckID-independent) identity a
// finding is grouped by for dedupFindingsForTable.
func findingDedupKey(f check.Finding) string {
	return strings.Join([]string{f.Kind, f.Name, f.Namespace, f.Container, f.Path, f.Value, f.Message}, "\x1f")
}

// joinBackticked renders a sorted file list as backtick-quoted, comma-joined
// markdown (e.g. "`a.yaml`, `b.yaml`"), used for dedupFindingsForTable's
// synthesized multi-location File cell.
func joinBackticked(files []string) string {
	quoted := make([]string, len(files))
	for i, f := range files {
		quoted[i] = fmt.Sprintf("`%s`", f)
	}
	return strings.Join(quoted, ", ")
}

// renderAcceptedExemptions writes the "Accepted Exemptions" audit sub-block
// from the applied exemptions (check.Result.Exempted / exempt.Applied),
// distinguishing exemptions applied to a directly-modified resource
// (e.Direct) from pre-existing ones. This data already existed
// (exempt.Applied.Direct) but was never rendered anywhere before this.
func renderAcceptedExemptions(b *strings.Builder, exemptions []exempt.Applied) {
	var haveDirect bool
	for _, e := range exemptions {
		if e.Direct {
			haveDirect = true
			break
		}
	}
	label := "Accepted Exemptions"
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

// ComposeKustomizeBuildSection renders the Kustomize Build section as four
// always-shown sub-dropdowns - Overlay Build, Hooks, Kustomize Fix, Ghost
// Patches (see the composeXChild helpers below) - via the same
// composeParentFromChildren/ReportSection machinery ComposeLintingSection/
// ComposeStaticChecksSection/ComposePRChecksSection already use. Each child
// renders as a single icon-bearing <details> line (never a separate
// icon-less bullet followed by a differently-titled nested dropdown, which
// is how this used to render before it was rewritten to use
// composeParentFromChildren), and the parent's own icon correctly inherits
// the *worst* child status: a non-blocking-only Ghost Patches finding rolls
// the parent up to ⚠️, not plain ✅ (which would hide it) or ❌ (which would
// overstate it) - see docs/CI.md's "Ghost Patch Detection".
func ComposeKustomizeBuildSection(overlayCount int, buildErrs []string, hookTable string, hookFailed bool, fixNeeded []string, fixCheckErr error, fixCheckEnabled bool, ghostTable string, ghostBlockingCount int, binaryName string) ReportSection {
	children := []ReportSection{
		composeOverlayBuildChild(overlayCount, buildErrs),
		composeHooksChild(hookTable, hookFailed),
		composeKustomizeFixChild(fixNeeded, fixCheckErr, fixCheckEnabled, binaryName),
		composeGhostPatchesChild(ghostTable, ghostBlockingCount),
	}
	return composeParentFromChildren("Kustomize Build", children)
}

// composeOverlayBuildChild builds the "Overlay Build" sub-check: grouped
// build errors by root cause (groupBuildErrors/formatBuildErrors, so N
// overlays sharing one underlying error don't repeat it N times), or a
// passing summary with the built overlay count. buildErrs are raw error
// strings in this repo's real overlay-build format ("kustomize build
// <overlay>: <cause>", see pkg/overlay/overlay.go and comments.go's
// groupBuildErrors doc comment).
func composeOverlayBuildChild(overlayCount int, buildErrs []string) ReportSection {
	groups, other := groupBuildErrors(buildErrs)
	if len(groups) == 0 && len(other) == 0 {
		return ReportSection{Name: "Overlay Build", Status: StatusPassed, Summary: fmt.Sprintf("%d overlay(s) built successfully.", overlayCount)}
	}
	var b strings.Builder
	if len(groups) > 0 {
		formatBuildErrors(&b, groups)
	}
	for _, e := range other {
		fmt.Fprintf(&b, "> - %s\n", e)
	}
	return ReportSection{Name: "Overlay Build", Status: StatusError, Body: b.String()}
}

// composeHooksChild builds the "Hooks" sub-check from hookTable (a
// pre-rendered markdown matrix built by the caller from pkg/hook data via
// buildHookTable in build_wiring.go; empty means no app in scope defines
// any hook) and hookFailed (anyHookFailed in hook_wiring.go - whether any
// app's hook actually failed). A hook failure is always also folded into
// buildErrs, so composeOverlayBuildChild's "Overlay Build" child already
// reflects it too; this just gives the hook-matrix's own line a matching
// icon instead of always showing ✅ regardless of outcome.
func composeHooksChild(hookTable string, hookFailed bool) ReportSection {
	if hookTable == "" {
		return ReportSection{Name: "Hooks", Status: StatusPassed, Summary: "No hooks defined."}
	}
	status := StatusPassed
	if hookFailed {
		status = StatusError
	}
	return ReportSection{Name: "Hooks", Status: status, Body: hookTable}
}

// composeKustomizeFixChild builds the "Kustomize Fix" sub-check from the
// list of kustomization.yaml files kustomize.CheckFix found needing
// `kustomize edit fix --vars`, plus an actionable fix command per affected
// directory (`k8s-gitops-ci kustomize-fix -dir <dir>` - see
// cmd/k8s-gitops-ci/main.go's runKustomizeFix, which actually applies
// kustomize.Fix and writes the file(s) back, unlike this read-only check)
// - matching the "Fix command:" convention composeCheckChild's fixHints
// already use for Linting/Static Checks findings, so a reviewer never has
// to go find the right command themselves.
//
// checkErr is non-nil when CheckFix itself couldn't run (most commonly
// kustomize.ErrCLINotFound - see pkg/kustomize's package doc comment for
// why that's a hard failure here, not a graceful skip): this renders as
// its own StatusError body distinct from "no fix needed", since silently
// reporting a clean bill of health for a check that never actually ran
// would be worse than surfacing the failure. enabled is false when the
// "kustomize-fix" step itself was disabled (--disable-checks
// kustomize-fix - see stepKustomizeFix in phases.go), rendering a
// "Disabled." summary matching the same convention golangci/scaffold
// table use, rather than a misleading "up to date" nothing actually
// checked.
func composeKustomizeFixChild(fixNeeded []string, checkErr error, enabled bool, binaryName string) ReportSection {
	if !enabled {
		return ReportSection{Name: "Kustomize Fix", Status: StatusPassed, Summary: "Disabled."}
	}
	if checkErr != nil {
		return ReportSection{Name: "Kustomize Fix", Status: StatusError, Body: fmt.Sprintf("Could not check kustomization.yaml files: %s", checkErr)}
	}
	if len(fixNeeded) == 0 {
		return ReportSection{Name: "Kustomize Fix", Status: StatusPassed, Summary: "All kustomization.yaml files are up to date."}
	}
	if binaryName == "" {
		binaryName = defaultProviderBinary()
	}
	var b strings.Builder
	b.WriteString("The following files need `kustomize edit fix --vars`:\n\n")
	for _, f := range fixNeeded {
		fmt.Fprintf(&b, "- `%s`\n", f)
	}
	b.WriteString("\n**Fix command:**\n")
	seenDir := map[string]bool{}
	for _, f := range fixNeeded {
		dir := filepath.Dir(f)
		if seenDir[dir] {
			continue
		}
		seenDir[dir] = true
		fmt.Fprintf(&b, "- `%s kustomize-fix -dir %s`\n", binaryName, dir)
	}
	return ReportSection{Name: "Kustomize Fix", Status: StatusError, Body: b.String()}
}

// composeGhostPatchesChild builds the "Ghost Patches" sub-check from
// ghostTable (a pre-rendered markdown table built by the caller from
// pkg/ghostpatch data via buildGhostTable in build_wiring.go; empty means
// none found) and ghostBlockingCount (buildGhostTable's second return).
// Per docs/CI.md's "Ghost Patch Detection": a ghost patch on a
// kustomization.yaml whose patches section changed in this PR (and isn't
// itself newly added) is blocking (❌); a ghost patch that predates this PR
// or is on a brand-new overlay is surfaced for visibility only (⚠️), never
// promoted to an error.
func composeGhostPatchesChild(ghostTable string, ghostBlockingCount int) ReportSection {
	if ghostTable == "" {
		return ReportSection{Name: "Ghost Patches", Status: StatusPassed, Summary: "None detected."}
	}
	status := StatusWarning
	if ghostBlockingCount > 0 {
		status = StatusError
	}
	return ReportSection{Name: "Ghost Patches", Status: status, Body: ghostTable}
}

// ComposeScaffoldValidationSection renders scaffold validation results as
// five always-shown sub-dropdowns (Scaffold Drift, Scaffold Exec, Disabled
// Overlays, Pre-Existing Scaffold Drift, Cluster Coverage - see the
// composeXChild helpers below), via the same
// composeParentFromChildren/ReportSection
// machinery ComposeKustomizeBuildSection uses, for the same reason: a
// single icon-bearing <details> line per child, and a parent icon that
// correctly inherits the worst child status instead of collapsing to a
// binary ✅/❌.
func ComposeScaffoldValidationSection(driftSummary string, execErrors, missingClusters []string, preExistingDriftSummary string, disabledOverlays []string) ReportSection {
	children := []ReportSection{
		composeScaffoldDriftChild(driftSummary),
		composeScaffoldExecChild(execErrors),
		composeDisabledOverlaysChild(disabledOverlays),
		composePreExistingDriftChild(preExistingDriftSummary),
		composeClusterCoverageChild(missingClusters),
	}
	return composeParentFromChildren("Scaffold Validation", children)
}

// composeScaffoldDriftChild builds the blocking "Scaffold Drift" sub-check
// from driftSummary (pre-joined drift-line markdown; empty means none).
func composeScaffoldDriftChild(driftSummary string) ReportSection {
	if driftSummary == "" {
		return ReportSection{Name: "Scaffold Drift", Status: StatusPassed, Summary: "No drift detected."}
	}
	return ReportSection{Name: "Scaffold Drift", Status: StatusError, Body: driftSummary}
}

// composeScaffoldExecChild builds the blocking "Scaffold Exec" sub-check
// from scaffold-run execution errors (as opposed to detected drift).
func composeScaffoldExecChild(execErrors []string) ReportSection {
	if len(execErrors) == 0 {
		return ReportSection{Name: "Scaffold Exec", Status: StatusPassed, Summary: "All scaffold runs succeeded."}
	}
	var b strings.Builder
	for _, e := range execErrors {
		fmt.Fprintf(&b, "- %s\n", e)
	}
	return ReportSection{Name: "Scaffold Exec", Status: StatusError, Body: b.String()}
}

// composeDisabledOverlaysChild builds the warning "Disabled Overlays"
// sub-check from disabledOverlays - overlays this PR modified that scaffold
// validation skipped because their scaffold config marks them disabled
// (`overlayDefinitions.overrides.<cluster>.disabled`, see
// scaffold.OverlayConfigDisabled). Deliberately StatusWarning, never
// StatusError: it is an "are you sure you meant to edit a disabled overlay?"
// signal, not a blocking finding - the overlay is skipped by intent and the
// pipeline still passes, but the author is told to check/remove the
// disabled flag if that wasn't intended.
func composeDisabledOverlaysChild(disabledOverlays []string) ReportSection {
	if len(disabledOverlays) == 0 {
		return ReportSection{Name: "Disabled Overlays", Status: StatusPassed, Summary: "No modified overlay is disabled."}
	}
	var b strings.Builder
	b.WriteString("The following overlay(s) were modified by this PR but are marked `disabled: true` in their scaffold config, so scaffolding ")
	b.WriteString("was skipped for them. Remove `disabled: true` to enable scaffolding, or leave as-is if intentional:\n\n")
	for _, o := range disabledOverlays {
		fmt.Fprintf(&b, "- `%s`\n", o)
	}
	return ReportSection{Name: "Disabled Overlays", Status: StatusWarning, Body: b.String()}
}

// composePreExistingDriftChild builds the non-blocking "Pre-Existing
// Scaffold Drift" sub-check: overlays that also drift against the
// merge-base template/config (see computeBaselineMismatches in
// scaffold_wiring.go) and that this PR doesn't itself touch. Surfaced for
// visibility (⚠️), never promoted to an error - this PR isn't responsible
// for fixing it.
func composePreExistingDriftChild(preExistingDriftSummary string) ReportSection {
	if preExistingDriftSummary == "" {
		return ReportSection{Name: "Pre-Existing Scaffold Drift", Status: StatusPassed, Summary: "None detected."}
	}
	return ReportSection{Name: "Pre-Existing Scaffold Drift", Status: StatusWarning, Body: preExistingDriftSummary}
}

// composeClusterCoverageChild builds the "Cluster Coverage" sub-check from
// missingClusters - overlays scaffold.Run skipped rather than validated
// (disabled, or no on-disk directory yet: not yet rolled out, or removed by
// this PR). Deliberately StatusInfo rather than StatusWarning/StatusError:
// this is an informational "here's what wasn't checked and why" list, not
// a finding - see scaffold.Run's own doc comment ("skipped ... never
// Failed").
func composeClusterCoverageChild(missingClusters []string) ReportSection {
	if len(missingClusters) == 0 {
		return ReportSection{Name: "Cluster Coverage", Status: StatusPassed, Summary: "All clusters accounted for."}
	}
	var b strings.Builder
	b.WriteString("Skipped (not yet rolled out, or removed by this PR):\n\n")
	for _, c := range missingClusters {
		fmt.Fprintf(&b, "- `%s`\n", c)
	}
	return ReportSection{Name: "Cluster Coverage", Status: StatusInfo, Body: b.String()}
}

// ComposeDriftProtectionSection renders a warning for every app that has a
// scaffold template (drift detection is available for it) but hasn't opted
// into scaffold drift protection via test.sh (see
// scaffold.HasScaffoldEnabled/docs/HOOKS.md's SCAFFOLD directive) - see
// findUnprotectedApps in scaffold_wiring.go. Unlike Scaffold Validation
// (which reports drift for apps that ARE protected), this is the "you have
// no coverage here at all" gap, non-blocking (it's a coverage warning, not
// a drift finding) - StatusWarning, never StatusError.
func ComposeDriftProtectionSection(unprotectedApps []string) ReportSection {
	if len(unprotectedApps) == 0 {
		return ReportSection{Name: "Scaffold Drift Protection", Status: StatusPassed, Body: "All modified overlays with a scaffold template have drift protection enabled."}
	}

	var b strings.Builder
	b.WriteString("The following app(s) have a scaffold template but drift protection is not enabled ")
	b.WriteString("(`export SCAFFOLD=false` is set in `test.sh`), so scaffold drift is not being checked for them:\n\n")
	for _, app := range unprotectedApps {
		fmt.Fprintf(&b, "- `%s`\n", app)
	}

	return ReportSection{Name: "Scaffold Drift Protection", Status: StatusWarning, Body: b.String()}
}

// ComposeKyvernoSection renders the Kyverno subsection.
func ComposeKyvernoSection(body string) ReportSection {
	if body == "" {
		return ReportSection{Name: "Kyverno Policies", Status: StatusPassed, Body: "No Kyverno findings."}
	}
	return ReportSection{Name: "Kyverno Policies", Status: StatusError, Body: body}
}

// ComposeKubeconformRenderedSection renders schema-validation results over the
// AVP/Helm-rendered overlay output (see validateRenderedOverlays). Invalid/error
// counts gate with ❌; otherwise it passes with a compact summary. Non-manifest
// skips surface as an informational note so they're never silent.
func ComposeKubeconformRenderedSection(res *kubeconform.Result) ReportSection {
	const name = "Kubeconform (Rendered)"
	if res == nil {
		return ReportSection{Name: name, Status: StatusPassed, Body: "No rendered overlays to validate."}
	}
	if res.Invalid > 0 || res.Errors > 0 {
		return ReportSection{Name: name, Status: StatusError, Body: res.Summary()}
	}
	return ReportSection{Name: name, Status: StatusPassed, Summary: res.Summary()}
}

// ComposeCINotesSection renders CI notes. Purely informational (build
// metadata/reproduce-command, never a finding), so always StatusPassed.
func ComposeCINotesSection(body string) ReportSection {
	return ReportSection{Name: "CI Notes", Status: StatusPassed, Body: body}
}

// ComposeNADSection renders NetworkAttachmentDefinition validation results.
// Hard errors (nadErrors) gate the run and render ❌; advisory warnings
// (nadWarnings, e.g. an unrecognized CNI/IPAM type on a non-OVN NAD) are
// surfaced for visibility and render ⚠️ but never gate. The section rolls up
// to the worst present severity: StatusError if any hard error, else
// StatusWarning if any advisory, else StatusPassed.
func ComposeNADSection(nadErrors, nadWarnings []nad.ValidationError) ReportSection {
	const name = "NetworkAttachmentDefinition Validation"
	if len(nadErrors) == 0 && len(nadWarnings) == 0 {
		return ReportSection{Name: name, Status: StatusPassed, Body: "All NetworkAttachmentDefinition resources passed validation."}
	}

	var b strings.Builder
	if len(nadErrors) > 0 {
		fmt.Fprintf(&b, "**%d invalid NetworkAttachmentDefinition(s):**\n\n", len(nadErrors))
		b.WriteString("| File | Error |\n| --- | --- |\n")
		for _, e := range nadErrors {
			fmt.Fprintf(&b, "| %s | %s |\n", e.File, strings.ReplaceAll(e.Message, "|", "\\|"))
		}
	}
	if len(nadWarnings) > 0 {
		if len(nadErrors) > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "**%d NetworkAttachmentDefinition advisory warning(s)** (non-blocking):\n\n", len(nadWarnings))
		b.WriteString("| File | Warning |\n| --- | --- |\n")
		for _, w := range nadWarnings {
			fmt.Fprintf(&b, "| %s | %s |\n", w.File, strings.ReplaceAll(w.Message, "|", "\\|"))
		}
	}

	status := StatusWarning
	if len(nadErrors) > 0 {
		status = StatusError
	}
	return ReportSection{Name: name, Status: status, Body: b.String()}
}
