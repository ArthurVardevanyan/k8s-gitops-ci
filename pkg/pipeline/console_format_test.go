package pipeline

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close() //nolint:errcheck
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

// TestSanitizeSectionBodyForConsole_StripsGitHubMarkdown guards against a
// regression where --verbose console output for a failed section (e.g.
// Resource Compliance) dumped its raw GitHub-comment markdown - literal
// <details>/<summary> dropdown tags and &nbsp; indentation - straight to
// the terminal instead of readable plain text. Fixture mirrors real output
// composed by pkg/validator/compose_sections.go's Resource Compliance
// section.
func TestSanitizeSectionBodyForConsole_StripsGitHubMarkdown(t *testing.T) {
	body := "If the affected resource is being modified in this PR, these issues **must** be corrected.\n" +
		"Otherwise, these are non-blocking warnings for pre-existing issues.\n\n" +
		"<details>\n<summary>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;❌ Image Digest Pinning (1 finding(s))</summary>\n\n" +
		"Container images not pinned to a SHA256 digest.\n\n" +
		"| Kind | Name | Image | File |\n| --- | --- | --- | --- |\n" +
		"| CronJob | renovate-bot | registry.example.com/toolbox:not_latest | `kubernetes/renovate/base/cronjob.yaml` |\n\n" +
		"</details>\n\n" +
		"<details>\n<summary>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;❌ PodSpec Defaults (2 finding(s))</summary>\n\n" +
		"Pods missing required resource requests/limits or security context fields.\n\n" +
		"</details>\n"

	got := SanitizeSectionBodyForConsole(body)

	for _, unwanted := range []string{"<details>", "</details>", "<summary>", "</summary>", "&nbsp;", "**"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("expected sanitized output to not contain %q, got: %s", unwanted, got)
		}
	}
	for _, want := range []string{
		"must be corrected",
		"❌ Image Digest Pinning (1 finding(s)):",
		"❌ PodSpec Defaults (2 finding(s)):",
		"Container images not pinned to a SHA256 digest.",
		"| CronJob | renovate-bot |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected sanitized output to contain %q, got: %s", want, got)
		}
	}
}

// TestSanitizeSectionBodyForConsole_PlainBodyUnchanged ensures a body with
// no markdown-dropdown artifacts (the common case for most section types)
// passes through with only whitespace trimmed.
func TestSanitizeSectionBodyForConsole_PlainBodyUnchanged(t *testing.T) {
	got := SanitizeSectionBodyForConsole("kustomize build apps/foo/overlays/bar: some error")
	want := "kustomize build apps/foo/overlays/bar: some error"
	if got != want {
		t.Errorf("SanitizeSectionBodyForConsole() = %q, want %q", got, want)
	}
}

// TestSanitizeSectionBodyForConsole_CollapsesBlankRuns ensures stripping
// tag-only lines (<details>/</details>) doesn't leave behind runs of 3+
// blank lines between dropdown sections.
func TestSanitizeSectionBodyForConsole_CollapsesBlankRuns(t *testing.T) {
	body := "<details>\n<summary>A</summary>\n\nbody A\n\n</details>\n\n<details>\n<summary>B</summary>\n\nbody B\n\n</details>"
	got := SanitizeSectionBodyForConsole(body)
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("expected no 3+ newline runs, got: %q", got)
	}
	for _, want := range []string{"A:", "body A", "B:", "body B"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got: %s", want, got)
		}
	}
}

