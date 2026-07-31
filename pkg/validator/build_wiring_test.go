package validator

import (
	"path/filepath"
	"strings"
	"testing"
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
	if got := buildHookTable(nil); got != "" {
		t.Errorf("expected empty table for no apps, got %q", got)
	}
}

func TestBuildHookTable_AppWithNoTestScript(t *testing.T) {
	d := t.TempDir()
	if got := buildHookTable([]string{filepath.Join(d, "app-without-test-sh")}); got != "" {
		t.Errorf("expected empty table when no app defines any hook, got %q", got)
	}
}

func TestBuildHookTable_AppWithHooksDefined(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "test.sh"), "#!/bin/sh\nPRE_BUILD_HOOK=./pre.sh\nPOST_VALIDATE_HOOK=./post-validate.sh\n")

	got := buildHookTable([]string{app})
	if got == "" {
		t.Fatal("expected a non-empty hook table")
	}
	if !strings.Contains(got, "PRE_BUILD") || !strings.Contains(got, "POST_BUILD") || !strings.Contains(got, "POST_VALIDATE") {
		t.Errorf("expected all three hook columns in the header, got:\n%s", got)
	}
	if !strings.Contains(got, "✅ defined") {
		t.Errorf("expected at least one '✅ defined' cell, got:\n%s", got)
	}
	if !strings.Contains(got, "—") {
		t.Errorf("expected a '—' cell for the hook that wasn't defined (POST_BUILD), got:\n%s", got)
	}
}

func TestBuildGhostTable_NoApps(t *testing.T) {
	if got := buildGhostTable(nil); got != "" {
		t.Errorf("expected empty table for no apps, got %q", got)
	}
}

func TestBuildGhostTable_AppWithNoOverlays(t *testing.T) {
	d := t.TempDir()
	if got := buildGhostTable([]string{filepath.Join(d, "app-without-overlays")}); got != "" {
		t.Errorf("expected empty table when the app has no overlays, got %q", got)
	}
}

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

	got := buildGhostTable([]string{app})
	if got == "" {
		t.Fatal("expected a non-empty ghost patch table")
	}
	if !strings.Contains(got, "Deployment/missing") {
		t.Errorf("expected the ghost patch target in the table, got:\n%s", got)
	}
}
