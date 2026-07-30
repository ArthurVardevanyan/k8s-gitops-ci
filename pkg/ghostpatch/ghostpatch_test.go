package ghostpatch

import (
	"path/filepath"
	"testing"
)

func TestTargetString(t *testing.T) {
	if got := (Target{}).String(); got != "<unknown>/<all>" {
		t.Errorf("empty target = %q", got)
	}
	if got := (Target{Kind: "Deployment", Name: "x", Namespace: "ns"}).String(); got != "Deployment/x (ns: ns)" {
		t.Errorf("target = %q", got)
	}
}

func TestCheckOverlay_NoKustomizationYAML(t *testing.T) {
	ghosts, err := CheckOverlay("/tmp/not-an-overlay-lkj", "")
	if err != nil || len(ghosts) != 0 {
		t.Errorf("expected nil for missing kustomization: %v err %v", ghosts, err)
	}
}

func TestCheckOverlay_NoPatches(t *testing.T) {
	ov := makeOverlay(t, `resources: []
`)
	ghosts, err := CheckOverlay(ov, "")
	if err != nil || len(ghosts) != 0 {
		t.Errorf("expected no ghosts: %v err %v", ghosts, err)
	}
}

func TestCheckOverlay_GhostPatchDetected(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    [ { "op": "replace", "path": "/spec/replicas", "value": 1 } ]
`)
	rendered := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: other
spec: {}
`
	ghosts, err := CheckOverlay(ov, rendered)
	if err != nil || len(ghosts) != 1 {
		t.Fatalf("expected one ghost: %v err %v", ghosts, err)
	}
}

func TestCheckOverlay_MatchingPatchNotGhost(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: app
  patch: |-
    [ { "op": "replace", "path": "/spec/replicas", "value": 1 } ]
`)
	rendered := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec: {}
`
	ghosts, err := CheckOverlay(ov, rendered)
	if err != nil || len(ghosts) != 0 {
		t.Errorf("expected no ghosts: %v err %v", ghosts, err)
	}
}

func TestCheckOverlay_RenameViaYAMLPatchNotGhost(t *testing.T) {
	// Regression: a rename patch written in real YAML list syntax (the
	// form kustomize overlays actually use) must be recognized as a
	// rename, so the patch's target (the pre-rename name) correctly
	// pairs with the renamed resource in the rendered output instead of
	// being reported as a ghost patch.
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: old-name
  patch: |-
    - op: replace
      path: /metadata/name
      value: new-name
`)
	rendered := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: new-name
spec: {}
`
	ghosts, err := CheckOverlay(ov, rendered)
	if err != nil || len(ghosts) != 0 {
		t.Errorf("expected the renamed resource to be recognized, not a ghost: %v err %v", ghosts, err)
	}
}

func TestCheckOverlay_DeletePatchIgnored(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: gone
  patch: |-
    $patch: delete
`)
	ghosts, err := CheckOverlay(ov, "")
	if err != nil || len(ghosts) != 0 {
		t.Errorf("expected delete patch ignored: %v err %v", ghosts, err)
	}
}

func TestPatchesSectionChanged_NoMain(t *testing.T) {
	ov := makeOverlay(t, `patches: []
`)
	changed, err := PatchesSectionChanged(filepath.Join(ov, "kustomization.yaml"))
	if err != nil || changed {
		t.Errorf("new file should report unchanged: %v err %v", changed, err)
	}
}

// ── renameFromPatch ───────────────────────────────────────────────────────
//
// Real kustomize patches[].patch blocks use YAML list syntax, not a
// JSON-bracket-and-quotes literal - these tests exercise that real-world
// form directly, since it's the form a JSON-object-literal regex (the
// original implementation) would silently fail to match.

func TestRenameFromPatch_YAMLListForm(t *testing.T) {
	patch := `- op: replace
  path: /metadata/name
  value: new-name
`
	if got := renameFromPatch(patch); got != "new-name" {
		t.Errorf("renameFromPatch = %q, want %q", got, "new-name")
	}
}

func TestRenameFromPatch_AddOp(t *testing.T) {
	patch := `- op: add
  path: /metadata/name
  value: added-name
`
	if got := renameFromPatch(patch); got != "added-name" {
		t.Errorf("renameFromPatch = %q, want %q", got, "added-name")
	}
}

func TestRenameFromPatch_PathWithoutLeadingSlash(t *testing.T) {
	patch := `- op: replace
  path: metadata/name
  value: new-name
`
	if got := renameFromPatch(patch); got != "new-name" {
		t.Errorf("renameFromPatch = %q, want %q", got, "new-name")
	}
}

func TestRenameFromPatch_JSONArrayFormStillWorks(t *testing.T) {
	// JSON is valid YAML flow syntax, so the JSON-bracket form some
	// overlays may still use should continue to work too.
	patch := `[{"op": "replace", "path": "/metadata/name", "value": "new-name"}]`
	if got := renameFromPatch(patch); got != "new-name" {
		t.Errorf("renameFromPatch = %q, want %q", got, "new-name")
	}
}

func TestRenameFromPatch_UnrelatedPathIgnored(t *testing.T) {
	patch := `- op: replace
  path: /spec/replicas
  value: 3
`
	if got := renameFromPatch(patch); got != "" {
		t.Errorf("renameFromPatch = %q, want empty for a non-rename patch", got)
	}
}

func TestRenameFromPatch_MalformedYAML_NoPanic(t *testing.T) {
	if got := renameFromPatch("not: [valid"); got != "" {
		t.Errorf("renameFromPatch = %q, want empty for malformed input", got)
	}
}

func TestClassifyOverlay_NewFileNonBlocking(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    []
`)
	res, err := ClassifyOverlay(ov, "", []string{filepath.Join(ov, "kustomization.yaml")})
	if err != nil || len(res) != 1 || res[0].Blocking {
		t.Errorf("new file should be warning: %v err %v", res, err)
	}
}
