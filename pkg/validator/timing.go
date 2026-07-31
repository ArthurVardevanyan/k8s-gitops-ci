package validator

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// StepTiming records the wall-clock duration of a pipeline step.
type StepTiming struct {
	Name     string
	Duration time.Duration
	Parallel bool // true if this step ran concurrently with others
}

// TimingCollector accumulates phase and sub-step timings in a thread-safe manner.
// It can be created externally (e.g. in pipeline.go) to record pre-validation
// phases like clone and extraction, then passed into RunAll via Options.
type TimingCollector struct {
	mu          sync.Mutex
	phases      []StepTiming            // Phase-level timings (recorded in order)
	subSteps    map[string][]StepTiming // Sub-steps keyed by parent phase name
	cpus        int                     // Detected runtime.NumCPU()
	concurrency int                     // Effective worker pool size
}

// NewTimingCollector creates a TimingCollector for use across the full pipeline.
// cpus and concurrency can be set to 0 and updated later via SetConcurrency.
func NewTimingCollector() *TimingCollector {
	return &TimingCollector{
		subSteps: make(map[string][]StepTiming),
	}
}

// SetConcurrency sets the CPU/concurrency metadata shown in the summary footer.
func (tc *TimingCollector) SetConcurrency(cpus, concurrency int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cpus = cpus
	tc.concurrency = concurrency
}

// Record adds a phase-level timing. Safe for concurrent use.
func (tc *TimingCollector) Record(name string, duration time.Duration, parallel bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.phases = append(tc.phases, StepTiming{Name: name, Duration: duration, Parallel: parallel})
}

// RecordStep adds a sub-step timing under a parent phase. Safe for concurrent use.
func (tc *TimingCollector) RecordStep(phase, name string, duration time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.subSteps == nil {
		tc.subSteps = make(map[string][]StepTiming)
	}
	tc.subSteps[phase] = append(tc.subSteps[phase], StepTiming{Name: name, Duration: duration, Parallel: true})
}

// Summary formats all recorded timings as a structured table.
// Phases are shown flush-left; sub-steps are indented beneath their parent phase
// and sorted by duration (longest first). Also computes parallelism efficiency.
func (tc *TimingCollector) Summary(totalDuration time.Duration) string {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if len(tc.phases) == 0 {
		return ""
	}

	// Find max display name width for alignment (account for indent on sub-steps)
	const indent = "  "
	maxName := len("TOTAL (wall)")
	for _, t := range tc.phases {
		if len(t.Name) > maxName {
			maxName = len(t.Name)
		}
		for _, s := range tc.subSteps[t.Name] {
			w := len(indent) + len(s.Name)
			if w > maxName {
				maxName = w
			}
		}
	}

	var sb strings.Builder
	sep := strings.Repeat("-", maxName+30)

	sb.WriteString(sep)
	sb.WriteByte('\n')
	fmt.Fprintf(&sb, "%-*s  %10s  %s\n", maxName, "Step", "Duration", "Mode")
	sb.WriteString(sep)
	sb.WriteByte('\n')

	var sumPhaseDurations time.Duration
	for _, t := range tc.phases {
		mode := "seq"
		if t.Parallel {
			mode = "parallel"
		}
		fmt.Fprintf(&sb, "%-*s  %10s  %s\n", maxName, t.Name, t.Duration.Truncate(time.Millisecond), mode)
		sumPhaseDurations += t.Duration

		// Print sub-steps sorted by duration (longest first)
		if steps, ok := tc.subSteps[t.Name]; ok && len(steps) > 0 {
			sorted := make([]StepTiming, len(steps))
			copy(sorted, steps)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Duration > sorted[j].Duration
			})
			for _, s := range sorted {
				fmt.Fprintf(&sb, "%-*s  %10s  %s\n", maxName, indent+s.Name, s.Duration.Truncate(time.Millisecond), "parallel")
			}
		}
	}

	sb.WriteString(sep)
	sb.WriteByte('\n')
	fmt.Fprintf(&sb, "%-*s  %10s\n", maxName, "TOTAL (wall)", totalDuration.Truncate(time.Millisecond))
	fmt.Fprintf(&sb, "%-*s  %10s\n", maxName, "TOTAL (sum)", sumPhaseDurations.Truncate(time.Millisecond))

	if totalDuration > 0 {
		efficiency := float64(sumPhaseDurations) / float64(totalDuration)
		fmt.Fprintf(&sb, "%-*s  %9.1fx\n", maxName, "Parallelism", efficiency)
	}

	if tc.cpus > 0 {
		if tc.concurrency == tc.cpus*2 {
			fmt.Fprintf(&sb, "%-*s  %9d  (%d CPUs × 2)\n", maxName, "Concurrency", tc.concurrency, tc.cpus)
		} else {
			fmt.Fprintf(&sb, "%-*s  %9d  (%d CPUs)\n", maxName, "Concurrency", tc.concurrency, tc.cpus)
		}
	}

	sb.WriteString(sep)
	sb.WriteByte('\n')

	return sb.String()
}
