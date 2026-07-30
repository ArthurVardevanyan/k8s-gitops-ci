package validator

import (
	"os"
	"path/filepath"
	"testing"

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
