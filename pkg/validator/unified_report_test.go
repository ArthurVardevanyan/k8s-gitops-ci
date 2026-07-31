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
	if !strings.Contains(out, "T") || !strings.Contains(out, "S") {
		t.Errorf("missing rendered parts: %s", out)
	}
}

// TestReportRender_DoesNotDuplicateHeader guards against the PR comment
// regressing to rendering both Header ("# GitOps CI Pipeline") and Title
// ("## GitOps CI Results") as two separate, redundant top-level headings.
// Only Title should ever be rendered into the comment body.
func TestReportRender_DoesNotDuplicateHeader(t *testing.T) {
	r := &Report{
		Marker: "<!-- m -->",
		Title:  "GitOps CI Results",
		Header: "GitOps CI Pipeline",
	}
	out := r.Render()
	if strings.Contains(out, "# GitOps CI Pipeline") {
		t.Errorf("Header must not be rendered as a separate heading anymore: %s", out)
	}
	if !strings.Contains(out, "## GitOps CI Results") {
		t.Errorf("expected Title to render as the sole heading: %s", out)
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

// TestReproduceCommand_IncludesScopingFlags guards against the reproduce
// command silently dropping --dirs/--disable-checks/--enable-checks, which
// would make it reproduce a broader (or narrower) changeset than the run
// that actually failed.
func TestReproduceCommand_IncludesScopingFlags(t *testing.T) {
	got := ReproduceCommand(Options{
		RepoURL:         "https://example.com/repo.git",
		PR:              "42",
		BaseRef:         "main",
		IncludePrefixes: []string{"kubernetes/", "tekton/", ".tekton/", "okd/"},
		DisabledChecks:  []string{"sync-options"},
		EnabledChecks:   []string{"kyverno"},
	})
	for _, want := range []string{
		`--dirs="kubernetes/,tekton/,.tekton/,okd/"`,
		`--disable-checks="sync-options"`,
		`--enable-checks="kyverno"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ReproduceCommand() = %q, want it to contain %q", got, want)
		}
	}
}

// TestReproduceCommand_NoScopingFlagsWhenUnset ensures the base case (no
// --dirs/--disable-checks/--enable-checks passed) doesn't grow spurious
// empty flags.
func TestReproduceCommand_NoScopingFlagsWhenUnset(t *testing.T) {
	got := ReproduceCommand(Options{RepoURL: "https://example.com/repo.git", PR: "42", BaseRef: "main"})
	for _, unwanted := range []string{"--dirs=", "--disable-checks=", "--enable-checks="} {
		if strings.Contains(got, unwanted) {
			t.Errorf("ReproduceCommand() = %q, unexpectedly contains %q", got, unwanted)
		}
	}
}

// TestReproduceCommand_OmitsTargetBranch guards against re-introducing
// --target-branch into the reproduce command: in PR mode the base ref is
// resolved from the PR itself, so printing the original run's resolved
// BaseRef back at the user is redundant and was reported as confusing
// noise in the reproduce snippet.
func TestReproduceCommand_OmitsTargetBranch(t *testing.T) {
	got := ReproduceCommand(Options{RepoURL: "https://example.com/repo.git", PR: "42", BaseRef: "origin/main"})
	if strings.Contains(got, "--target-branch") {
		t.Errorf("ReproduceCommand() = %q, want it to omit --target-branch", got)
	}
}
