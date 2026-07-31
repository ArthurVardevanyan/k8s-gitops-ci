package validator

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTimingCollector_Record(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	tc.Record("step1", 100*time.Millisecond, false)
	tc.Record("step2", 200*time.Millisecond, true)

	if len(tc.phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(tc.phases))
	}
	if tc.phases[0].Name != "step1" {
		t.Errorf("expected name 'step1', got %q", tc.phases[0].Name)
	}
	if tc.phases[0].Parallel {
		t.Error("step1 should not be parallel")
	}
	if !tc.phases[1].Parallel {
		t.Error("step2 should be parallel")
	}
}

func TestTimingCollector_RecordConcurrent(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tc.Record("step", time.Duration(n)*time.Millisecond, true)
		}(i)
	}
	wg.Wait()

	if len(tc.phases) != 50 {
		t.Fatalf("expected 50 phases after concurrent writes, got %d", len(tc.phases))
	}
}

func TestTimingCollector_RecordStep(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	tc.RecordStep("Linters", "markdownlint", 120*time.Millisecond)
	tc.RecordStep("Linters", "shellcheck", 3400*time.Millisecond)
	tc.RecordStep("Linters", "prettier", 80*time.Millisecond)
	tc.RecordStep("Static Checks", "config sort", 15*time.Millisecond)

	if len(tc.subSteps) != 2 {
		t.Fatalf("expected 2 parent phases in subSteps, got %d", len(tc.subSteps))
	}
	if len(tc.subSteps["Linters"]) != 3 {
		t.Fatalf("expected 3 sub-steps under Linters, got %d", len(tc.subSteps["Linters"]))
	}
	if len(tc.subSteps["Static Checks"]) != 1 {
		t.Fatalf("expected 1 sub-step under Static Checks, got %d", len(tc.subSteps["Static Checks"]))
	}
}

func TestTimingCollector_RecordStepConcurrent(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tc.RecordStep("Phase", "step", time.Duration(n)*time.Millisecond)
		}(i)
	}
	wg.Wait()

	if len(tc.subSteps["Phase"]) != 50 {
		t.Fatalf("expected 50 sub-steps after concurrent writes, got %d", len(tc.subSteps["Phase"]))
	}
}

func TestTimingCollector_SummaryEmpty(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	result := tc.Summary(time.Second)
	if result != "" {
		t.Errorf("expected empty summary for no timings, got %q", result)
	}
}

func TestTimingCollector_SummaryFormat(t *testing.T) {
	t.Parallel()
	tc := NewTimingCollector()
	tc.SetConcurrency(8, 16)

	tc.Record("Large File Check", 12*time.Millisecond, false)
	tc.Record("Linters", 4500*time.Millisecond, true)
	tc.Record("Build YAML", 8700*time.Millisecond, true)

	result := tc.Summary(10 * time.Second)

	// Verify table structure
	if !strings.Contains(result, "Step") {
		t.Error("summary should contain 'Step' header")
	}
	if !strings.Contains(result, "Duration") {
		t.Error("summary should contain 'Duration' header")
	}
	if !strings.Contains(result, "Mode") {
		t.Error("summary should contain 'Mode' header")
	}

	// Verify step names present
	if !strings.Contains(result, "Large File Check") {
		t.Error("summary should contain 'Large File Check'")
	}
	if !strings.Contains(result, "Linters") {
		t.Error("summary should contain 'Linters'")
	}
	if !strings.Contains(result, "Build YAML") {
		t.Error("summary should contain 'Build YAML'")
	}

	// Verify modes
	if !strings.Contains(result, "seq") {
		t.Error("summary should contain 'seq' mode")
	}
	if !strings.Contains(result, "parallel") {
		t.Error("summary should contain 'parallel' mode")
	}

	// Verify totals
	if !strings.Contains(result, "TOTAL (wall)") {
		t.Error("summary should contain 'TOTAL (wall)'")
	}
	if !strings.Contains(result, "TOTAL (sum)") {
		t.Error("summary should contain 'TOTAL (sum)'")
	}
	if !strings.Contains(result, "Parallelism") {
		t.Error("summary should contain 'Parallelism' ratio")
	}
	if !strings.Contains(result, "Concurrency") {
		t.Error("summary should contain 'Concurrency' line")
	}
	if !strings.Contains(result, "(8 CPUs × 2)") {
		t.Errorf("summary should contain '(8 CPUs × 2)', got:\n%s", result)
	}
}

