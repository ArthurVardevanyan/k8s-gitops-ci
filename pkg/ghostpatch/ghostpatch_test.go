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
