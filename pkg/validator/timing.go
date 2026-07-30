package validator

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// stepTiming records a single named phase's wall-clock duration.
type stepTiming struct {
	Name     string
	Duration time.Duration
}

// TimingCollector is a small, mutex-protected collector of coarse-grained
// phase timings (e.g. "Linting", "Static Checks", "Build+Compliance").
//
// This is intentionally a much smaller scope than the reference
// implementation's TimingCollector, which additionally supports sub-steps
// keyed by parent phase and a parallelism-efficiency ratio. This repo's
// runLintAndStaticChecks/runBuildAndPostBuild phases are currently fully
// sequential (no goroutine fan-out per linter), so there is no "sub-step
// under a phase" concept to support yet - RecordStep/SetConcurrency/
// parallelism-efficiency math should only be added if and when this
// repo's linters are actually parallelized to mirror that fan-out.
type TimingCollector struct {
	mu     sync.Mutex
	timing []stepTiming
}

// NewTimingCollector constructs an empty TimingCollector.
func NewTimingCollector() *TimingCollector {
	return &TimingCollector{}
}

// Record appends a named phase's duration to the collector. Safe for
// concurrent use.
func (tc *TimingCollector) Record(name string, d time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.timing = append(tc.timing, stepTiming{Name: name, Duration: d})
}

// Summary renders a simple per-phase duration table plus a total line.
func (tc *TimingCollector) Summary(total time.Duration) string {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("Step timings:\n")
	for _, s := range tc.timing {
		fmt.Fprintf(&sb, "  %-20s %s\n", s.Name, s.Duration.Round(time.Millisecond))
	}
	fmt.Fprintf(&sb, "  %-20s %s\n", "Total", total.Round(time.Millisecond))
	return sb.String()
}
