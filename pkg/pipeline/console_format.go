package pipeline

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
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
// printFailedSectionDetail here, and by cmd/k8s-gitops-ci's test/
// build-yaml handlers), so the PR comment's actual rendering is
// unaffected. Exported so any console-output path outside this package can
// reuse it instead of dumping raw PR-comment markdown to stdout.
func SanitizeSectionBodyForConsole(body string) string {
	body = summaryTagRe.ReplaceAllString(body, "$1:")
	body = consoleReplacer.Replace(body)
	body = blankRunRe.ReplaceAllString(body, "\n\n")
	return strings.TrimSpace(body)
}

// SectionHasConsoleDetail reports whether a section should print its full
// (console-sanitized) Body to the terminal: errored (❌), warned (⚠️), or
// info (ℹ️) sections with a non-empty body. Info covers accepted exemptions
// - "nothing wrong, but here's an audit trail" - which should surface in
// quiet mode alongside errors and warnings. Single source of truth shared
// by pipeline, test and build-yaml so their console rendering can't drift
// on the "which sections get per-finding detail" rule.
func SectionHasConsoleDetail(s validator.ReportSection) bool {
	return (s.Status == validator.StatusError || s.Status == validator.StatusWarning || s.Status == validator.StatusInfo) &&
		strings.TrimSpace(s.Body) != ""
}

// RunTest is the "test" command's full run: it runs validator.RunAll(opts)
// and renders the result to the console exactly as cmd/k8s-gitops-ci's own
// "test" subcommand used to render it in-process (version banner, per-section
// console output, timing/summary footer, exit rule) — exported here so any
// consumer CLI (e.g. an org layer wrapping this core with its own
// provider.Providers seam via opts.Providers) gets identical "test" behavior
// without reimplementing the renderer. opts.Quiet selects between full
// section output (PrintAllSectionsConsole) and failure-only output
// (PrintQuietSectionsConsole); opts.FullScan ("--all") is handled inside
// validator.RunAll itself (see validator.Options.FullScan).
//
// The returned error is non-nil only when the run should be treated as a
// failure: opts.Quiet always returns nil (matching test --quiet's "did I
// break anything?" pre-commit contract of always exiting 0), otherwise a
// non-nil error is returned whenever res.Failed() reports true.
func RunTest(opts validator.Options) error {
	start := time.Now()
	fmt.Println(version.String())
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	if opts.Quiet {
		PrintQuietSectionsConsole(res.Logger, res.Sections)
	} else {
		PrintAllSectionsConsole(res.Logger, res.Sections)
	}
	PrintRunFooter(res, start)
	if !opts.Quiet && res.Failed() {
		return fmt.Errorf("test: validation failed")
	}
	return nil
}

// PrintRunFooter prints the run's TimingCollector.Summary() table (the
// "Step/Duration/Mode" timing breakdown) followed by Logger.Summary(), for
// RunTest — the same footer pipeline.Run already prints via its own Logger.
// Both are written via res.Logger.Raw (not fmt.Println) so they go through
// the same console/logFile handling as the rest of a run's output, matching
// pipeline.Run's footer (see pipeline.go's Run). start is the time.Time
// captured before RunAll was called, used as the timing table's wall-clock
// total.
func PrintRunFooter(res *validator.Result, start time.Time) {
	if res == nil || res.Logger == nil {
		return
	}
	if res.Timing != nil {
		if summary := res.Timing.Summary(time.Since(start)); summary != "" {
			res.Logger.Raw(summary)
		}
	}
	res.Logger.Raw(res.Logger.Summary(len(res.Sections), res.WarnedSectionCount(), res.FailedSectionCount()))
}

// PrintAllSectionsConsole prints every section's result to the console: a
// compact "✅ Name: passed" line for passing sections (full detail was
// already streamed live by log during RunAll - see phases.go - so repeating
// it here would just duplicate that output in a different style), and the
// full console-sanitized Body under a log.SubHeader box for failing ones.
func PrintAllSectionsConsole(log *logger.Logger, sections []validator.ReportSection) {
	for _, s := range sections {
		if SectionHasConsoleDetail(s) {
			PrintFailedSectionConsole(log, s)
			continue
		}
		PrintPassedSectionConsole(log, s.Name)
	}
}

// PrintQuietSectionsConsole prints only the failed and warned sections'
// full detail — no passing sections are shown at all (not even a one-line
// summary). This is the rendering used by --quiet mode, which prints only
// failed/warned sections and always exits 0 regardless of findings.
func PrintQuietSectionsConsole(log *logger.Logger, sections []validator.ReportSection) {
	for _, s := range sections {
		if SectionHasConsoleDetail(s) {
			PrintFailedSectionConsole(log, s)
		}
	}
}

// PrintPassedSectionConsole prints a single-line "✅ Name: passed" summary
// for a section that produced no errors and no warnings to render.
func PrintPassedSectionConsole(log *logger.Logger, name string) {
	line := "✅ " + name + ": passed"
	if log != nil {
		log.Raw(line)
		return
	}
	fmt.Println(line)
}

// PrintFailedSectionConsole prints a section's console-sanitized Body under
// a log.SubHeader(s.Name) box, so every console entry point sharing this
// renderer gets a consistent header style.
func PrintFailedSectionConsole(log *logger.Logger, s validator.ReportSection) {
	body := SanitizeSectionBodyForConsole(s.Body)
	if log != nil {
		log.Raw("")
		log.SubHeader(s.Name)
		log.Raw(body)
		return
	}
	fmt.Printf("\n--- %s ---\n%s\n", s.Name, body)
}
