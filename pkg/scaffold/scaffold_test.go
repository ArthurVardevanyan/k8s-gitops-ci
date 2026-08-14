package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
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
