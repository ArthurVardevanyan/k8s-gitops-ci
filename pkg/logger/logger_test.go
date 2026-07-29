package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	l := NewLogger(false, "")
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
	if l.ErrorCount() != 0 {
		t.Error("expected zero errors")
	}
}

func TestCounter(t *testing.T) {
	l := NewLogger(false, "")
	l.Info("a")
	l.Warn("b")
	l.Error("c")
	l.Error("d")
	if l.InfoCount() != 1 || l.WarnCount() != 1 || l.ErrorCount() != 2 {
		t.Errorf("counts: info=%d warn=%d error=%d", l.InfoCount(), l.WarnCount(), l.ErrorCount())
	}
}

func TestWrites(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(true, "test: ")
	l.SetWriter(&buf)
	l.Info("hello")
	l.Debug("world")
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("output missing message: %s", out)
	}
}

func TestSummary(t *testing.T) {
	l := NewLogger(false, "")
	l.Info("a")
	l.Warn("b")
	l.Error("c")
	s := l.Summary()
	if !strings.Contains(s, "info=1") || !strings.Contains(s, "warn=1") || !strings.Contains(s, "error=1") {
		t.Errorf("summary unexpected: %s", s)
	}
}
