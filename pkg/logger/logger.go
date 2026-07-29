package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// Logger provides thread-safe structured console logging with counters.
type Logger struct {
	mu      sync.Mutex
	w       io.Writer
	verbose bool
	prefix  string
	errors  atomic.Int32
	warns   atomic.Int32
	infos   atomic.Int32
}

// NewLogger constructs a Logger. The empty prefix suppresses section headers.
func NewLogger(verbose bool, prefix string) *Logger {
	return &Logger{w: os.Stderr, verbose: verbose, prefix: prefix}
}

// SetWriter overrides the default stderr writer.
func (l *Logger) SetWriter(w io.Writer) {
	l.mu.Lock()
	l.w = w
	l.mu.Unlock()
}

func (l *Logger) write(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.w, level+" "+l.prefix+msg)
}

// Header logs a top-level header.
func (l *Logger) Header(msg string) { l.write("=", msg) }

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.infos.Add(1)
	l.write("•", msg)
}

// Warn logs a warning.
func (l *Logger) Warn(msg string) {
	l.warns.Add(1)
	l.write("⚠", msg)
}

// Error logs an error message and increments the error counter.
func (l *Logger) Error(msg string) {
	l.errors.Add(1)
	l.write("✖", msg)
}

// Errorf formats and logs an error.
func (l *Logger) Errorf(format string, args ...any) { l.Error(fmt.Sprintf(format, args...)) }

// Debug logs a debug message when verbose.
func (l *Logger) Debug(msg string) {
	if l.verbose {
		l.write("…", msg)
	}
}

// Debugf formats and logs a debug message when verbose.
func (l *Logger) Debugf(format string, args ...any) { l.Debug(fmt.Sprintf(format, args...)) }

// ErrorInSection logs an error attributed to a named section.
func (l *Logger) ErrorInSection(section, format string, args ...any) {
	l.Errorf("[%s] %s", section, fmt.Sprintf(format, args...))
}

// ErrorCount returns the number of Error calls.
func (l *Logger) ErrorCount() int { return int(l.errors.Load()) }

// WarnCount returns the number of Warn calls.
func (l *Logger) WarnCount() int { return int(l.warns.Load()) }

// InfoCount returns the number of Info calls.
func (l *Logger) InfoCount() int { return int(l.infos.Load()) }

// Summary renders pass/warn/error counts.
func (l *Logger) Summary() string {
	parts := []string{
		fmt.Sprintf("info=%d", l.InfoCount()),
		fmt.Sprintf("warn=%d", l.WarnCount()),
		fmt.Sprintf("error=%d", l.ErrorCount()),
	}
	return "Summary: " + strings.Join(parts, ", ")
}

// ScopedLogger returns a child logger that prefixes messages with the given scope.
func (l *Logger) ScopedLogger(scope string) *Logger {
	child := NewLogger(l.verbose, l.prefix+scope+": ")
	child.w = l.w
	return child
}
