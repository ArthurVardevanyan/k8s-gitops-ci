package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
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
	s := ComposeResourceComplianceSection([]check.Finding{{CheckID: "x", Message: "m"}})
	if !s.Error {
		t.Errorf("expected error section")
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
