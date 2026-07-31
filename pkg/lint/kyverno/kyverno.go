package kyverno

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const Marker = "<!-- kyverno-policy-warning -->"

// ErrCLINotFound returned when the kyverno CLI is absent.
var ErrCLINotFound = errors.New("kyverno CLI not found in PATH")

// NamespaceSelectorLabelKeys strips namespaceSelector labels when set.
var NamespaceSelectorLabelKeys []string

// Violation records a Kyverno policy finding.
type Violation struct {
	Policy, Rule, Resource, File, Message, Severity string
}

// Result holds kyverno CLI results.
type Result struct {
	Pass, Fail, Warn, Error, Skip int
	Violations                    []Violation
}

func (r *Result) Summary() string {
	return fmt.Sprintf("%d pass, %d fail, %d warn, %d error, %d skip", r.Pass, r.Fail, r.Warn, r.Error, r.Skip)
}

// DeduplicatedViolation groups violations.
type DeduplicatedViolation struct {
	Policy, Rule, Message, Severity string
	Count                           int
	Resources, Files                []string
}

// ValidateDir validates YAML files in dir against policies.
func ValidateDir(dir string) (*Result, func(), error) {
	policyDir, cleanup, err := PreparePolicies()
	if err != nil {
		return nil, nil, err
	}
	files := CollectYAML(dir)
	res, err := ValidateFiles(files, policyDir)
	return res, cleanup, err
}

// ValidateFiles runs kyverno apply on files.
func ValidateFiles(files []string, policyDir string) (*Result, error) {
	if len(files) == 0 {
		return &Result{}, nil
	}
	if _, err := exec.LookPath("kyverno"); err != nil {
		return nil, ErrCLINotFound
	}
	args := []string{"apply", policyDir, "--policy-report", "--output-format", "json"}
	for _, f := range files {
		args = append(args, "--resource", f)
	}
	cmd := exec.CommandContext(context.Background(), "kyverno", args...)
	out, err := cmd.Output()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() > 1 {
				return nil, fmt.Errorf("kyverno apply failed (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
			}
			out = exitErr.Stderr
		} else {
			return nil, fmt.Errorf("running kyverno: %w", err)
		}
	}
	return parseOutput(out, files)
}

// ValidateFilesBatched runs one kyverno subprocess per app in batches.
func ValidateFilesBatched(policyPath string, filesByApp map[string][]string, workers int) (*Result, []error) {
	if workers <= 0 {
		// Kyverno's CLI is I/O-bound (subprocess + policy engine startup
		// per invocation), so oversubscribing beyond physical cores keeps
		// throughput up while individual jobs wait on each other.
		workers = runtime.NumCPU() * 2
	}
	apps := make([]string, 0, len(filesByApp))
	for app := range filesByApp {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	if workers > len(apps) {
		workers = len(apps)
	}
	type job struct {
		app   string
		files []string
	}
	jobs := make(chan job, len(apps))
	var mu sync.Mutex
	var combined Result
	var errs []error
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				res, err := ValidateFiles(j.files, policyPath)
				mu.Lock()
				if err != nil {
					errs = append(errs, fmt.Errorf("app %s: %w", j.app, err))
				} else {
					mergeResults(&combined, res)
				}
				mu.Unlock()
			}
		}()
	}
	for _, app := range apps {
		jobs <- job{app: app, files: filesByApp[app]}
	}
	close(jobs)
	wg.Wait()
	return &combined, errs
}

// CollectYAML finds .yaml/.yml files in dir.
func CollectYAML(dir string) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil //nolint:nilerr // filepath.Walk convention: skip entry, keep walking
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// policyReportResource is a single resource identity as reported by
// kyverno's PolicyReport/ClusterReport JSON output.
type policyReportResource struct {
	Kind, Name, Namespace, APIVersion string
}

// policyReportResult is one result entry within a kyverno PolicyReport or
// ClusterReport. Kyverno's `--policy-report` JSON output uses the
// wgpolicyk8s.io PolicyReport shape, whose per-result outcome field is
// `result` (pass/fail/warn/error/skip) and whose severity (low/medium/high)
// is a distinct field - this package previously conflated the two by
// reading a nonexistent `status` field and reusing its value as severity,
// which meant severity-based sorting/icons never worked. A `resource`
// (singular) or `resources` (plural) field may be present depending on
// kyverno CLI version/output mode; both are honored.
type policyReportResult struct {
	Policy    string                 `json:"policy"`
	Rule      string                 `json:"rule"`
	Result    string                 `json:"result"`
	Status    string                 `json:"status"` // fallback for older/alternate output shapes
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Resource  *policyReportResource  `json:"resource"`
	Resources []policyReportResource `json:"resources"`
}

