package validator

import (
	"os"
	"path/filepath"
	"strings"
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
