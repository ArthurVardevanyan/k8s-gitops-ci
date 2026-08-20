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

// TestPlaceholderCheck_DoesNotFlagAVPReferences guards that the placeholder
// check does NOT flag argocd-vault-plugin-scheme references (<path:...>,
// <vault:...>, etc.). The check runs over RAW changed source, where AVP
// references are the intended committed state (resolved by AVP at deploy
// time / by the overlay build), not unresolved template placeholders.
// Flagging them made every AVP-managed secret a blocking false positive.
func TestPlaceholderCheck_DoesNotFlagAVPReferences(t *testing.T) {
	data := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: x\nstringData:\n  password: <path:secret/data/foo#password>\n")
	findings := (placeholderCheck{}).CheckDoc(data, "secret.yaml")
	if len(findings) != 0 {
		t.Fatalf("expected AVP references to NOT be flagged, got %d: %+v", len(findings), findings)
	}
}

// TestPlaceholderCheck_FlagsGenuinePlaceholders guards that real unresolved
// template placeholders (sentinels, angle-bracket tokens) ARE still flagged.
func TestPlaceholderCheck_FlagsGenuinePlaceholders(t *testing.T) {
	data := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\ndata:\n  a: CHANGEME\n  b: <NAMESPACE>\n")
	findings := (placeholderCheck{}).CheckDoc(data, "cm.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 genuine placeholder findings (CHANGEME, <NAMESPACE>), got %d: %+v", len(findings), findings)
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

// The following three tests are regressions for a gap where rbac-wildcards,
// named-ports, and podspec-defaults never populated check.Finding.Value/
// Annotations, so a gitops-ci.k8s.io/exempt-<check-id> annotation on the
// resource itself silently never matched anything (exempt.Accepts fails
// closed whenever value == "") - only an EXEMPTIONS=(...) test.sh selector
// could exempt these three checks. See docs/EXEMPTIONS.md's "Adding
// exemption support to a new check".

func TestRbacWildcardCheck_AnnotationExemptionEndToEnd(t *testing.T) {
	dir := t.TempDir()
	data := `kind: ClusterRole
metadata:
  name: admin
  annotations:
    gitops-ci.k8s.io/exempt-rbac-wildcards: "resources"
rules:
  - apiGroups: [""]
    resources: ["*"]
    verbs: ["get"]
`
	f := filepath.Join(dir, "clusterrole.yaml")
	if err := os.WriteFile(f, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	for _, finding := range res.Findings {
		if finding.CheckID == "rbac-wildcards" {
			t.Errorf("expected the rbac-wildcards finding to be excluded by its own annotation, got %+v", finding)
		}
	}
	found := false
	for _, ex := range res.Exempted {
		if ex.CheckID == "rbac-wildcards" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an audit-trail entry for the exempted rbac-wildcards finding, got %+v", res.Exempted)
	}
}

func TestRbacWildcardCheck_CommaSeparatedAnnotationExemptionEndToEnd(t *testing.T) {
	// A single annotation with comma-separated values ("apiGroups,resources,verbs")
	// must exempt all three wildcard fields in a single rule.
	dir := t.TempDir()
	data := `kind: ClusterRole
metadata:
  name: admin
  annotations:
    gitops-ci.k8s.io/exempt-rbac-wildcards: "apiGroups,resources,verbs"
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
`
	f := filepath.Join(dir, "clusterrole.yaml")
	if err := os.WriteFile(f, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	for _, finding := range res.Findings {
		if finding.CheckID == "rbac-wildcards" {
			t.Errorf("expected the rbac-wildcards finding to be excluded by comma-separated annotation, got %+v", finding)
		}
	}
	// Two findings (apiGroups wildcard + resources wildcard) should both be exempted.
	if len(res.Exempted) != 3 {
		t.Errorf("expected 3 exempted entries for comma-separated annotation, got %d: %+v", len(res.Exempted), res.Exempted)
	}
}

func TestRbacWildcardCheck_CommaSeparatedPartialExemption(t *testing.T) {
	// Comma-separated annotation exempts only the listed fields; other wildcards
	// should still appear as findings.
	dir := t.TempDir()
	data := `kind: ClusterRole
metadata:
  name: admin
  annotations:
    gitops-ci.k8s.io/exempt-rbac-wildcards: "resources"
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
`
	f := filepath.Join(dir, "clusterrole.yaml")
	if err := os.WriteFile(f, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	// Should still have 2 findings (apiGroups + verbs), resources is exempted.
	var wildcardFindings int
	for _, finding := range res.Findings {
		if finding.CheckID == "rbac-wildcards" {
			wildcardFindings++
		}
	}
	if wildcardFindings != 2 {
		t.Errorf("expected 2 non-exempted rbac-wildcards findings (apiGroups + verbs), got %d", wildcardFindings)
	}
	if len(res.Exempted) != 1 {
		t.Errorf("expected 1 exempted entry (resources only), got %d: %+v", len(res.Exempted), res.Exempted)
	}
}

func TestNamedportCheck_AnnotationExemptionEndToEnd(t *testing.T) {
	dir := t.TempDir()
	data := `kind: Service
metadata:
  name: svc
  annotations:
    gitops-ci.k8s.io/exempt-named-ports: "8080"
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8080
`
	f := filepath.Join(dir, "service.yaml")
	if err := os.WriteFile(f, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	for _, finding := range res.Findings {
		if finding.CheckID == "named-ports" {
			t.Errorf("expected the named-ports finding to be excluded by its own annotation, got %+v", finding)
		}
	}
	found := false
	for _, ex := range res.Exempted {
		if ex.CheckID == "named-ports" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an audit-trail entry for the exempted named-ports finding, got %+v", res.Exempted)
	}
}

func TestPodspecCheck_AnnotationExemptionEndToEnd(t *testing.T) {
	dir := t.TempDir()
	data := `kind: Deployment
metadata:
  name: bad
  annotations:
    gitops-ci.k8s.io/exempt-podspec-defaults: "enableServiceLinks, restartPolicy, schedulerName, dnsPolicy, automountServiceAccountToken"
spec:
  template:
    spec:
      containers:
        - name: c
          image: x
`
	f := filepath.Join(dir, "deployment.yaml")
	if err := os.WriteFile(f, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	for _, finding := range res.Findings {
		if finding.CheckID == "podspec-defaults" && finding.Container == "" {
			t.Errorf("expected the pod-level podspec-defaults finding to be excluded by its own annotation, got %+v", finding)
		}
	}
	found := false
	for _, ex := range res.Exempted {
		if ex.CheckID == "podspec-defaults" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an audit-trail entry for the exempted podspec-defaults finding, got %+v", res.Exempted)
	}
}
