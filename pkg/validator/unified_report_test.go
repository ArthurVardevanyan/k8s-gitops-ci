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
	if StatusIcon(StatusInfo) != "ℹ️" {
		t.Errorf("unexpected info icon")
	}
	if StatusIcon(StatusWarning) != "⚠️" {
		t.Errorf("unexpected warning icon")
	}
	if StatusIcon(StatusError) != "❌" {
		t.Errorf("unexpected error icon")
	}
}

func TestLegacyMarkers(t *testing.T) {
	if len(LegacyMarkers()) == 0 {
		t.Errorf("expected legacy markers")
	}
}

// TestReproduceCommand_UsesRealBinaryPath guards against the reproduce
// command regressing to the literal, non-functional "./cmd/<binary>"
// placeholder - the actual module's binary lives at ./cmd/k8s-gitops-ci
// (see go.mod's module path and cmd/k8s-gitops-ci/main.go).
func TestReproduceCommand_UsesRealBinaryPath(t *testing.T) {
	got := ReproduceCommand(Options{RepoURL: "https://example.com/repo.git", PR: "42", BaseRef: "main"})
	if strings.Contains(got, "<binary>") {
		t.Errorf("ReproduceCommand still contains the unresolved <binary> placeholder: %q", got)
	}
	if !strings.Contains(got, "go run ./cmd/k8s-gitops-ci pipeline") {
		t.Errorf("ReproduceCommand = %q, want it to invoke ./cmd/k8s-gitops-ci", got)
	}
}
