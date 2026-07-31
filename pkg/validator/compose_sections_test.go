package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

func TestComposePRChecksSection(t *testing.T) {
	s := ComposePRChecksSection(errors.New("title"), nil, nil)
	if !s.Error {
		t.Errorf("expected error section")
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