// policyReport is the top-level kyverno `--policy-report` JSON document
// (either a single PolicyReport or an aggregate ClusterReport - both share
// this shape).
type policyReport struct {
	Kind    string               `json:"kind"`
	Results []policyReportResult `json:"results"`
}

// reportStartMarkers are the JSON substrings identifying where the actual
// kyverno report object begins in CLI output, which may be preceded by
// unrelated log/preamble text. Kyverno's JSON encoder may or may not insert
// a space after the colon depending on version, so both compact and
// space-separated forms are matched.
var reportStartMarkers = []string{
	`"kind":"ClusterReport"`,
	`"kind":"PolicyReport"`,
	`"kind": "ClusterReport"`,
	`"kind": "PolicyReport"`,
}

// findReportStart locates the byte offset of the `{` that begins the
// kyverno report object within out. It finds the earliest matching
// kind-marker (see reportStartMarkers), then walks backward to the nearest
// enclosing `{`, so the returned offset is always a valid JSON object start
// rather than the middle of one. Falls back to the first `{` in out if no
// marker is found, and to -1 if out contains no `{` at all.
func findReportStart(out []byte) int {
	best := -1
	for _, marker := range reportStartMarkers {
		if idx := bytes.Index(out, []byte(marker)); idx != -1 && (best == -1 || idx < best) {
			best = idx
		}
	}
	if best == -1 {
		return bytes.IndexAny(out, "{")
	}
	for i := best; i >= 0; i-- {
		if out[i] == '{' {
			return i
		}
	}
	return best
}

// resultResources returns the resources named by a single policy report
// result, whether kyverno reported them as a singular `resource` object or
// a plural `resources` array.
func resultResources(r policyReportResult) []policyReportResource {
	if len(r.Resources) > 0 {
		return r.Resources
	}
	if r.Resource != nil {
		return []policyReportResource{*r.Resource}
	}
	return nil
}

func parseOutput(out []byte, resourceFiles []string) (*Result, error) {
	start := findReportStart(out)
	if start == -1 {
		return &Result{}, nil
	}
	var report policyReport
	if err := json.Unmarshal(out[start:], &report); err != nil {
		return nil, fmt.Errorf("parsing kyverno output: %w (raw: %s)", err, truncate(string(out), 200))
	}
	res := &Result{}
	for _, r := range report.Results {
		if isExcludedRule(r.Policy, r.Rule) {
			continue
		}
		status := r.Result
		if status == "" {
			status = r.Status
		}
		switch status {
		case "pass":
			res.Pass++
		case "fail":
			res.Fail++
		case "warn":
			res.Warn++
		case "error":
			res.Error++
		case "skip":
			res.Skip++
		}
		if status != "fail" && status != "warn" && status != "error" {
			continue
		}
		severity := r.Severity
		if severity == "" {
			// Kyverno policies aren't required to set a severity
			// annotation; default to "medium" rather than leaving this
			// empty (which would otherwise rank as highest-priority
			// ("high") under a naive zero-value map lookup) or reusing
			// the pass/fail status as a fake severity (this package's
			// previous, incorrect behavior).
			severity = "medium"
		}
		for _, rs := range resultResources(r) {
			res.Violations = append(res.Violations, Violation{
				Policy:   r.Policy,
				Rule:     r.Rule,
				Resource: fmt.Sprintf("%s/%s", rs.Kind, rs.Name),
				File:     findResourceFile(rs.Name, resourceFiles),
				Message:  r.Message,
				Severity: severity,
			})
		}
	}
	return res, nil
}

// findResourceFile is a best-effort match of a violation's resource name
// back to the resourceFiles it was validated from, so Violation.File points
// at something a reviewer can actually open - a resource's Kind (this
// package's previous behavior) never identifies a source file. Matching by
// filename containing the resource name is inherently approximate (kyverno's
// JSON report doesn't otherwise carry the originating file), so this
// deliberately checks every file for a substring match rather than assuming
// a 1:1 file-to-resource layout; callers that already know the exact
// resource-to-file mapping (e.g. one rendered overlay per file) should
// prefer remapping Violation.File themselves afterward instead of relying on
// this heuristic. Returns "" when no file's basename contains the name (kept
// as an empty, honestly-unknown value rather than a misleading guess).
func findResourceFile(name string, resourceFiles []string) string {
	if name == "" {
		return ""
	}
	for _, f := range resourceFiles {
		if strings.Contains(filepath.Base(f), name) {
			return f
		}
	}
	return ""
}

