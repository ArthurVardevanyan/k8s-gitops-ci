package overlay

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeRefsFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// makeRefsApp builds:
//
//	<root>/kyverno/base/{kustomization.yaml,namespace.yaml}
//	<root>/kyverno/components/admission-controller/1.15.1/{kustomization.yaml,deployment.yaml}
//	<root>/kyverno/components/admission-controller/1.18.1/{kustomization.yaml,deployment.yaml}
//	<root>/kyverno/overlays/pd01 -> refs base + 1.15.1
//	<root>/kyverno/overlays/pd02 -> refs base + 1.18.1
//	<root>/kyverno/overlays/pd03 -> refs base + 1.18.1
//
// mirroring the legacy reference-implementation's TestFilterOverlaysByRefs
// fixture, so a change to the 1.15.1 component only scopes to pd01, and a
// change to 1.18.1 scopes to pd02+pd03.
func makeRefsApp(t *testing.T) (app string, overlays []string) {
	t.Helper()
	root := t.TempDir()
	app = filepath.Join(root, "kyverno")

	writeRefsFile(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - namespace.yaml\n")
	writeRefsFile(t, filepath.Join(app, "base", "namespace.yaml"), "kind: Namespace\n")

	writeRefsFile(t, filepath.Join(app, "components", "admission-controller", "1.15.1", "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	writeRefsFile(t, filepath.Join(app, "components", "admission-controller", "1.15.1", "deployment.yaml"), "kind: Deployment\n")

	writeRefsFile(t, filepath.Join(app, "components", "admission-controller", "1.18.1", "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	writeRefsFile(t, filepath.Join(app, "components", "admission-controller", "1.18.1", "deployment.yaml"), "kind: Deployment\n")

	pd01 := filepath.Join(app, "overlays", "pd01")
	writeRefsFile(t, filepath.Join(pd01, "kustomization.yaml"), "resources:\n  - ../../base\n  - ../../components/admission-controller/1.15.1\n")

	pd02 := filepath.Join(app, "overlays", "pd02")
	writeRefsFile(t, filepath.Join(pd02, "kustomization.yaml"), "resources:\n  - ../../base\n  - ../../components/admission-controller/1.18.1\n")

	pd03 := filepath.Join(app, "overlays", "pd03")
	writeRefsFile(t, filepath.Join(pd03, "kustomization.yaml"), "resources:\n  - ../../base\n  - ../../components/admission-controller/1.18.1\n")

	return app, []string{pd01, pd02, pd03}
}

func TestFilterOverlaysByRefs_BaseChangeKeepsAllOverlays(t *testing.T) {
	app, overlays := makeRefsApp(t)
	changed := []string{app + "/base/namespace.yaml"}
	got := FilterOverlaysByRefs(app, overlays, changed)
	if len(got) != 3 {
		t.Fatalf("expected all 3 overlays, got %d: %v", len(got), got)
	}
}

func TestFilterOverlaysByRefs_ComponentChangeScopesToReferencingOverlays(t *testing.T) {
	app, overlays := makeRefsApp(t)

	t.Run("1.15.1 change scopes to pd01 only", func(t *testing.T) {
		changed := []string{app + "/components/admission-controller/1.15.1/deployment.yaml"}
		got := FilterOverlaysByRefs(app, overlays, changed)
		if len(got) != 1 || filepath.Base(got[0]) != "pd01" {
			t.Fatalf("expected [pd01], got %v", got)
		}
	})

	t.Run("1.18.1 change scopes to pd02+pd03", func(t *testing.T) {
		changed := []string{app + "/components/admission-controller/1.18.1/deployment.yaml"}
		got := FilterOverlaysByRefs(app, overlays, changed)
		names := make([]string, len(got))
		for i, g := range got {
			names[i] = filepath.Base(g)
		}
		sort.Strings(names)
		if len(names) != 2 || names[0] != "pd02" || names[1] != "pd03" {
			t.Fatalf("expected [pd02 pd03], got %v", names)
		}
	})
}

func TestFilterOverlaysByRefs_UnrelatedAppNotIncluded(t *testing.T) {
	app, overlays := makeRefsApp(t)
	// A change entirely under a different app should find no changed dirs
	// for this app and hit the safety valve (return all) rather than
	// incorrectly narrowing/expanding based on unrelated files.
	changed := []string{"external-secrets/base/deployment.yaml"}
	got := FilterOverlaysByRefs(app, overlays, changed)
	if len(got) != 3 {
		t.Fatalf("expected safety-valve fallback to all 3 overlays, got %d: %v", len(got), got)
	}
}

func TestFilterOverlaysByRefs_AppRootFileTriggersSafetyValve(t *testing.T) {
	app, overlays := makeRefsApp(t)
	changed := []string{app + "/test.sh"}
	got := FilterOverlaysByRefs(app, overlays, changed)
	if len(got) != 3 {
		t.Fatalf("expected safety-valve fallback to all 3 overlays, got %d: %v", len(got), got)
	}
}

func TestRefsMatchChangedDirs_NoMatch(t *testing.T) {
	pairs := []dirPair{{rel: "a/b", abs: "/tmp/a/b"}}
	if refsMatchChangedDirs([]string{"/tmp/x/y"}, pairs) {
		t.Error("expected no match for unrelated ref/dir")
	}
}

func TestHasOverlays(t *testing.T) {
	app, _ := makeRefsApp(t)
	if !HasOverlays(app) {
		t.Error("expected HasOverlays to be true")
	}
	if HasOverlays(filepath.Join(app, "nonexistent")) {
		t.Error("expected HasOverlays to be false for a dir with no overlays/")
	}
}
