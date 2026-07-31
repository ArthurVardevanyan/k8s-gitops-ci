package shellcheck

import (
	"os/exec"
	"strings"
	"testing"
)

// hasShellcheckBinary reports whether a real shellcheck binary is
// available in PATH - RunTekton/RunEmbedded end-to-end tests are skipped
// (not failed) when it isn't, matching this repo's documented
// graceful-degradation convention for pkg/lint/* CLI wrappers.
func hasShellcheckBinary() bool {
	_, err := exec.LookPath("shellcheck")
	return err == nil
}

func TestIsBashScript(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   bool
	}{
		{"env-bash", "#!/usr/bin/env bash\necho hi", true},
		{"bin-bash", "#!/bin/bash\necho hi", true},
		{"usr-bin-bash", "#!/usr/bin/bash\necho hi", true},
		{"env-bash-with-args", "#!/usr/bin/env bash -eu\necho hi", true},
		{"python", "#!/usr/bin/env python3\nprint('hi')", false},
		{"plain-sh", "#!/bin/sh\necho hi", false},
		{"no-shebang", "echo hi\n", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isBashScript(c.script); got != c.want {
				t.Errorf("isBashScript(%q) = %v, want %v", c.script, got, c.want)
			}
		})
	}
}

func TestAdjustLineRef(t *testing.T) {
	cases := []struct {
		name       string
		output     string
		tmpPath    string
		lineOffset int
		want       string
	}{
		{
			name:       "offset math",
			output:     "/tmp/shellcheck-123.sh:3:10: warning: msg [SC2045]\n",
			tmpPath:    "/tmp/shellcheck-123.sh",
			lineOffset: 9,
			want:       "<extracted-script>:11:10: warning: msg [SC2045]\n",
		},
		{
			name:       "zero-offset passthrough",
			output:     "/tmp/shellcheck-123.sh:3:10: warning: msg [SC2045]\n",
			tmpPath:    "/tmp/shellcheck-123.sh",
			lineOffset: 0,
			want:       "<extracted-script>:3:10: warning: msg [SC2045]\n",
		},
		{
			name:       "non-matching line untouched",
			output:     "some unrelated line with no file ref\n",
			tmpPath:    "/tmp/shellcheck-123.sh",
			lineOffset: 9,
			want:       "some unrelated line with no file ref\n",
		},
		{
			name:       "empty output",
			output:     "",
			tmpPath:    "/tmp/shellcheck-123.sh",
			lineOffset: 9,
			want:       "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := adjustLineRef(c.output, c.tmpPath, "<extracted-script>", c.lineOffset)
			if got != c.want {
				t.Errorf("adjustLineRef() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFilterTektonTasks(t *testing.T) {
	out := FilterTektonTasks([]string{"testdata/good-task.yaml", "testdata/not-task.yaml", "readme.md", "script.sh"})
	if len(out) != 2 {
		t.Fatalf("expected the 2 YAML files, got %d: %v", len(out), out)
	}
}

func TestExtractScripts_GoodTask(t *testing.T) {
	scripts, err := ExtractScripts("testdata/good-task.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 extracted script, got %d: %v", len(scripts), scripts)
	}
	if scripts[0].TaskName != "build" || scripts[0].StepName != "build" {
		t.Errorf("unexpected identity: %+v", scripts[0])
	}
	if !strings.Contains(scripts[0].Script, "echo \"building\"") {
		t.Errorf("unexpected script content: %q", scripts[0].Script)
	}
	if scripts[0].LineOffset != 9 {
		t.Errorf("expected LineOffset 9 (the shebang line), got %d", scripts[0].LineOffset)
	}
}

func TestExtractScripts_PythonTaskSkipped(t *testing.T) {
	scripts, err := ExtractScripts("testdata/python-task.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 0 {
		t.Fatalf("expected a python step to be skipped, got %d: %v", len(scripts), scripts)
	}
}

func TestExtractScripts_MultiStep(t *testing.T) {
	// 3 steps: bash / python / no-script - only 1 extracted.
	scripts, err := ExtractScripts("testdata/multi-step.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected exactly 1 extracted (bash) script, got %d: %v", len(scripts), scripts)
	}
	if scripts[0].StepName != "bash-step" {
		t.Errorf("expected the bash step, got: %+v", scripts[0])
	}
}

func TestExtractScripts_NotTask(t *testing.T) {
	scripts, err := ExtractScripts("testdata/not-task.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 0 {
		t.Fatalf("expected 0 scripts from a ConfigMap, got %d: %v", len(scripts), scripts)
	}
}

func TestExtractScripts_MissingFile(t *testing.T) {
	_, err := ExtractScripts("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected an error for a nonexistent file")
	}
}

func TestRunTekton_EndToEnd(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	results, err := RunTekton([]string{"testdata/invalid/bad-task.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if len(r.Violations) == 0 {
		t.Fatal("expected at least 1 violation for the known-bad script")
	}
	if r.Violations[0].File != "testdata/invalid/bad-task.yaml" {
		t.Errorf("expected the violation's File to be the original YAML file, not a temp path, got: %q", r.Violations[0].File)
	}
	if strings.Contains(r.Output, "shellcheck-") {
		t.Errorf("expected the random temp filename to be replaced with a stable placeholder in Output, got: %q", r.Output)
	}
	if !strings.Contains(r.Output, "<extracted-script>") {
		t.Errorf("expected the stable placeholder in Output, got: %q", r.Output)
	}
}

func TestRunTekton_GoodTaskNoViolations(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	results, err := RunTekton([]string{"testdata/good-task.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if len(r.Violations) != 0 {
			t.Errorf("expected no violations for a clean script, got: %v", r.Violations)
		}
	}
}

func TestRunTekton_NoShellcheckBinary_NoPanic(t *testing.T) {
	// Regardless of whether shellcheck is installed in this environment,
	// calling RunTekton over a file with no bash steps at all must never
	// panic and must return cleanly.
	results, err := RunTekton([]string{"testdata/not-task.yaml"})
	if err != nil {
		t.Fatalf("expected no error for a file with nothing to extract, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got: %v", results)
	}
}
