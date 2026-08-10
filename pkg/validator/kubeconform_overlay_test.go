package validator

import (
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
)

// kcTestOpts disables schema enforcement so unit tests assert on rendering /
// coverage orchestration rather than the kubeconform schema bundle (which is
// environment-dependent). Schema correctness is covered by
// pkg/lint/kubeconform's own tests.
func kcTestOpts() kubeconform.Options {
	o := kubeconform.DefaultOptions()
	o.IgnoreMissingSchemas = true
	return o
}

// TestValidateRenderedOverlays_SchemaValidatesEachOverlay verifies the
// rendered pass runs kubeconform over each rendered overlay's bytes (the
// AVP/Helm-resolved output the Build YAML phase produced) and merges results.
// Each overlay's data is validated; kubeconform's non-schema-loading options
// keep this deterministic without the full schema bundle.
func TestValidateRenderedOverlays(t *testing.T) {
	t.Parallel()
	rendered := []renderedOverlay{
		{overlay: "appA/overlays/pd01", data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n")},
		{overlay: "appA/overlays/pd02", data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n")},
	}
	res := validateRenderedOverlays(rendered, kcTestOpts(), 2)
	if res == nil {
		t.Fatal("expected a non-nil merged result")
	}
	if res.Invalid > 0 || res.Errors > 0 {
		t.Errorf("unexpected validation failures: %s", res.Summary())
	}
}

// TestValidateRenderedOverlays_EmptyNoError guards that an empty rendered set
// yields an empty, non-nil result without error.
func TestValidateRenderedOverlays_Empty(t *testing.T) {
	t.Parallel()
	res := validateRenderedOverlays(nil, kcTestOpts(), 4)
	if res == nil {
		t.Fatal("expected a non-nil result")
	}
	if res.Valid != 0 || res.Invalid != 0 {
		t.Errorf("expected empty result, got %s", res.Summary())
	}
}

// TestValidateRenderedOverlays_InvalidFlagged verifies schema failures surface
// in the result so the "Kubeconform (Rendered)" section can gate on them.
func TestValidateRenderedOverlays_InvalidFlagged(t *testing.T) {
	t.Parallel()
	rendered := []renderedOverlay{
		{overlay: "app/overlays/pd01", data: []byte("apiVersion: v1\nkind: }\nmetadata:\n  name: broken\n")},
	}
	res := validateRenderedOverlays(rendered, kcTestOpts(), 1)
	if res == nil {
		t.Fatal("expected a non-nil result")
	}
	if res.Invalid == 0 && res.Errors == 0 {
		t.Errorf("expected at least one invalid/error for a malformed manifest, got %s", res.Summary())
	}
}

// TestCoverByScopedOverlays exercises the pre-build exclusion: changed files
// participating in a scoped overlay's build chain (its own overlay dir, its
// app base, referenced components) are covered - and will be validated by the
// post-build rendered pass - while unrelated files are left for the raw pass.
func TestCoverByScopedOverlays(t *testing.T) {
	t.Parallel()
	scoped := []overlayRef{
		{path: filepath.ToSlash("app/overlays/pd01"), cluster: "pd01"},
	}
	cases := []struct {
		name string
		file string
		want bool
	}{
		{"overlay dir file", "app/overlays/pd01/kustomization.yaml", true},
		{"base file referenced by overlay", "app/base/cm.yaml", true},
		{"different overlay", "app/overlays/pd02/kustomization.yaml", false},
		{"different app", "other/base/cm.yaml", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := coverByScopedOverlays(scoped, []string{c.file})
			if (got != nil && got[filepath.Clean(c.file)]) != c.want {
				t.Errorf("coverByScopedOverlays(%q) = %v, want %v", c.file, got[c.file], c.want)
			}
		})
	}
}

// TestFilesNotCovered verifies uncovered files are returned for raw
// validation while covered ones are excluded.
func TestFilesNotCovered(t *testing.T) {
	t.Parallel()
	files := []string{"a/base/cm.yaml", "b/overlays/pd01/x.yaml", "b/base/y.yaml"}
	covered := map[string]bool{
		filepath.Clean("b/overlays/pd01/x.yaml"): true,
	}
	got := filesNotCovered(files, covered)
	want := []string{filepath.Clean("a/base/cm.yaml"), filepath.Clean("b/base/y.yaml")}
	if len(got) != len(want) {
		t.Fatalf("filesNotCovered = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filesNotCovered[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestKubeconformSchemaOpts_PrefetchedDir verifies opts.SchemaDir (from
// pkg/pipeline's Setup prefetch) is honored and no lazy cleanup is returned.
func TestKubeconformSchemaOpts_PrefetchedDir(t *testing.T) {
	t.Parallel()
	opts := Options{SchemaDir: "/tmp/schemas"}
	kcOpts, cleanup := kubeconformSchemaOpts(opts)
	defer cleanup()
	if kcOpts.SchemaDir != "/tmp/schemas" {
		t.Errorf("SchemaDir = %q, want %q", kcOpts.SchemaDir, "/tmp/schemas")
	}
}

// TestKubeconformSchemaOpts_LazyExtract falls back to lazy extraction when no
// SchemaDir is prefetched; the path must never panic and always return a
// callable cleanup regardless of whether extraction succeeds offline.
func TestKubeconformSchemaOpts_LazyExtract(t *testing.T) {
	t.Parallel()
	kcOpts, cleanup := kubeconformSchemaOpts(Options{})
	defer cleanup()
	if kcOpts.SchemaLocations == nil {
		t.Error("expected default schema locations to be set")
	}
}
