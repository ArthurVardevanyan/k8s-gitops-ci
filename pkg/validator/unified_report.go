package validator

import (
	"fmt"
	"strings"
	"time"
)

// PRValidationResult carries the outcomes of PR-level checks (title, signing,
// checklist) from the pipeline layer into validator.RunAll so they appear in
// the unified report.
type PRValidationResult struct {
	// TitlePassed is true if PR title follows conventions.
	TitlePassed bool
	// TitleMsg is a brief description when there's a title issue.
	TitleMsg string
	// TitleBlocking is true when the title error should fail the pipeline.
	TitleBlocking bool

	// CommitsPassed is true if all commits are signed.
	CommitsPassed bool
	// UnsignedCount is the number of unsigned commits.
	UnsignedCount int
	// TotalCommits is the total number of commits in the PR.
	TotalCommits int

	// ChecklistPassed is true if the PR checklist is valid.
	ChecklistPassed bool
	// ChecklistMsg is the checklist error message if any.
	ChecklistMsg string
}

// Report renders the unified PR comment.
type Report struct {
	Marker    string
	Title     string
	Header    string
	Body      string
	Sections  []ReportSection
	Timestamp time.Time
}

// SectionStatus represents the outcome of a report section. Unlike a bare
// pass/fail bool, it distinguishes an accepted (non-blocking) exception -
// StatusInfo - from an active warning or a blocking error, so a section can
// render "0 blocking findings, 2 accepted exceptions" instead of collapsing
// both into the same "passed" bucket.
type SectionStatus int

// Section statuses, ordered from least to most severe.
const (
	StatusPassed SectionStatus = iota
	StatusInfo
	StatusWarning
	StatusError
)

// Icon returns the emoji icon for a status.
func (s SectionStatus) Icon() string {
	switch s {
	case StatusPassed:
		return "✅"
	case StatusInfo:
		return "ℹ️"
	case StatusWarning:
		return "⚠️"
	case StatusError:
		return "❌"
	default:
		return "⚪"
	}
}

// StatusIcon returns the icon for a status. Kept as a package-level function
// (in addition to SectionStatus.Icon) since it's the call shape this repo's
// existing callers/tests already use.
func StatusIcon(status SectionStatus) string { return status.Icon() }

// CheckOutcome records whether an individual lint/static check ran, was
// skipped, or passed. The Linting and Static Checks sections use these to
// always show a full sub-check breakdown (each check with ✅/⚠️/❌) even when
// everything passed, since the absence of a finding alone can't distinguish
// "passed" from "skipped/not run".
type CheckOutcome struct {
	Name    string        // Raw check name (e.g. "markdownlint")
	Status  SectionStatus // Pass/Warning/Error (failures also carry a finding elsewhere)
	Skipped bool          // True when the check did not run (no applicable files / disabled)
	Note    string        // Short note shown when expanded (e.g. "No markdown files changed.")
}

// ReportSection represents a single collapsible report section - both a
// top-level "Expand: <Name>" section (Report.Sections) and a nested
// sub-dropdown beneath one (via renderSubDropdown). It carries a full
// SectionStatus (rather than a bare pass/fail bool) plus a short Summary
// shown when the section passed and there's no need for a full Body, so a
// section can render "0 blocking findings, 2 accepted exceptions" instead
// of collapsing everything into the same binary pass/fail icon - and so a
// parent section's icon can correctly inherit the worst status among its
// children (composeParentFromChildren) instead of only ever showing ✅/❌
// and silently hiding an internal ⚠️/ℹ️.
type ReportSection struct {
	Name    string        // Display name (e.g. "Markdownlint")
	Status  SectionStatus // Pass/Warning/Error
	Summary string        // Brief note shown when expanded and Body is empty (e.g. "Passed.")
	Body    string        // Full markdown content (tables, code fences, etc.) - empty when passed
}

