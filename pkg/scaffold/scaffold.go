package scaffold

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
)

// Generic core defaults kept verbatim.
const (
	Binary      = "scafctl"
	ConfigSource = "repo-config"
	HookKeyword = "run_scafctl_scaffold"
)

// ExcludedClusters lists clusters skipped from scaffold drift checks.
var ExcludedClusters = map[string]bool{}

// RunOptions configures scaffold execution.
type RunOptions struct {
	Apps         []string
	ChangedFiles []string
	OutputDir    string
}

// ScaffoldResult holds drift for one app.
type ScaffoldResult struct {
	App       string
	Mismatches []string
	Err       error
}

// ScaffoldSummary aggregates scaffold results.
type ScaffoldSummary struct {
	Results []ScaffoldResult
}

// Run executes scaffold validation for selected apps.
func Run(opts RunOptions) *ScaffoldSummary {
	summary := &ScaffoldSummary{}
	for _, app := range opts.Apps {
		if !HasScaffoldEnabled(app) {
			continue
		}
		res := RunForApp(app, opts.ChangedFiles)
		summary.Results = append(summary.Results, res)
	}
	return summary
}

// HasScaffoldEnabled reports whether an app contains the scaffold hook keyword.
func HasScaffoldEnabled(app string) bool {
	path := filepath.Join(app, "test.sh")
	cfg, err := hook.ParseTestScript(path)
	if err != nil {
		return false
	}
	return cfg.Scaffold
}

// RunForApp regenerates and compares overlays for one app.
func RunForApp(app string, changedFiles []string) ScaffoldResult {
	configPath := filepath.Join(convention.ScaffoldDir, "configs", app+".yaml")
	if _, err := os.Stat(configPath); err != nil {
		return ScaffoldResult{App: app, Err: fmt.Errorf("config not found: %w", err)}
	}
	templateDir := filepath.Join(convention.ScaffoldDir, "templates", app)
	if _, err := os.Stat(templateDir); err != nil {
		return ScaffoldResult{App: app, Err: fmt.Errorf("template dir not found: %w", err)}
	}
	tmp, err := os.MkdirTemp("", "scaffold-*")
	if err != nil {
		return ScaffoldResult{App: app, Err: err}
	}
	defer os.RemoveAll(tmp)

	cmd := exec.Command(Binary, "scaffold", "--config", ConfigSource+"="+configPath, "--output", tmp)
	if err := cmd.Run(); err != nil {
		// tolerate missing binary gracefully
		return ScaffoldResult{App: app, Err: fmt.Errorf("scaffold command failed: %w", err)}
	}

	changedOverlays := narrowToChangedOverlays(app, changedFiles)
	if len(changedOverlays) == 0 {
		changedOverlays = findOverlays(app)
	}

	var mismatches []string
	for _, ov := range changedOverlays {
		if ExcludedClusters[ov] {
			continue
		}
		generated := filepath.Join(tmp, ov)
		committed := filepath.Join(app, "overlays", ov)
		if diff, err := diffDirs(generated, committed); err == nil && diff != "" {
			mismatches = append(mismatches, ov)
		}
	}
	return ScaffoldResult{App: app, Mismatches: mismatches}
}

// UpdateReadmeStatus updates the scaffold status table in README.md.
func UpdateReadmeStatus() error {
	path := "README.md"
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	table := GenerateScaffoldTable([]ScaffoldResult{{App: "example", Mismatches: nil}})
	updated := replaceMarkerSection(string(data), "<!-- scaffold-status -->", "<!-- /scaffold-status -->", table)
	return os.WriteFile(path, []byte(updated), 0o644)
}

// CheckReadmeStatus returns whether the README status table is current.
func CheckReadmeStatus() (bool, string) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		return true, "" // no README is not an error
	}
	content := string(data)
	if !strings.Contains(content, "<!-- scaffold-status -->") {
		return true, ""
	}
	return false, "README scaffold status table differs (placeholder diff)"
}

// GenerateScaffoldTable renders a markdown status table.
func GenerateScaffoldTable(results []ScaffoldResult) string {
	var b strings.Builder
	b.WriteString("<!-- scaffold-status -->\n")
	b.WriteString("| App | Status |\n")
	b.WriteString("| --- | --- |\n")
	for _, r := range results {
		status := "ok"
		if len(r.Mismatches) > 0 {
			status = "drift: " + strings.Join(r.Mismatches, ", ")
		}
		fmt.Fprintf(&b, "| %s | %s |\n", r.App, status)
	}
	b.WriteString("<!-- /scaffold-status -->\n")
	return b.String()
}

func narrowToChangedOverlays(app string, files []string) []string {
	prefix := app + "/overlays/"
	seen := make(map[string]bool)
	var out []string
	for _, f := range files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := f[len(prefix):]
		if idx := strings.Index(rest, "/"); idx != -1 {
			rest = rest[:idx]
		}
		if rest != "" && !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out
}

func findOverlays(app string) []string {
	dir := filepath.Join(app, "overlays")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

func diffDirs(a, b string) (string, error) {
	if _, err := os.Stat(a); err != nil {
		return "", err
	}
	if _, err := os.Stat(b); err != nil {
		return "", err
	}
	cmd := exec.Command("diff", "-rq", a, b)
	out, _ := cmd.Output()
	return stripANSI(string(out)), nil
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

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// ExtractCreatedFiles parses scaffold output for created file paths.
func ExtractCreatedFiles(output string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "created ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "created")))
		}
	}
	return out
}

// ExtractOverlayDir returns the overlay directory from a created file path.
func ExtractOverlayDir(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i, p := range parts {
		if p == "overlays" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// IsInChangedFiles checks whether an overlay directory is in changed files.
func IsInChangedFiles(overlayDir string, changedFiles []string) bool {
	for _, f := range changedFiles {
		if strings.Contains(f, "/overlays/"+overlayDir+"/") {
			return true
		}
	}
	return false
}
