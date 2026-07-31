package kubeconform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kfv "github.com/yannh/kubeconform/pkg/validator"
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

// localValidator returns a validator rooted at the embedded schema archive
// (SchemaLocations=nil, so only the local archive is consulted - no remote
// fallback masking a bug), plus its cleanup func.
func localValidator(t *testing.T) (v kfv.Validator, cleanup func()) {
	t.Helper()
	var dir string
	var err error
	dir, cleanup, err = ExtractSchemas()
	if err != nil {
		t.Fatalf("ExtractSchemas: %v", err)
	}
	opts := DefaultOptions()
	opts.SchemaDir = dir
	opts.SchemaLocations = nil
	v, err = NewValidator(opts)
	if err != nil {
		cleanup()
		t.Fatalf("NewValidator: %v", err)
	}
	return v, cleanup
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

// TestValidateFileBytes_MultiDocAggregatesAllCounters is a regression test
// for a bug where a multi-document file's FileResult.Status reflected only
// the last-processed document (rather than the most severe outcome across
// all documents), and Result-level aggregation (updateResult) then only
// added whichever single counter matched that final status - silently
// dropping the other documents' counts from the totals. It's run in both
// document orders to prove the aggregate isn't order-dependent.
func TestValidateFileBytes_MultiDocAggregatesAllCounters(t *testing.T) {
	v, cleanup := localValidator(t)
	defer cleanup()

	validDoc := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: valid-cm\ndata:\n  key: \"value\"\n"
	invalidDoc := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: invalid-cm\ndata: \"not-a-map\"\n"

	for _, tc := range []struct {
		name string
		data string
	}{
		{"valid-then-invalid", validDoc + "---\n" + invalidDoc},
		{"invalid-then-valid", invalidDoc + "---\n" + validDoc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fr := ValidateFileBytes(v, "multi.yaml", []byte(tc.data))
			if fr.ValidCount != 1 {
				t.Errorf("ValidCount = %d, want 1 (regardless of document order)", fr.ValidCount)
			}
			if fr.InvalidCount != 1 {
				t.Errorf("InvalidCount = %d, want 1 (regardless of document order)", fr.InvalidCount)
			}
			if fr.Status != "invalid" {
				t.Errorf("Status = %q, want %q (error > invalid > valid precedence)", fr.Status, "invalid")
			}

			res := &Result{}
			updateResult(res, fr)
			if res.Valid != 1 || res.Invalid != 1 {
				t.Errorf("Result after updateResult = %+v, want Valid=1 Invalid=1 (both counters must survive aggregation)", res)
			}
		})
	}
}

// TestValidateFileBytes_SignaturePrefixOnValidationError verifies genuine
// validation errors are prefixed with the resource's Kind/Name so distinct
// resources' errors don't collapse together under Result.Deduplicate.
func TestValidateFileBytes_SignaturePrefixOnValidationError(t *testing.T) {
	v, cleanup := localValidator(t)
	defer cleanup()

	data := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: invalid-cm\ndata: \"not-a-map\"\n"
	fr := ValidateFileBytes(v, "bad.yaml", []byte(data))
	if fr.InvalidCount != 1 || len(fr.Errors) == 0 {
		t.Fatalf("expected an invalid ConfigMap with errors, got %+v", fr)
	}
	if !strings.Contains(fr.Errors[0], `ConfigMap "invalid-cm"`) {
		t.Errorf("expected error prefixed with resource signature, got: %s", fr.Errors[0])
	}
}

// TestValidateFileBytes_MissingSchemaHintAppendedWithoutSignaturePrefix
// verifies MissingSchemaHint is actually surfaced (it was previously
// declared but never referenced anywhere - dead code) and that
// missing-schema errors are deliberately left unprefixed by resource
// identity, so identical missing-schema errors across many files collapse
// into a single Result.Deduplicate entry instead of staying separated by
// resource name.
func TestValidateFileBytes_MissingSchemaHintAppendedWithoutSignaturePrefix(t *testing.T) {
	orig := MissingSchemaHint
	MissingSchemaHint = "see docs/SCHEMAS.md for how to add a custom schema"
	defer func() { MissingSchemaHint = orig }()

	v, cleanup := localValidator(t)
	defer cleanup()

	data := "apiVersion: totally.unknown/v1\nkind: TotallyUnknownKind\nmetadata:\n  name: mystery\n"
	fr := ValidateFileBytes(v, "unknown.yaml", []byte(data))
	if fr.ErrorCount != 1 || len(fr.Errors) != 1 {
		t.Fatalf("expected a single schema-not-found error, got %+v", fr)
	}
	if !strings.Contains(fr.Errors[0], missingSchemaMarker) {
		t.Fatalf("expected a missing-schema error, got: %s", fr.Errors[0])
	}
	if !strings.Contains(fr.Errors[0], MissingSchemaHint) {
		t.Errorf("expected MissingSchemaHint appended, got: %s", fr.Errors[0])
	}
	if strings.Contains(fr.Errors[0], `TotallyUnknownKind "mystery"`) {
		t.Errorf("expected missing-schema errors to stay unprefixed (for cross-file dedup), got: %s", fr.Errors[0])
	}
}

func TestDeduplicatedError_StringWithFiles_DefensivelyDedupesDuplicatePaths(t *testing.T) {
	// 12 duplicate entries for the same path must collapse to one in the
	// rendered listing, independent of whatever dedup discipline the
	// caller that built Files did or didn't apply.
	files := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		files = append(files, filepath.Join("app", "file.yaml"))
	}
	d := DeduplicatedError{Message: "boom", Count: 12, Files: files}

	out := d.StringWithFiles()
	if strings.Count(out, "app/file.yaml") != 1 {
		t.Errorf("expected the duplicate path to appear exactly once, got: %s", out)
	}
	if strings.Contains(out, "and") && strings.Contains(out, "more") {
		t.Errorf("expected no overflow suffix once duplicates collapse below the cap, got: %s", out)
	}
}

func TestDeduplicatedError_StringWithFiles_CapsAtMaxListedFiles(t *testing.T) {
	files := make([]string, 0, maxListedFiles+3)
	for i := 0; i < maxListedFiles+3; i++ {
		files = append(files, filepath.Join("app", fmt.Sprintf("file-%d.yaml", i)))
	}
	d := DeduplicatedError{Message: "boom", Count: len(files), Files: files}

	out := d.StringWithFiles()
	if !strings.Contains(out, "and 3 more") {
		t.Errorf("expected overflow suffix for the 3 files beyond the %d-file cap, got: %s", maxListedFiles, out)
	}
	// String() (no files) must stay compact for callers that don't want
	// the file listing.
	if strings.Contains(d.String(), "files:") {
		t.Errorf("expected String() to omit the file listing, got: %s", d.String())
	}
}
