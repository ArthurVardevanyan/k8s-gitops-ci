package validator

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestResolveTargetOverlays_AppAndCluster_ReturnsBaseAndMatchingOverlayOnly(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(app, "overlays", "clusterA", "kustomization.yaml"), "resources:\n  - ../../base\n")
	mustWrite(t, filepath.Join(app, "overlays", "clusterB", "kustomization.yaml"), "resources:\n  - ../../base\n")

	files, err := resolveTargetOverlays(Options{Apps: []string{app}, Clusters: []string{"clusterA"}})
	if err != nil {
		t.Fatalf("resolveTargetOverlays: %v", err)
	}
	sort.Strings(files)
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "clusterB") {
			t.Errorf("expected clusterB to be excluded, got file: %s", f)
		}
	}
	foundBase, foundClusterA := false, false
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "/base/") {
			foundBase = true
		}
		if strings.Contains(filepath.ToSlash(f), "clusterA") {
			foundClusterA = true
		}
	}
	if !foundBase {
		t.Error("expected base/ files to be included")
	}
	if !foundClusterA {
		t.Error("expected clusterA overlay files to be included")
	}
}

func TestResolveTargetOverlays_AppOnly_ReturnsWholeAppDir(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(app, "overlays", "clusterA", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "overlays", "clusterB", "kustomization.yaml"), "resources: []\n")

	files, err := resolveTargetOverlays(Options{Apps: []string{app}})
	if err != nil {
		t.Fatalf("resolveTargetOverlays: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected all 3 files under the app dir, got %d: %v", len(files), files)
	}
}

func TestResolveTargetOverlays_SkipsNonexistentAppAndCluster(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(app, "overlays", "clusterA", "kustomization.yaml"), "resources: []\n")

	files, err := resolveTargetOverlays(Options{
		Apps:     []string{app, filepath.Join(d, "does-not-exist")},
		Clusters: []string{"clusterA", "typo-cluster"},
	})
	if err != nil {
		t.Fatalf("resolveTargetOverlays: %v", err)
	}
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "does-not-exist") || strings.Contains(filepath.ToSlash(f), "typo-cluster") {
			t.Errorf("expected nonexistent app/cluster to be skipped, got: %s", f)
		}
	}
	if len(files) != 2 {
		t.Errorf("expected base + clusterA files only, got %d: %v", len(files), files)
	}
}

func TestResolveTargetOverlays_AppWithNoMatchingClusterSkipsBaseToo(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(app, "overlays", "clusterB", "kustomization.yaml"), "resources: []\n")

	_, err := resolveTargetOverlays(Options{Apps: []string{app}, Clusters: []string{"clusterA"}})
	if err == nil {
		t.Error("expected an error when no app has any of the requested clusters")
	}
}

func TestResolveTargetOverlays_ErrorsWhenNothingMatches(t *testing.T) {
	d := t.TempDir()
	_, err := resolveTargetOverlays(Options{Apps: []string{filepath.Join(d, "does-not-exist")}})
	if err == nil {
		t.Error("expected an error when no app/cluster resolves to anything on disk")
	}
}

func TestResolveTargetOverlays_ClustersOnly_DiscoversAppsAcrossRepo(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d) // GetAllFiles/discoverAppRoots operate relative to cwd

	appWithCluster := filepath.Join(d, "app1")
	mustWrite(t, filepath.Join(appWithCluster, "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(appWithCluster, "overlays", "clusterA", "kustomization.yaml"), "resources: []\n")

	appWithoutCluster := filepath.Join(d, "app2")
	mustWrite(t, filepath.Join(appWithoutCluster, "base", "deployment.yaml"), "kind: Deployment\n")
	mustWrite(t, filepath.Join(appWithoutCluster, "overlays", "clusterB", "kustomization.yaml"), "resources: []\n")

	files, err := resolveTargetOverlays(Options{Clusters: []string{"clusterA"}})
	if err != nil {
		t.Fatalf("resolveTargetOverlays: %v", err)
	}
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "app2") {
			t.Errorf("expected app2 (no clusterA overlay) to be excluded, got: %s", f)
		}
	}
	if len(files) == 0 {
		t.Error("expected app1's base+clusterA files to be included")
	}
}

func TestResolveChangeset_TargetModeTakesPriorityOverDirs(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"), "kind: Deployment\n")
	otherDir := filepath.Join(d, "other")
	mustWrite(t, filepath.Join(otherDir, "unrelated.yaml"), "kind: ConfigMap\n")

	files, err := resolveChangeset(Options{Apps: []string{app}, Dirs: []string{otherDir}})
	if err != nil {
		t.Fatalf("resolveChangeset: %v", err)
	}
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "unrelated") {
			t.Errorf("expected Dirs to be ignored when Apps/Clusters targeting is active, got: %s", f)
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected only the targeted app's file, got %d: %v", len(files), files)
	}
}
