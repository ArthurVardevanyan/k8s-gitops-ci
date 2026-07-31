package validator

import (
	"os"
	"os/exec"
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

func TestBuildGhostTable_NoApps(t *testing.T) {
	if got, blocking := buildGhostTable(nil, nil); got != "" || blocking != 0 {
		t.Errorf("expected empty table for no apps, got %q blocking=%d", got, blocking)
	}
}

func TestBuildGhostTable_AppWithNoOverlays(t *testing.T) {
	d := t.TempDir()
	if got, blocking := buildGhostTable([]string{filepath.Join(d, "app-without-overlays")}, nil); got != "" || blocking != 0 {
		t.Errorf("expected empty table when the app has no overlays, got %q blocking=%d", got, blocking)
	}
}

// TestBuildGhostTable_GhostPatchDetected uses a plain (non-git) temp dir,
// so ghostpatch.PatchesSectionChanged always falls back to "unchanged" (no
// git history to diff against) - this only exercises the warning path.
// See ghostpatch_test.go's git-backed tests for blocking-classification
// coverage.
func TestBuildGhostTable_GhostPatchDetected(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	ov := filepath.Join(app, "overlays", "clusterA")
	mustWrite(t, filepath.Join(ov, "kustomization.yaml"), `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    [ { "op": "replace", "path": "/spec/replicas", "value": 1 } ]
`)

	got, blocking := buildGhostTable([]string{app}, nil)
	if got == "" {
		t.Fatal("expected a non-empty ghost patch table")
	}
	if !strings.Contains(got, "Deployment/missing") {
		t.Errorf("expected the ghost patch target in the table, got:\n%s", got)
	}
	if blocking != 0 {
		t.Errorf("expected 0 blocking (no git history to diff against), got %d", blocking)
	}
}

// TestBuildGhostTable_BlockingGhostReflectedInCount uses a real git repo
// (via ghostpatch's own test conventions) so ClassifyOverlay's blocking
// path actually triggers, proving buildGhostTable's blockingCount return
// reflects it (not just the rendered table's cosmetic marker).
func TestBuildGhostTable_BlockingGhostReflectedInCount(t *testing.T) {
	appDir := t.TempDir()
	overlaysDir := filepath.Join(appDir, "overlays", "prod")
	if err := os.MkdirAll(overlaysDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(appDir)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	kustPath := filepath.Join(overlaysDir, "kustomization.yaml")
	mustWrite(t, kustPath, "patches: []\n")
	runGit("init", "-b", "main")
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	mustWrite(t, kustPath, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    []
`)

	got, blocking := buildGhostTable([]string{"."}, nil)
	if got == "" {
		t.Fatal("expected a non-empty ghost patch table")
	}
	if blocking != 1 {
		t.Errorf("expected 1 blocking ghost patch, got %d (table: %s)", blocking, got)
	}
}
