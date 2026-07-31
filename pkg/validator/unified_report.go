package validator

import (
	"fmt"
	"strings"
	"time"
)

// Report renders the unified PR comment.
type Report struct {
	Marker    string
	Title     string
	Header    string
	Body      string
	Sections  []Section
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

// ReportSection represents a single collapsible sub-section that can be
// nested under a top-level Section via renderSubDropdown. Unlike the
// top-level Section type (Name/Body/bool Error), it carries a full
// SectionStatus plus a short Summary shown when the section passed and
// there's no need for a full Body.
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
	if r.Header != "" {
		fmt.Fprintf(&b, "# %s\n\n", r.Header)
	}
	fmt.Fprintf(&b, "## %s\n\n", r.Title)
	if !r.Timestamp.IsZero() {
		fmt.Fprintf(&b, "_Last Updated: %s_\n\n", r.Timestamp.UTC().Format(time.RFC3339))
	}
	for _, s := range r.Sections {
		status := StatusPassed
		if s.Error {
			status = StatusError
		}
		fmt.Fprintf(&b, "<details>\n<summary>%s Expand: %s</summary>\n\n%s\n\n</details>\n\n", status.Icon(), s.Name, s.Body)
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
func ReproduceCommand(opts Options) string {
	return fmt.Sprintf("go run ./cmd/k8s-gitops-ci pipeline --url=%q --pr=%s --target-branch=%q", opts.RepoURL, opts.PR, opts.BaseRef)
}
