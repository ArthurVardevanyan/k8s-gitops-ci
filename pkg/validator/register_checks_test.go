package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/cluster"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/clusterid"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// withClusterIndex temporarily overrides ClusterIndexProvider for the
// duration of a test, restoring the original afterward.
func withClusterIndex(t *testing.T, idx clusterid.ClusterIndex) {
	t.Helper()
	orig := ClusterIndexProvider
	ClusterIndexProvider = func() clusterid.ClusterIndex { return idx }
	t.Cleanup(func() { ClusterIndexProvider = orig })
}

func TestClusterIdentityAdapter_UsesFindingsOwnCheckID(t *testing.T) {
	// Regression: CheckOverlay used to hardcode CheckID: exempt.IDClusterIdentity
	// for every finding, discarding clusterid.RawFindings' own, more
	// specific (and exemptable) CheckID.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("foo: projects/123456/locations/us-central1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withClusterIndex(t, clusterid.ClusterIndex{
		NumberToCluster: map[string]string{"123456": "other-cluster"},
	})

	findings := (clusterIdentityAdapter{}).CheckOverlay(dir, "my-cluster")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].CheckID != exempt.IDProjectRef {
		t.Errorf("CheckID = %q, want %q - must not hardcode IDClusterIdentity for an exemptable finding type",
			findings[0].CheckID, exempt.IDProjectRef)
	}
}

func TestClusterIdentityAdapter_ProjectRefFindingIsExemptableEndToEnd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("foo: projects/123456/locations/us-central1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withClusterIndex(t, clusterid.ClusterIndex{
		NumberToCluster: map[string]string{"123456": "other-cluster"},
	})

	selectors := []exempt.Selector{{Check: exempt.IDProjectRef, Value: "123456"}}
	res := runOverlayChecks([]string{dir}, "my-cluster", selectors, 1, nil)
	for _, f := range res.Findings {
		if f.CheckID == exempt.IDProjectRef {
			t.Errorf("expected the project-ref finding to be excluded by the EXEMPTIONS selector, got %+v", f)
		}
	}
	found := false
	for _, ex := range res.Exempted {
		if ex.CheckID == exempt.IDProjectRef {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an audit-trail entry for the exempted project-ref finding, got %+v", res.Exempted)
	}
}

// TestPlaceholderCheck_ScansAVPPlaceholders is a regression test: the
// placeholder check used to pass placeholder.Options{} (CheckAVP defaults
// false), so argocd-vault-plugin-scheme placeholders were never actually
// scanned despite the check's own table description advertising it.
func TestPlaceholderCheck_ScansAVPPlaceholders(t *testing.T) {
	data := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: x\nstringData:\n  password: <path:secret/data/foo#password>\n")
	findings := (placeholderCheck{}).CheckDoc(data, "secret.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 AVP placeholder finding, got %d: %+v", len(findings), findings)
	}
}

func TestPlaceholderCheck_SkipsCustomResourceDefinitions(t *testing.T) {
	pc := placeholderCheck{}
	if !pc.SkipDoc("CustomResourceDefinition") {
		t.Error("expected CustomResourceDefinition documents to be skipped")
	}
	if pc.SkipDoc("Secret") {
		t.Error("expected non-CRD documents not to be skipped")
	}
}

// TestClusterIdentityAdapter_DisabledWithoutProvider is a regression test:
// CheckOverlay used to call clusterid.RawFindings with a zero-value
// ClusterIndex whenever ClusterIndexProvider was nil, and RawFindings'
// infraID-mismatch/invalid-JSON structural findings don't depend on the
// index's contents at all - so a generic run with no cluster-identity
// provider configured could still produce findings on an overlay
// referencing a mismatched infraID or containing malformed JSON. The check
// must instead be fully disabled (return nil, not "findings against an
// empty index") in that case.
func TestClusterIdentityAdapter_DisabledWithoutProvider(t *testing.T) {
	orig := ClusterIndexProvider
	ClusterIndexProvider = nil
	t.Cleanup(func() { ClusterIndexProvider = orig })

	dir := t.TempDir()
	// infraID that clearly doesn't match the overlay folder name, and an
	// invalid JSON file - both of which RawFindings would flag
	// unconditionally if it were still invoked against a zero-value index.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("infraID: totally-different-cluster-abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings := (clusterIdentityAdapter{}).CheckOverlay(dir, "my-cluster")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no ClusterIndexProvider is configured, got %+v", findings)
	}
}

func TestConfigureClusterIdentityFromProviders_DisablesWhenNotConfigured(t *testing.T) {
	orig := ClusterIndexProvider
	ClusterIndexProvider = func() clusterid.ClusterIndex {
		return clusterid.ClusterIndex{IDToCluster: map[string]string{"leftover": "state"}}
	}
	t.Cleanup(func() { ClusterIndexProvider = orig })

	configureClusterIdentityFromProviders(Options{})
	if ClusterIndexProvider != nil {
		t.Error("expected ClusterIndexProvider to be reset to nil for an unconfigured provider.Providers")
	}
}

func TestConfigureClusterIdentityFromProviders_WiresRealProvider(t *testing.T) {
	orig := ClusterIndexProvider
	t.Cleanup(func() { ClusterIndexProvider = orig })

	opts := Options{Providers: provider.Providers{ClusterMetadata: testClusterMetadata{}}}
	configureClusterIdentityFromProviders(opts)
	if ClusterIndexProvider == nil {
		t.Fatal("expected ClusterIndexProvider to be wired from a configured ClusterMetadata provider")
	}
	idx := ClusterIndexProvider()
	if idx.IDToCluster["proj-a"] != "cluster-a" {
		t.Errorf("expected the bridged index to carry the provider's IDToCluster entries, got %+v", idx)
	}
	if !idx.KnownClusters["cluster-a"] {
		t.Errorf("expected the bridged index to carry the provider's known-clusters set, got %+v", idx)
	}
}

type testClusterMetadata struct{}

func (testClusterMetadata) ProjectIdentity() (idx cluster.ProjectIndex, knownClusters map[string]bool, ok bool, err error) {
	return cluster.ProjectIndex{IDToCluster: map[string]string{"proj-a": "cluster-a"}},
		map[string]bool{"cluster-a": true}, true, nil
}

func (testClusterMetadata) ChangeGroups() (map[string]int, bool) { return nil, false }

func TestPsaCheck_CarriesMissingLabelsInExtra(t *testing.T) {
	data := []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: foo\n")
	findings := (psaCheck{}).CheckDoc(data, "ns.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 psa finding, got %d: %+v", len(findings), findings)
	}
	extra := findings[0].Get(psaMissingLabelsExtraKey)
	if extra == "" {
		t.Fatal("expected missing_labels to be populated in Extra")
	}
	if !strings.Contains(extra, "pod-security.kubernetes.io/enforce") {
		t.Errorf("expected the enforce label in Extra, got: %s", extra)
	}
}
