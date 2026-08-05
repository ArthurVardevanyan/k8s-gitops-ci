package validator

import (
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
)

func TestResolveNonAppHookConfigs_ExactDirectoryMatch(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	dir := filepath.Join(d, "okd", "node-config")
	mustWrite(t, filepath.Join(dir, "test.sh"), "EXEMPTIONS=(check=kubeconform,file=gpu-1.yaml)\n")
	f := filepath.Join(dir, "gpu-1.yaml")
	mustWrite(t, f, "hosts: []\n")

	cfgs := resolveNonAppHookConfigs([]string{f}, hook.SourceLocal)
	defer cleanupNonAppHookConfigs(cfgs)

	cfg, ok := cfgs[dir]
	if !ok || cfg == nil {
		t.Fatalf("expected a resolved config for %q, got %v", dir, cfgs)
	}
	if len(cfg.ExemptSelectors) != 1 || cfg.ExemptSelectors[0].Check != "kubeconform" {
		t.Errorf("expected the exact-directory test.sh's selectors, got %+v", cfg.ExemptSelectors)
	}
}

// TestResolveNonAppHookConfigs_WalksUpToParentDirectory is the regression
// guard for consolidating a shared test.sh at a parent directory (e.g.
// okd/test.sh) covering both files directly in that directory and files in
// a non-app subdirectory (e.g. okd/node-config/*.yaml) that has no test.sh
// of its own.
func TestResolveNonAppHookConfigs_WalksUpToParentDirectory(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	parent := filepath.Join(d, "okd")
	child := filepath.Join(parent, "node-config")
	mustWrite(t, filepath.Join(parent, "test.sh"), "EXEMPTIONS=(check=kubeconform,file=node-config/gpu-1.yaml)\n")
	f := filepath.Join(child, "gpu-1.yaml")
	mustWrite(t, f, "hosts: []\n")

	cfgs := resolveNonAppHookConfigs([]string{f}, hook.SourceLocal)
	defer cleanupNonAppHookConfigs(cfgs)

	cfg, ok := cfgs[parent]
	if !ok || cfg == nil {
		t.Fatalf("expected the parent directory's test.sh to be resolved for a child file with no test.sh of its own; got %v", cfgs)
	}
	if len(cfg.ExemptSelectors) != 1 || cfg.ExemptSelectors[0].Check != "kubeconform" {
		t.Errorf("expected the parent test.sh's selectors, got %+v", cfg.ExemptSelectors)
	}
	// No config should be cached under the child directory itself - the
	// parent's test.sh is what actually got used.
	if _, ok := cfgs[child]; ok {
		t.Errorf("did not expect a config keyed under the child directory %q", child)
	}
}

// TestResolveNonAppHookConfigs_ClosestAncestorWins guards the
// closest-match-wins (not merge-across-ancestors) semantics: when both a
// directory and its parent declare a test.sh, the closer one applies and
// the parent's is never consulted for that file.
func TestResolveNonAppHookConfigs_ClosestAncestorWins(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	parent := filepath.Join(d, "okd")
	child := filepath.Join(parent, "node-config")
	mustWrite(t, filepath.Join(parent, "test.sh"), "EXEMPTIONS=(check=kubeconform,file=install-config.yaml)\n")
	mustWrite(t, filepath.Join(child, "test.sh"), "EXEMPTIONS=(check=kubeconform,file=gpu-1.yaml)\n")
	f := filepath.Join(child, "gpu-1.yaml")
	mustWrite(t, f, "hosts: []\n")

	cfgs := resolveNonAppHookConfigs([]string{f}, hook.SourceLocal)
	defer cleanupNonAppHookConfigs(cfgs)

	if _, ok := cfgs[parent]; ok {
		t.Errorf("did not expect the parent's test.sh to be consulted when the child has its own")
	}
	cfg, ok := cfgs[child]
	if !ok || cfg == nil {
		t.Fatalf("expected the child directory's own test.sh to be resolved, got %v", cfgs)
	}
	if cfg.ExemptSelectors[0].File != "gpu-1.yaml" {
		t.Errorf("expected the child's selector, got %+v", cfg.ExemptSelectors)
	}
}

func TestResolveNonAppHookConfigs_SkipsFilesUnderAppRoot(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources:\n  - ../../base\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "EXEMPTIONS=(check=kubeconform,file=should-not-be-read.yaml)\n")
	f := filepath.Join(app, "overlays", "prod", "kustomization.yaml")

	cfgs := resolveNonAppHookConfigs([]string{f}, hook.SourceLocal)
	defer cleanupNonAppHookConfigs(cfgs)

	if len(cfgs) != 0 {
		t.Errorf("expected no non-app configs for a file under a kustomize app root, got %v", cfgs)
	}
}

func TestResolveNonAppHookConfigs_NoTestShAnywhereReturnsEmpty(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	f := filepath.Join(d, "some", "nested", "dir", "plain.yaml")
	mustWrite(t, f, "hosts: []\n")

	cfgs := resolveNonAppHookConfigs([]string{f}, hook.SourceLocal)
	defer cleanupNonAppHookConfigs(cfgs)

	if len(cfgs) != 0 {
		t.Errorf("expected no configs when no ancestor declares a test.sh, got %v", cfgs)
	}
}

func TestNonAppExemptSelectors_ExtractsFromMultipleConfigs(t *testing.T) {
	t.Parallel()
	cfgs := map[string]*hook.Config{
		"dirA": {ExemptSelectors: []hook.ExemptSelector{{Check: "kubeconform", File: "a.yaml"}}},
		"dirB": {ExemptSelectors: []hook.ExemptSelector{{Check: "kubeconform", File: "b.yaml"}}},
	}
	selectors := nonAppExemptSelectors(cfgs)
	if len(selectors) != 2 {
		t.Fatalf("expected 2 selectors, got %d: %+v", len(selectors), selectors)
	}
}