func TestTimingCollector_SummaryWithSubSteps(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	tc.Record("Linters", 4500*time.Millisecond, true)
	tc.RecordStep("Linters", "shellcheck", 3400*time.Millisecond)
	tc.RecordStep("Linters", "markdownlint", 120*time.Millisecond)
	tc.RecordStep("Linters", "golangci", 4200*time.Millisecond)

	result := tc.Summary(5 * time.Second)

	// Sub-steps should appear indented
	if !strings.Contains(result, "  golangci") {
		t.Error("summary should contain indented 'golangci' sub-step")
	}
	if !strings.Contains(result, "  shellcheck") {
		t.Error("summary should contain indented 'shellcheck' sub-step")
	}
	if !strings.Contains(result, "  markdownlint") {
		t.Error("summary should contain indented 'markdownlint' sub-step")
	}

	// Verify sub-steps are sorted by duration (longest first):
	// golangci (4200ms) > shellcheck (3400ms) > markdownlint (120ms)
	golangciIdx := strings.Index(result, "  golangci")
	shellcheckIdx := strings.Index(result, "  shellcheck")
	markdownlintIdx := strings.Index(result, "  markdownlint")

	if golangciIdx > shellcheckIdx {
		t.Error("golangci should appear before shellcheck (longer duration first)")
	}
	if shellcheckIdx > markdownlintIdx {
		t.Error("shellcheck should appear before markdownlint (longer duration first)")
	}
}

func TestTimingCollector_SummaryParallelismRatio(t *testing.T) {
	t.Parallel()
	tc := NewTimingCollector()
	tc.SetConcurrency(4, 8)

	// Two parallel steps each taking 5s, wall time 5s → ratio should be 2.0x
	tc.Record("step1", 5*time.Second, true)
	tc.Record("step2", 5*time.Second, true)

	result := tc.Summary(5 * time.Second)

	if !strings.Contains(result, "2.0x") {
		t.Errorf("expected parallelism ratio 2.0x in summary, got:\n%s", result)
	}
}

func TestTimingCollector_SummaryConcurrencyDisplay(t *testing.T) {
	t.Parallel()
	tc := NewTimingCollector()
	tc.SetConcurrency(4, 8)

	tc.Record("step1", time.Second, false)
	result := tc.Summary(time.Second)

	if !strings.Contains(result, "Concurrency") {
		t.Error("summary should contain 'Concurrency' line")
	}
	if !strings.Contains(result, "(4 CPUs × 2)") {
		t.Errorf("expected '(4 CPUs × 2)' in summary, got:\n%s", result)
	}
}

func TestTimingCollector_SummaryConcurrencyHiddenWhenZero(t *testing.T) {
	t.Parallel()
	tc := &TimingCollector{}

	tc.Record("step1", time.Second, false)
	result := tc.Summary(time.Second)

	if strings.Contains(result, "Concurrency") {
		t.Errorf("summary should NOT contain 'Concurrency' when cpus is 0, got:\n%s", result)
	}
}

func TestTimingCollector_SummaryConcurrencyOverride(t *testing.T) {
	t.Parallel()
	tc := NewTimingCollector()
	tc.SetConcurrency(4, 7) // non-default: 7 != 4*2

	tc.Record("step1", time.Second, false)
	result := tc.Summary(time.Second)

	if !strings.Contains(result, "(4 CPUs)") {
		t.Errorf("expected '(4 CPUs)' without '× 2' for non-default concurrency, got:\n%s", result)
	}
	if strings.Contains(result, "× 2") {
		t.Errorf("should NOT contain '× 2' when concurrency is overridden, got:\n%s", result)
	}
}
