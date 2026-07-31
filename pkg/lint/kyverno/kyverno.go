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
		workers = 1
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

func parseOutput(out []byte, resourceFiles []string) (*Result, error) {
	start := bytes.Index(out, []byte(`{"kind":"ClusterReport"`))
	if start == -1 {
		start = bytes.Index(out, []byte(`{"kind":"PolicyReport"`))
	}
	if start == -1 {
		start = bytes.IndexAny(out, "{")
	}
	if start == -1 {
		return &Result{}, nil
	}
	var report map[string]any
	if err := json.Unmarshal(out[start:], &report); err != nil {
		return nil, fmt.Errorf("parsing kyverno output: %w (raw: %s)", err, truncate(string(out), 200))
	}
	res := &Result{}
	results, _ := report["results"].([]any)
	for _, r := range results {
		m, _ := r.(map[string]any)
		if m == nil {
			continue
		}
		policy, _ := m["policy"].(string)
		rule, _ := m["rule"].(string)
		status, _ := m["status"].(string)
		resource, _ := m["resources"].([]any)
		if isExcludedRule(policy, rule) {
			continue
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
		if status == "fail" || status == "warn" || status == "error" {
			for _, rs := range resource {
				resMap, _ := rs.(map[string]any)
				if resMap == nil {
					continue
				}
				name := fmt.Sprintf("%v", resMap["name"])
				kind := fmt.Sprintf("%v", resMap["kind"])
				res.Violations = append(res.Violations, Violation{
					Policy:   policy,
					Rule:     rule,
					Resource: fmt.Sprintf("%s/%s", kind, name),
					File:     findResourceFile(name, resourceFiles),
					Message:  fmt.Sprintf("%v", m["message"]),
					Severity: status,
				})
			}
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

// Deduplicate groups violations.
func Deduplicate(violations []Violation, maxSamples int) []DeduplicatedViolation {
	if maxSamples <= 0 {
		maxSamples = 3
	}
	seen := make(map[string]*DeduplicatedViolation)
	order := make([]string, 0, len(violations))
	for _, v := range violations {
		key := v.Policy + "/" + v.Rule + "/" + v.Message
		if d, ok := seen[key]; ok {
			d.Count++
			if len(d.Resources) < maxSamples && !contains(d.Resources, v.Resource) {
				d.Resources = append(d.Resources, v.Resource)
			}
			if len(d.Files) < maxSamples && !contains(d.Files, v.File) {
				d.Files = append(d.Files, v.File)
			}
			continue
		}
		order = append(order, key)
		seen[key] = &DeduplicatedViolation{
			Policy:    v.Policy,
			Rule:      v.Rule,
			Message:   v.Message,
			Severity:  v.Severity,
			Count:     1,
			Resources: []string{v.Resource},
			Files:     []string{v.File},
		}
	}
	out := make([]DeduplicatedViolation, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// FormatComment renders kyverno findings.
func FormatComment(violations []DeduplicatedViolation) string {
	if len(violations) == 0 {
		return ""
	}
	severityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.Slice(violations, func(i, j int) bool {
		si := severityOrder[strings.ToLower(violations[i].Severity)]
		sj := severityOrder[strings.ToLower(violations[j].Severity)]
		if si != sj {
			return si < sj
		}
		return violations[i].Count > violations[j].Count
	})
	var b strings.Builder
	b.WriteString(Marker + "\n")
	b.WriteString("> [!WARNING] **Kyverno Policy Violations** — non-blocking advisory\n\n")
	b.WriteString("| Policy | Rule | Severity | Count |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, v := range violations {
		icon := ":white_circle:"
		switch strings.ToLower(v.Severity) {
		case "high":
			icon = ":red_circle:"
		case "medium":
			icon = ":orange_circle:"
		case "low":
			icon = ":yellow_circle:"
		}
		fmt.Fprintf(&b, "| %s | %s | %s %s | %d |\n", v.Policy, v.Rule, icon, v.Severity, v.Count)
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
