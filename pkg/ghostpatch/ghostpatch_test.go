package ghostpatch

import (
	"os"
	"os/exec"
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

// makeGitRepoOverlay creates a temp git repo (chdir'd into via t.Chdir, so
// git commands - which resolve paths relative to the process's cwd, see
// gitShow - and the returned relOverlayDir both refer to the same tree)
// with relOverlayDir/kustomization.yaml committed to "main" with
// mainContent, then rewritten on disk (without committing) to
// workingContent - simulating an uncommitted PR change to an existing
// (not newly-added) file's patches section, so PatchesSectionChanged/
// ClassifyOverlay can be exercised against a real diff instead of always
// taking the "no git history" fallback path makeOverlay's plain (non-git,
// cwd-independent) temp dirs hit.
func makeGitRepoOverlay(t *testing.T, relOverlayDir, mainContent, workingContent string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-b", "main")
	if relOverlayDir != "." {
		if err := os.MkdirAll(relOverlayDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	kustPath := filepath.Join(relOverlayDir, "kustomization.yaml")
	if err := os.WriteFile(kustPath, []byte(mainContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	if err := os.WriteFile(kustPath, []byte(workingContent), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPatchesSectionChanged_DetectsRealDiff(t *testing.T) {
	makeGitRepoOverlay(t, ".",
		"patches:\n- target:\n    kind: Deployment\n    name: missing\n  patch: |-\n    []\n",
		"patches:\n- target:\n    kind: Deployment\n    name: missing-renamed\n  patch: |-\n    []\n",
	)
	changed, err := PatchesSectionChanged("kustomization.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected a modified patches section to be reported as changed")
	}
}

func TestClassifyOverlay_ExistingFileWithChangedPatchesIsBlocking(t *testing.T) {
	patches := "patches:\n- target:\n    kind: Deployment\n    name: missing\n  patch: |-\n    []\n"
	makeGitRepoOverlay(t, ".", "patches: []\n", patches)
	// Not in addedFiles - this kustomization.yaml already existed on main,
	// only its patches section was modified in the working tree.
	res, err := ClassifyOverlay(".", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || !res[0].Blocking {
		t.Errorf("expected a ghost patch on a modified existing file to be blocking: %+v", res)
	}
}

func TestClassifyOverlay_ExistingFileUnchangedPatchesIsWarning(t *testing.T) {
	// Same patches content on main and in the working tree - the ghost
	// (if the target doesn't exist in renderedYAML) predates this PR, so
	// it's a warning, not something this PR introduced.
	patches := "patches:\n- target:\n    kind: Deployment\n    name: missing\n  patch: |-\n    []\n"
	makeGitRepoOverlay(t, ".", patches, patches)
	res, err := ClassifyOverlay(".", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].Blocking {
		t.Errorf("expected a pre-existing, unchanged ghost patch to be a warning: %+v", res)
	}
}

func TestClassifyApp_NoOverlaysDir(t *testing.T) {
	results, err := ClassifyApp(t.TempDir(), nil)
	if err != nil || len(results) != 0 {
		t.Errorf("expected no results for an app with no overlays/: %v err %v", results, err)
	}
}

func TestClassifyApp_ClassifiesGhostsAcrossOverlays(t *testing.T) {
	patches := "patches:\n- target:\n    kind: Deployment\n    name: missing\n  patch: |-\n    []\n"
	makeGitRepoOverlay(t, filepath.Join("overlays", "prod"), "patches: []\n", patches)

	results, err := ClassifyApp(".", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || len(results[0].Ghosts) != 1 || !results[0].Ghosts[0].Blocking {
		t.Fatalf("expected 1 blocking ghost from the prod overlay: %+v", results)
	}
}
