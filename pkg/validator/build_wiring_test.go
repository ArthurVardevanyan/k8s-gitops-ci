package validator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
)

func TestAppFromOverlayPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{filepath.Join("apps", "myapp", "overlays", "mycluster"), filepath.Join("apps", "myapp")},
		{filepath.Join("myapp", "overlays", "clusterA"), "myapp"},
		{"no-overlays-segment", "no-overlays-segment"},
	}
	for _, c := range cases {
		if got := appFromOverlayPath(c.in); got != c.want {
			t.Errorf("appFromOverlayPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniqueApps(t *testing.T) {
	overlays := []overlayRef{
		{path: filepath.Join("app1", "overlays", "a"), cluster: "a"},
		{path: filepath.Join("app1", "overlays", "b"), cluster: "b"},
		{path: filepath.Join("app2", "overlays", "a"), cluster: "a"},
	}
	apps := uniqueApps(overlays)
	if len(apps) != 2 {
		t.Fatalf("expected 2 unique apps, got %d: %v", len(apps), apps)
	}
	if apps[0] != "app1" || apps[1] != "app2" {
		t.Errorf("expected sorted [app1 app2], got %v", apps)
	}
}

func TestUniqueApps_Empty(t *testing.T) {
	if apps := uniqueApps(nil); len(apps) != 0 {
		t.Errorf("expected no apps, got %v", apps)
	}
}

// TestDetectOverlaysForChanges_BaseChangeResolvesOverlay guards the fix for
// a change to a shared base file (no "overlays/" path segment anywhere)
// silently resolving to zero overlays - previously the naive
// path-segment-based detector couldn't see any overlay was affected at all.
// This reproduces the reported HomeLab scenario: a PR only touching
// "<app>/base/config.json" must still resolve to that app's overlay(s).
func TestDetectOverlaysForChanges_BaseChangeResolvesOverlay(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "kubernetes", "renovate")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - config.json\n")
	mustWrite(t, filepath.Join(app, "base", "config.json"), "{}\n")
	mustWrite(t, filepath.Join(app, "overlays", "okd", "kustomization.yaml"), "resources:\n  - ../../base\n")

	changed := []string{filepath.Join(app, "base", "config.json")}
	overlays := detectOverlaysForChanges(changed)
	if len(overlays) != 1 {
		t.Fatalf("expected 1 overlay detected from a base-only change, got %d: %v", len(overlays), overlays)
	}
	want := filepath.ToSlash(filepath.Join(app, "overlays", "okd"))
	if overlays[0].path != want {
		t.Errorf("overlays[0].path = %q, want %q", overlays[0].path, want)
	}
	if overlays[0].cluster != "okd" {
		t.Errorf("overlays[0].cluster = %q, want %q", overlays[0].cluster, "okd")
	}
}

// TestDetectOverlaysForChanges_ComponentChangeScopesToReferencingOverlay
// exercises the ref-chain-scoping path (overlay.FilterOverlaysByRefs): a
// component change affecting several overlays should only resolve to the
// overlay(s) that actually reference that component version, not every
// overlay under the app.
func TestDetectOverlaysForChanges_ComponentChangeScopesToReferencingOverlay(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "kyverno")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - namespace.yaml\n")
	mustWrite(t, filepath.Join(app, "base", "namespace.yaml"), "kind: Namespace\n")
	mustWrite(t, filepath.Join(app, "components", "widget", "1.0.0", "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "components", "widget", "1.0.0", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(app, "components", "widget", "2.0.0", "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "components", "widget", "2.0.0", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(app, "overlays", "pd01", "kustomization.yaml"), "resources:\n  - ../../base\ncomponents:\n  - ../../components/widget/1.0.0\n")
	mustWrite(t, filepath.Join(app, "overlays", "pd02", "kustomization.yaml"), "resources:\n  - ../../base\ncomponents:\n  - ../../components/widget/2.0.0\n")

	changed := []string{filepath.Join(app, "components", "widget", "2.0.0", "deployment.yaml")}
	overlays := detectOverlaysForChanges(changed)
	if len(overlays) != 1 {
		t.Fatalf("expected 1 overlay scoped to the 2.0.0-referencing overlay, got %d: %v", len(overlays), overlays)
	}
	if overlays[0].cluster != "pd02" {
		t.Errorf("overlays[0].cluster = %q, want %q", overlays[0].cluster, "pd02")
	}
}

// TestDetectOverlaysForChanges_DirectOverlayChange keeps covering the
// simple, already-working "overlays/<cluster>/..." case.
func TestDetectOverlaysForChanges_DirectOverlayChange(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	changed := []string{filepath.Join(app, "overlays", "dev", "kustomization.yaml")}
	overlays := detectOverlaysForChanges(changed)
	if len(overlays) != 1 || overlays[0].cluster != "dev" {
		t.Fatalf("expected 1 overlay for cluster dev, got %v", overlays)
	}
}

// TestDetectOverlaysForChanges_UnrelatedChangeYieldsNoOverlays ensures
// changes outside any app (no overlays/base/components ancestor) don't
// spuriously produce overlays.
func TestDetectOverlaysForChanges_UnrelatedChangeYieldsNoOverlays(t *testing.T) {
	overlays := detectOverlaysForChanges([]string{"README.md", "docs/foo.md"})
	if len(overlays) != 0 {
		t.Errorf("expected no overlays, got %v", overlays)
	}
}

func TestBuildOverlayError_MissingOverlay(t *testing.T) {
	d := t.TempDir()
	err := buildOverlayError(filepath.Join(d, "does-not-exist"))
	if err == "" {
		t.Fatal("expected a non-empty build error for a missing overlay")
	}
	const prefix = "kustomize build "
	if len(err) < len(prefix) || err[:len(prefix)] != prefix {
		t.Errorf("expected error to start with %q (matching comments.go's groupBuildErrors format), got %q", prefix, err)
	}
}

func TestBuildOverlayError_ValidOverlay(t *testing.T) {
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "deployment.yaml"), "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	mustWrite(t, filepath.Join(d, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")

	if err := buildOverlayError(d); err != "" {
		t.Errorf("expected a valid overlay to build cleanly, got error: %q", err)
	}
}

