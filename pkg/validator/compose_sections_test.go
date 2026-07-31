package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

func TestComposePRChecksSection(t *testing.T) {
	s := ComposePRChecksSection(errors.New("title"), nil, nil, "")
	if !s.Error {
		t.Errorf("expected error section")
	}
}

// TestComposePRChecksSection_EachCheckIsItsOwnSubDropdown guards that PR
// Title/Signed Commits/PR Checklist each render as their own independent
// collapsible sub-dropdown (via composeParentFromChildren, matching
// ComposeLintingSection/ComposeStaticChecksSection's convention) rather
// than a single flat bullet list, so a reader can tell exactly which
// check(s) failed at a glance and expand only the failing one(s).
func TestComposePRChecksSection_EachCheckIsItsOwnSubDropdown(t *testing.T) {
	s := ComposePRChecksSection(errors.New("bad title"), nil, errors.New("missing checklist item"), "")
	if strings.Count(s.Body, "<details>") != 3 {
		t.Errorf("expected 3 sub-dropdowns (one per PR check), got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "PR Title") || !strings.Contains(s.Body, "bad title") {
		t.Errorf("expected the PR Title sub-dropdown with its error, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "Signed Commits") {
		t.Errorf("expected a Signed Commits sub-dropdown even when it passed, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "PR Checklist") || !strings.Contains(s.Body, "missing checklist item") {
		t.Errorf("expected the PR Checklist sub-dropdown with its error, got:\n%s", s.Body)
	}
}

func TestComposePRChecksSection_AllPassed(t *testing.T) {
	s := ComposePRChecksSection(nil, nil, nil, "")
	if s.Error {
		t.Errorf("expected no error when all three checks pass")
	}
	if strings.Count(s.Body, "Passed.") != 3 {
		t.Errorf("expected all 3 checks to report Passed., got:\n%s", s.Body)
	}
}

// TestComposePRChecksSection_TitleSuggestionIsNonBlocking guards that a
// non-empty titleSuggestion (see github.PRTitleSuggestion) surfaces as a
// warning note on an otherwise-passing "PR Title" child, without ever
// tripping the section's blocking Error flag - an org's optional title
// convention must never fail the pipeline the way titleErr does.
func TestComposePRChecksSection_TitleSuggestionIsNonBlocking(t *testing.T) {
	s := ComposePRChecksSection(nil, nil, nil, "consider referencing a ticket, e.g. JIRA-123")
	if s.Error {
		t.Errorf("expected a title suggestion to never be blocking")
	}
	if !strings.Contains(s.Body, "PR Title") || !strings.Contains(s.Body, "consider referencing a ticket") {
		t.Errorf("expected the PR Title sub-dropdown to carry the suggestion, got:\n%s", s.Body)
	}
}

// TestComposePRChecksSection_TitleErrSuppressesSuggestion guards that a
// blocking titleErr is never diluted by also showing titleSuggestion -
// callers (github.PRTitleSuggestion) already withhold a suggestion once
// the required check fails, but the render layer must not surface a
// stale/mistakenly-passed suggestion string alongside a hard failure either.
func TestComposePRChecksSection_TitleErrSuppressesSuggestion(t *testing.T) {
	s := ComposePRChecksSection(errors.New("bad title"), nil, nil, "consider referencing a ticket")
	if !s.Error {
		t.Errorf("expected the section to report an error")
	}
	if strings.Contains(s.Body, "consider referencing a ticket") {
		t.Errorf("expected a blocking title error to suppress the suggestion, got:\n%s", s.Body)
	}
}

func TestComposeLintingSection(t *testing.T) {
	outcomes := []CheckOutcome{{Name: "golangci", Status: StatusError}}
	s := ComposeLintingSection(outcomes, map[string]string{"golangci": "issues"})
	if !s.Error {
		t.Errorf("expected error section")
	}
	if !strings.Contains(s.Body, "issues") {
		t.Errorf("expected failure report body in section, got:\n%s", s.Body)
	}
}

func TestComposeLintingSection_AllPassedStillShowsFullBreakdown(t *testing.T) {
	outcomes := []CheckOutcome{
		{Name: "markdownlint", Status: StatusPassed, Skipped: true, Note: "No markdown files changed."},
		{Name: "prettier", Status: StatusPassed},
		{Name: "shellcheck", Status: StatusPassed},
		{Name: "golangci", Status: StatusPassed},
		{Name: "kubeconform", Status: StatusPassed},
	}
	s := ComposeLintingSection(outcomes, map[string]string{})
	if s.Error {
		t.Errorf("expected no error section")
	}
	for _, name := range []string{"Markdownlint", "Prettier", "Shellcheck", "golangci-lint", "Kubeconform"} {
		if !strings.Contains(s.Body, name) {
			t.Errorf("expected every linter to always render its own sub-dropdown, missing %q in:\n%s", name, s.Body)
		}
	}
	if !strings.Contains(s.Body, "No markdown files changed.") {
		t.Errorf("expected the skipped markdownlint note to render, got:\n%s", s.Body)
	}
}

func TestComposeLintingSection_MissingOutcomeRendersAsNotRun(t *testing.T) {
	// No outcomes recorded at all (e.g. lint phase didn't run) - every check
	// must still render, as a non-failing "Not run." child, rather than
	// silently vanishing from the report.
	s := ComposeLintingSection(nil, map[string]string{})
	if s.Error {
		t.Errorf("expected no error section")
	}
	if !strings.Contains(s.Body, "Not run.") {
		t.Errorf("expected a 'Not run.' child for every check with no recorded outcome, got:\n%s", s.Body)
	}
}

func TestComposeStaticChecksSection(t *testing.T) {
	s := ComposeStaticChecksSection(nil, map[string]string{})
	if s.Error {
		t.Errorf("expected no error section")
	}
}

func TestComposeStaticChecksSection_FailureIncludesFixHint(t *testing.T) {
	outcomes := []CheckOutcome{{Name: "config-sort", Status: StatusError}}
	s := ComposeStaticChecksSection(outcomes, map[string]string{"config-sort": "some.yaml is unsorted"})
	if !s.Error {
		t.Errorf("expected error section")
	}
	if !strings.Contains(s.Body, "k8s-gitops-ci sort-configs") {
		t.Errorf("expected the config-sort fix hint in the section body, got:\n%s", s.Body)
	}
}

// TestComposeStaticChecksSection_ScaffoldTableFailureIncludesFixHint guards
// that the "scaffold table" static check - wired in phases.go to
// scaffold.CheckReadmeStatus, named to match hintByCheck's "scaffold table"
// key - automatically gets its update-scaffold-status fix-command hint the
// same way every other named static check does, with no bespoke rendering
// needed.
func TestComposeStaticChecksSection_ScaffoldTableFailureIncludesFixHint(t *testing.T) {
	outcomes := []CheckOutcome{{Name: "scaffold table", Status: StatusError}}
	s := ComposeStaticChecksSection(outcomes, map[string]string{"scaffold table": "stale entries no longer on disk: myapp/removed"})
	if !s.Error {
		t.Errorf("expected error section")
	}
	if !strings.Contains(s.Body, "Scaffold Table") {
		t.Errorf("expected the Scaffold Table display name, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "k8s-gitops-ci update-scaffold-status") {
		t.Errorf("expected the scaffold table fix hint in the section body, got:\n%s", s.Body)
	}
}

// TestComposeStaticChecksSection_ScaffoldTableDisabledByDefault guards that
// the "scaffold table" check, when phases.go's default-off gating leaves it
// unenabled, renders as "Disabled." (matching golangci's same convention)
// rather than the generic "Not run." a check phases.go never even attempted
// to record an outcome for would get.
func TestComposeStaticChecksSection_ScaffoldTableDisabledByDefault(t *testing.T) {
	outcomes := []CheckOutcome{{Name: "scaffold table", Status: StatusPassed, Skipped: true, Note: "Disabled."}}
	s := ComposeStaticChecksSection(outcomes, map[string]string{})
	if s.Error {
		t.Errorf("expected no error section")
	}
	if !strings.Contains(s.Body, "Disabled.") {
		t.Errorf("expected the Disabled. note, got:\n%s", s.Body)
	}
}

func TestComposeResourceComplianceSection(t *testing.T) {
	s := ComposeResourceComplianceSection([]check.Finding{{CheckID: "x", Message: "m"}}, nil, nil)
	if !s.Error {
		t.Errorf("expected error section")
	}
}

func TestComposeResourceComplianceSection_NoFindingsOrExemptions(t *testing.T) {
	s := ComposeResourceComplianceSection(nil, nil, nil)
	if s.Error {
		t.Errorf("expected no error section")
	}
	if !strings.Contains(s.Body, "No compliance findings.") {
		t.Errorf("expected the no-findings message, got:\n%s", s.Body)
	}
}

func TestComposeResourceComplianceSection_WarningOnlyIsNonBlocking(t *testing.T) {
	s := ComposeResourceComplianceSection(nil, []check.Finding{{CheckID: "image-checksum", File: "a.yaml", Message: "unpinned"}}, nil)
	if s.Error {
		t.Errorf("expected no error section for pre-existing (indirect) findings only")
	}
	if !strings.Contains(s.Body, "⚠️") {
		t.Errorf("expected the warning icon for a non-blocking check, got:\n%s", s.Body)
	}
}

func TestComposeResourceComplianceSection_GroupsByCheckID(t *testing.T) {
	findings := []check.Finding{
		{CheckID: "image-checksum", File: "a.yaml", Message: "unpinned a"},
		{CheckID: "image-checksum", File: "b.yaml", Message: "unpinned b"},
		{CheckID: "rbac-wildcard", File: "c.yaml", Message: "wildcard verb"},
	}
	s := ComposeResourceComplianceSection(findings, nil, nil)
	if strings.Count(s.Body, "<details>") != 2 {
		t.Errorf("expected exactly 2 per-check dropdowns (one per distinct CheckID), got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "(2 finding(s))") {
		t.Errorf("expected the image-checksum group to report 2 findings, got:\n%s", s.Body)
	}
}

// TestComposeResourceComplianceSection_UsesRegisteredTableSpec guards that a
// registered check id (register_tables.go) renders its own richer columns
// and descriptive Preamble via RenderColumnedTable, instead of the generic
// flat File/Message table.
func TestComposeResourceComplianceSection_UsesRegisteredTableSpec(t *testing.T) {
	findings := []check.Finding{
		{CheckID: "namespace", Kind: "Pod", Name: "my-pod", File: "a.yaml", Message: "missing namespace"},
	}
	s := ComposeResourceComplianceSection(findings, nil, nil)
	if !strings.Contains(s.Body, "my-pod") {
		t.Errorf("expected the Name column to render, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "Resources that are missing a `namespace` field") {
		t.Errorf("expected the namespace TableSpec's descriptive Preamble to render, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "Namespace Scope") {
		t.Errorf("expected the namespace TableSpec's descriptive Title, got:\n%s", s.Body)
	}
}

// TestComposeResourceComplianceSection_UnregisteredCheckFallsBack guards
// that a check id with no TableSpec entry (e.g. brand new, not yet given
// one) still renders something useful via the generic File/Message
// fallback, rather than being silently dropped.
func TestComposeResourceComplianceSection_UnregisteredCheckFallsBack(t *testing.T) {
	findings := []check.Finding{{CheckID: "brand-new-check", File: "a.yaml", Message: "some issue"}}
	s := ComposeResourceComplianceSection(findings, nil, nil)
	if !strings.Contains(s.Body, "| File | Message |") {
		t.Errorf("expected the generic fallback table, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "a.yaml") || !strings.Contains(s.Body, "some issue") {
		t.Errorf("expected the finding to still render, got:\n%s", s.Body)
	}
}

// TestComposeResourceComplianceSection_DedupsFanOutAcrossFiles guards the
// core Phase 2 richness fix: the same underlying finding fanned out across
// multiple overlays (identical Kind/Name/Message, different File - see
// engine.go's per-unique-document fan-out) renders as ONE table row listing
// every affected file, not one row per file, while the header count above
// the table still reports every raw (pre-dedup) finding.
func TestComposeResourceComplianceSection_DedupsFanOutAcrossFiles(t *testing.T) {
	findings := []check.Finding{
		{CheckID: "namespace", Kind: "Pod", Name: "my-pod", File: "overlays/dev/a.yaml", Message: "missing namespace"},
		{CheckID: "namespace", Kind: "Pod", Name: "my-pod", File: "overlays/prod/a.yaml", Message: "missing namespace"},
	}
	s := ComposeResourceComplianceSection(findings, nil, nil)
	if !strings.Contains(s.Body, "(2 finding(s))") {
		t.Errorf("expected the raw (pre-dedup) count of 2 in the header, got:\n%s", s.Body)
	}
	if got := strings.Count(s.Body, "my-pod"); got != 1 {
		t.Errorf("expected exactly 1 deduped row for the same resource/issue, got %d occurrences in:\n%s", got, s.Body)
	}
	if !strings.Contains(s.Body, "`overlays/dev/a.yaml`, `overlays/prod/a.yaml`") {
		t.Errorf("expected both files listed, backtick-quoted, in the deduped row, got:\n%s", s.Body)
	}
}

// TestComposeResourceComplianceSection_DoesNotOverDedup guards that findings
// for the same resource but genuinely distinct issues (different Message)
// are NOT collapsed together - only true fan-out duplicates are deduped.
func TestComposeResourceComplianceSection_DoesNotOverDedup(t *testing.T) {
	findings := []check.Finding{
		{CheckID: "image-checksum", File: "a.yaml", Message: "unpinned a"},
		{CheckID: "image-checksum", File: "b.yaml", Message: "unpinned b"},
	}
	s := ComposeResourceComplianceSection(findings, nil, nil)
	// Distinct Messages (part of the dedup key) must keep these as two
	// separate rows rather than merging into one comma-joined-File row.
	if !strings.Contains(s.Body, "`a.yaml`") || !strings.Contains(s.Body, "`b.yaml`") {
		t.Errorf("expected both files to render, got:\n%s", s.Body)
	}
	if strings.Contains(s.Body, "`a.yaml`, `b.yaml`") {
		t.Errorf("expected two separate rows, not one deduped/merged row, got:\n%s", s.Body)
	}
}

func TestComposeResourceComplianceSection_RendersAcceptedExceptions(t *testing.T) {
	exempted := []exempt.Applied{
		{CheckID: "image-checksum", Kind: "Deployment", Name: "app", Value: "nginx:latest", Direct: true},
	}
	s := ComposeResourceComplianceSection(nil, nil, exempted)
	if s.Error {
		t.Errorf("expected no error section when only exemptions are present (no findings)")
	}
	if !strings.Contains(s.Body, "Accepted Exceptions") {
		t.Errorf("expected an Accepted Exceptions block, got:\n%s", s.Body)
	}
	if strings.Contains(s.Body, "(pre-existing)") {
		t.Errorf("a directly-applied exemption should not be labeled pre-existing, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "directly modified") {
		t.Errorf("expected the exemption's scope column to say 'directly modified', got:\n%s", s.Body)
	}
}

func TestComposeResourceComplianceSection_AcceptedExceptionsPreExistingLabel(t *testing.T) {
	exempted := []exempt.Applied{
		{CheckID: "image-checksum", Kind: "Deployment", Name: "app", Value: "nginx:latest", Direct: false},
	}
	s := ComposeResourceComplianceSection(nil, nil, exempted)
	if !strings.Contains(s.Body, "Accepted Exceptions (pre-existing)") {
		t.Errorf("expected the pre-existing qualifier when no exemption is direct, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "pre-existing") {
		t.Errorf("expected the exemption's scope column to say 'pre-existing', got:\n%s", s.Body)
	}
}

func TestComposeKyvernoSection(t *testing.T) {
	s := ComposeKyvernoSection("")
	if s.Error {
		t.Errorf("expected no error section")
	}
}

func TestRenderSubDropdown(t *testing.T) {
	out := RenderSubDropdown("Title", "Body")
	if out == "" {
		t.Errorf("expected dropdown")
	}
}

func TestSummaryIndent(t *testing.T) {
	if got := summaryIndent(0); got != "" {
		t.Errorf("summaryIndent(0) = %q, want empty (top-level sections aren't indented)", got)
	}
	if got := summaryIndent(-1); got != "" {
		t.Errorf("summaryIndent(-1) = %q, want empty", got)
	}
	one := summaryIndent(1)
	if one == "" {
		t.Fatal("summaryIndent(1) = \"\", want a non-empty indent")
	}
	two := summaryIndent(2)
	if two != one+one {
		t.Errorf("summaryIndent(2) = %q, want two repetitions of summaryIndent(1) (%q)", two, one)
	}
}

func TestRenderSubDropdown_ReportSection_Nested(t *testing.T) {
	var sb strings.Builder
	renderSubDropdown(&sb, ReportSection{Name: "Child", Status: StatusError, Body: "details here"}, 1)
	out := sb.String()
	if !strings.Contains(out, "❌ Child") {
		t.Errorf("expected the status icon + name in the summary, got:\n%s", out)
	}
	if !strings.Contains(out, "details here") {
		t.Errorf("expected the Body to be rendered, got:\n%s", out)
	}
	if !strings.Contains(out, summaryIndent(1)) {
		t.Errorf("expected the depth-1 indent prefix, got:\n%s", out)
	}
}

func TestRenderSubDropdown_ReportSection_UsesSummaryWhenBodyEmpty(t *testing.T) {
	var sb strings.Builder
	renderSubDropdown(&sb, ReportSection{Name: "Child", Status: StatusPassed, Summary: "Passed."}, 1)
	out := sb.String()
	if !strings.Contains(out, "✅ Child") {
		t.Errorf("expected passed icon + name, got:\n%s", out)
	}
	if !strings.Contains(out, "Passed.") {
		t.Errorf("expected the Summary to render when Body is empty, got:\n%s", out)
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName("markdownlint"); got != "Markdownlint" {
		t.Errorf("displayName(markdownlint) = %q, want Markdownlint", got)
	}
	if got := displayName("golangci"); got != "golangci-lint" {
		t.Errorf("displayName(golangci) = %q, want golangci-lint", got)
	}
	if got := displayName("some-unknown-check"); got != "Some-unknown-check" {
		t.Errorf("displayName(some-unknown-check) = %q, want a titleCase fallback", got)
	}
}
