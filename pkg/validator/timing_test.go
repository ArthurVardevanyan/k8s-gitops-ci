package validator

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTimingCollector_RecordAndSummary(t *testing.T) {
	tc := NewTimingCollector()
	tc.Record("Linting", 150*time.Millisecond)
	tc.Record("Static Checks", 50*time.Millisecond)

	summary := tc.Summary(250 * time.Millisecond)

	if !strings.Contains(summary, "Linting") {
		t.Errorf("expected summary to mention Linting, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Static Checks") {
		t.Errorf("expected summary to mention Static Checks, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Total") {
		t.Errorf("expected summary to include a Total line, got:\n%s", summary)
	}
	if !strings.Contains(summary, "250ms") {
		t.Errorf("expected summary to include the total duration, got:\n%s", summary)
	}
}

func TestTimingCollector_ConcurrentRecord(t *testing.T) {
	tc := NewTimingCollector()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tc.Record("phase", time.Duration(n)*time.Millisecond)
		}(i)
	}
	wg.Wait()

	if len(tc.timing) != 20 {
		t.Fatalf("expected 20 recorded timings, got %d", len(tc.timing))
	}
}
