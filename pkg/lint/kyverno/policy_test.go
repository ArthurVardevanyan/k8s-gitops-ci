package kyverno

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func withNamespaceSelectorLabelKeys(t *testing.T, keys []string) {
	t.Helper()
	orig := NamespaceSelectorLabelKeys
	NamespaceSelectorLabelKeys = keys
	t.Cleanup(func() { NamespaceSelectorLabelKeys = orig })
}

func TestStripNSSelectors_NoOpWhenNoKeysConfigured(t *testing.T) {
	withNamespaceSelectorLabelKeys(t, nil)
	in := []byte(`match:
  namespaceSelector:
    matchLabels:
      example.com/app-id: "foo"
  resources:
    kinds:
    - Pod
`)
	out, err := stripNSSelectors(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(in) {
		t.Errorf("expected data unchanged when no keys configured, got:\n%s", out)
	}
}

func TestStripNSSelectors_RemovesMatchingSelector(t *testing.T) {
	withNamespaceSelectorLabelKeys(t, []string{"example.com/app-id"})
	in := []byte(`match:
  namespaceSelector:
    matchLabels:
      example.com/app-id: "foo"
  resources:
    kinds:
    - Pod
`)
	out, err := stripNSSelectors(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "namespaceSelector") {
		t.Errorf("expected namespaceSelector to be stripped, got:\n%s", out)
	}
	if !strings.Contains(string(out), "kinds") {
		t.Errorf("expected sibling keys to survive, got:\n%s", out)
	}
}

func TestStripNSSelectors_LeavesNonMatchingSelectorIntact(t *testing.T) {
	// A namespaceSelector gated by an unconfigured label key must survive -
	// this is the real regression guard: the old line-based stripper
	// removed every namespaceSelector unconditionally, regardless of which
	// label key (if any) it was actually gated on.
	withNamespaceSelectorLabelKeys(t, []string{"example.com/app-id"})
	in := []byte(`match:
  namespaceSelector:
    matchLabels:
      some.other/key: "bar"
  resources:
    kinds:
    - Pod
`)
	out, err := stripNSSelectors(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "namespaceSelector") {
		t.Errorf("expected the non-matching namespaceSelector to survive, got:\n%s", out)
	}
	if !strings.Contains(string(out), "some.other/key") {
		t.Errorf("expected the original matchLabels key to survive, got:\n%s", out)
	}
}

func TestStripNSSelectors_MultipleKeysAndDocuments(t *testing.T) {
	withNamespaceSelectorLabelKeys(t, []string{"a.com/id", "b.com/id"})
	in := []byte(`match:
  namespaceSelector:
    matchLabels:
      b.com/id: "x"
---
match:
  namespaceSelector:
    matchLabels:
      unrelated: "y"
`)
	out, err := stripNSSelectors(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	docs := strings.Split(string(out), "---")
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents preserved, got %d: %s", len(docs), out)
	}
	if strings.Contains(docs[0], "namespaceSelector") {
		t.Errorf("expected first doc's matching selector stripped, got:\n%s", docs[0])
	}
	if !strings.Contains(docs[1], "namespaceSelector") {
		t.Errorf("expected second doc's non-matching selector to survive, got:\n%s", docs[1])
	}
}

func TestStripNSSelectors_InvalidYAML(t *testing.T) {
	withNamespaceSelectorLabelKeys(t, []string{"a.com/id"})
	if _, err := stripNSSelectors([]byte("not: valid: yaml: [")); err == nil {
		t.Error("expected an error for invalid YAML")
	}
}

func TestBuildPolicies_BaseOnly(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	orig := IncludeComponents
	IncludeComponents = nil
	defer func() { IncludeComponents = orig }()

	dir := t.TempDir()
	writePolicyBase(t, dir)

	out, err := buildPolicies(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "kind: ClusterPolicy") {
		t.Errorf("expected the base policy rendered, got:\n%s", out)
	}
}

func TestBuildPolicies_WithComponent(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	orig := IncludeComponents
	defer func() { IncludeComponents = orig }()

	dir := t.TempDir()
	writePolicyBase(t, dir)
	writePolicyComponent(t, dir, "extra")
	IncludeComponents = []string{"../../components/extra"}

	out, err := buildPolicies(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "kind: ClusterPolicy") {
		t.Errorf("expected the base policy rendered, got:\n%s", out)
	}
	if !strings.Contains(string(out), "name: extra-policy") {
		t.Errorf("expected the component policy rendered, got:\n%s", out)
	}
}

func TestPreparePolicies_StripsConfiguredNamespaceSelector(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	withNamespaceSelectorLabelKeys(t, []string{"example.com/app-id"})

	dir := t.TempDir()
	base := filepath.Join(dir, "kyverno-policies", "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "kustomization.yaml"), []byte("resources:\n  - policy.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: base-policy
spec:
  rules:
  - name: rule1
    match:
      any:
      - resources:
          kinds:
          - Pod
        namespaceSelector:
          matchLabels:
            example.com/app-id: "foo"
`
	if err := os.WriteFile(filepath.Join(base, "policy.yaml"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}

	policyPath, err := preparePoliciesFrom(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("reading prepared policy: %v", err)
	}
	if strings.Contains(string(out), "namespaceSelector") {
		t.Errorf("expected namespaceSelector to be stripped from the prepared policy, got:\n%s", out)
	}
	if !strings.Contains(string(out), "base-policy") {
		t.Errorf("expected the policy itself to survive, got:\n%s", out)
	}
}

func TestPreparePoliciesFrom_FallsBackToBasePoliciesWhenKustomizeBuildFails(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kyverno-policies", "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately omit base/kustomization.yaml so `kustomize build` on the
	// synthesized _ci overlay (which references ../../base as a resource)
	// fails, regardless of whether the kustomize binary is installed.
	policy := "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: base-policy\nspec:\n  rules: []\n"
	if err := os.WriteFile(filepath.Join(base, "policy.yaml"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}

	policyPath, err := preparePoliciesFrom(dir)
	if err != nil {
		t.Fatalf("expected fallback to base policies instead of a hard failure, got error: %v", err)
	}
	if policyPath != base {
		t.Errorf("policyPath = %q, want the base dir %q", policyPath, base)
	}
}

func TestPreparePoliciesFrom_HardFailsWhenNoBasePoliciesExistEither(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kyverno-policies", "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	// No policy files at all under base/ - nothing to fall back to, so
	// this must still surface the original kustomize build error.
	if _, err := preparePoliciesFrom(dir); err == nil {
		t.Error("expected an error when kustomize build fails and no base policies exist")
	}
}

// TestPreparePoliciesFrom_ExportedWrapperDelegatesToInternalHelper guards
// that the exported PreparePoliciesFrom (the entry point for an org layer
// supplying its own policy archive source via the overridable
// PreparePolicies seam - see docs/SCHEMAS.md) actually calls through to
// preparePoliciesFrom's render/strip behavior, rather than the two ever
// silently drifting apart.
func TestPreparePoliciesFrom_ExportedWrapperDelegatesToInternalHelper(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kyverno-policies", "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately omit base/kustomization.yaml so `kustomize build` fails
	// and preparePoliciesFrom falls back to the base policies directory -
	// exercising the exact same code path TestPreparePoliciesFrom_
	// FallsBackToBasePoliciesWhenKustomizeBuildFails guards for the
	// unexported helper.
	policy := "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: base-policy\nspec:\n  rules: []\n"
	if err := os.WriteFile(filepath.Join(base, "policy.yaml"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}

	policyPath, err := PreparePoliciesFrom(dir)
	if err != nil {
		t.Fatalf("PreparePoliciesFrom: %v", err)
	}
	if policyPath != base {
		t.Errorf("policyPath = %q, want the base dir %q", policyPath, base)
	}
}

// TestPreparePolicies_IsOverridable guards the exported-override-var seam
// (see docs/SCHEMAS.md/docs/DEVELOPMENT.md): an org/consumer layer must be
// able to replace PreparePolicies wholesale with its own function - e.g. one
// backed by an OCI-pulled archive instead of the embedded/embedschemas-gated
// one - and have every caller (pipeline Setup, kyverno_wiring.go) pick it up
// automatically since they all call the var, never defaultPreparePolicies
// directly.
func TestPreparePolicies_IsOverridable(t *testing.T) {
	orig := PreparePolicies
	defer func() { PreparePolicies = orig }()

	called := false
	PreparePolicies = func() (string, func(), error) {
		called = true
		return "/custom/policy/path", func() {}, nil
	}

	path, cleanup, err := PreparePolicies()
	defer cleanup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected the overridden PreparePolicies to be invoked")
	}
	if path != "/custom/policy/path" {
		t.Errorf("path = %q, want the overridden path", path)
	}
}

func TestCollectPolicies_MissingDir(t *testing.T) {
	if _, err := collectPolicies(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestCollectPolicies_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.yaml")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := collectPolicies(f); err == nil {
		t.Error("expected an error when the path is not a directory")
	}
}

func TestCollectPolicies_FindsYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := collectPolicies(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 yaml files, got %d: %v", len(files), files)
	}
}

// writePolicyBase writes a minimal base/ ClusterPolicy + kustomization.yaml
// under dir, matching the on-disk shape PreparePolicies/buildPolicies
// expect (dir/base/..., with overlays/_ci created alongside it).
func writePolicyBase(t *testing.T, dir string) {
	t.Helper()
	base := filepath.Join(dir, "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "kustomization.yaml"), []byte("resources:\n  - policy.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: base-policy\nspec:\n  rules: []\n"
	if err := os.WriteFile(filepath.Join(base, "policy.yaml"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writePolicyComponent writes a minimal kustomize Component under
// dir/components/<name> containing one additional ClusterPolicy.
func writePolicyComponent(t *testing.T, dir, name string) {
	t.Helper()
	comp := filepath.Join(dir, "components", name)
	if err := os.MkdirAll(comp, 0o755); err != nil {
		t.Fatal(err)
	}
	kustomization := "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n  - policy.yaml\n"
	if err := os.WriteFile(filepath.Join(comp, "kustomization.yaml"), []byte(kustomization), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: extra-policy\nspec:\n  rules: []\n"
	if err := os.WriteFile(filepath.Join(comp, "policy.yaml"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
}
