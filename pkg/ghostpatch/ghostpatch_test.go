package ghostpatch

import (
	"os"
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

// TestCheckOverlay_NameLessKindTargetNotGhost is a regression guard: a patch
// that targets by kind only (no name - e.g. `target: {kind:
// CustomResourceDefinition}`, typically paired with a label/annotation
// selector to patch every resource of that kind) must NOT be reported as a
// ghost patch as long as at least one resource of that kind was rendered.
// The bug: exists() only returned true inside `if name != "" && ...`, so a
// name-less target never matched and every such patch was a false positive.
func TestCheckOverlay_NameLessKindTargetNotGhost(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: CustomResourceDefinition
  patch: |-
    - op: add
      path: /metadata/labels/foo
      value: bar
`)
	rendered := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec: {}
`
	ghosts, err := CheckOverlay(ov, rendered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ghosts) != 0 {
		t.Errorf("name-less kind target should match any rendered CRD, not be a ghost: %v", ghosts)
	}
}

// TestCheckOverlay_NameLessKindTargetGhostWhenAbsent confirms the name-less
// case is still flagged when NO resource of that kind exists in the render.
func TestCheckOverlay_NameLessKindTargetGhostWhenAbsent(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: CustomResourceDefinition
  patch: |-
    - op: add
      path: /metadata/labels/foo
      value: bar
`)
	rendered := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec: {}
`
	ghosts, err := CheckOverlay(ov, rendered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ghosts) != 1 {
		t.Errorf("expected 1 ghost when no CRD is rendered, got %v", ghosts)
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

// TestCheckOverlay_RenameWithMapValuedOtherOpNotGhost is the end-to-end
// regression: a kustomize patch that renames a resource
// (IngressController/my-ingress-old -> my-ingress) while also carrying an
// `add` op whose value is a map (the /spec/logging block). Before the
// jsonPatchOp.Value -> yaml.Node fix, decoding the op list failed on the map
// value, the rename was missed, and the pre-rename target was falsely reported
// as a ghost.
func TestCheckOverlay_RenameWithMapValuedOtherOpNotGhost(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: IngressController
    name: my-ingress-old
  patch: |-
    - op: replace
      path: /metadata/name
      value: my-ingress
    - op: replace
      path: /spec/domain
      value: apps.example.com
    - op: add
      path: /spec/logging
      value:
        access:
          destination:
            type: Syslog
            syslog:
              address: "172.30.0.200"
              port: 10514
`)
	rendered := `apiVersion: operator.openshift.io/v1
kind: IngressController
metadata:
  name: my-ingress
spec:
  domain: apps.example.com
`
	ghosts, err := CheckOverlay(ov, rendered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ghosts) != 0 {
		t.Errorf("expected the renamed resource to be recognized, not a ghost: %v", ghosts)
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

// TestRenameFromPatch_MetadataNameRenameWithMapValuedOtherOp is a regression
// guard: real kustomize patches carry ops whose values are maps/arrays (an
// `add /spec/logging` with a map value) alongside a rename. The
// jsonPatchOp.Value field must be tolerant of those non-scalar values on
// unrelated ops, or the whole op-list decode fails and the `/metadata/name`
// rename is missed.
func TestRenameFromPatch_MetadataNameRenameWithMapValuedOtherOp(t *testing.T) {
	patch := `- op: replace
  path: /metadata/name
  value: new-name
- op: add
  path: /spec/logging
  value:
    access:
      destination:
        type: Syslog
`
	if got := renameFromPatch(patch); got != "new-name" {
		t.Errorf("renameFromPatch = %q, want %q", got, "new-name")
	}
}

// TestRenameFromPatch_MapValuedMetadataNameIsNotRename guards the scalar
// guard: a /metadata/name op whose value is itself non-scalar is not a real
// rename and must not be treated as one.
func TestRenameFromPatch_MapValuedMetadataNameIsNotRename(t *testing.T) {
	patch := `- op: replace
  path: /metadata/name
  value:
    nested: true
`
	if got := renameFromPatch(patch); got != "" {
		t.Errorf("renameFromPatch = %q, want empty for a non-scalar metadata/name value", got)
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
	kustPath := filepath.Join(ov, "kustomization.yaml")
	// The kustomization.yaml is both changed and newly-added this PR: a
	// ghost on a brand-new overlay is a warning, not blocking.
	res, err := ClassifyOverlay(ov, "", []string{kustPath}, []string{kustPath})
	if err != nil || len(res) != 1 || res[0].Blocking {
		t.Errorf("new file should be warning: %v err %v", res, err)
	}
}

// TestClassifyOverlay_ExistingTouchedIsBlocking verifies a ghost on an
// existing overlay whose own kustomization.yaml this PR changed is blocking.
func TestClassifyOverlay_ExistingTouchedIsBlocking(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    []
`)
	kustPath := filepath.Join(ov, "kustomization.yaml")
	// Changed but not newly added: this PR modified an existing overlay's
	// kustomization.yaml, so its ghost is blocking.
	res, err := ClassifyOverlay(ov, "", []string{kustPath}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || !res[0].Blocking {
		t.Errorf("expected a ghost on an overlay this PR touched to be blocking: %+v", res)
	}
}

// TestClassifyOverlay_UntouchedIsWarning verifies a ghost on an overlay whose
// kustomization.yaml this PR did not change - pre-existing drift - is a
// non-blocking warning even though it shows up in the rendered table.
func TestClassifyOverlay_UntouchedIsWarning(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    []
`)
	// Overlay not in the changed set -> the ghost predates this PR.
	res, err := ClassifyOverlay(ov, "", []string{"unrelated/change.yaml"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].Blocking {
		t.Errorf("expected an untouched overlay's ghost to be a warning: %+v", res)
	}
}

// TestClassifyOverlay_NewlyAddedIsWarning verifies a ghost on a brand-new
// overlay (kustomization.yaml newly added this PR) is non-blocking - nothing
// shipped with it yet.
func TestClassifyOverlay_NewlyAddedIsWarning(t *testing.T) {
	ov := makeOverlay(t, `patches:
- target:
    kind: Deployment
    name: missing
  patch: |-
    []
`)
	kustPath := filepath.Join(ov, "kustomization.yaml")
	res, err := ClassifyOverlay(ov, "", []string{kustPath}, []string{kustPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].Blocking {
		t.Errorf("expected a newly-added overlay's ghost to be a warning: %+v", res)
	}
}

func TestClassifyApp_NoOverlaysDir(t *testing.T) {
	results, err := ClassifyApp(t.TempDir(), nil, nil)
	if err != nil || len(results) != 0 {
		t.Errorf("expected no results for an app with no overlays/: %v err %v", results, err)
	}
}

// TestClassifyApp_UntouchedOverlayNotBlocking verifies ClassifyApp threads
// the changed-files set through, so a ghost on an overlay this PR did not
// touch stays non-blocking while a touched one is blocking.
func TestClassifyApp_BlockingReflectsChangedSet(t *testing.T) {
	app := filepath.Join(t.TempDir(), "myapp")
	touched := filepath.Join(app, "overlays", "prod")
	untouched := filepath.Join(app, "overlays", "stage")
	write := func(dir string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("patches:\n- target:\n    kind: Deployment\n    name: missing\n  patch: |-\n    []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(touched)
	write(untouched)

	// Only prod's kustomization.yaml was changed by this PR.
	results, err := ClassifyApp(app, []string{filepath.Join(touched, "kustomization.yaml")}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byOverlay := map[string]bool{}
	for _, r := range results {
		if len(r.Ghosts) >= 1 {
			byOverlay[r.Overlay] = r.Ghosts[0].Blocking
		}
	}
	if !byOverlay[touched] {
		t.Errorf("expected the touched overlay's ghost to be blocking: %v", byOverlay)
	}
	if byOverlay[untouched] {
		t.Errorf("expected the untouched overlay's ghost to be non-blocking: %v", byOverlay)
	}
}
