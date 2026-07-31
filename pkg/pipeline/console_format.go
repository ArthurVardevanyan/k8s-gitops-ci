package pipeline

import (
	"regexp"
	"strings"
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

// sanitizeSectionBodyForConsole converts a Section.Body - always
// GitHub-flavored markdown built for the PR-comment renderer (literal
// <details>/<summary> dropdown tags, &nbsp; used purely for visual nesting
// indentation since GitHub strips inline CSS, **bold** emphasis) - into
// plain text suitable for a terminal. Section.Body itself is never mutated;
// this only applies to the copy printed to the console by
// printFailedSectionDetail, so the PR comment's actual rendering is
// unaffected.
func sanitizeSectionBodyForConsole(body string) string {
	body = summaryTagRe.ReplaceAllString(body, "$1:")
	body = consoleReplacer.Replace(body)
	body = blankRunRe.ReplaceAllString(body, "\n\n")
	return strings.TrimSpace(body)
}