// Deduplicate groups violations by policy+rule, collapsing repeated hits
// against the same resource (e.g. from kustomize/kyverno autogen
// duplicating a workload across multiple rendered variants) so Count
// reflects the number of distinct resources affected rather than the raw
// number of report entries. Resource identity is Violation.Resource
// ("Kind/Name") - note this doesn't include namespace, so two
// identically-named resources in different namespaces are treated as one;
// that's an existing limitation of Violation's shape, not introduced here.
func Deduplicate(violations []Violation, maxSamples int) []DeduplicatedViolation {
	if maxSamples <= 0 {
		maxSamples = 3
	}
	type group struct {
		v       DeduplicatedViolation
		seenRes map[string]bool
	}
	seen := make(map[string]*group)
	order := make([]string, 0, len(violations))
	for _, v := range violations {
		key := v.Policy + "/" + v.Rule
		g, ok := seen[key]
		if !ok {
			g = &group{
				v: DeduplicatedViolation{
					Policy:   v.Policy,
					Rule:     v.Rule,
					Message:  v.Message,
					Severity: v.Severity,
				},
				seenRes: make(map[string]bool),
			}
			seen[key] = g
			order = append(order, key)
		}
		if !g.seenRes[v.Resource] {
			g.seenRes[v.Resource] = true
			g.v.Count++
			if len(g.v.Resources) < maxSamples {
				g.v.Resources = append(g.v.Resources, v.Resource)
			}
		}
		if v.File != "" && len(g.v.Files) < maxSamples && !contains(g.v.Files, v.File) {
			g.v.Files = append(g.v.Files, v.File)
		}
	}
	out := make([]DeduplicatedViolation, 0, len(order))
	for _, k := range order {
		out = append(out, seen[k].v)
	}
	return out
}

// severityRank maps a severity string to a sort priority (lower sorts
// first). Unknown/missing severities rank after every known severity
// rather than defaulting to the same weight as "high", so a malformed or
// absent severity can't masquerade as the most urgent finding.
var severityRankOrder = map[string]int{"high": 0, "medium": 1, "low": 2}

func severityRank(severity string) int {
	if r, ok := severityRankOrder[strings.ToLower(severity)]; ok {
		return r
	}
	return len(severityRankOrder)
}

func severityIcon(severity string) string {
	switch strings.ToLower(severity) {
	case "high":
		return ":red_circle:"
	case "medium":
		return ":orange_circle:"
	case "low":
		return ":yellow_circle:"
	default:
		return ":white_circle:"
	}
}

// FormatComment renders kyverno findings as a non-blocking PR advisory:
// one section per policy/rule violation group, sorted by severity then by
// how many resources it affects, each with its message, an affected-count
// line, and a capped, bulleted sample of the resources involved.
func FormatComment(violations []DeduplicatedViolation) string {
	if len(violations) == 0 {
		return ""
	}
	sort.Slice(violations, func(i, j int) bool {
		ri, rj := severityRank(violations[i].Severity), severityRank(violations[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return violations[i].Count > violations[j].Count
	})
	var b strings.Builder
	b.WriteString(Marker + "\n")
	b.WriteString("> [!WARNING]\n")
	b.WriteString("> **Kyverno Policy Violations** — non-blocking advisory. These\n")
	b.WriteString("> findings do not block this PR but should be reviewed.\n\n")
	for _, v := range violations {
		fmt.Fprintf(&b, "### %s `%s` / `%s` (%s)\n\n", severityIcon(v.Severity), v.Policy, v.Rule, v.Severity)
		if v.Message != "" {
			fmt.Fprintf(&b, "> %s\n\n", v.Message)
		}
		plural := "s"
		if v.Count == 1 {
			plural = ""
		}
		fmt.Fprintf(&b, "%d resource%s affected.\n\n", v.Count, plural)
		for _, r := range v.Resources {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		if v.Count > len(v.Resources) {
			fmt.Fprintf(&b, "- …and %d more\n", v.Count-len(v.Resources))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func mergeResults(dst, src *Result) {
	dst.Pass += src.Pass
	dst.Fail += src.Fail
	dst.Warn += src.Warn
	dst.Error += src.Error
	dst.Skip += src.Skip
	dst.Violations = append(dst.Violations, src.Violations...)
}