// TestSectionHasConsoleDetail verifies the single rule all console entry
// points (pipeline, test) share for deciding which sections
// print their full per-finding Body: errored (❌), warned (⚠️), or info (ℹ️)
// sections with a non-empty body. Info covers accepted exemptions - "nothing
// wrong, but here's an audit trail" - which should surface in quiet mode.
// Anything else (passed, or empty/whitespace body) must render as a terse
// summary or be omitted.
func TestSectionHasConsoleDetail(t *testing.T) {
	cases := []struct {
		name string
		s    validator.ReportSection
		want bool
	}{
		{
			name: "error with body",
			s:    validator.ReportSection{Name: "RC", Status: validator.StatusError, Body: "table"},
			want: true,
		},
		{
			name: "warning with body",
			s:    validator.ReportSection{Name: "RC", Status: validator.StatusWarning, Body: "table"},
			want: true,
		},
		{
			name: "error with empty body",
			s:    validator.ReportSection{Name: "RC", Status: validator.StatusError, Body: ""},
			want: false,
		},
		{
			name: "warning with whitespace body",
			s:    validator.ReportSection{Name: "RC", Status: validator.StatusWarning, Body: "   \n  "},
			want: false,
		},
		{
			name: "passed with body",
			s:    validator.ReportSection{Name: "RC", Status: validator.StatusPassed, Body: "table"},
			want: false,
		},
		{
			name: "info with body",
			s:    validator.ReportSection{Name: "RC", Status: validator.StatusInfo, Body: "table"},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SectionHasConsoleDetail(tc.s); got != tc.want {
				t.Errorf("SectionHasConsoleDetail(%+v) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

// markdownSection returns a validator.ReportSection whose Body contains the
// GitHub-PR-comment markdown artifacts (<details>/<summary>, &nbsp;, **bold**)
// built by pkg/validator/compose_sections.go, mirroring real output.
func markdownSection(name string, isError bool) validator.ReportSection {
	status := validator.StatusPassed
	if isError {
		status = validator.StatusError
	}
	return validator.ReportSection{
		Name: name,
		Body: "Some intro **bold** text.\n\n" +
			"<details>\n<summary>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;❌ Finding (1 finding(s))</summary>\n\n" +
			"| Kind | Name |\n| --- | --- |\n| Deployment | example |\n\n</details>\n",
		Status: status,
	}
}

// markdownSectionStatus builds a markdown-bodied section with an explicit
// status, so tests can exercise StatusWarning sections directly (the
// warning-finding detail path) rather than only error/passed fixtures.
func markdownSectionStatus(name string, status validator.SectionStatus) validator.ReportSection {
	return validator.ReportSection{
		Name: name,
		Body: "Some intro **bold** text.\n\n" +
			"<details>\n<summary>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;⚠️ Finding (1 finding(s))</summary>\n\n" +
			"| Kind | Name |\n| --- | --- |\n| Deployment | example |\n\n</details>\n",
		Status: status,
	}
}

// TestPrintAllSectionsConsole_StripsGitHubMarkdown guards against a
// regression (see docs/CI.md) where test/build-yaml dumped raw
// PR-comment markdown - literal <details>/<summary> tags, &nbsp;, and **bold**
// - straight into the terminal instead of console-sanitized plain text,
// interleaved unreadably with the plain "[INFO]/[ERROR]" logger lines.
func TestPrintAllSectionsConsole_StripsGitHubMarkdown(t *testing.T) {
	sections := []validator.ReportSection{markdownSection("ResourceCompliance", true)}

	out := captureStdout(t, func() { PrintAllSectionsConsole(nil, sections) })

	for _, unwanted := range []string{"<details>", "</details>", "<summary>", "</summary>", "&nbsp;", "**"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("PrintAllSectionsConsole output must not contain %q, got: %s", unwanted, out)
		}
	}
	for _, want := range []string{"--- ResourceCompliance ---", "❌ Finding (1 finding(s)):", "| Deployment | example |"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintAllSectionsConsole output missing %q, got: %s", want, out)
		}
	}
}

// TestPrintAllSectionsConsole_PassingSectionIsOneLine guards against a
// regression where a passing section's full body was dumped a second time
// (in a different style) after the per-check "[INFO] X: passed" line had
// already been streamed live by the Logger during RunAll - see
// PrintAllSectionsConsole's doc comment. Passing sections must render as a
// single terse summary line, not their full (duplicate) Body.
func TestPrintAllSectionsConsole_PassingSectionIsOneLine(t *testing.T) {
	sections := []validator.ReportSection{markdownSection("Linting", false)}

	out := captureStdout(t, func() { PrintAllSectionsConsole(nil, sections) })

	want := "✅ Linting: passed"
	if strings.TrimSpace(out) != want {
		t.Errorf("PrintAllSectionsConsole(passing) = %q, want %q", out, want)
	}
}

// TestPrintQuietSectionsConsole_StripsGitHubMarkdownAndFiltersPassing
// mirrors TestPrintAllSectionsConsole_StripsGitHubMarkdown for quiet
// mode's renderer, and additionally checks that passing (non-Error) sections
// are omitted entirely (not even a one-line summary - quiet mode only
// reports failures).
func TestPrintQuietSectionsConsole_StripsGitHubMarkdownAndFiltersPassing(t *testing.T) {
	sections := []validator.ReportSection{
		markdownSection("Passing", false),
		markdownSection("Failing", true),
	}

	out := captureStdout(t, func() { PrintQuietSectionsConsole(nil, sections) })

	for _, unwanted := range []string{"<details>", "</details>", "<summary>", "&nbsp;", "**", "Passing"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("PrintQuietSectionsConsole output must not contain %q, got: %s", unwanted, out)
		}
	}
	if !strings.Contains(out, "--- Failing ---") {
		t.Errorf("PrintQuietSectionsConsole output missing %q, got: %s", "--- Failing ---", out)
	}
}

// TestPrintAllSectionsConsole_WarningSectionPrintsDetail guards against a
// regression where test's console output reduced warning (⚠️) Resource
// Compliance sections to only the terse aggregate "[WARN] … N finding(s)
// non-blocking, pre-existing)" line, never showing which files/resources
// triggered each finding - even though pipeline mode already printed the
// per-check detail tables and the same body is built for test. Warning
// sections with a non-empty body must now print their full (sanitized)
// detail, matching the failing-section path.
func TestPrintAllSectionsConsole_WarningSectionPrintsDetail(t *testing.T) {
	sections := []validator.ReportSection{
		markdownSectionStatus("ResourceCompliance", validator.StatusWarning),
	}

	out := captureStdout(t, func() { PrintAllSectionsConsole(nil, sections) })

	// Must NOT collapse to the terse passing one-liner.
	if strings.TrimSpace(out) == "✅ ResourceCompliance: passed" {
		t.Fatalf("PrintAllSectionsConsole reduced warning section to a one-liner, got: %q", out)
	}
	for _, want := range []string{"--- ResourceCompliance ---", "| Deployment | example |"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintAllSectionsConsole warning output missing %q, got: %s", want, out)
		}
	}
}

// TestPrintAllSectionsConsole_WarningSectionEmptyBodyIsTerse ensures a
// warning section with an empty body (nothing actionable to render) still
// falls back to the single-line summary rather than printing an empty box.
func TestPrintAllSectionsConsole_WarningSectionEmptyBodyIsTerse(t *testing.T) {
	sections := []validator.ReportSection{
		markdownSectionStatus("ResourceCompliance", validator.StatusWarning),
	}
	sections[0].Body = ""

	out := captureStdout(t, func() { PrintAllSectionsConsole(nil, sections) })

	want := "✅ ResourceCompliance: passed"
	if strings.TrimSpace(out) != want {
		t.Errorf("PrintAllSectionsConsole(empty warning) = %q, want %q", out, want)
	}
}

// TestPrintQuietSectionsConsole_PrintsWarnings mirrors
// TestPrintAllSectionsConsole_WarningSectionPrintsDetail for quiet mode's
// renderer, confirming warning detail is surfaced there too (quiet mode only
// prints failed/warned sections, omitting passing ones entirely).
func TestPrintQuietSectionsConsole_PrintsWarnings(t *testing.T) {
	sections := []validator.ReportSection{
		markdownSectionStatus("Passing", validator.StatusPassed),
		markdownSectionStatus("ResourceCompliance", validator.StatusWarning),
	}

	out := captureStdout(t, func() { PrintQuietSectionsConsole(nil, sections) })

	if strings.Contains(out, "Passing") {
		t.Errorf("PrintQuietSectionsConsole must omit passing section, got: %q", out)
	}
	for _, want := range []string{"--- ResourceCompliance ---", "| Deployment | example |"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintQuietSectionsConsole warning output missing %q, got: %s", want, out)
		}
	}
}
