package pipeline

import (
	"strings"
	"testing"
)

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
