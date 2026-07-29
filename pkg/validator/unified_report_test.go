package validator

import (
	"strings"
	"testing"
	"time"
)

func TestReportRender(t *testing.T) {
	r := &Report{
		Marker:    "<!-- m -->",
		Title:     "T",
		Header:    "H",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Sections:  []Section{{Name: "S", Body: "b", Error: true}},
	}
	out := r.Render()
	if !strings.Contains(out, "H") || !strings.Contains(out, "S") {
		t.Errorf("missing rendered parts: %s", out)
	}
}

func TestStatusIcon(t *testing.T) {
	if StatusIcon(StatusPassed) != "✅" {
		t.Errorf("unexpected passed icon")
	}
}

func TestLegacyMarkers(t *testing.T) {
	if len(LegacyMarkers()) == 0 {
		t.Errorf("expected legacy markers")
	}
}
