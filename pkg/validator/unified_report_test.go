package validator

import (
	"strings"
	"testing"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
)

func TestReportRender(t *testing.T) {
	r := &Report{
		Marker:    "<!-- m -->",
		Title:     "T",
		Header:    "H",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Sections:  []ReportSection{{Name: "S", Status: StatusError, Body: "b"}},
	}
	out := r.Render()
	if !strings.Contains(out, "T") || !strings.Contains(out, "S") {
		t.Errorf("missing rendered parts: %s", out)
	}
}

// TestReportRender_UsesEachSectionsOwnStatusIcon guards the fix for a
// top-level section only ever being able to show ✅/❌ (a bare bool Error)
// even when its worst child was a non-blocking StatusWarning/StatusInfo -
// hiding that a section had something worth a look, or (for
// StatusWarning-only) overstating it as unchecked-but-fine. Render() must
// use each Section's own Status.Icon() directly.
func TestReportRender_UsesEachSectionsOwnStatusIcon(t *testing.T) {
	r := &Report{
		Marker: "<!-- m -->",
		Title:  "T",
		Sections: []ReportSection{
			{Name: "Passed", Status: StatusPassed, Body: "ok"},
			{Name: "Info", Status: StatusInfo, Body: "fyi"},
			{Name: "Warned", Status: StatusWarning, Body: "careful"},
			{Name: "Failed", Status: StatusError, Body: "boom"},
		},
	}
	out := r.Render()
	for _, want := range []string{
		"✅ Expand: Passed",
		"ℹ️ Expand: Info",
		"⚠️ Expand: Warned",
		"❌ Expand: Failed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in rendered output, got:\n%s", want, out)
		}
	}
}

// TestReportRender_FallsBackToSummaryWhenBodyEmpty guards that a top-level
// section with no Body (every current Compose* function always populates
// one, but a future/external caller might not) falls back to Summary
// rather than rendering an empty <details> body.
func TestReportRender_FallsBackToSummaryWhenBodyEmpty(t *testing.T) {
	r := &Report{
		Marker:   "<!-- m -->",
		Title:    "T",
		Sections: []ReportSection{{Name: "S", Status: StatusPassed, Summary: "All good."}},
	}
	out := r.Render()
	if !strings.Contains(out, "All good.") {
		t.Errorf("expected the Summary fallback to render, got:\n%s", out)
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

// TestReproduceCommand_UsesRealBinaryPath guards the reproduce command's
// invocation form: it must call the distributed CLI binary directly
// (k8s-gitops-ci, per go.mod's module path and cmd/k8s-gitops-ci/main.go)
// rather than the source-checkout-only "go run ./cmd/..." form or the
// literal, non-functional "<binary>" placeholder.
func TestReproduceCommand_UsesRealBinaryPath(t *testing.T) {
	got := ReproduceCommand(Options{RepoURL: "https://example.com/repo.git", PR: "42", BaseRef: "main"})
	if strings.Contains(got, "<binary>") {
		t.Errorf("ReproduceCommand still contains the unresolved <binary> placeholder: %q", got)
	}
	if strings.Contains(got, "go run") {
		t.Errorf("ReproduceCommand = %q, want it to invoke the k8s-gitops-ci binary directly, not via go run", got)
	}
	if !strings.HasPrefix(got, "k8s-gitops-ci pipeline") {
		t.Errorf("ReproduceCommand = %q, want it to invoke the k8s-gitops-ci binary", got)
	}
}

// stubBranding overrides only the binary name; the other branding methods
// return "" so Providers falls back to their generic defaults.
type stubBranding struct{ bin string }

func (s stubBranding) ReportMarker() string   { return "" }
func (s stubBranding) ReportTitle() string    { return "" }
func (s stubBranding) PipelineHeader() string { return "" }
func (s stubBranding) BinaryName() string     { return s.bin }

// TestReproduceCommand_UsesProviderBinaryName verifies the reproduce hint
// honors the org-injected binary name (provider.Branding.BinaryName) so
// downstream distributions (e.g. an org's own forked/renamed binary) emit a
// copy-pasteable command instead of the generic default.
func TestReproduceCommand_UsesProviderBinaryName(t *testing.T) {
	got := ReproduceCommand(Options{
		RepoURL:   "https://example.com/repo.git",
		PR:        "42",
		Providers: provider.Providers{Branding: stubBranding{bin: "acme-gitops-ci"}},
	})
	if !strings.HasPrefix(got, "acme-gitops-ci pipeline") {
		t.Errorf("ReproduceCommand = %q, want it to use the provider binary name", got)
	}
}

// TestReproduceCommand_IncludesScopingFlags guards against the reproduce
// command silently dropping --dirs/--disable-checks/--enable-checks, which
// would make it reproduce a broader (or narrower) changeset than the run
// that actually failed.
func TestReproduceCommand_IncludesScopingFlags(t *testing.T) {
	got := ReproduceCommand(Options{
		RepoURL:        "https://example.com/repo.git",
		PR:             "42",
		BaseRef:        "main",
		Dirs:           []string{"kubernetes/", "tekton/", ".tekton/", "okd/"},
		DisabledChecks: []string{"sync-options"},
		EnabledChecks:  []string{"kyverno"},
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
	for _, unwanted := range []string{"--dirs=", "--disable-checks=", "--enable-checks=", "--assume-openshift", "--lint-only"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("ReproduceCommand() = %q, unexpectedly contains %q", got, unwanted)
		}
	}
}

// TestReproduceCommand_IncludesAssumeOpenShift guards against the reproduce
// command silently dropping --assume-openshift, which would make a local
// reproduction see different sync-options findings than a run made by a
// caller (e.g. a Tekton Task) that always passes --assume-openshift.
func TestReproduceCommand_IncludesAssumeOpenShift(t *testing.T) {
	got := ReproduceCommand(Options{
		RepoURL:         "https://example.com/repo.git",
		PR:              "42",
		BaseRef:         "main",
		AssumeOpenShift: true,
	})
	if !strings.Contains(got, "--assume-openshift") {
		t.Errorf("ReproduceCommand() = %q, want it to contain --assume-openshift", got)
	}
}

// TestReproduceCommand_IncludesLintOnly guards against the reproduce
// command silently dropping --lint-only, which would make a local
// reproduction additionally run (and potentially fail on) the entire Build
// YAML/Post-Build Validation phase that a --lint-only caller (e.g. this
// repo's own self-lint Tekton Task) never even attempted.
func TestReproduceCommand_IncludesLintOnly(t *testing.T) {
	got := ReproduceCommand(Options{
		RepoURL:  "https://example.com/repo.git",
		PR:       "42",
		BaseRef:  "main",
		LintOnly: true,
	})
	if !strings.Contains(got, "--lint-only") {
		t.Errorf("ReproduceCommand() = %q, want it to contain --lint-only", got)
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