func TestBuildHookTable_NoApps(t *testing.T) {
	if got := buildHookTable(nil, nil, nil); got != "" {
		t.Errorf("expected empty table for no apps, got %q", got)
	}
}

func TestBuildHookTable_AppWithNoTestScript(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "app-without-test-sh")
	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	if got := buildHookTable([]string{app}, cfgs, nil); got != "" {
		t.Errorf("expected empty table when no app defines any hook, got %q", got)
	}
}

func TestBuildHookTable_AppWithHooksDefined(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "test.sh"), "#!/bin/sh\nPRE_BUILD_HOOK=./pre.sh\nPOST_VALIDATE_HOOK=./post-validate.sh\n")

	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	results := map[string]*appHookResult{app: {PreBuild: hookRan, PostValidate: hookFailed}}
	got := buildHookTable([]string{app}, cfgs, results)
	if got == "" {
		t.Fatal("expected a non-empty hook table")
	}
	if !strings.Contains(got, "PRE_BUILD") || !strings.Contains(got, "POST_BUILD") || !strings.Contains(got, "POST_VALIDATE") {
		t.Errorf("expected all three hook columns in the header, got:\n%s", got)
	}
	if !strings.Contains(got, "✅ ran") {
		t.Errorf("expected the PRE_BUILD cell to show '✅ ran', got:\n%s", got)
	}
	if !strings.Contains(got, "❌ failed") {
		t.Errorf("expected the POST_VALIDATE cell to show '❌ failed', got:\n%s", got)
	}
	if !strings.Contains(got, "—") {
		t.Errorf("expected a '—' cell for the hook that wasn't defined (POST_BUILD), got:\n%s", got)
	}
}

