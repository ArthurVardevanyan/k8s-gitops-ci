package logger

import (
	"os"
	"strings"
	"sync"
	"testing"
)

func TestLogger_Info(t *testing.T) {
	l := NewLogger(true, "")
	l.Info("test message %d", 42)
	// No panic = pass
}

func TestLogger_Error(t *testing.T) {
	l := NewLogger(false, "")
	l.Error("something failed: %s", "reason")

	errors := l.Errors()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if !strings.Contains(errors[0], "something failed") {
		t.Errorf("error message mismatch: %s", errors[0])
	}
}

// TestLogger_Error_MultilinePrefixesEveryLine guards against a regression
// where a multi-line Error/ErrorInSection message (e.g. a lint tool's
// multi-finding summary, such as kubeconform's Result.Summary()) only got
// the "[time] [ERROR]" prefix on its first line, with every subsequent line
// printed bare - see write()'s per-line-split loop.
func TestLogger_Error_MultilinePrefixesEveryLine(t *testing.T) {
	logPath := t.TempDir() + "/test.log"
	l := NewLogger(false, logPath)
	l.Error("line one\nline two\nline three")
	l.Close()

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	data := string(raw)
	for _, want := range []string{"] [ERROR] line one", "] [ERROR] line two", "] [ERROR] line three"} {
		if !strings.Contains(data, want) {
			t.Errorf("expected every line to carry its own prefix; missing %q in: %s", want, data)
		}
	}
}

// TestLogger_Raw guards Raw()'s no-prefix contract for pre-formatted,
// potentially multi-line blocks (e.g. Summary() or a rendered section body)
// - unlike Info/Warn/Error/Debug, none of Raw's lines should carry a
// "[time] [LEVEL]" tag, matching the existing Header/SubHeader convention.
func TestLogger_Raw(t *testing.T) {
	logPath := t.TempDir() + "/test.log"
	l := NewLogger(false, logPath)
	l.Raw("first line\nsecond line")
	l.Raw("")
	l.Close()

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	data := string(raw)
	if strings.Contains(data, "[INFO]") || strings.Contains(data, "[ERROR]") {
		t.Errorf("expected Raw() output to carry no level tag, got: %s", data)
	}
	for _, want := range []string{"first line", "second line"} {
		if !strings.Contains(data, want) {
			t.Errorf("expected %q in Raw() output, got: %s", want, data)
		}
	}
}

func TestLogger_HasFailures(t *testing.T) {
	l := NewLogger(false, "")
	if l.HasFailures() {
		t.Error("expected no failures initially")
	}

	l.Error("fail")
	if !l.HasFailures() {
		t.Error("expected failures after Error()")
	}
}

func TestLogger_Summary(t *testing.T) {
	l := NewLogger(false, "")
	l.Error("test error")

	summary := l.Summary(0, 0, 0)
	if !strings.Contains(summary, "RESULTS SUMMARY") {
		t.Error("expected RESULTS SUMMARY header")
	}
	if !strings.Contains(summary, "Errors: 1") {
		t.Error("expected error count in summary")
	}
}

// TestLogger_SummaryOmitsSectionsLineWhenZero guards the "0 for both omits
// the line" contract documented on Summary's totalSections parameter:
// callers with no validator.Result (e.g. this package's own pre-existing
// tests, or standalone lint-only helpers) shouldn't suddenly grow a
// "Sections: 0 passed, 0 failed" line just because the parameter exists.
func TestLogger_SummaryOmitsSectionsLineWhenZero(t *testing.T) {
	l := NewLogger(false, "")
	summary := l.Summary(0, 0, 0)
	if strings.Contains(summary, "Sections:") {
		t.Errorf("expected no 'Sections:' line when totalSections is 0, got: %s", summary)
	}
}

// TestLogger_SummarySectionCounts guards the "Sections: N passed, M
// failed" line - the generic-core equivalent of a per-run pass/fail tally
// (see docs comparing this to a downstream fork's "Builds: N | Passes: N |
// Failures: N" line) - rendering correctly from the totalSections/
// failedSections counts a caller passes in (typically
// len(validator.Result.Sections) and validator.Result.FailedSectionCount()).
func TestLogger_SummarySectionCounts(t *testing.T) {
	l := NewLogger(false, "")
	summary := l.Summary(5, 1, 2)
	if !strings.Contains(summary, "Sections: 2 passed, 1 warned, 2 failed") {
		t.Errorf("expected 'Sections: 2 passed, 1 warned, 2 failed' in summary, got: %s", summary)
	}
}

