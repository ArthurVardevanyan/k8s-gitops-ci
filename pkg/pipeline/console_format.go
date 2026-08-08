package pipeline

import (
	"regexp"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
)

// consoleReplacer strips the remaining GitHub-markdown-only artifacts from a
// Section.Body once <summary> tags have been converted to plain labels:
// <details>/</details> dropdown tags (no plain-text equivalent needed - the
// converted <summary> label already conveys the section), &nbsp; (used
// purely for visual indentation, since GitHub strips inline CSS), and **
// bold-emphasis markers.
var consoleReplacer = strings.NewReplacer(
	"<details>\n", "",
	"<details>", "",
	"</details>", "",
	"&nbsp;", "",
	"**", "",
)

// summaryTagRe matches a GitHub-markdown <summary>...</summary> tag pair
// (used to build collapsible <details> dropdowns - see
// pkg/validator/compose_sections.go) and captures its label text. Summary
// content is always emitted on a single line by compose_sections.go, so a
// non-DOTALL "." (not matching newline) match is sufficient here.
var summaryTagRe = regexp.MustCompile(`<summary>(.*?)</summary>`)

// blankRunRe collapses 3+ consecutive newlines (left behind once
// tag-only lines like "<details>"/"</details>" are stripped) down to a
// single blank line, so stripped output doesn't accumulate extra vertical
// whitespace for every dropdown that gets removed.
var blankRunRe = regexp.MustCompile(`\n{3,}`)

// SanitizeSectionBodyForConsole converts a Section.Body - always
// GitHub-flavored markdown built for the PR-comment renderer (literal
// <details>/<summary> dropdown tags, &nbsp; used purely for visual nesting
// indentation since GitHub strips inline CSS, **bold** emphasis) - into
// plain text suitable for a terminal. Section.Body itself is never mutated;
// this only applies to the copy printed to the console (by
// printFailedSectionDetail here, and by cmd/k8s-gitops-ci's test-all/
// scan-all/build-yaml handlers), so the PR comment's actual rendering is
// unaffected. Exported so any console-output path outside this package can
// reuse it instead of dumping raw PR-comment markdown to stdout.
func SanitizeSectionBodyForConsole(body string) string {
	body = summaryTagRe.ReplaceAllString(body, "$1:")
	body = consoleReplacer.Replace(body)
	body = blankRunRe.ReplaceAllString(body, "\n\n")
	return strings.TrimSpace(body)
}

// SectionHasConsoleDetail reports whether a section should print its full
// (console-sanitized) Body to the terminal: errored (❌) or warning (⚠️)
// sections with a non-empty body. Single source of truth shared by pipeline,
// test-all, and scan-all so their console rendering can't drift on the
// "which sections get per-finding detail" rule.
func SectionHasConsoleDetail(s validator.ReportSection) bool {
	return (s.Status == validator.StatusError || s.Status == validator.StatusWarning) &&
		strings.TrimSpace(s.Body) != ""
}
