// Package cireport builds and posts a "self-CI" status comment on a
// repository's own pull requests — a single marker-based comment summarizing
// the blocking `task ci` verdict plus a non-blocking, informational live
// regression-replay section.
//
// It is org-agnostic: the corpus label ("HomeLab", a Ford repo set, etc.) and
// the docs link are injectable via Options, so both the upstream binary and a
// downstream (e.g. Ford) binary can post a consistent comment. The marker is
// deliberately distinct from the product's "<!-- ci-unified-report -->" comment
// (which the built binary posts on DOWNSTREAM consumer repos): this one is about
// the tool's OWN meta-CI.
package cireport

import (
	"fmt"
	"os"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/github"
)

// Marker is the stable HTML-comment marker identifying the single self-CI
// status comment on a repo's OWN pull requests.
const Marker = "<!-- ci-self-report -->"

// Options is the resolved input to Build/Run. Status fields accept the raw
// operator-supplied strings (pass|warn|fail|skipped, plus common synonyms) and
// are normalized internally.
type Options struct {
	// URL and PR identify the pull request to comment on. When either is
	// empty (e.g. a push event or a local run), Run is a no-op.
	URL string
	PR  string

	// CIStatus is the blocking `task ci` result: pass|fail (others → unknown).
	CIStatus string
	// CIDetail is already-read `task ci` failure detail (embedded when failed).
	CIDetail string

	// ReplayStatus is the non-blocking replay result: pass|warn|fail|skipped.
	ReplayStatus string
	// ReplayReport is the already-read replay Markdown report (embedded as-is).
	ReplayReport string

	// ReplayLabel names the replay corpus for the section summary, e.g.
	// "HomeLab" or "Ford GitOps repos". Defaults to "live GitOps repo".
	ReplayLabel string
	// DocsURL, when set, is linked from the replay section for the full
	// rationale/limitations. Optional.
	DocsURL string
}

// Run builds the comment body and upserts it on the PR. It NEVER returns an
// error for an unavailable client or a failed post — the authoritative gate is
// `task ci`, not this reporter — so a transient GitHub hiccup can't turn a
// green build red. It returns a short human-readable status string.
func Run(o Options) (string, error) {
	client := github.NewClient(o.URL, o.PR)
	if !client.IsAvailable() {
		return "no PR/repo context available, skipping comment", nil
	}
	body := Build(o)
	if err := github.UpsertComment(client, Marker, body); err != nil {
		fmt.Fprintf(os.Stderr, "cireport: failed to post comment: %v\n", err)
		return "comment post failed (non-fatal)", nil
	}
	return "self-CI status comment posted/updated", nil
}

// resolved is the internal, normalized form of Options.
type resolved struct {
	ciStatus     status
	ciDetail     string
	replayStatus status
	replayReport string
	replayLabel  string
	docsURL      string
}

func (o Options) resolve() resolved {
	label := strings.TrimSpace(o.ReplayLabel)
	if label == "" {
		label = "live GitOps repo"
	}
	return resolved{
		ciStatus:     normalizeStatus(o.CIStatus),
		ciDetail:     strings.TrimSpace(o.CIDetail),
		replayStatus: normalizeStatus(o.ReplayStatus),
		replayReport: strings.TrimSpace(o.ReplayReport),
		replayLabel:  label,
		docsURL:      strings.TrimSpace(o.DocsURL),
	}
}

// status is a normalized CI outcome.
type status string

const (
	statusPass    status = "pass"
	statusWarn    status = "warn"
	statusFail    status = "fail"
	statusSkipped status = "skipped"
	statusUnknown status = "unknown"
)

func normalizeStatus(s string) status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pass", "passed", "ok", "success", "0":
		return statusPass
	case "warn", "warning", "1":
		return statusWarn
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
	case statusWarn:
		return "⚠️"
	case statusFail:
		return "❌"
	case statusSkipped:
		return "⚪"
	default:
		return "⚠️"
	}
}

// ReadDetailFile reads a detail/report file if a path was given and it exists;
// returns "" otherwise. Callers use it for both the `task ci` failure detail
// and the replay Markdown report before populating Options.
func ReadDetailFile(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	b, err := os.ReadFile(path) //nolint:gosec // operator-supplied CI path, trusted input
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Build renders the self-CI status comment body. It always leads with the
// marker (so UpsertComment can find and update it), a title, and the blocking
// task-ci verdict, followed by a collapsible, explicitly-non-blocking
// live-replay section.
func Build(o Options) string {
	in := o.resolve()
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n", Marker)
	b.WriteString("## CI Pipeline Status\n\n")

	// Blocking verdict.
	switch in.ciStatus {
	case statusPass:
		fmt.Fprintf(&b, "%s **`task ci` passed** — lint, tests, vulncheck, and build all succeeded.\n\n", statusPass.icon())
	case statusFail:
		fmt.Fprintf(&b, "%s **`task ci` failed** — this blocks merge. See the pipeline logs for full output.\n\n", statusFail.icon())
		if in.ciDetail != "" {
			b.WriteString("<details>\n<summary>Failing step detail</summary>\n\n```\n")
			b.WriteString(in.ciDetail)
			b.WriteString("\n```\n\n</details>\n\n")
		}
	default:
		fmt.Fprintf(&b, "%s **`task ci` status unknown** — the reporter was not told the outcome.\n\n", statusUnknown.icon())
	}

	// Non-blocking live-replay section.
	b.WriteString(buildReplaySection(in))

	return b.String()
}

// buildReplaySection renders the collapsible, non-blocking live-replay section.
func buildReplaySection(in resolved) string {
	var b strings.Builder

	fmt.Fprintf(&b, "<details>\n<summary>%s Expand: Live regression replay (%s)</summary>\n\n", in.replayStatus.icon(), in.replayLabel)
	b.WriteString(replaySummaryLine(in.replayStatus) + "\n\n")
	b.WriteString("> **Informational only — this does not block merge.** The replay runs the freshly-built binary\n")
	b.WriteString("> against recent real merged PRs of a live GitOps repo. It is a *false-positive* smoke gate: a PR\n")
	b.WriteString("> that newly fails is worth a look, but the replay reads live GitHub state (so it can flake) and is\n")
	b.WriteString("> structurally blind to *false negatives* (a check that silently stops firing).")
	if in.docsURL != "" {
		fmt.Fprintf(&b, " See\n> [the docs](%s) for the full rationale and limitations.", in.docsURL)
	}
	b.WriteString("\n\n")

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
	case statusWarn:
		return "One or more replayed PRs failed — **review the diff below**; a newer, stricter check may be legitimately flagging an older PR (not necessarily a regression)."
	case statusFail:
		return "The replay could not run (harness/setup error) — the result is unavailable for this run."
	case statusSkipped:
		return "The replay was skipped for this run."
	default:
		return "The replay result is unknown for this run."
	}
}
