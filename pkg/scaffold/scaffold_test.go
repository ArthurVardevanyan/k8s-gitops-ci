package scaffold

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHasScaffoldEnabled_Default(t *testing.T) {
	dir := t.TempDir()
	if !HasScaffoldEnabled(dir) {
		t.Error("expected scaffold enabled by default (no test.sh)")
	}
}

func TestHasScaffoldEnabled_ExplicitlyDisabled(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "test.sh"), "SCAFFOLD=false\n")
	if HasScaffoldEnabled(dir) {
		t.Error("expected scaffold disabled")
	}
}

func TestIsOverlayDisabled(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "scaffoldDisabled:\n  - dev\n")

	if !IsOverlayDisabled("myapp", "dev") {
		t.Error("expected dev to be disabled")
	}
	if IsOverlayDisabled("myapp", "prod") {
		t.Error("expected prod to not be disabled")
	}
	if IsOverlayDisabled("no-such-app", "dev") {
		t.Error("expected an app with no config to have nothing disabled")
	}
}

func TestHasScaffoldConfig(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	if HasScaffoldConfig("myapp") {
		t.Error("expected no config initially")
	}
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	if !HasScaffoldConfig("myapp") {
		t.Error("expected a config to be found")
	}
}

func TestIsChangeGroupDisabled(t *testing.T) {
	groups := map[string]int{"dev": 0, "prod": 1}
	if !IsChangeGroupDisabled("dev", groups) {
		t.Error("expected group 0 to be disabled")
	}
	if IsChangeGroupDisabled("prod", groups) {
		t.Error("expected group 1 to not be disabled")
	}
	if IsChangeGroupDisabled("unknown", groups) {
		t.Error("expected an absent cluster to not be disabled")
	}
	if IsChangeGroupDisabled("dev", nil) {
		t.Error("expected a nil map to disable nothing")
	}
}

func TestIsExcludedCluster(t *testing.T) {
	orig := ExcludedClusters
	ExcludedClusters = map[string]bool{"decommissioned-cluster": true}
	t.Cleanup(func() { ExcludedClusters = orig })

	if !IsExcludedCluster("decommissioned-cluster") {
		t.Error("expected decommissioned-cluster to be excluded")
	}
	if IsExcludedCluster("dev") {
		t.Error("expected dev to not be excluded")
	}
}

func TestChangedOverlayNames(t *testing.T) {
	got := ChangedOverlayNames("app", []string{"app/overlays/dev/file.yaml", "app/overlays/prod/file.yaml", "app/base/x.yaml"})
	if len(got) != 2 || got[0] != "dev" || got[1] != "prod" {
		t.Errorf("unexpected overlays: %v", got)
	}
}

func TestExtractOverlayDir(t *testing.T) {
	if got := ExtractOverlayDir("app/overlays/dev/base/kustomization.yaml"); got != "dev" {
		t.Errorf("ExtractOverlayDir = %q", got)
	}
}

func TestExtractCreatedFiles(t *testing.T) {
	got := ExtractCreatedFiles("some log line\ncreated app/overlays/dev/foo.yaml\nother line\ncreated app/overlays/prod/foo.yaml\n")
	if len(got) != 2 {
		t.Fatalf("expected 2 created files, got %v", got)
	}
}

func TestIsInChangedFiles(t *testing.T) {
	if !IsInChangedFiles("dev", []string{"app/overlays/dev/base.yaml"}) {
		t.Error("expected in changed files")
	}
	if IsInChangedFiles("prod", []string{"app/overlays/dev/base.yaml"}) {
		t.Error("expected prod to not be in changed files")
	}
}

func TestStripANSI(t *testing.T) {
	got := stripANSI("\x1b[31mred\x1b[0m")
	if got != "red" {
		t.Errorf("stripANSI = %q", got)
	}
}

func TestFindOverlays(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	got := FindOverlays(app)
	if len(got) != 2 {
		t.Errorf("expected 2 overlays, got %v", got)
	}
}

// ── Run ──────────────────────────────────────────────────────────────────────

func withFakeScafctl(t *testing.T, fn func(ctx context.Context, configPath, outputDir string) error) {
	t.Helper()
	orig := runScafctl
	runScafctl = fn
	t.Cleanup(func() { runScafctl = orig })
}

func TestRun_NoConfigFails(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if len(summary.Errors) == 0 {
		t.Error("expected an error for a missing scaffold config")
	}
	if summary.Failed != 1 {
		t.Errorf("expected 1 failure, got %d", summary.Failed)
	}
}