func TestLogger_ErrorInSectionSurfacesInSummary(t *testing.T) {
	// Guards the changeset-failure path in the validator: a changeset resolution
	// error is reported via ErrorInSection so it both increments the error count
	// (visible in Summary) and is attributed to a named section, rather than
	// being silently stashed and printed as a clean 0/0/0 run.
	l := NewLogger(false, "")
	l.ErrorInSection("Changeset", "changeset: %v", "boom")

	if len(l.Errors()) != 1 {
		t.Fatalf("expected 1 error, got %d", len(l.Errors()))
	}
	summary := l.Summary(0, 0, 0)
	if !strings.Contains(summary, "Errors: 1") {
		t.Errorf("expected Errors: 1 in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "Changeset") {
		t.Errorf("expected Changeset section in summary, got: %s", summary)
	}
}

func TestLogger_SummaryFailedSections(t *testing.T) {
	l := NewLogger(false, "")
	l.Header("YAML Syntax")
	l.Header("Shellcheck")
	l.Error("shellcheck: something bad")
	l.Header("Kubeconform")
	l.Error("kubeconform: invalid schema")

	summary := l.Summary(0, 0, 0)
	if !strings.Contains(summary, "Failed sections:") {
		t.Errorf("expected 'Failed sections:' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "- Shellcheck") {
		t.Errorf("expected '- Shellcheck' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "- Kubeconform") {
		t.Errorf("expected '- Kubeconform' in summary, got: %s", summary)
	}
	if strings.Contains(summary, "- YAML Syntax") {
		t.Errorf("did not expect '- YAML Syntax' in summary (it passed), got: %s", summary)
	}

	// Verify FailedSections accessor
	sections := l.FailedSections()
	if len(sections) != 2 {
		t.Fatalf("expected 2 failed sections, got %d: %v", len(sections), sections)
	}
	if sections[0] != "Shellcheck" || sections[1] != "Kubeconform" {
		t.Errorf("unexpected failed sections: %v", sections)
	}
}

func TestLogger_SetSection(t *testing.T) {
	l := NewLogger(false, "")
	l.Header("Build")

	// SetSection changes the active section without printing a header
	l.SetSection("Sync Options Check")
	l.Error("missing annotation")

	summary := l.Summary(0, 0, 0)
	if !strings.Contains(summary, "- Sync Options Check") {
		t.Errorf("expected '- Sync Options Check' in failed sections, got: %s", summary)
	}
	if strings.Contains(summary, "- Build") {
		t.Errorf("did not expect '- Build' in failed sections (it passed), got: %s", summary)
	}
}

func TestLogger_LogFile(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	l := NewLogger(true, tmpFile)
	l.Info("logged message")
	l.Close()

	// Verify log file was written
	// (just checking it doesn't panic)
}

func TestLogger_ErrorInSection(t *testing.T) {
	l := NewLogger(false, "")
	// Simulate concurrent steps: set currentSection to something different
	l.Header("Step A")
	// But log the error attributed to a different section
	l.ErrorInSection("Step B", "error in B: %s", "details")

	sections := l.FailedSections()
	if len(sections) != 1 {
		t.Fatalf("expected 1 failed section, got %d: %v", len(sections), sections)
	}
	if sections[0] != "Step B" {
		t.Errorf("expected 'Step B' in failed sections, got: %v", sections)
	}

	errors := l.Errors()
	if len(errors) != 1 || !strings.Contains(errors[0], "error in B") {
		t.Errorf("unexpected errors: %v", errors)
	}
}

func TestLogger_ErrorInSection_ConcurrentSafety(t *testing.T) {
	l := NewLogger(false, "")
	// Simulate the race scenario: multiple goroutines attribute errors to their own sections
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			l.ErrorInSection("Section A", "error %d", i)
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		l.ErrorInSection("Section B", "error %d", i)
	}
	<-done

	sections := l.FailedSections()
	hasA, hasB := false, false
	for _, s := range sections {
		if s == "Section A" {
			hasA = true
		}
		if s == "Section B" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Errorf("expected both Section A and Section B in failed sections, got: %v", sections)
	}
}

func TestLogger_Verbose(t *testing.T) {
	l := NewLogger(true, "")
	if !l.Verbose() {
		t.Error("expected Verbose() == true for verbose logger")
	}
	l2 := NewLogger(false, "")
	if l2.Verbose() {
		t.Error("expected Verbose() == false for non-verbose logger")
	}
}

func TestScopedLogger_BufferedMode(t *testing.T) {
	l := NewLogger(false, "") // non-verbose → buffered
	s := l.Scope()

	s.Info("first message")
	s.Info("second message")

	// Before Flush, lines are buffered
	s.mu.Lock()
	count := len(s.lines)
	s.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 buffered lines, got %d", count)
	}

	// Flush should drain
	s.Flush()
	s.mu.Lock()
	count = len(s.lines)
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 buffered lines after Flush, got %d", count)
	}
}

