package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
)

// ciReportMarker is the stable HTML-comment marker identifying the single
// self-CI status comment on this repo's OWN pull requests. It is deliberately
// distinct from the product's "<!-- ci-unified-report -->" marker (which the
// built binary posts on DOWNSTREAM consumer repos): this comment summarizes
// the tool's own meta-CI (task ci verdict + the non-blocking live replay),
// not a manifest-validation run.
const ciReportMarker = "<!-- ci-self-report -->"

// runCIReport posts or updates a self-CI status comment on this repo's own PR.
//
// It is invoked from the meta-CI pipeline (.tekton/k8s-gitops-ci.yaml's build
// task) AFTER `task ci` and the live regression replay have run, with their
// outcomes passed in via flags. The comment always reflects the blocking
// task-ci verdict, plus a non-blocking, informational section for the live
// HomeLab replay (a false-positive smoke gate — see
// docs/DEVELOPMENT.md#end-to-end--regression-replay).
//
// This command NEVER fails the build itself: a missing PR context, an
// unavailable GitHub client, or a comment-post error is reported but returns
// nil, because the authoritative pass/fail gate is task ci's own exit code,
// re-asserted by the calling pipeline step — not this reporter.
func runCIReport(args []string) error {
	fs := flag.NewFlagSet("ci-report", flag.ExitOnError)
	var (
		url          = fs.String("url", "", "repository URL (e.g. https://github.com/org/repo)")
		pr           = fs.String("pr", "", "pull request number")
		ciStatus     = fs.String("ci-status", "", "overall `task ci` result: pass|fail")
		replayStatus = fs.String("replay-status", "skipped", "live replay result: pass|fail|skipped")
		replayReport = fs.String("replay-report", "", "path to the replay's Markdown report (optional)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client := github.NewClient(*url, *pr)
	if !client.IsAvailable() {
		// No PR/repo context (e.g. a push event, or a local run): nothing to
		// comment on. Not an error — the gate is task ci, not this reporter.
		fmt.Println("ci-report: no PR/repo context available, skipping comment")
		return nil
	}

	body := buildCIReport(ciReportBody{
		ciStatus:     normalizeStatus(*ciStatus),
		replayStatus: normalizeStatus(*replayStatus),
		replayReport: readReplayReport(*replayReport),
	})

	if err := github.UpsertComment(client, ciReportMarker, body); err != nil {
		// Best-effort: log and move on so a transient GitHub hiccup can't turn
		// a green build red (or a red build's real cause into a comment error).
		fmt.Fprintf(os.Stderr, "ci-report: failed to post comment: %v\n", err)
		return nil
	}
	fmt.Println("ci-report: self-CI status comment posted/updated")
	return nil
}

// ciReportBody is the resolved input to the markdown builder.
type ciReportBody struct {
	ciStatus     status
	replayStatus status
	replayReport string // already-read Markdown (may be empty)
}

// status is a normalized CI outcome.
type status string

const (
	statusPass    status = "pass"
	statusFail    status = "fail"
	statusSkipped status = "skipped"
	statusUnknown status = "unknown"
)

func normalizeStatus(s string) status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pass", "passed", "ok", "success", "0":
		return statusPass
	case "fail", "failed", "failure", "error":
		return statusFail
	case "skip", "skipped", "":
		return statusSkipped
	default:
		return statusUnknown
	}
}

// icon maps a status to the same emoji vocabulary the product's unified report
// uses (pkg/validator SectionStatus.Icon), so the tool's own CI comment reads
// consistently with the comments it posts on consumer repos.
func (s status) icon() string {
	switch s {
	case statusPass:
		return "✅"
	case statusFail:
		return "❌"
	case statusSkipped:
		return "⚪"
	default:
		return "⚠️"
	}
}

// readReplayReport reads the replay Markdown report if a path was given and it
// exists; returns "" otherwise (the builder degrades gracefully).
func readReplayReport(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	b, err := os.ReadFile(path) //nolint:gosec // operator-supplied CI path, trusted input
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// buildCIReport renders the self-CI status comment body. It always leads with
// the marker (so UpsertComment can find and update it), a title, and the
// blocking task-ci verdict, followed by a collapsible, explicitly-non-blocking
// live-replay section.
func buildCIReport(in ciReportBody) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n", ciReportMarker)
	b.WriteString("## CI Pipeline Status\n\n")

	// Blocking verdict.
	switch in.ciStatus {
	case statusPass:
		fmt.Fprintf(&b, "%s **`task ci` passed** — lint, tests, vulncheck, and build all succeeded.\n\n", statusPass.icon())
	case statusFail:
		fmt.Fprintf(&b, "%s **`task ci` failed** — see the pipeline logs for the failing step. This blocks merge.\n\n", statusFail.icon())
	default:
		fmt.Fprintf(&b, "%s **`task ci` status unknown** — the reporter was not told the outcome.\n\n", statusUnknown.icon())
	}

	// Non-blocking live-replay section.
	b.WriteString(buildReplaySection(in))

	b.WriteString("\n---\n")
	b.WriteString("_Posted by `k8s-gitops-ci ci-report`. The live regression replay is a non-blocking smoke gate; only `task ci` blocks merge._\n")

	return b.String()
}

// buildReplaySection renders the collapsible, non-blocking live-replay section.
func buildReplaySection(in ciReportBody) string {
	var b strings.Builder

	summary := replaySummaryLine(in.replayStatus)
	fmt.Fprintf(&b, "<details>\n<summary>%s Expand: Live regression replay (HomeLab)</summary>\n\n", in.replayStatus.icon())
	b.WriteString(summary + "\n\n")
	b.WriteString("> **Informational only — this does not block merge.** The replay runs the freshly-built binary\n")
	b.WriteString("> against recent real merged PRs of a live GitOps repo. It is a *false-positive* smoke gate: a PR\n")
	b.WriteString("> that newly fails is worth a look, but the replay reads live GitHub state (so it can flake) and is\n")
	b.WriteString("> structurally blind to *false negatives* (a check that silently stops firing). See\n")
	b.WriteString("> [docs/DEVELOPMENT.md](../blob/main/docs/DEVELOPMENT.md#end-to-end--regression-replay).\n\n")

	if in.replayReport != "" {
		b.WriteString(in.replayReport)
		b.WriteString("\n\n")
	}
	b.WriteString("</details>\n")
	return b.String()
}

func replaySummaryLine(s status) string {
	switch s {
	case statusPass:
		return "All replayed PRs passed. ✅"
	case statusFail:
		return "One or more replayed PRs failed — **review the diff below**; a newer, stricter check may be legitimately flagging an older PR (not necessarily a regression)."
	case statusSkipped:
		return "The replay was skipped for this run."
	default:
		return "The replay result is unknown for this run."
	}
}
