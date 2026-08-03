package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveChangeset_DirsWithIncludePrefixes(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "kubernetes", "app", "base.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(d, "ansible", "playbook.yaml"), "- hosts: all\n")

	kubernetesDir := filepath.Join(d, "kubernetes")
	ansibleDir := filepath.Join(d, "ansible")

	files, err := resolveChangeset(Options{
		Dirs:            []string{kubernetesDir, ansibleDir},
		IncludePrefixes: []string{kubernetesDir},
	})
	if err != nil {
		t.Fatalf("resolveChangeset: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file after prefix filter, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "base.yaml" {
		t.Errorf("unexpected file: %s", files[0])
	}
}

func TestResolveChangeset_NoIncludePrefixes(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")

	files, err := resolveChangeset(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("resolveChangeset: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
}

func TestRunAll_PopulatesLogger(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")

	res, err := RunAll(Options{Dirs: []string{d}, LintOnly: true, Verbose: true})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil {
		t.Fatal("expected Result.Logger to be populated, got nil")
	}
}

func TestRunAll_LogsPhasesAndRecordsTiming(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")

	res, err := RunAll(Options{Dirs: []string{d}, LintOnly: true})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	summaryOut := res.Logger.Summary(len(res.Sections), res.FailedSectionCount())
	if !strings.Contains(summaryOut, "RESULTS SUMMARY") {
		t.Errorf("expected a rendered RESULTS SUMMARY banner, got:\n%s", summaryOut)
	}
	if res.Timing == nil {
		t.Fatal("expected Result.Timing to be populated, got nil")
	}
	summary := res.Timing.Summary(0)
	if !strings.Contains(summary, "Linting") {
		t.Errorf("expected timing summary to record the Linting phase, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Static Checks") {
		t.Errorf("expected timing summary to record the Static Checks phase, got:\n%s", summary)
	}
}

func TestRunAll_RecordsBuildPhaseTimingWhenNotLintOnly(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	summary := res.Timing.Summary(0)
	for _, phase := range []string{"Build YAML", "Post-Build Validation"} {
		if !strings.Contains(summary, phase) {
			t.Errorf("expected timing summary to record the %q phase, got:\n%s", phase, summary)
		}
	}
}

// TestRunAll_LintingAndStaticChecksRunInParallel guards the Phase 2
// parallelization of runLintAndStaticChecks: the 5 linters and remaining
// static checks (config-sort/startingCSV/scaffold table) fan out across
// goroutines instead of running one at a time, so both phases must be
// recorded as parallel ("parallel" mode, not "seq") and each individual
// linter/check must show up as an indented sub-step under its parent phase
// in the timing table. large-file/YAML-syntax are their own standalone,
// sequential top-level phases (matching a downstream fork's equivalent
// phase breakdown - see docs/DEVELOPMENT.md), not sub-steps of either.
func TestRunAll_LintingAndStaticChecksRunInParallel(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")

	res, err := RunAll(Options{Dirs: []string{d}, LintOnly: true})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	summary := res.Timing.Summary(time.Second)

	for _, sub := range []string{"markdownlint", "prettier", "shellcheck", "kubeconform"} {
		if !strings.Contains(summary, sub) {
			t.Errorf("expected timing summary to record the %q sub-step, got:\n%s", sub, summary)
		}
	}
	for _, sub := range []string{"config-sort", "startingCSV"} {
		if !strings.Contains(summary, sub) {
			t.Errorf("expected timing summary to record the %q sub-step, got:\n%s", sub, summary)
		}
	}
	for _, top := range []string{"Large File Check", "YAML Syntax"} {
		if !strings.Contains(summary, top) {
			t.Errorf("expected timing summary to record the %q top-level phase, got:\n%s", top, summary)
		}
	}
	if !strings.Contains(summary, "parallel") {
		t.Errorf("expected the Linting/Static Checks phases to be recorded as parallel, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Concurrency") {
		t.Errorf("expected SetConcurrency to be wired so the summary shows a Concurrency line, got:\n%s", summary)
	}
}

// TestRunAll_BuildPhaseFansOutOverlaysInParallel guards the Phase 2
// parallelization of runBuildAndPostBuild's per-overlay loop: each detected
// overlay is now checked concurrently via a bounded worker pool instead of
// one at a time, and each overlay's check duration is recorded as its own
// sub-step under "Build YAML" in the timing table.
func TestRunAll_BuildPhaseFansOutOverlaysInParallel(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "app1", "overlays", "clusterA", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(d, "app2", "overlays", "clusterB", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	summary := res.Timing.Summary(time.Second)

	overlayA := filepath.Join("app1", "overlays", "clusterA")
	overlayB := filepath.Join("app2", "overlays", "clusterB")
	if !strings.Contains(summary, overlayA) {
		t.Errorf("expected timing summary to record a sub-step for %q, got:\n%s", overlayA, summary)
	}
	if !strings.Contains(summary, overlayB) {
		t.Errorf("expected timing summary to record a sub-step for %q, got:\n%s", overlayB, summary)
	}
}

func TestResult_HasErrorSection(t *testing.T) {
	r := &Result{Sections: []Section{{Name: "a", Error: false}, {Name: "b", Error: true}}}
	if !r.HasErrorSection() {
		t.Error("expected HasErrorSection to report true when a section has Error=true")
	}
	r2 := &Result{Sections: []Section{{Name: "a", Error: false}}}
	if r2.HasErrorSection() {
		t.Error("expected HasErrorSection to report false when no section has Error=true")
	}
}

// TestResult_FailedSectionCount guards the count Logger.Summary's "Sections:
// N passed, M failed" line depends on (see logger_test.go's
// TestLogger_SummarySectionCounts) - it must count Sections with Error=true
// exactly, independent of len(r.Sections) itself.
func TestResult_FailedSectionCount(t *testing.T) {
	r := &Result{Sections: []Section{
		{Name: "a", Error: false},
		{Name: "b", Error: true},
		{Name: "c", Error: true},
	}}
	if got := r.FailedSectionCount(); got != 2 {
		t.Errorf("FailedSectionCount() = %d, want 2", got)
	}
	r2 := &Result{}
	if got := r2.FailedSectionCount(); got != 0 {
		t.Errorf("FailedSectionCount() on empty Result = %d, want 0", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