func TestRun_SkipsNonExistentOverlay(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")

	withFakeScafctl(t, func(context.Context, string, string) error {
		t.Fatal("scafctl should never be invoked when every overlay is skipped")
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"not-rolled-out-yet"}})
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped overlay, got %d", summary.Skipped)
	}
	if len(summary.SkippedClusters) != 1 || summary.SkippedClusters[0] != "not-rolled-out-yet" {
		t.Errorf("expected SkippedClusters to list the missing overlay, got %v", summary.SkippedClusters)
	}
	if summary.Failed != 0 {
		t.Errorf("expected 0 failures for a skipped (not failed) overlay, got %d", summary.Failed)
	}
}

func TestRun_SkipsDisabledOverlay(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "scaffoldDisabled:\n  - dev\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	withFakeScafctl(t, func(context.Context, string, string) error {
		t.Fatal("scafctl should never be invoked when every overlay is disabled")
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped overlay, got %d", summary.Skipped)
	}
}

// TestRun_SkipsConfigDisabledOverlay guards the `overlayDefinitions.overrides.<cluster>.disabled`
// flag: an overlay marked disabled in its scaffold config's override section
// is skipped for scaffolding - even though its on-disk directory exists -
// and is recorded in DisabledClusters (distinct from the missing-directory
// / scaffoldDisabled skips) so callers can warn on it specifically. This is
// the mechanism that keeps a downstream consumer from hard-failing when a PR
// edits files under an overlay whose config intentionally disables it.
func TestRun_SkipsConfigDisabledOverlay(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "overlayDefinitions:\n  overrides:\n    retired1:\n      disabled: true\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "retired1", "kustomization.yaml"), "resources: []\n")

	withFakeScafctl(t, func(context.Context, string, string) error {
		t.Fatal("scafctl should never be invoked when the only overlay is disabled in config")
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"retired1"}})
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped overlay, got %d", summary.Skipped)
	}
	if len(summary.DisabledClusters) != 1 || summary.DisabledClusters[0] != "retired1" {
		t.Errorf("expected DisabledClusters to list the config-disabled overlay, got %v", summary.DisabledClusters)
	}
	if summary.Failed != 0 {
		t.Errorf("expected 0 failures for a config-disabled (skipped) overlay, got %d", summary.Failed)
	}
}

// TestRun_DisabledButMissingOverlayIsPlainSkip guards that a missing on-disk
// directory takes precedence over a config-disabled flag: an overlay both
// marked `disabled: true` AND absent from disk (e.g. deleted by this PR) is a
// plain skip - it must land in SkippedClusters, never in DisabledClusters,
// because warning about a deleted overlay would be misleading.
func TestRun_DisabledButMissingOverlayIsPlainSkip(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "overlayDefinitions:\n  overrides:\n    retired2:\n      disabled: true\n")

	withFakeScafctl(t, func(context.Context, string, string) error {
		t.Fatal("scafctl should never be invoked when the overlay is missing")
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"retired2"}})
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped overlay, got %d", summary.Skipped)
	}
	if len(summary.SkippedClusters) != 1 || summary.SkippedClusters[0] != "retired2" {
		t.Errorf("expected SkippedClusters to list the missing overlay, got %v", summary.SkippedClusters)
	}
	if len(summary.DisabledClusters) != 0 {
		t.Errorf("expected no DisabledClusters for a missing overlay, got %v", summary.DisabledClusters)
	}
}

func TestRun_SkipsExcludedCluster(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "decommissioned", "kustomization.yaml"), "resources: []\n")

	origExcluded := ExcludedClusters
	ExcludedClusters = map[string]bool{"decommissioned": true}
	t.Cleanup(func() { ExcludedClusters = origExcluded })

	withFakeScafctl(t, func(context.Context, string, string) error {
		t.Fatal("scafctl should never be invoked when every overlay is excluded")
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"decommissioned"}})
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped overlay, got %d", summary.Skipped)
	}
	if len(summary.SkippedClusters) != 1 || summary.SkippedClusters[0] != "decommissioned" {
		t.Errorf("expected SkippedClusters to list the excluded overlay, got %v", summary.SkippedClusters)
	}
}