// Render produces the markdown report.
func (r *Report) Render() string {
	var b strings.Builder
	b.WriteString(r.Marker + "\n")
	// Only Title is rendered as the comment's single heading. Header
	// (org-injectable via Providers.PipelineHeader()) is kept on the
	// struct for other consumers (e.g. the console banner in
	// pkg/pipeline), but rendering both here previously produced two
	// redundant top-level headings ("# GitOps CI Pipeline" followed
	// immediately by "## GitOps CI Results") in every PR comment.
	fmt.Fprintf(&b, "## %s\n\n", r.Title)
	if !r.Timestamp.IsZero() {
		fmt.Fprintf(&b, "_Last Updated: %s_\n\n", r.Timestamp.UTC().Format(time.RFC3339))
	}
	for _, s := range r.Sections {
		body := s.Body
		if body == "" {
			body = s.Summary
		}
		fmt.Fprintf(&b, "<details>\n<summary>%s Expand: %s</summary>\n\n%s\n\n</details>\n\n", s.Status.Icon(), s.Name, body)
	}
	if r.Body != "" {
		b.WriteString(r.Body + "\n")
	}
	return b.String()
}

// LegacyMarkers returns markers to clean up after posting the unified comment.
func LegacyMarkers() []string {
	return []string{
		"<!-- resource-compliance-warnings -->",
		"<!-- ci-error-summary -->",
		"<!-- sync-options-warning -->",
		"<!-- psa-namespace-labels -->",
		"<!-- pr-title-convention-warning -->",
		"<!-- unsigned-commits-warning -->",
		"<!-- drift-protection-warning -->",
		"<!-- unresolved-placeholders -->",
		"<!-- external-drift-placeholders -->",
		"<!-- shellcheck-external-warning -->",
		"<!-- ci-notes -->",
		"<!-- podspec-defaults-warning -->",
		"<!-- rbac-wildcard-warning -->",
		"<!-- missing-clusters-warning -->",
		"<!-- scaffold-pre-existing-drift -->",
	}
}

// ReproduceCommand returns a shell snippet to reproduce the run locally.
//
// This must stay in sync with every Options field that changes what actually
// gets validated (as opposed to purely cosmetic/output fields like Verbose) -
// otherwise the printed command silently reproduces a different, narrower
// run than the one that actually failed. Dirs (--dirs) in particular
// replaces the whole changeset with a full-tree walk of the given paths, so
// omitting it here previously meant "reproduce locally" could pass locally
// while the original run (scoped to specific directories) failed, or vice
// versa. AssumeOpenShift is the same kind of gap: it exempts OpenShift/OKD
// API groups from the sync-options check, so a caller (e.g. a Tekton Task
// that always passes --assume-openshift) whose run isn't reflected here
// could see different sync-options findings locally than the original run.
//
// --target-branch (BaseRef) is deliberately omitted: in PR mode the pipeline
// resolves the base ref from the PR itself (see resolveBaseRef in
// pkg/pipeline), so re-passing the original run's resolved BaseRef here would
// just be redundant noise, not something needed to reproduce the failure.
func ReproduceCommand(opts Options) string {
	cmd := fmt.Sprintf("%s pipeline --url=%q --pr=%s", opts.Providers.BinaryName(), opts.RepoURL, opts.PR)
	if len(opts.Dirs) > 0 {
		cmd += fmt.Sprintf(" --dirs=%q", strings.Join(opts.Dirs, ","))
	}
	if opts.AssumeOpenShift {
		cmd += " --assume-openshift"
	}
	if len(opts.DisabledChecks) > 0 {
		cmd += fmt.Sprintf(" --disable-checks=%q", strings.Join(opts.DisabledChecks, ","))
	}
	if len(opts.EnabledChecks) > 0 {
		cmd += fmt.Sprintf(" --enable-checks=%q", strings.Join(opts.EnabledChecks, ","))
	}
	return cmd
}
