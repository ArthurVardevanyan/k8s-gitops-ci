package shellcheck

import (
	"strings"
	"testing"
)

func TestExtractEmbedded_JobBash(t *testing.T) {
	scripts, err := ExtractEmbedded("testdata/job-bash.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 extracted script, got %d: %v", len(scripts), scripts)
	}
	if scripts[0].ResourceKind != "Job" || scripts[0].ResourceName != "migrate" || scripts[0].ContainerName != "migrate" {
		t.Errorf("unexpected identity: %+v", scripts[0])
	}
}

func TestExtractEmbedded_CronJobBash(t *testing.T) {
	// The trickiest case: CronJob nests its pod spec three levels deeper
	// (spec.jobTemplate.spec.template.spec).
	scripts, err := ExtractEmbedded("testdata/invalid/cronjob-bash.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 extracted script from the nested CronJob pod spec, got %d: %v", len(scripts), scripts)
	}
	if scripts[0].ResourceKind != "CronJob" {
		t.Errorf("unexpected kind: %q", scripts[0].ResourceKind)
	}
}

func TestExtractEmbedded_DeploymentNoBash(t *testing.T) {
	scripts, err := ExtractEmbedded("testdata/deployment-no-bash.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 0 {
		t.Fatalf("expected 0 scripts for a python command, got %d: %v", len(scripts), scripts)
	}
}

func TestExtractEmbedded_ConfigMapScript(t *testing.T) {
	// One .sh key extracted, one non-.sh key ignored.
	scripts, err := ExtractEmbedded("testdata/configmap-script.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected exactly 1 extracted .sh key, got %d: %v", len(scripts), scripts)
	}
	if scripts[0].ContainerName != "entrypoint.sh" {
		t.Errorf("expected the .sh key as ContainerName, got: %q", scripts[0].ContainerName)
	}
}

func TestExtractEmbedded_NotAWorkloadOrConfigMap(t *testing.T) {
	scripts, err := ExtractEmbedded("testdata/good-task.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 0 {
		t.Fatalf("expected 0 scripts for a Task (not a workload/ConfigMap kind for this extractor), got %d: %v", len(scripts), scripts)
	}
}

func TestExtractEmbedded_MissingFile(t *testing.T) {
	_, err := ExtractEmbedded("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected an error for a nonexistent file")
	}
}

func TestRunEmbedded_EndToEnd(t *testing.T) {
	if !hasShellcheckBinary() {
		t.Skip("shellcheck not installed; skipping end-to-end test")
	}
	results, err := RunEmbedded([]string{"testdata/invalid/cronjob-bash.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if len(results[0].Violations) == 0 {
		t.Fatal("expected at least 1 violation for the known-bad embedded script")
	}
	if results[0].Violations[0].File != "testdata/invalid/cronjob-bash.yaml" {
		t.Errorf("expected the violation's File to be the original YAML file, got: %q", results[0].Violations[0].File)
	}
}

func TestRunEmbedded_SkipsNonYAML(t *testing.T) {
	results, err := RunEmbedded([]string{"testdata/entrypoint.sh"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for a non-YAML file, got: %v", results)
	}
}

func TestCommandScript_ShFormVariants(t *testing.T) {
	// Regression coverage for the shell-detection heuristic in
	// commandScript: "/bin/sh" (suffix match) must also be accepted as a
	// shell invocation, even though isBashScript itself will still reject
	// a non-bash shebang inside it.
	data := []byte(`kind: Pod
metadata:
  name: p
spec:
  containers:
  - name: c
    command:
    - /bin/sh
    - -c
    - "echo hi"
`)
	scripts, err := extractEmbeddedFromBytes(data, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 0 {
		t.Fatalf("expected 0 scripts (a plain /bin/sh command with no bash shebang is not linted), got %d: %v", len(scripts), scripts)
	}
}

func TestCommandScript_NotDashC(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: p
spec:
  containers:
  - name: c
    command:
    - bash
    - myscript.sh
`)
	scripts, err := extractEmbeddedFromBytes(data, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 0 {
		t.Fatalf("expected 0 scripts for a non -c command form, got %d: %v", len(scripts), scripts)
	}
}

func TestExtractEmbedded_InitContainers(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      initContainers:
      - name: init
        command:
        - bash
        - -c
        - |
          #!/usr/bin/env bash
          echo "initializing"
`)
	scripts, err := extractEmbeddedFromBytes(data, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 extracted initContainer script, got %d: %v", len(scripts), scripts)
	}
	if scripts[0].ContainerName != "init" {
		t.Errorf("unexpected container name: %q", scripts[0].ContainerName)
	}
	if !strings.Contains(scripts[0].Script, "initializing") {
		t.Errorf("unexpected script content: %q", scripts[0].Script)
	}
}