func TestRun_ScafctlExecutionFailure(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	withFakeScafctl(t, func(context.Context, string, string) error {
		return errors.New("boom")
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed != 1 {
		t.Errorf("expected 1 failure, got %d", summary.Failed)
	}
	if len(summary.Errors) != 1 {
		t.Errorf("expected 1 error, got %v", summary.Errors)
	}
}

func TestRun_PassesWhenGeneratedMatchesCommitted(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	withFakeScafctl(t, func(_ context.Context, _, outputDir string) error {
		genDir := filepath.Join(outputDir, "dev")
		if err := os.MkdirAll(genDir, 0o750); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(genDir, "kustomization.yaml"), []byte("resources: []\n"), 0o600)
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Passed != 1 {
		t.Errorf("expected 1 pass, got %d (errors: %v, mismatches: %v)", summary.Passed, summary.Errors, summary.MismatchFiles)
	}
	if summary.Failed != 0 {
		t.Errorf("expected 0 failures, got %d", summary.Failed)
	}
}

func TestRun_DetectsMismatch(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	withFakeScafctl(t, func(_ context.Context, _, outputDir string) error {
		genDir := filepath.Join(outputDir, "dev")
		if err := os.MkdirAll(genDir, 0o750); err != nil {
			return err
		}
		// Different content than what's committed -> a real mismatch.
		return os.WriteFile(filepath.Join(genDir, "kustomization.yaml"), []byte("resources:\n  - extra.yaml\n"), 0o600)
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed != 1 {
		t.Errorf("expected 1 failure, got %d", summary.Failed)
	}
	if len(summary.MismatchFiles) != 1 || summary.MismatchFiles[0] != "dev" {
		t.Errorf("expected dev in MismatchFiles, got %v", summary.MismatchFiles)
	}
}

func TestRun_BoundedParallelMultipleOverlays(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	clusters := []string{"a", "b", "c", "d"}
	for _, c := range clusters {
		mustWrite(t, filepath.Join("myapp", "overlays", c, "kustomization.yaml"), "resources: []\n")
	}

	withFakeScafctl(t, func(_ context.Context, _, outputDir string) error {
		for _, c := range clusters {
			genDir := filepath.Join(outputDir, c)
			if err := os.MkdirAll(genDir, 0o750); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(genDir, "kustomization.yaml"), []byte("resources: []\n"), 0o600); err != nil {
				return err
			}
		}
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: clusters})
	if summary.Passed != len(clusters) {
		t.Errorf("expected all %d overlays to pass, got %d passed (errors: %v)", len(clusters), summary.Passed, summary.Errors)
	}
	if summary.Total != len(clusters) {
		t.Errorf("Total = %d, want %d", summary.Total, len(clusters))
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── DryRunParse mode ─────────────────────────────────────────────────────────

// fakeScaffoldTool writes a fake scaffold CLI to a temp dir and points
// Binary at it, restoring Binary/DriftMode/ScaffoldArgs/CreatedFileMarkers
// on cleanup. The script echoes the SCAFFOLD_OUTPUT env var to stdout, so a
// test controls exactly what "created" lines the tool reports. exitCode
// lets a test simulate a tool execution failure.
func fakeScaffoldTool(t *testing.T, output string, exitCode int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell tool not supported on windows")
	}
	bin := filepath.Join(t.TempDir(), "faketool")
	script := "#!/bin/sh\nprintf '%s' \"$SCAFFOLD_OUTPUT\"\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // test-only fake tool
		t.Fatal(err)
	}
	t.Setenv("SCAFFOLD_OUTPUT", output)

	origBin, origMode, origArgs, origMarkers := Binary, DriftMode, ScaffoldArgs, CreatedFileMarkers
	Binary = bin
	DriftMode = DryRunParse
	CreatedFileMarkers = []string{"Created File:"}
	ScaffoldArgs = func(_, _ string, _ bool) []string { return []string{"scaffold"} }
	t.Cleanup(func() {
		Binary, DriftMode, ScaffoldArgs, CreatedFileMarkers = origBin, origMode, origArgs, origMarkers
	})
}

func TestRun_DryRunParse_PerClusterPass(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	fakeScaffoldTool(t, "", 0) // no "created" lines -> no drift

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Passed != 1 || summary.Failed != 0 {
		t.Errorf("expected clean pass, got passed=%d failed=%d errors=%v", summary.Passed, summary.Failed, summary.Errors)
	}
}

func TestRun_DryRunParse_PerClusterDrift(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	// The tool reports it would create a file for an existing overlay -> drift.
	fakeScaffoldTool(t, "Created File: myapp/overlays/dev/new.yaml\n", 0)

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed == 0 {
		t.Errorf("expected drift failure, got passed=%d failed=%d", summary.Passed, summary.Failed)
	}
	if len(summary.MismatchFiles) != 1 || summary.MismatchFiles[0] != "dev" {
		t.Errorf("expected [dev] in MismatchFiles (normalized to cluster name), got %v", summary.MismatchFiles)
	}
	// Drift must be reported only via MismatchFiles, never as an execution
	// error - otherwise it would unconditionally block, bypassing the
	// blocking-vs-pre-existing drift classification.
	if len(summary.Errors) != 0 {
		t.Errorf("expected no execution errors for pure drift, got %v", summary.Errors)
	}
}

func TestRun_DryRunParse_FullTestSkipsNewCluster(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	// One created file for an existing overlay (drift) and one for a cluster
	// with no on-disk overlay and not in the changeset (new cluster -> skip).
	fakeScaffoldTool(t, "Created File: myapp/overlays/dev/new.yaml\nCreated File: myapp/overlays/future/new.yaml\n", 0)

	// FullTest runs a single all-overlays invocation; overlays list must
	// include an on-disk cluster so it survives the pre-filter to toRun.
	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}, FullTest: true})
	if len(summary.MismatchFiles) != 1 || summary.MismatchFiles[0] != "dev" {
		t.Errorf("expected [dev] mismatch (normalized to cluster name), got %v", summary.MismatchFiles)
	}
	if len(summary.Errors) != 0 {
		t.Errorf("expected no execution errors for pure drift, got %v", summary.Errors)
	}
	if len(summary.SkippedClusters) != 1 || summary.SkippedClusters[0] != "future" {
		t.Errorf("expected 'future' skipped as a new cluster, got %v", summary.SkippedClusters)
	}
}

func TestRun_DryRunParse_ExecFailure(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	// Tool exits non-zero for an existing overlay -> execution error.
	fakeScaffoldTool(t, "boom\n", 1)

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed == 0 || len(summary.Errors) == 0 {
		t.Errorf("expected an execution failure, got passed=%d failed=%d errors=%v", summary.Passed, summary.Failed, summary.Errors)
	}
}

func TestRun_DryRunParse_MissingScaffoldArgs(t *testing.T) {
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	origMode, origArgs := DriftMode, ScaffoldArgs
	DriftMode = DryRunParse
	ScaffoldArgs = nil
	t.Cleanup(func() { DriftMode, ScaffoldArgs = origMode, origArgs })

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if len(summary.Errors) == 0 || summary.Failed == 0 {
		t.Error("expected an error when ScaffoldArgs is unset in DryRunParse mode")
	}
}

func TestExtractCreatedFiles_CldctlMarkers(t *testing.T) {
	orig := CreatedFileMarkers
	CreatedFileMarkers = []string{"Created File:", "✓ Dry Run - Would Have Created File:"}
	t.Cleanup(func() { CreatedFileMarkers = orig })

	got := ExtractCreatedFiles(
		"noise\nCreated File: a/overlays/dev/x.yaml\n" +
			"✓ Dry Run - Would Have Created File: a/overlays/prod/y.yaml\nmore noise\n",
	)
	if len(got) != 2 {
		t.Fatalf("expected 2 created files, got %v", got)
	}
	if got[0] != "a/overlays/dev/x.yaml" || got[1] != "a/overlays/prod/y.yaml" {
		t.Errorf("unexpected parse: %v", got)
	}
}

// ── Transient-failure retry ──────────────────────────────────────────────────

// applyRetryDefaults sets RetryAttempts for a test, clears RetryBackoff (so
// retry tests stay fast), OnRetry and IsTransientError (so hooks/matchers set
// by an earlier test can't leak in; retryExec falls back to the default matcher
// when IsTransientError is nil), restoring all four on cleanup.
func applyRetryDefaults(t *testing.T, attempts int) func() {
	t.Helper()
	origAttempts, origBackoff, origOnRetry, origIsTransient := RetryAttempts, RetryBackoff, OnRetry, IsTransientError
	RetryAttempts, RetryBackoff, OnRetry, IsTransientError = attempts, 0, nil, nil
	return func() {
		RetryAttempts, RetryBackoff, OnRetry, IsTransientError = origAttempts, origBackoff, origOnRetry, origIsTransient
	}
}

// fakeScaffoldToolCounting points Binary at a stateful fake that records the
// invocation count in countFile and, on each call, runs flakyScript. flakyScript
// can reference the shell variable $n (the 1-based invocation count) to vary
// behavior per call, e.g. to fail once then succeed. It omits the
// SCAFFOLD_OUTPUT env var intent of fakeScaffoldTool, letting the script fully
// control output. Callers must restore package vars via applyRetryDefaults.
func fakeScaffoldToolCounting(t *testing.T, countFile, flakyScript string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell tool not supported on windows")
	}
	bin := filepath.Join(t.TempDir(), "faketool")
	script := "#!/bin/sh\n" +
		"n=$(cat \"$COUNT\" 2>/dev/null || echo 0)\n" +
		"n=$((n+1))\n" +
		"echo \"$n\" > \"$COUNT\"\n" +
		flakyScript + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // test-only fake tool
		t.Fatal(err)
	}
	t.Setenv("COUNT", countFile)

	origBin, origMode, origArgs, origMarkers := Binary, DriftMode, ScaffoldArgs, CreatedFileMarkers
	Binary = bin
	DriftMode = DryRunParse
	CreatedFileMarkers = []string{"Created File:"}
	ScaffoldArgs = func(_, _ string, _ bool) []string { return []string{"scaffold"} }
	t.Cleanup(func() {
		Binary, DriftMode, ScaffoldArgs, CreatedFileMarkers = origBin, origMode, origArgs, origMarkers
	})
}