func TestScopedLogger_StreamingMode(t *testing.T) {
	l := NewLogger(true, "") // verbose → streaming
	s := l.Scope()

	s.Info("streaming message")
	s.Debug("debug message")

	// In streaming mode, nothing is buffered
	s.mu.Lock()
	count := len(s.lines)
	s.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 buffered lines in streaming mode, got %d", count)
	}

	// Flush is a no-op in streaming mode
	s.Flush()
}

func TestScopedLogger_DebugNotBuffered(t *testing.T) {
	l := NewLogger(false, "") // non-verbose → buffered
	s := l.Scope()

	s.Debug("should not appear in buffer")
	s.Info("should appear in buffer")

	s.mu.Lock()
	count := len(s.lines)
	s.mu.Unlock()
	// Only Info should be buffered; Debug is suppressed in non-verbose
	if count != 1 {
		t.Fatalf("expected 1 buffered line (DEBUG suppressed), got %d", count)
	}
}

func TestScopedLogger_ErrorTracksInParent(t *testing.T) {
	l := NewLogger(false, "")
	l.SetSection("Build YAML")
	s := l.Scope()

	s.Error("build failed: %s", "timeout")

	// Error should be tracked in parent immediately
	errors := l.Errors()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error in parent, got %d", len(errors))
	}
	if !strings.Contains(errors[0], "build failed") {
		t.Errorf("error message mismatch: %s", errors[0])
	}
	if !l.HasFailures() {
		t.Error("expected HasFailures() after scoped Error()")
	}
}

func TestScopedLogger_ErrorInSectionTracksInParent(t *testing.T) {
	l := NewLogger(false, "")
	s := l.Scope()

	s.ErrorInSection("Build YAML", "overlay failed: %s", "network")

	sections := l.FailedSections()
	if len(sections) != 1 || sections[0] != "Build YAML" {
		t.Errorf("expected 'Build YAML' in failed sections, got: %v", sections)
	}
}

func TestScopedLogger_WarnTracksInParent(t *testing.T) {
	l := NewLogger(false, "")
	s := l.Scope()

	s.Warn("something suspicious: %s", "drift")

	// Check parent summary mentions warnings
	summary := l.Summary(0, 0, 0)
	if !strings.Contains(summary, "Warnings: 1") {
		t.Errorf("expected Warnings: 1 in summary, got: %s", summary)
	}
}

// TestScopedLogger_ConcurrentFlush guards concurrent-safety of independent
// ScopedLoggers sharing one parent Logger: many goroutines each buffer
// output and record an error via their own scope, then Flush - the parent's
// error count must end up exactly right with no lost/duplicated updates
// (run with -race to catch data races on the shared parent state).
func TestScopedLogger_ConcurrentFlush(t *testing.T) {
	l := NewLogger(false, "")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s := l.Scope()
			for j := 0; j < 50; j++ {
				s.Info("goroutine %d message %d", id, j)
			}
			s.ErrorInSection("Concurrent", "goroutine %d failed", id)
			s.Flush()
		}(i)
	}
	wg.Wait()

	if len(l.Errors()) != 10 {
		t.Errorf("expected 10 errors, got %d", len(l.Errors()))
	}
}

func TestScopedLogger_SubHeader(t *testing.T) {
	l := NewLogger(false, "")
	s := l.Scope()

	s.SubHeader("Building: my-app")

	s.mu.Lock()
	count := len(s.lines)
	s.mu.Unlock()
	// SubHeader produces 3 lines: separator, title, separator
	if count != 3 {
		t.Fatalf("expected 3 buffered lines from SubHeader, got %d", count)
	}
}

func TestScopedLogger_FlushIdempotent(t *testing.T) {
	l := NewLogger(false, "")
	s := l.Scope()

	s.Info("one line")
	s.Flush()
	s.Flush() // second flush should be no-op

	s.mu.Lock()
	count := len(s.lines)
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 lines after double Flush, got %d", count)
	}
}
