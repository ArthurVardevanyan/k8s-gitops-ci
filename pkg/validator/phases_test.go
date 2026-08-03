package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

func TestToIDSet_Empty(t *testing.T) {
	if got := toIDSet(nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
	if got := toIDSet([]string{}); got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

func TestToIDSet_Populated(t *testing.T) {
	set := toIDSet([]string{"sync-options", "golangci"})
	if !set["sync-options"] || !set["golangci"] {
		t.Fatalf("expected both ids present, got %v", set)
	}
	if set["kyverno"] {
		t.Errorf("expected kyverno absent")
	}
}

func TestStepEnabled_DefaultOnStepEnabledByDefault(t *testing.T) {
	if !stepEnabled("sync-options", nil, nil) {
		t.Errorf("default-on step should be enabled with no lists set")
	}
}

func TestStepEnabled_DefaultOnStepDisabledViaDisabledChecks(t *testing.T) {
	disabled := toIDSet([]string{"sync-options"})
	if stepEnabled("sync-options", disabled, nil) {
		t.Errorf("expected sync-options disabled")
	}
	// An unrelated ID must remain enabled.
	if !stepEnabled(stepGolangci, disabled, nil) {
		t.Errorf("expected golangci to remain enabled")
	}
}

func TestStepEnabled_DefaultOffStepDisabledByDefault(t *testing.T) {
	if stepEnabled(stepKyverno, nil, nil) {
		t.Errorf("kyverno should default to disabled")
	}
}

func TestStepEnabled_DefaultOffStepEnabledViaEnabledChecks(t *testing.T) {
	enabled := toIDSet([]string{stepKyverno})
	if !stepEnabled(stepKyverno, nil, enabled) {
		t.Errorf("expected kyverno enabled once listed in EnabledChecks")
	}
}

func TestStepEnabled_EnabledChecksHasNoEffectOnDefaultOnSteps(t *testing.T) {
	// Listing a default-on step in EnabledChecks is a no-op - it's already
	// enabled, and only DisabledChecks can turn it off.
	enabled := toIDSet([]string{stepGolangci})
	if !stepEnabled(stepGolangci, nil, enabled) {
		t.Errorf("expected golangci enabled")
	}
	disabled := toIDSet([]string{stepGolangci})
	if stepEnabled(stepGolangci, disabled, enabled) {
		t.Errorf("DisabledChecks must win over an unrelated EnabledChecks entry for a default-on step")
	}
}

// --- shellcheck extraction wiring (PR-7) -----------------------------------

func hasShellcheckBinary() bool {
	_, err := exec.LookPath("shellcheck")
	return err == nil
}

func TestWriteShellcheckExtractionReport_TektonAndEmbedded(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	d := t.TempDir()
	taskFile := filepath.Join(d, "task.yaml")
	mustWrite(t, taskFile, `kind: Task
metadata:
  name: build
spec:
  steps:
  - name: build
    script: |
      #!/usr/bin/env bash
      for f in $(ls); do
        echo $f
      done
`)
	deployFile := filepath.Join(d, "deploy.yaml")
	mustWrite(t, deployFile, `kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      containers:
      - name: main
        command:
        - bash
        - -c
        - |
          #!/usr/bin/env bash
          for f in $(ls); do
            echo $f
          done
`)
	var sb strings.Builder
	total := writeShellcheckExtractionReport(&sb, "", []string{taskFile, deployFile})
	if total != 2 {
		t.Fatalf("expected 2 total violations (1 Tekton + 1 embedded), got %d: %s", total, sb.String())
	}
	if !strings.Contains(sb.String(), "[Tekton build/build]") {
		t.Errorf("expected a Tekton-labeled report line, got: %s", sb.String())
	}
	if !strings.Contains(sb.String(), "[Deployment/app main]") {
		t.Errorf("expected an embedded-labeled report line, got: %s", sb.String())
	}
}

func TestWriteShellcheckExtractionReport_LabelSuffix(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	d := t.TempDir()
	taskFile := filepath.Join(d, "task.yaml")
	mustWrite(t, taskFile, `kind: Task
metadata:
  name: build
spec:
  steps:
  - name: build
    script: |
      #!/usr/bin/env bash
      for f in $(ls); do
        echo $f
      done
`)
	var sb strings.Builder
	total := writeShellcheckExtractionReport(&sb, " (external)", []string{taskFile})
	if total != 1 {
		t.Fatalf("expected 1 violation, got %d", total)
	}
	if !strings.Contains(sb.String(), "(external)") {
		t.Errorf("expected the label suffix to appear in the report, got: %s", sb.String())
	}
}

func TestWriteShellcheckExtractionReport_NoFindings(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "cm.yaml")
	mustWrite(t, f, "kind: ConfigMap\nmetadata:\n  name: cm\n")
	var sb strings.Builder
	total := writeShellcheckExtractionReport(&sb, "", []string{f})
	if total != 0 || sb.String() != "" {
		t.Errorf("expected no findings/report for a plain ConfigMap, got total=%d report=%q", total, sb.String())
	}
}

// TestExternalOverlayYAMLFiles_BaseChangeExposesUnchangedOverlayFile
// exercises the direct-vs-external classification's file-set half: a
// change to a shared base file must resolve to the overlay that depends
// on it, and that overlay's own (unchanged) files must show up as
// "external" candidates - while the base file that WAS changed must be
// excluded (it's already covered as "direct").
func TestExternalOverlayYAMLFiles_BaseChangeExposesUnchangedOverlayFile(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - task.yaml\n")
	mustWrite(t, filepath.Join(app, "base", "task.yaml"), "kind: Task\nmetadata:\n  name: build\n")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources:\n  - ../../base\n")

	changed := []string{filepath.Join(app, "base", "kustomization.yaml")}
	external := externalOverlayYAMLFiles(changed)

	foundOverlayKustomization := false
	for _, f := range external {
		if filepath.Clean(f) == filepath.Clean(filepath.Join(app, "overlays", "dev", "kustomization.yaml")) {
			foundOverlayKustomization = true
		}
		if filepath.Clean(f) == filepath.Clean(changed[0]) {
			t.Errorf("expected the directly-changed file to be excluded from external files, got it in: %v", external)
		}
	}
	if !foundOverlayKustomization {
		t.Errorf("expected the affected overlay's own (unchanged) YAML file to be listed as external, got: %v", external)
	}
}

func TestExternalOverlayYAMLFiles_NoOverlaysAffected(t *testing.T) {
	external := externalOverlayYAMLFiles([]string{"README.md"})
	if len(external) != 0 {
		t.Errorf("expected no external files for a change with no affected overlays, got: %v", external)
	}
}

// TestRunLintAndStaticChecks_LogsDurationSuffix guards a regression where
// each check's "<name>: passed" console line gave no indication of how long
// that check took - unlike the fully-built (but previously unprinted, see
// pkg/validator/timing.go) per-step timing already recorded into the
// TimingCollector for the same checks. Every runLintStep/runStaticStep
// closure now also prints a "(<duration>)" suffix on its terminal pass/fail
// line, using the same time.Since(start) already computed for
// tc.RecordStep - this exercises a real check (config-sort, cheap and
// dependency-free) end-to-end through the logger's file-backed writer to
// assert the suffix actually reaches the printed line, not just the
// TimingCollector.
func TestRunLintAndStaticChecks_LogsDurationSuffix(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "a.yaml"), "kind: Pod\n")
	logPath := filepath.Join(d, "out.log")
	log := logger.NewLogger(true, logPath)
	defer log.Close()

	res := &Result{}
	tc := NewTimingCollector()
	runLintAndStaticChecks([]string{filepath.Join(d, "a.yaml")}, Options{}, res, log, tc, nil)
	log.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	durationSuffixRe := regexp.MustCompile(`config-sort check: passed \([0-9µm.a-zµ]*s\)`)
	if !durationSuffixRe.MatchString(got) {
		t.Errorf("expected a '(<duration>)' suffix on the config-sort check line, got:\n%s", got)
	}
}