// invocationCount reads countFile (1-based invocation count) or 0 if absent.
func invocationCount(t *testing.T, countFile string) int {
	t.Helper()
	data, err := os.ReadFile(countFile)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return n
}

// setupRetryApp writes the app/overlay scaffolding needed for a DryRunParse
// per-cluster run and returns the count file path.
func setupRetryApp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	return filepath.Join(dir, "calls")
}

func TestDefaultIsTransientError(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "eof", in: "doWebCall - failed to read the response body (Status Code: 200): unexpected EOF", want: true},
		{name: "connection reset", in: "read tcp: connection reset by peer", want: true},
		{name: "broken pipe", in: "write tcp: broken pipe", want: true},
		{name: "i/o timeout", in: "dial tcp: i/o timeout", want: true},
		{name: "no such host", in: "lookup host.example: no such host", want: true},
		{name: "connection refused", in: "connect: connection refused", want: true},
		{name: "tls handshake timeout", in: "tls: handshake timeout", want: true},
		{name: "context deadline exceeded is NOT transient", in: "context deadline exceeded", want: false},
		{name: "plain drift text is not transient", in: "config parse error: unexpected token", want: false},
		{name: "empty", in: "", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := defaultIsTransientError(c.in); got != c.want {
				t.Errorf("defaultIsTransientError(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestRetryExec_DoesNotMutatePackageVars(t *testing.T) {
	restore := applyRetryDefaults(t, 0) // 0 attempts clamped to 1 internally
	defer restore()
	var calls int
	_, err := retryExec(func() (string, error) { calls++; return "", errors.New("exit status 1") })
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("expected a single attempt (attempts<1 clamps to 1), got %d", calls)
	}
	if RetryAttempts != 0 {
		t.Errorf("retryExec must not mutate the package var RetryAttempts, got %d", RetryAttempts)
	}
}

// TestRetryExec_DoesNotRetryTimeout guards the fail-fast contract: a context
// deadline (timeout) is never retried even when the partial output happens to
// match a transient signature, so a genuinely hung tool doesn't extend
// wall-clock.
func TestRetryExec_DoesNotRetryTimeout(t *testing.T) {
	restore := applyRetryDefaults(t, 5)
	defer restore()

	var calls int
	const transientOutput = "doWebCall - failed to read the response body (Status Code: 200): unexpected EOF"
	_, err := retryExec(func() (string, error) {
		calls++
		return transientOutput, fmt.Errorf("%w: %s", context.DeadlineExceeded, transientOutput)
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a context deadline error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected a timeout to fail fast (single invocation), got %d", calls)
	}
}

func TestComputeBackoff(t *testing.T) {
	base := 3 * time.Second
	cases := []struct {
		name     string
		base     time.Duration
		attempt  int
		want     time.Duration
		overflow bool
	}{
		{name: "non-positive base is zero", base: 0, attempt: 2, want: 0},
		{name: "negative base is zero", base: -time.Second, attempt: 2, want: 0},
		{name: "attempt zero is zero", base: base, attempt: 0, want: 0},
		{name: "attempt negative is zero", base: base, attempt: -1, want: 0},
		{name: "first attempt is base", base: base, attempt: 1, want: base},
		{name: "second attempt doubles", base: base, attempt: 2, want: 2 * base},
		{name: "third attempt quadruples", base: base, attempt: 3, want: 4 * base},
		{name: "huge attempt clamps (no overflow)", base: time.Nanosecond, attempt: maxBackoffShift + 100, want: time.Duration(int64(1) << uint(maxBackoffShift))},
		{name: "overflow clamps to max duration", base: time.Duration(math.MaxInt64 / 100), attempt: 25, want: 0, overflow: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computeBackoff(c.base, c.attempt)
			if got < 0 {
				t.Fatalf("computeBackoff returned a negative duration: %v (overflow)", got)
			}
			if c.overflow {
				if got != time.Duration(math.MaxInt64) {
					t.Errorf("expected max time.Duration clamp, got %v", got)
				}
				return
			}
			if got != c.want {
				t.Errorf("computeBackoff(%v, %d) = %v, want %v", c.base, c.attempt, got, c.want)
			}
		})
	}
}

func TestRun_DryRunParse_TransientRetrySucceeds(t *testing.T) {
	countFile := setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	var retries int
	OnRetry = func(_, _ int, _ error) { retries++ }

	// First invocation fails with a transient network EOF; second succeeds.
	fakeScaffoldToolCounting(t, countFile,
		"if [ \"$n\" -eq 1 ]; then\n"+
			"  printf 'doWebCall - failed to read the response body (Status Code: 200): unexpected EOF'\n"+
			"  exit 1\n"+
			"fi\n"+
			"exit 0\n",
	)

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed != 0 || len(summary.Errors) != 0 {
		t.Errorf("expected transient failure to be retried to success, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if summary.Passed != 1 {
		t.Errorf("expected 1 pass after retry, got passed=%d", summary.Passed)
	}
	if got := invocationCount(t, countFile); got != 2 {
		t.Errorf("expected 2 invocations (1 fail + 1 retry), got %d", got)
	}
	if retries != 1 {
		t.Errorf("expected OnRetry called once, got %d", retries)
	}
}

func TestRun_DryRunParse_NonTransientErrorNotRetried(t *testing.T) {
	countFile := setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	var retries int
	OnRetry = func(_, _ int, _ error) { retries++ }

	// Always fails with a non-transient signature - must fail fast, once.
	fakeScaffoldToolCounting(t, countFile, "printf 'config parse error: boom'\nexit 1\n")

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed == 0 || len(summary.Errors) == 0 {
		t.Errorf("expected an execution failure, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if got := invocationCount(t, countFile); got != 1 {
		t.Errorf("expected no retry for non-transient error, got %d invocations", got)
	}
	if retries != 0 {
		t.Errorf("expected OnRetry not called for non-transient error, got %d", retries)
	}
}

func TestRun_DryRunParse_TransientRetryExhausted(t *testing.T) {
	countFile := setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	// Always fails transiently - after RetryAttempts it must still fail.
	fakeScaffoldToolCounting(t, countFile,
		"printf 'doWebCall - failed to read the response body (Status Code: 200): unexpected EOF'\nexit 1\n",
	)

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed == 0 || len(summary.Errors) == 0 {
		t.Errorf("expected an execution failure after exhausting retries, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if got := invocationCount(t, countFile); got != 3 {
		t.Errorf("expected %d invocations (attempts exhausted), got %d", RetryAttempts, got)
	}
	if fullErr := func() bool { return len(summary.Errors) > 0 }(); fullErr {
		// The final error surfaced should still be the transient message (the
		// build attempt's own output), not a "retries exhausted" wrapper.
		if !strings.Contains(summary.Errors[0], "unexpected EOF") {
			t.Errorf("expected final error to surface the underlying failure, got %q", summary.Errors[0])
		}
	}
}

// ── DiffDirs retry path ──────────────────────────────────────────────────────

// TestRun_DiffDirs_TransientRetrySucceeds verifies the DiffDirs execution path
// (runScafctl) retries a transient failure and succeeds on retry.
func TestRun_DiffDirs_TransientRetrySucceeds(t *testing.T) {
	_ = setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	calls := 0
	withFakeScafctl(t, func(_ context.Context, _, outputDir string) error {
		calls++
		if calls == 1 {
			return errors.New("doWebCall - failed to read the response body (Status Code: 200): unexpected EOF")
		}
		// Generate matching content for the committed overlay so the diff passes.
		mustWrite(t, filepath.Join(outputDir, "dev", "kustomization.yaml"), "resources: []\n")
		return nil
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}}) // DriftMode defaults to DiffDirs
	if summary.Failed != 0 || len(summary.Errors) != 0 {
		t.Errorf("expected DiffDirs transient failure to be retried to success, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if summary.Passed != 1 {
		t.Errorf("expected 1 pass after DiffDirs retry, got passed=%d", summary.Passed)
	}
	if calls != 2 {
		t.Errorf("expected 2 scafctl invocations in DiffDirs (1 fail + 1 retry), got %d", calls)
	}
}

// TestRun_DiffDirs_NonTransientNotRetried verifies DiffDirs fails fast (single
// invocation) on a non-transient failure.
func TestRun_DiffDirs_NonTransientNotRetried(t *testing.T) {
	_ = setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	calls := 0
	withFakeScafctl(t, func(_ context.Context, _, _ string) error {
		calls++
		return errors.New("config parse error")
	})

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}})
	if summary.Failed == 0 || len(summary.Errors) == 0 {
		t.Errorf("expected a DiffDirs execution failure, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if calls != 1 {
		t.Errorf("expected no DiffDirs retry for a non-transient error, got %d invocations", calls)
	}
}

// ── FullTest retry path ─────────────────────────────────────────────────────

// TestRun_DryRunParse_FullTest_TransientRetrySucceeds verifies the FullTest
// dry-run path uses the same retry-then-succeed semantics as the per-cluster
// path.
func TestRun_DryRunParse_FullTest_TransientRetrySucceeds(t *testing.T) {
	countFile := setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	fakeScaffoldToolCounting(t, countFile,
		"if [ \"$n\" -eq 1 ]; then\n"+
			"  printf 'doWebCall - failed to read the response body (Status Code: 200): unexpected EOF'\n"+
			"  exit 1\n"+
			"fi\n"+
			"exit 0\n",
	)

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}, FullTest: true})
	if summary.Failed != 0 || len(summary.Errors) != 0 {
		t.Errorf("expected FullTest transient failure to be retried to success, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if got := invocationCount(t, countFile); got != 2 {
		t.Errorf("expected 2 FullTest invocations (1 fail + 1 retry), got %d", got)
	}
}

