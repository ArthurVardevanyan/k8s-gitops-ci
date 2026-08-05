// Package logger provides structured logging, counters, and error tracking for pipeline execution.
package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Logger provides structured test output.
type Logger struct {
	mu             sync.Mutex
	verbose        bool
	logFile        *os.File
	errors         []string
	warnings       []string
	currentSection string
	failedSections []string
}

// NewLogger creates a new test logger.
func NewLogger(verbose bool, logPath string) *Logger {
	l := &Logger{verbose: verbose}
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err == nil {
			l.logFile = f
		}
	}
	return l
}

// Close closes the log file if open.
func (l *Logger) Close() {
	if l.logFile != nil {
		_ = l.logFile.Close()
	}
}

// Info logs an informational message (always printed).
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (l *Logger) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.write("INFO", msg)
}

// Debug logs a verbose message (only printed with --verbose).
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (l *Logger) Debug(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.write("DEBUG", msg)
}

// Raw prints a pre-formatted, potentially multi-line block verbatim, with no
// "[time] [LEVEL]" prefix on any line - the same no-prefix convention
// Header/SubHeader already use for banner lines. Use this for content that's
// already human-formatted as a standalone block (e.g. a Summary() report or
// a section's rendered detail), rather than Info/Warn/Error/Debug, which are
// for single structured log lines: passing a multi-line string to those
// instead would only prefix the first line, since each call still writes
// its message as one line-oriented log event.
func (l *Logger) Raw(msg string) {
	l.write("", msg)
}

// Warn logs a warning.
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (l *Logger) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.mu.Lock()
	l.warnings = append(l.warnings, msg)
	l.mu.Unlock()
	l.write("WARN", msg)
}

// Error logs an error.
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (l *Logger) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.mu.Lock()
	l.errors = append(l.errors, msg)
	l.trackFailedSection()
	l.mu.Unlock()
	l.write("ERROR", msg)
}

// ErrorInSection logs an error and attributes it to the given section name.
// Use this instead of Error() in concurrent goroutines where the shared
// currentSection may have been overwritten by another goroutine.
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (l *Logger) ErrorInSection(section, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.mu.Lock()
	l.errors = append(l.errors, msg)
	l.trackNamedSection(section)
	l.mu.Unlock()
	l.write("ERROR", msg)
}

// SetSection updates the current section name without printing a header.
func (l *Logger) SetSection(title string) {
	l.mu.Lock()
	l.currentSection = title
	l.mu.Unlock()
}

// Header prints a section header.
func (l *Logger) Header(title string) {
	l.mu.Lock()
	l.currentSection = title
	l.mu.Unlock()
	separator := strings.Repeat("=", 60)
	l.write("", separator)
	l.write("", "  "+title)
	l.write("", separator)
}

// SubHeader prints a subsection header.
func (l *Logger) SubHeader(title string) {
	separator := strings.Repeat("-", 40)
	l.write("", separator)
	l.write("", "  "+title)
	l.write("", separator)
}

// trackFailedSection records the current section as failed (must be called with mu held).
func (l *Logger) trackFailedSection() {
	l.trackNamedSection(l.currentSection)
}

// trackNamedSection records the given section name as failed (must be called with mu held).
func (l *Logger) trackNamedSection(section string) {
	if section == "" {
		return
	}
	for _, s := range l.failedSections {
		if s == section {
			return
		}
	}
	l.failedSections = append(l.failedSections, section)
}

// Summary returns a formatted summary string. totalSections/warnedSections/
// failedSections (typically len(validator.Result.Sections),
// validator.Result.WarnedSectionCount(), and
// validator.Result.FailedSectionCount()) render a leading "Sections: N
// passed, X warned, Y failed" line. pkg/logger can't import pkg/validator
// itself (validator already imports logger), so callers pass the counts as
// plain ints. Passing 0 for all three (e.g. callers with no
// validator.Result, like standalone lint-only helpers or pre-existing tests)
// omits the line entirely.
func (l *Logger) Summary(totalSections, warnedSections, failedSections int) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n")
	sb.WriteString("  RESULTS SUMMARY\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n")

	if totalSections > 0 {
		passed := totalSections - warnedSections - failedSections
		fmt.Fprintf(&sb, "  Sections: %d passed, %d warned, %d failed\n", passed, warnedSections, failedSections)
	}
	if len(l.warnings) > 0 {
		fmt.Fprintf(&sb, "  Warnings: %d\n", len(l.warnings))
	}
	if len(l.errors) > 0 {
		fmt.Fprintf(&sb, "  Errors: %d (see details above)\n", len(l.errors))
	}
	if len(l.failedSections) > 0 {
		sb.WriteString("  Failed sections:\n")
		for _, s := range l.failedSections {
			fmt.Fprintf(&sb, "    - %s\n", s)
		}
	}
	sb.WriteString(strings.Repeat("=", 60) + "\n")
	return sb.String()
}

// HasFailures returns true if any failures were recorded.
func (l *Logger) HasFailures() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors) > 0
}

// Errors returns all recorded errors.
func (l *Logger) Errors() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.errors...)
}

