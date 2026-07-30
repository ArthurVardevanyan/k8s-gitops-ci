package kubeconform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.KubernetesVersion != "1.29.0" || !opts.Strict || !opts.UseSchemas {
		t.Errorf("unexpected defaults: %+v", opts)
	}
}

func TestDeduplicate(t *testing.T) {
	r := &Result{Details: []FileResult{
		{Filename: "a.yaml", Errors: []string{"err1"}},
		{Filename: "b.yaml", Errors: []string{"err1"}},
	}}
	ded := r.Deduplicate()
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}

func TestResultSummary(t *testing.T) {
	r := &Result{Valid: 1, Invalid: 0, Errors: 0, Skipped: 0}
	if !strings.Contains(r.Summary(), "1 valid") {
		t.Errorf("summary missing valid: %s", r.Summary())
	}
}

func TestValidateFiles_EmptyList(t *testing.T) {
	r, err := ValidateFiles([]string{}, DefaultOptions())
	if err != nil || r.Valid != 0 {
		t.Errorf("expected empty result: %+v err %v", r, err)
	}
}

// TestSchemaLocations_CustomStandaloneStrict verifies the schema location
// template used for OKD/OpenShift-style CRDs (multi-segment API group)
// renders to the archive's actual, flat {kind}-{full-group}-{version}.json
// filename convention. KindSuffix-based templates cannot represent this,
// since KindSuffix truncates the group at the first dot.
func TestSchemaLocations_CustomStandaloneStrict(t *testing.T) {
	locs := SchemaLocations("/schemas")
	want := filepath.Join("/schemas", "custom-standalone-strict", "{{.ResourceKind}}-{{.Group}}-{{.ResourceAPIVersion}}.json")
	found := false
	for _, l := range locs {
		if l == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected custom-standalone-strict template %q in %v", want, locs)
	}
}

// TestExtractSchemas_ContainsExpectedSubdirs verifies ExtractSchemas returns
// a directory that directly contains the archive's flat-folder layout
// (custom-standalone-strict, master-standalone-strict, master-local),
// without needing a further "kubernetes-json-schema" path segment.
func TestExtractSchemas_ContainsExpectedSubdirs(t *testing.T) {
	dir, cleanup, err := ExtractSchemas()
	if err != nil {
		t.Fatalf("ExtractSchemas: %v", err)
	}
	defer cleanup()
	for _, sub := range []string{"custom-standalone-strict", "master-standalone-strict", "master-local"} {
		if fi, err := os.Stat(filepath.Join(dir, sub)); err != nil || !fi.IsDir() {
			t.Errorf("expected %s to exist under %s: %v", sub, dir, err)
		}
	}
}

// TestValidateFiles_OKDCustomResourceSchema is a regression test for the
// "could not find schema for KubeletConfig" class of bug: OKD/OpenShift CRDs
// whose schema is only present in the embedded custom-standalone-strict
// archive must resolve when Options.SchemaDir is wired up, instead of
// silently falling through to remote registries that don't carry them.
func TestValidateFiles_OKDCustomResourceSchema(t *testing.T) {
	dir, cleanup, err := ExtractSchemas()
	if err != nil {
		t.Fatalf("ExtractSchemas: %v", err)
	}
	defer cleanup()

	tmp := t.TempDir()
	file := filepath.Join(tmp, "kubelet-config.yaml")
	content := `apiVersion: machineconfiguration.openshift.io/v1
kind: KubeletConfig
metadata:
  name: worker-kubelet-config
spec:
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/worker: ""
  kubeletConfig:
    maxPods: 500
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	opts := DefaultOptions()
	opts.SchemaDir = dir
	// Only consult the local archive: any remaining "could not find
	// schema" failures here must come from our own SchemaLocations
	// wiring, not from a remote fallback masking the bug.
	opts.SchemaLocations = nil

	res, err := ValidateFiles([]string{file}, opts)
	if err != nil {
		t.Fatalf("ValidateFiles: %v", err)
	}
	if res.Errors > 0 {
		for _, d := range res.Details {
			for _, e := range d.Errors {
				t.Errorf("unexpected error for %s: %s", d.Filename, e)
			}
		}
	}
	if res.Valid == 0 && res.Invalid == 0 {
		t.Errorf("expected KubeletConfig schema to be found (valid or invalid), got %+v", res)
	}
}
