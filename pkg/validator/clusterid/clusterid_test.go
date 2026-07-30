package clusterid

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/cluster"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

func TestSelfClusterName(t *testing.T) {
	cases := map[string]string{
		"prod_us-east_mycluster": "mycluster",
		"mycluster":              "mycluster",
		"a_b_c":                  "c",
	}
	for in, want := range cases {
		if got := SelfClusterName(in); got != want {
			t.Errorf("SelfClusterName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetIdentity_ProjectNumbers(t *testing.T) {
	d := t.TempDir()
	content := "workloadIdentityPool: projects/123456789/locations/global/workloadIdentityPools/pool"
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte(content), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.ProjectNumbers) != 1 || id.ProjectNumbers[0] != "123456789" {
		t.Errorf("unexpected project numbers: %v", id.ProjectNumbers)
	}
}

func TestGetIdentity_ProjectIDs(t *testing.T) {
	d := t.TempDir()
	content := "serviceAccountEmail: myapp@my-project.iam.gserviceaccount.com"
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte(content), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.ProjectIDs) != 1 || id.ProjectIDs[0] != "my-project" {
		t.Errorf("unexpected project ids: %v", id.ProjectIDs)
	}
}

func TestGetIdentity_ClusterToken(t *testing.T) {
	old := ClusterTokenRe
	defer func() { ClusterTokenRe = old }()
	ClusterTokenRe = regexp.MustCompile(`^prod-east$`)

	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte("cluster: prod-east"), 0o644)
	id := GetIdentity(d, "prod-west")
	if len(id.ClusterNames) != 1 || id.ClusterNames[0] != "prod-east" {
		t.Errorf("unexpected cluster names: %v", id.ClusterNames)
	}
}

func TestRawFindings_ForeignProjectNumber(t *testing.T) {
	d := t.TempDir()
	content := "workloadIdentityPool: projects/999/locations/global/workloadIdentityPools/pool"
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte(content), 0o644)
	idx := ClusterIndex{
		NumberToCluster: map[string]string{"999": "other-cluster"},
	}
	findings := RawFindings(d, "my-cluster", idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Value != "999" {
		t.Errorf("unexpected value: %q", findings[0].Value)
	}
}

func TestRawFindings_SameCluster_NoFinding(t *testing.T) {
	d := t.TempDir()
	content := "workloadIdentityPool: projects/999/locations/global/workloadIdentityPools/pool"
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte(content), 0o644)
	idx := ClusterIndex{
		NumberToCluster: map[string]string{"999": "my-cluster"},
	}
	findings := RawFindings(d, "my-cluster", idx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for same cluster, got %d", len(findings))
	}
}

func TestRawFindings_EmptyDir(t *testing.T) {
	findings := RawFindings(t.TempDir(), "mycluster", ClusterIndex{})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty dir")
	}
}

func TestFormatIdentity(t *testing.T) {
	id := &OverlayIdentity{ClusterName: "c", ProjectIDs: []string{"p"}, Sources: map[string][]string{}}
	out := FormatIdentity(id)
	if out == "" {
		t.Error("expected non-empty identity format")
	}
}

func TestBuildClusterIndex(t *testing.T) {
	from := buildTestProjectIndex()
	idx := BuildClusterIndex(from, nil)
	if idx.IDToCluster["proj-a"] != "cluster-a" {
		t.Errorf("unexpected id mapping: %v", idx.IDToCluster)
	}
}

func TestAllowField_SkipsProjectNumber(t *testing.T) {
	old := AllowField
	defer func() { AllowField = old }()
	AllowField = func(field string) bool { return field == "projectNumber" }

	d := t.TempDir()
	content := "workloadIdentityPool: projects/123/locations/global/workloadIdentityPools/pool"
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte(content), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.ProjectNumbers) != 0 {
		t.Errorf("AllowField should have skipped projectNumber; got %v", id.ProjectNumbers)
	}
}

// ── infraID ───────────────────────────────────────────────────────────────

func TestGetIdentity_InfraID(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "install-config.yaml"), []byte("infraID: mycluster-ab12c"), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.InfraIDs) != 1 || id.InfraIDs[0] != "mycluster-ab12c" {
		t.Errorf("unexpected infraIDs: %v", id.InfraIDs)
	}
}

func TestGetIdentity_InfraID_Underscore(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "vars.tf"), []byte(`infra_id = "othercluster-xy99z"`), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.InfraIDs) != 1 || id.InfraIDs[0] != "othercluster-xy99z" {
		t.Errorf("unexpected infraIDs: %v", id.InfraIDs)
	}
}

func TestRawFindings_InfraIDMismatch(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "install-config.yaml"), []byte("infraID: othercluster-ab12c"), 0o644)
	findings := RawFindings(d, "mycluster", ClusterIndex{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].CheckID != exempt.IDClusterIdentity {
		t.Errorf("CheckID = %q, want %q (infraID mismatches must be non-exemptable)", findings[0].CheckID, exempt.IDClusterIdentity)
	}
	if findings[0].Value != "othercluster-ab12c" {
		t.Errorf("unexpected value: %q", findings[0].Value)
	}
}

func TestRawFindings_InfraIDMatchesClusterName_NoFinding(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "install-config.yaml"), []byte("infraID: mycluster"), 0o644)
	findings := RawFindings(d, "mycluster", ClusterIndex{})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when infraID matches the overlay folder name, got %d: %+v", len(findings), findings)
	}
}

func TestAllowField_SkipsInfraID(t *testing.T) {
	old := AllowField
	defer func() { AllowField = old }()
	AllowField = func(field string) bool { return field == "infraID" }

	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "install-config.yaml"), []byte("infraID: othercluster-ab12c"), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.InfraIDs) != 0 {
		t.Errorf("AllowField should have skipped infraID; got %v", id.InfraIDs)
	}
}

func TestClusterTokenRe_NilDoesNotScanNames(t *testing.T) {
	old := ClusterTokenRe
	defer func() { ClusterTokenRe = old }()
	ClusterTokenRe = nil

	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte("cluster: othername"), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.ClusterNames) != 0 {
		t.Errorf("nil ClusterTokenRe should produce no cluster names; got %v", id.ClusterNames)
	}
}

// helper
func buildTestProjectIndex() cluster.ProjectIndex {
	idx := cluster.ProjectIndex{
		IDToCluster:     map[string]string{"proj-a": "cluster-a"},
		NumberToCluster: map[string]string{"111": "cluster-a"},
	}
	return idx
}
