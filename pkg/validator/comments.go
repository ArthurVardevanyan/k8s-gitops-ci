package validator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
)

// defaultProviderBinary returns the configured binary name, falling back to
// the provider package's own default when no branding override is set.
func defaultProviderBinary() string {
	return provider.Providers{}.BinaryName()
}

// LintFinding records a single lint/static-check finding: which check
// produced it and which file(s) it applies to. fixHints uses this to
// generate an actionable "run this command to fix it" hint. Not yet wired
// into runLintAndStaticChecks's report maps (pkg/validator/phases.go) - that
// wiring, along with richer per-check nesting, belongs to a later phase;
// this type and fixHints are landed standalone here and unit-tested first.
type LintFinding struct {
	Check string   // check name (e.g. "config-sort", matching the keys phases.go's report maps use)
	Files []string // files the finding applies to, if any
}

// buildErrorGroup represents a set of overlays that failed with the same
// root cause.
type buildErrorGroup struct {
	Cause    string
	Overlays []string
}

// groupBuildErrors separates overlay-build errors from other errors and
// groups the build errors by root cause, so the PR comment can show "N
// overlays failed with the same underlying cause" instead of repeating an
// identical error message once per overlay.
//
// Build errors in this repo have the form "kustomize build <overlay>:
// <cause>" (see pkg/overlay.buildKustomize's fmt.Errorf("kustomize build
// %s: %w", overlay, err)) - there is no "<overlay> build failed: " prefix
// wrapping it, unlike the reference implementation this was ported from;
// the parser below is adapted to this repo's actual format rather than
// introducing an error-string wrapper purely to satisfy a shared parser.
func groupBuildErrors(errs []string) (groups []buildErrorGroup, other []string) {
	const prefix = "kustomize build "
	causeMap := make(map[string][]string)
	var causeOrder []string

	for _, e := range errs {
		if !strings.HasPrefix(e, prefix) {
			other = append(other, e)
			continue
		}
		rest := strings.TrimPrefix(e, prefix)
		overlay, cause, found := strings.Cut(rest, ": ")
		if !found {
			other = append(other, e)
			continue
		}
		if _, exists := causeMap[cause]; !exists {
			causeOrder = append(causeOrder, cause)
		}
		causeMap[cause] = append(causeMap[cause], overlay)
	}

	for _, cause := range causeOrder {
		overlays := causeMap[cause]
		sort.Strings(overlays)
		groups = append(groups, buildErrorGroup{Cause: cause, Overlays: overlays})
	}
	return groups, other
}

// maxBuildErrorCauseCap is the largest build-error cause string rendered into
// the PR comment before it is truncated. It is generous enough to surface the
// meaningful part of nested kustomize errors (e.g. the "accumulateDirectory:
// '...recursed accumulation of path ...'" chain), which can easily exceed a
// few hundred characters once the full paths from a CI scratch clone are
// included. Truncation still happens (see truncateCause) so the comment stays
// readable, but only when truly necessary.
const maxBuildErrorCauseCap = 600

// truncateCause shortens a build-error cause for the PR comment. The cut point
// is moved back to the last whitespace before the cap so an error is never
// split mid-path/mid-token, and a clear "(truncated)" marker is appended.
func truncateCause(cause string) string {
	if len(cause) <= maxBuildErrorCauseCap {
		return cause
	}
	cut := cause[:maxBuildErrorCauseCap]
	if idx := strings.LastIndexAny(cut, " \n\t"); idx > 0 {
		cut = cut[:idx]
	}
	return cut + " ... (truncated)"
}

// kustomizeMissingFileHint returns a short, org-neutral, actionable hint for
// common kustomize component-accumulation failures where a referenced resource
// file does not exist in the checkout (the error the kustomize SDK emits as
// "accumulateDirectory: '...recursed accumulation of path ...'" wrapping an
// "evalsymlink failure ... no such file or directory"). These are inherently
// hard to read (long absolute paths in a scratch clone + nested quoting), so an
// explicit hint makes the fix obvious instead of leaving the user to parse the
// raw krusty stack trace. Returns "" when the cause does not match this class.
func kustomizeMissingFileHint(cause string) string {
	if strings.Contains(cause, "recursed accumulation of path") ||
		strings.Contains(cause, "accumulateDirectory") {
		if strings.Contains(cause, "no such file or directory") ||
			strings.Contains(cause, "not found") ||
			strings.Contains(cause, "evalsymlink") {
			return "a component's kustomization.yaml references a resource file " +
				"that does not exist in this checkout; check the referenced path " +
				"for a typo, a moved/renamed file, or a stale component or git ref."
		}
	}
	return ""
}