func TestBuildGhostTable_NoOverlays(t *testing.T) {
	if got, blocking := buildGhostTable(nil, nil, nil); got != "" || blocking != 0 {
		t.Errorf("expected empty table for no rendered overlays, got %q blocking=%d", got, blocking)
	}
}

// TestBuildGhostTable_GhostPatchDetected lays down an overlay whose
// kustomization.yaml is NOT in this run's changed set - i.e. the ghost
// predates this PR - and asserts it surfaces as a non-blocking warning, not
// a blocking finding.
func TestBuildGhostTable_GhostPatchDetected(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	ov := filepath.Join(app, "overlays", "clusterA")
	kustPath := filepath.Join(ov, "kustomization.yaml")
	mustWrite(t, kustPath, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    [ { "op": "replace", "path": "/spec/replicas", "value": 1 } ]
`)

	// The overlay's kustomization.yaml was NOT changed by this PR, so the
	// ghost is pre-existing drift: non-blocking. renderedOverlays mirrors
	// what runBuildAndPostBuild's own build loop would have collected for
	// this overlay - buildGhostTable no longer re-renders or discovers
	// overlays on disk itself.
	rendered := []renderedOverlay{{overlay: ov}}
	got, blocking := buildGhostTable(rendered, []string{filepath.Join(ov, "other.yaml")}, nil)
	if got == "" {
		t.Fatal("expected a non-empty ghost patch table")
	}
	if !strings.Contains(got, "Deployment/missing") {
		t.Errorf("expected the ghost patch target in the table, got:\n%s", got)
	}
	if blocking != 0 {
		t.Errorf("expected 0 blocking for an overlay untouched by this PR, got %d", blocking)
	}
}

// TestBuildGhostTable_BlockingGhostReflectedInCount verifies that a ghost on
// an overlay whose own kustomization.yaml this PR changed counts as blocking
// (proving buildGhostTable's blockingCount return reflects it, not just the
// rendered table's cosmetic marker).
func TestBuildGhostTable_BlockingGhostReflectedInCount(t *testing.T) {
	app := filepath.Join(t.TempDir(), "myapp")
	ov := filepath.Join(app, "overlays", "prod")
	kustPath := filepath.Join(ov, "kustomization.yaml")
	mustWrite(t, kustPath, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    []
`)

	rendered := []renderedOverlay{{overlay: ov}}
	got, blocking := buildGhostTable(rendered, []string{kustPath}, nil)
	if got == "" {
		t.Fatal("expected a non-empty ghost patch table")
	}
	if blocking != 1 {
		t.Errorf("expected 1 blocking ghost patch (overlay kustomization changed by this PR), got %d (table: %s)", blocking, got)
	}
}

// TestBuildGhostTable_OnlyIncludesRenderedOverlays verifies buildGhostTable
// never classifies an overlay that wasn't in renderedOverlays, even if it
// exists on disk with the exact same ghost-producing content - this is the
// entire point of scoping ghost-patch detection to overlays this run
// actually built (see buildGhostTable's doc comment): an overlay this PR
// didn't touch/build is not this run's concern, and previously walking
// every overlay on disk under each app dominated the Build YAML phase's
// wall time for large apps.
func TestBuildGhostTable_OnlyIncludesRenderedOverlays(t *testing.T) {
	app := filepath.Join(t.TempDir(), "myapp")
	rendered := filepath.Join(app, "overlays", "prod")
	notRendered := filepath.Join(app, "overlays", "stage")
	write := func(dir string) {
		t.Helper()
		mustWrite(t, filepath.Join(dir, "kustomization.yaml"), "patches:\n- target:\n    kind: Deployment\n    name: missing\n  patch: |-\n    []\n")
	}
	write(rendered)
	write(notRendered)

	got, _ := buildGhostTable([]renderedOverlay{{overlay: rendered}}, nil, nil)
	if !strings.Contains(got, rendered) {
		t.Errorf("expected the rendered overlay's ghost in the table, got:\n%s", got)
	}
	if strings.Contains(got, notRendered) {
		t.Errorf("expected the non-rendered overlay to be absent from the table, got:\n%s", got)
	}
}
