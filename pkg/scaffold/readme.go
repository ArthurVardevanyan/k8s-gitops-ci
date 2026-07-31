package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

const (
	readmePath   = "README.md"
	tableStart   = "<!-- scaffold-status -->"
	tableEnd     = "<!-- /scaffold-status -->"
	tableHeader  = "| App | Overlay | Status |"
	tableDivider = "| --- | --- | --- |"
)

// StatusRow is one app+overlay's scaffold status, as rendered in the
// README's scaffold-status table.
type StatusRow struct {
	App, Overlay, Status string
}

// DiscoverApps lists every app with a scafctl config under
// convention.ScaffoldDir/configs/ - the full set UpdateReadmeStatus scans,
// and the set CheckReadmeStatus expects the committed README to have a row
// for (per overlay).
func DiscoverApps() []string {
	dir := filepath.Join(convention.ScaffoldDir, "configs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var apps []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		app := strings.TrimSuffix(name, ext)
		if app != "" && !seen[app] {
			seen[app] = true
			apps = append(apps, app)
		}
	}
	sort.Strings(apps)
	return apps
}

// UpdateReadmeStatus regenerates the README's scaffold-status table from a
// real, full scaffold run (Run, IsFullTest-equivalent: every overlay of
// every discovered app) and writes the result back - this is the
// expensive, full-repo-scan path, meant to be invoked deliberately (e.g.
// the `update-scaffold-status` CLI command), not on every PR - see
// CheckReadmeStatus for the cheap, per-PR structural check.
func UpdateReadmeStatus() error {
	if _, err := os.Stat(readmePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}

	var rows []StatusRow
	for _, app := range DiscoverApps() {
		overlays := FindOverlays(app)
		if len(overlays) == 0 {
			continue
		}
		summary := Run(RunOptions{App: app, Trigger: "update-scaffold-status", Overlays: overlays})
		rows = append(rows, statusRowsForApp(app, overlays, summary)...)
	}

	table := GenerateScaffoldTable(rows)
	updated := replaceMarkerSection(string(data), tableStart, tableEnd, table)
	return os.WriteFile(readmePath, []byte(updated), 0o600)
}

// statusRowsForApp builds one StatusRow per overlay from a Run Summary,
// classifying each overlay as drifted (in summary.MismatchFiles), skipped
// (in summary.SkippedClusters), errored (summary.Errors is non-empty and
// the overlay is neither of the above - an execution failure covering the
// whole app), or ok.
func statusRowsForApp(app string, overlays []string, summary *Summary) []StatusRow {
	drifted := make(map[string]bool, len(summary.MismatchFiles))
	for _, m := range summary.MismatchFiles {
		drifted[m] = true
	}
	skipped := make(map[string]bool, len(summary.SkippedClusters))
	for _, s := range summary.SkippedClusters {
		skipped[s] = true
	}
	execFailed := len(summary.Errors) > 0

	rows := make([]StatusRow, 0, len(overlays))
	for _, ov := range overlays {
		status := "✅ ok"
		switch {
		case drifted[ov]:
			status = "❌ drift"
		case skipped[ov]:
			status = "⏭️ skipped"
		case execFailed:
			status = "❌ error"
		}
		rows = append(rows, StatusRow{App: app, Overlay: ov, Status: status})
	}
	return rows
}

// GenerateScaffoldTable renders rows as the README's
// `| App | Overlay | Status |` scaffold-status table, sorted by App then
// Overlay for a deterministic, easily-diffable output.
func GenerateScaffoldTable(rows []StatusRow) string {
	sorted := make([]StatusRow, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].App != sorted[j].App {
			return sorted[i].App < sorted[j].App
		}
		return sorted[i].Overlay < sorted[j].Overlay
	})

	var b strings.Builder
	b.WriteString(tableStart + "\n")
	b.WriteString(tableHeader + "\n")
	b.WriteString(tableDivider + "\n")
	for _, r := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.App, r.Overlay, r.Status)
	}
	b.WriteString(tableEnd + "\n")
	return b.String()
}

// CheckReadmeStatus is the cheap, per-PR structural check: it verifies the
// committed README's scaffold-status table lists exactly the (app,
// overlay) pairs that actually exist on disk today - not whether they're
// currently drifted (that's what the Build + Compliance phase's own
// scaffold.Run calls already report separately; recomputing it here too
// would mean scanning and scaffolding every app in the repo on every PR,
// not just the ones it touched). A missing README, or one with no
// scaffold-status marker at all, is not an error - the table is opt-in.
func CheckReadmeStatus() (current bool, diff string) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return true, ""
	}
	content := string(data)
	if !strings.Contains(content, tableStart) {
		return true, ""
	}

	existing := parseScaffoldTable(content)
	actual := make(map[string]bool)
	for _, app := range DiscoverApps() {
		for _, ov := range FindOverlays(app) {
			actual[app+"/"+ov] = true
		}
	}

	var missing, stale []string
	for key := range actual {
		if !existing[key] {
			missing = append(missing, key)
		}
	}
	for key := range existing {
		if !actual[key] {
			stale = append(stale, key)
		}
	}
	if len(missing) == 0 && len(stale) == 0 {
		return true, ""
	}

	sort.Strings(missing)
	sort.Strings(stale)
	var b strings.Builder
	if len(missing) > 0 {
		fmt.Fprintf(&b, "missing from README scaffold-status table (run `update-scaffold-status`): %s", strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "stale entries no longer on disk (run `update-scaffold-status`): %s", strings.Join(stale, ", "))
	}
	return false, b.String()
}

// parseScaffoldTable extracts the "app/overlay" key set from every data row
// (skipping the header/divider) between content's scaffold-status markers.
func parseScaffoldTable(content string) map[string]bool {
	out := make(map[string]bool)
	start := strings.Index(content, tableStart)
	if start == -1 {
		return out
	}
	end := strings.Index(content[start:], tableEnd)
	section := content[start:]
	if end != -1 {
		section = content[start : start+end]
	}
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cols := strings.Split(strings.Trim(line, "|"), "|")
		if len(cols) < 2 {
			continue
		}
		app := strings.TrimSpace(cols[0])
		overlay := strings.TrimSpace(cols[1])
		if app == "" || app == "App" || strings.HasPrefix(app, "---") {
			continue
		}
		out[app+"/"+overlay] = true
	}
	return out
}

func replaceMarkerSection(content, start, end, replacement string) string {
	idxStart := strings.Index(content, start)
	if idxStart == -1 {
		return content + "\n" + replacement
	}
	idxEnd := strings.Index(content[idxStart:], end)
	if idxEnd == -1 {
		return content[:idxStart] + replacement + content[idxStart+len(start):]
	}
	return content[:idxStart] + replacement + content[idxStart+idxEnd+len(end):]
}