// formatBuildErrors renders grouped build errors as a compact markdown
// blockquote. Overlays sharing the same root cause are listed together
// instead of repeating the error once per overlay.
func formatBuildErrors(sb *strings.Builder, groups []buildErrorGroup) {
	sb.WriteString("> **Build Errors:**\n")
	for _, g := range groups {
		cause := truncateCause(g.Cause)
		fmt.Fprintf(sb, "> - **%d overlay(s)** failed:\n", len(g.Overlays))
		fmt.Fprintf(sb, ">   ```\n>   %s\n>   ```\n", cause)
		if hint := kustomizeMissingFileHint(g.Cause); hint != "" {
			fmt.Fprintf(sb, ">   Hint: %s\n", hint)
		}

		const maxShow = 5
		if len(g.Overlays) <= maxShow {
			names := make([]string, len(g.Overlays))
			for i, o := range g.Overlays {
				names[i] = "`" + filepath.Base(o) + "`"
			}
			fmt.Fprintf(sb, ">   Overlays: %s\n", strings.Join(names, ", "))
		} else {
			names := make([]string, maxShow)
			for i := 0; i < maxShow; i++ {
				names[i] = "`" + filepath.Base(g.Overlays[i]) + "`"
			}
			fmt.Fprintf(sb, ">   Overlays: %s, ... (+%d more)\n", strings.Join(names, ", "), len(g.Overlays)-maxShow)
		}
	}
	sb.WriteString(">\n")
}

// lintFixHint describes an actionable fix command for a lint/static check.
type lintFixHint struct {
	command string // command template ("%s" substitutes the file list, "{bin}" the binary, or static)
	binary  bool   // whether the leading "{bin}" placeholder is present for the invoked executable
}

// hintByCheck maps a check name (matching LintFinding.Check, and the keys
// phases.go's report maps use) to its fix command. Every command here is a
// real, currently-registered subcommand of this binary (verified against
// cmd/k8s-gitops-ci/main.go) or a real third-party CLI this repo already
// wraps - no hint is added for a command that doesn't exist.
var hintByCheck = map[string]lintFixHint{
	"config-sort":    {command: "{bin} sort-configs", binary: true},
	"kustomize fix":  {command: "{bin} kustomize-fix -dir %s", binary: true},
	"prettier":       {command: "prettier --write %s"},
	"markdownlint":   {command: "markdownlint %s"},
	"scaffold table": {command: "{bin} update-scaffold-status", binary: true},
}

// fixHints returns actionable fix commands for the given lint findings,
// keyed by their Check field. binaryName is the invoked executable name used
// to expand any "{bin}" placeholder; when empty it falls back to the default
// binary name. Findings with files produce file-specific commands; findings
// without files fall back to a "<file>" placeholder. Findings for checks with
// no known fix command (e.g. "shellcheck", "golangci-lint" - there's no single
// mechanical fix for those) are skipped. Order is preserved (first-seen), and
// repeated identical hints are deduplicated.
func fixHints(findings []LintFinding, binaryName string) []string {
	if binaryName == "" {
		binaryName = defaultProviderBinary()
	}
	bin := func(h lintFixHint) string {
		if !h.binary {
			return h.command
		}
		return strings.ReplaceAll(h.command, "{bin}", binaryName)
	}
	var hints []string
	seen := map[string]bool{}
	for _, f := range findings {
		h, ok := hintByCheck[f.Check]
		if !ok {
			continue
		}
		command := bin(h)
		var hint string
		switch {
		case len(f.Files) > 0 && strings.Contains(command, "%s"):
			hint = fmt.Sprintf(command, strings.Join(f.Files, " "))
		case strings.Contains(command, "%s"):
			hint = strings.ReplaceAll(command, "%s", "<file>")
		default:
			hint = command
		}
		if !seen[hint] {
			hints = append(hints, hint)
			seen[hint] = true
		}
	}
	return hints
}

// truncateDetails truncates long tool output to maxLen bytes, appending a
// truncation notice. The cut point is moved back to the last newline before
// maxLen so a line is never split mid-way.
func truncateDetails(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := s[:maxLen]
	if idx := strings.LastIndex(cut, "\n"); idx > 0 {
		cut = cut[:idx]
	}
	return cut + "\n... (truncated)"
}