func TestScaffoldExecError(t *testing.T) {
	timeout := fmt.Errorf("%w: boom", context.DeadlineExceeded)
	cases := []struct {
		name   string
		err    error
		output string
		want   string
	}{
		{name: "timeout", err: timeout, output: "context deadline exceeded", want: fmt.Sprintf("scaffold timed out for myapp (%s)", runTimeout)},
		{name: "transient network", err: errors.New("exit status 1"), output: "doWebCall - unexpected EOF", want: "scaffold command failed for myapp: doWebCall - unexpected EOF"},
		{name: "config error", err: errors.New("exit status 2"), output: "config parse error", want: "scaffold command failed for myapp: config parse error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scaffoldExecError("myapp", c.output, c.err); got != c.want {
				t.Errorf("scaffoldExecError() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestRun_DryRunParse_FullTest_TimeoutNotRetried verifies a context-deadline
// timeout text is not treated as transient, so the FullTest path fails fast on
// a single invocation.
func TestRun_DryRunParse_FullTest_TimeoutNotRetried(t *testing.T) {
	countFile := setupRetryApp(t)
	restore := applyRetryDefaults(t, 3)
	defer restore()

	fakeScaffoldToolCounting(t, countFile,
		"printf 'scaffold timed out: context deadline exceeded'\nexit 1\n",
	)

	summary := Run(RunOptions{App: "myapp", Overlays: []string{"dev"}, FullTest: true})
	if summary.Failed == 0 || len(summary.Errors) == 0 {
		t.Errorf("expected a FullTest execution failure, got failed=%d errors=%v", summary.Failed, summary.Errors)
	}
	if got := invocationCount(t, countFile); got != 1 {
		t.Errorf("expected no retry for context deadline exceeded, got %d invocations", got)
	}
}
