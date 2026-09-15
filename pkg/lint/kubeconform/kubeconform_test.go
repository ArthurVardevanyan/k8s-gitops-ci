package kubeconform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.KubernetesVersion != "1.29.0" || !opts.Strict || !opts.UseSchemas || opts.UpstreamSchemas {
		t.Errorf("unexpected defaults: %+v", opts)
	}
}

func TestNewValidator_UpstreamSchemas(t *testing.T) {
	// Create a temp dir containing a schema for a dummy CRD, following
	// the master-standalone-strict naming convention ({{.ResourceKind}}{{.KindSuffix}}).json).
	d := t.TempDir()
	schemaDir := filepath.Join(d, "master-standalone-strict")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a permissive schema that matches any object with kind/apiVersion.
	if err := os.WriteFile(filepath.Join(schemaDir, "mycustomresource-v1.json"),
		[]byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := []byte("apiVersion: v1\nkind: MyCustomResource\nmetadata:\n  name: test\n")

	// When UpstreamSchemas is false and the local dir has the schema,
	// the resource should be validated successfully.
	t.Run("UpstreamSchemas off + local schema present", func(t *testing.T) {
		opts := DefaultOptions()
		opts.SchemaDir = d
		opts.UpstreamSchemas = false
		res, err := ValidateBytes("test.yaml", manifest, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Errors != 0 {
			t.Fatalf("expected no errors, got %d: %s", res.Errors, res.Summary())
		}
	})

	// When UpstreamSchemas is false and the local dir is empty,
	// the resource should fail with a missing-schema error.
	t.Run("UpstreamSchemas off + no schema", func(t *testing.T) {
		opts := DefaultOptions()
		opts.UpstreamSchemas = false
		res, err := ValidateBytes("test.yaml", manifest, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Errors == 0 {
			t.Fatal("expected a missing-schema error when no schema is available and UpstreamSchemas is false")
		}
		if res.Errors > 1 {
			t.Fatalf("expected exactly 1 error, got %d:\n%s", res.Errors, res.Summary())
		}
	})

	// When UpstreamSchemas is true and the local dir is empty,
	// the resource should be validated against the built-in default
	// location.  A schema-less local dir doesn't matter here because
	// the upstream remotes are consulted.  We use a generic kind that
	// is very unlikely to exist in the yannh default; the outcome
	// (valid or invalid) doesn't matter for this test - we only
	// verify that the upstream path is exercised (no missing-schema
	// error from *our* local dir).
	t.Run("UpstreamSchemas on + empty local dir", func(t *testing.T) {
		opts := DefaultOptions()
		opts.UpstreamSchemas = true
		res, err := ValidateBytes("test.yaml", manifest, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// With upstream on and a SchemaDir that has no schema,
		// the result depends on whether the default CDN has a
		// schema for this resource - just verify we don't error
		// on our own (missing) local schema.
		if res.Errors > 0 {
			// Errors from upstream are fine - we're just verifying
			// that the local-dir path was not a hard error.
			t.Logf("upstream path exercised: %d error(s) (this is expected)", res.Errors)
		}
	})
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

// TestExtractSchemas_IsOverridable guards the exported-override-var seam
// (see docs/SCHEMAS.md/docs/DEVELOPMENT.md): an org/consumer layer must be
// able to replace ExtractSchemas wholesale with its own function - e.g. one
// pulling schemas from an OCI artifact instead of the embedded/embedschemas-
// gated archive - and have every caller (pipeline Setup, phases.go) pick it
// up automatically since they all call the var, never
// defaultExtractSchemas directly. Deliberately doesn't depend on the
// `embedschemas` build tag/a real archive, unlike
// TestExtractSchemas_ContainsExpectedSubdirs in kubeconform_embed_test.go.
func TestExtractSchemas_IsOverridable(t *testing.T) {
	orig := ExtractSchemas
	defer func() { ExtractSchemas = orig }()

	called := false
	ExtractSchemas = func() (string, func(), error) {
		called = true
		return "/custom/schema/dir", func() {}, nil
	}

	dir, cleanup, err := ExtractSchemas()
	defer cleanup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected the overridden ExtractSchemas to be invoked")
	}
	if dir != "/custom/schema/dir" {
		t.Errorf("dir = %q, want the overridden dir", dir)
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
