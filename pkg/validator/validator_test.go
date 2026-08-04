package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

func TestResolveChangeset_DirsWithDirs(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "kubernetes", "app", "base.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(d, "ansible", "playbook.yaml"), "- hosts: all\n")

	kubernetesDir := filepath.Join(d, "kubernetes")

	files, err := resolveChangeset(Options{
		Dirs: []string{kubernetesDir},
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

func TestResolveChangeset_NoDirs(t *testing.T) {
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
	r := &Result{Sections: []ReportSection{{Name: "a", Status: StatusPassed}, {Name: "b", Status: StatusError}}}
	if !r.HasErrorSection() {
		t.Error("expected HasErrorSection to report true when a section has StatusError")
	}
	r2 := &Result{Sections: []ReportSection{{Name: "a", Status: StatusPassed}}}
	if r2.HasErrorSection() {
		t.Error("expected HasErrorSection to report false when no section has StatusError")
	}
	// StatusWarning/StatusInfo are "worth a look" but not a hard failure -
	// HasErrorSection must not conflate them with StatusError.
	r3 := &Result{Sections: []ReportSection{{Name: "a", Status: StatusWarning}, {Name: "b", Status: StatusInfo}}}
	if r3.HasErrorSection() {
		t.Error("expected HasErrorSection to report false for StatusWarning/StatusInfo-only sections")
	}
}

// TestResult_FailedSectionCount guards the count Logger.Summary's "Sections:
// N passed, M failed" line depends on (see logger_test.go's
// TestLogger_SummarySectionCounts) - it must count Sections with
// StatusError exactly, independent of len(r.Sections) itself, and must not
// count StatusWarning/StatusInfo sections as failed.
func TestResult_FailedSectionCount(t *testing.T) {
	r := &Result{Sections: []ReportSection{
		{Name: "a", Status: StatusPassed},
		{Name: "b", Status: StatusError},
		{Name: "c", Status: StatusError},
		{Name: "d", Status: StatusWarning},
		{Name: "e", Status: StatusInfo},
	}}
	if got := r.FailedSectionCount(); got != 2 {
		t.Errorf("FailedSectionCount() = %d, want 2", got)
	}
	r2 := &Result{}
	if got := r2.FailedSectionCount(); got != 0 {
		t.Errorf("FailedSectionCount() on empty Result = %d, want 0", got)
	}
}

// TestResult_Failed guards the single source of truth every CLI entry
// point's exit code is now based on (pipeline's validatorResultFailed,
// test-all's runTestAll) - see the real bug this closed: Kustomize Fix
// findings rendered as a StatusError report section but never set
// Blocking nor called log.ErrorInSection, so Failed() (and, before it
// existed, both entry points' own hand-written checks) would have missed
// them. Also guards the nil-Result and nil-Logger cases so callers don't
// need to guard those themselves first.
func TestResult_Failed(t *testing.T) {
	var nilResult *Result
	if nilResult.Failed() {
		t.Error("expected a nil *Result to report Failed()==false")
	}

	if (&Result{}).Failed() {
		t.Error("expected a zero-value Result (nil Logger, Blocking=false) to report Failed()==false")
	}

	if !(&Result{Blocking: true}).Failed() {
		t.Error("expected Blocking=true to report Failed()==true")
	}

	log := logger.NewLogger(false, "")
	log.ErrorInSection("KustomizeBuild", "1 file needs `kustomize edit fix`")
	if !(&Result{Logger: log}).Failed() {
		t.Error("expected a Logger with a recorded ErrorInSection call to report Failed()==true, even with Blocking=false")
	}

	cleanLog := logger.NewLogger(false, "")
	if (&Result{Logger: cleanLog}).Failed() {
		t.Error("expected a clean Logger (no recorded failures) and Blocking=false to report Failed()==false")
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