// FailedSections returns the list of sections that had errors or failures.
func (l *Logger) FailedSections() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.failedSections...)
}

// Verbose returns whether the logger is in verbose mode.
func (l *Logger) Verbose() bool {
	return l.verbose
}

// Scope creates a new ScopedLogger that buffers display output for atomic flushing.
// In verbose mode, output streams immediately (for debugging hangs).
// Counters and error tracking always delegate to the parent immediately.
func (l *Logger) Scope() *ScopedLogger {
	return &ScopedLogger{
		parent: l,
		stream: l.verbose,
	}
}

// write formats and prints msg, prefixed with "[time] [level]" when level is
// non-empty. msg may itself be multi-line (e.g. a pre-formatted block passed
// via Raw, or - less commonly - a multi-line Info/Warn/Error message): every
// resulting line gets its own copy of the prefix (or none, for level=="")
// rather than only the first, so a multi-line message never degrades into
// "first line tagged, rest bare" output.
func (l *Logger) write(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	showOnConsole := level != "DEBUG" || l.verbose
	timestamp := time.Now().Format("15:04:05")
	for _, ln := range strings.Split(msg, "\n") {
		line := ln
		if level != "" {
			line = fmt.Sprintf("[%s] [%s] %s", timestamp, level, ln)
		}
		if showOnConsole {
			fmt.Println(line)
		}
		if l.logFile != nil {
			_, _ = fmt.Fprintln(l.logFile, line)
		}
	}
}

// ScopedLogger buffers display output for atomic flushing while delegating
// counters and error tracking to the parent Logger immediately.
// In verbose/debug mode (stream=true), output is written immediately for
// real-time debugging of hangs.
type ScopedLogger struct {
	parent *Logger
	mu     sync.Mutex
	lines  []string
	stream bool // true in verbose mode → writes immediately
}

// Info logs an informational message.
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (s *ScopedLogger) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.emit("INFO", msg)
}

// Debug logs a verbose message.
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (s *ScopedLogger) Debug(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.emit("DEBUG", msg)
}

// Warn logs a warning (also tracked in parent immediately).
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (s *ScopedLogger) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.parent.mu.Lock()
	s.parent.warnings = append(s.parent.warnings, msg)
	s.parent.mu.Unlock()
	s.emit("WARN", msg)
}

// Error logs an error (also tracked in parent immediately).
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (s *ScopedLogger) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.parent.mu.Lock()
	s.parent.errors = append(s.parent.errors, msg)
	s.parent.trackFailedSection()
	s.parent.mu.Unlock()
	s.emit("ERROR", msg)
}

// ErrorInSection logs an error attributed to the given section (tracked in parent immediately).
//
//nolint:goprintffuncname // Name matches Logger interface convention used throughout codebase.
func (s *ScopedLogger) ErrorInSection(section, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.parent.mu.Lock()
	s.parent.errors = append(s.parent.errors, msg)
	s.parent.trackNamedSection(section)
	s.parent.mu.Unlock()
	s.emit("ERROR", msg)
}

// SubHeader formats a subsection header.
func (s *ScopedLogger) SubHeader(title string) {
	separator := strings.Repeat("-", 40)
	s.emit("", separator)
	s.emit("", "  "+title)
	s.emit("", separator)
}

// Flush writes all buffered lines atomically to stdout.
// In streaming mode this is a no-op (lines were already written).
func (s *ScopedLogger) Flush() {
	if s.stream {
		return
	}
	s.mu.Lock()
	lines := s.lines
	s.lines = nil
	s.mu.Unlock()

	if len(lines) == 0 {
		return
	}
	// Write all buffered lines under the parent's lock for atomic output
	s.parent.mu.Lock()
	defer s.parent.mu.Unlock()
	for _, line := range lines {
		fmt.Println(line)
	}
}

// emit formats and either buffers or streams msg, one line at a time. msg
// may itself be multi-line (e.g. a multi-line Error/ErrorInSection message,
// such as a lint tool's multi-finding summary) - see the equivalent note on
// Logger.write for why every resulting line gets its own prefix instead of
// only the first.
func (s *ScopedLogger) emit(level, msg string) {
	timestamp := time.Now().Format("15:04:05")
	showOnConsole := level != "DEBUG" || s.stream

	for _, ln := range strings.Split(msg, "\n") {
		line := ln
		if level != "" {
			line = fmt.Sprintf("[%s] [%s] %s", timestamp, level, ln)
		}

		if s.stream {
			// Streaming mode: write to console + logFile atomically
			s.parent.mu.Lock()
			if showOnConsole {
				fmt.Println(line)
			}
			if s.parent.logFile != nil {
				_, _ = fmt.Fprintln(s.parent.logFile, line)
			}
			s.parent.mu.Unlock()
		} else {
			// Buffered mode: write to logFile immediately, buffer console output
			s.parent.mu.Lock()
			if s.parent.logFile != nil {
				_, _ = fmt.Fprintln(s.parent.logFile, line)
			}
			s.parent.mu.Unlock()

			if showOnConsole {
				s.mu.Lock()
				s.lines = append(s.lines, line)
				s.mu.Unlock()
			}
		}
	}
}
