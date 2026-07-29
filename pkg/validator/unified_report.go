package validator

import (
	"fmt"
	"strings"
	"time"
)

// Report renders the unified PR comment.
type Report struct {
	Marker   string
	Title    string
	Header   string
	Body     string
	Sections []Section
	Timestamp time.Time
}

// Status constants for icons.
const (
	StatusPassed  = "passed"
	StatusWarning = "warning"
	StatusError   = "error"
)

// StatusIcon returns the icon for a status.
func StatusIcon(status string) string {
	switch status {
	case StatusPassed:
		return "✅"
	case StatusWarning:
		return "⚠️"
	case StatusError:
		return "❌"
	}
	return "⚪"
}

// Render produces the markdown report.
func (r *Report) Render() string {
	var b strings.Builder
	b.WriteString(r.Marker + "\n")
	if r.Header != "" {
		fmt.Fprintf(&b, "# %s\n\n", r.Header)
	}
	fmt.Fprintf(&b, "## %s\n\n", r.Title)
	if !r.Timestamp.IsZero() {
		fmt.Fprintf(&b, "_Last Updated: %s_\n\n", r.Timestamp.UTC().Format(time.RFC3339))
	}
	for _, s := range r.Sections {
		status := StatusPassed
		if s.Error {
			status = StatusError
		}
		fmt.Fprintf(&b, "<details>\n<summary>%s Expand: %s</summary>\n\n%s\n\n</details>\n\n", StatusIcon(status), s.Name, s.Body)
	}
	if r.Body != "" {
		b.WriteString(r.Body + "\n")
	}
	return b.String()
}

// LegacyMarkers returns markers to clean up after posting the unified comment.
func LegacyMarkers() []string {
	return []string{
		"<!-- resource-compliance-warnings -->",
		"<!-- ci-error-summary -->",
		"<!-- sync-options-warning -->",
		"<!-- psa-namespace-labels -->",
		"<!-- pr-title-convention-warning -->",
		"<!-- unsigned-commits-warning -->",
		"<!-- drift-protection-warning -->",
		"<!-- unresolved-placeholders -->",
		"<!-- external-drift-placeholders -->",
		"<!-- shellcheck-external-warning -->",
		"<!-- ci-notes -->",
		"<!-- podspec-defaults-warning -->",
		"<!-- rbac-wildcard-warning -->",
		"<!-- missing-clusters-warning -->",
		"<!-- scaffold-pre-existing-drift -->",
	}
}

// ReproduceCommand returns a shell snippet to reproduce the run locally.
func ReproduceCommand(opts Options) string {
	return fmt.Sprintf("go run ./cmd/<binary> pipeline --url=%q --pr=%s --target-branch=%q", opts.RepoURL, opts.PR, opts.BaseRef)
}
