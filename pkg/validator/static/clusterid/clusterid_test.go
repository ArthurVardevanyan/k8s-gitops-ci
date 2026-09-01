package clusterid

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// ── JSON validation ─────────────────────────────────────────────────────────

func TestGetIdentity_InvalidJSON(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "credentials.json"), []byte(`{"not": valid json`), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.InvalidJSONFiles) != 1 {
		t.Fatalf("expected 1 invalid JSON file, got %d: %+v", len(id.InvalidJSONFiles), id.InvalidJSONFiles)
	}
	if id.InvalidJSONFiles[0].File != "credentials.json" {
		t.Errorf("unexpected file: %q", id.InvalidJSONFiles[0].File)
	}
}

func TestGetIdentity_ValidJSON_NoFinding(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "credentials.json"), []byte(`{"audience": "example"}`), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.InvalidJSONFiles) != 0 {
		t.Errorf("expected no invalid JSON files, got %+v", id.InvalidJSONFiles)
	}
}

func TestGetIdentity_NonJSONFile_NotValidated(t *testing.T) {
	// A .yaml (or any non-.json) file with JSON-invalid-looking content
	// must not be checked as JSON at all.
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "config.yaml"), []byte("this: is not json {{{"), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.InvalidJSONFiles) != 0 {
		t.Errorf("expected non-.json files to be skipped, got %+v", id.InvalidJSONFiles)
	}
}

func TestRawFindings_InvalidJSON(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "credentials.json"), []byte(`{"not": valid`), 0o644)
	findings := RawFindings(d, "mycluster", ClusterIndex{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].CheckID != exempt.IDClusterIdentity {
		t.Errorf("CheckID = %q, want %q (invalid JSON must be non-exemptable)", findings[0].CheckID, exempt.IDClusterIdentity)
	}
	if !strings.Contains(findings[0].Message, "invalid JSON") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

// ── real foreign-cluster-name detection (via ClusterIndex.KnownClusters) ────

func TestRawFindings_ForeignClusterName_KnownClusters_NoPatternMatch(t *testing.T) {
	// "othercluster" does not match any ClusterTokenRe shape (nil here), but
	// IS a known cluster per the live index — RawFindings must still catch
	// it. This is the concrete "real detection" gap: previously only
	// ClusterTokenRe-shaped tokens were ever checked at all.
	old := ClusterTokenRe
	defer func() { ClusterTokenRe = old }()
	ClusterTokenRe = nil

	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte("cluster: othercluster"), 0o644)
	idx := ClusterIndex{KnownClusters: map[string]bool{"othercluster": true}}
	findings := RawFindings(d, "mycluster", idx)

	var found bool
	for _, f := range findings {
		if f.CheckID == exempt.IDClusterName && f.Value == "othercluster" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a cluster-name finding for known foreign cluster %q, got %+v", "othercluster", findings)
	}
}

func TestRawFindings_ForeignClusterName_SelfNameNotFlagged(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte("cluster: mycluster"), 0o644)
	idx := ClusterIndex{KnownClusters: map[string]bool{"mycluster": true, "othercluster": true}}
	findings := RawFindings(d, "mycluster", idx)
	for _, f := range findings {
		if f.CheckID == exempt.IDClusterName {
			t.Errorf("self cluster name must never be flagged as foreign, got %+v", f)
		}
	}
}

func TestGetIdentity_KnownClustersNotConsulted(t *testing.T) {
	// The public, index-less GetIdentity entry point is documented as
	// pattern-only; confirm it does NOT catch a known-but-unpatterned
	// foreign name (only RawFindings, which threads the index through
	// getIdentity, does).
	old := ClusterTokenRe
	defer func() { ClusterTokenRe = old }()
	ClusterTokenRe = nil

	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "patch.yaml"), []byte("cluster: othercluster"), 0o644)
	id := GetIdentity(d, "mycluster")
	if len(id.ClusterNames) != 0 {
		t.Errorf("GetIdentity (index-less) should not detect foreign names by KnownClusters; got %v", id.ClusterNames)
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

// The value in this message is the cluster name, which selfClusterName
// derives by stripping everything up to the last "_". Calling it the overlay
// folder name is wrong for any directory that carries a prefix, and wrong in
// the way that costs a reader the most: it names a directory that does not
// exist, so searching for it finds nothing.
func TestInfraIDMismatchMessageNamesTheClusterNotTheFolder(t *testing.T) {
	const folder = "prod_us-east_cluster01"
	id := &OverlayIdentity{ClusterName: selfClusterName(folder), InfraIDs: []string{"othercluster"}}

	findings := rawInfraIDFindings("overlays/"+folder, id)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	msg := findings[0].Message

	// The derived name is correct to report - it is what was compared.
	if !strings.Contains(msg, `"cluster01"`) {
		t.Errorf("message does not report the compared value:\n%s", msg)
	}
	// But it must not be introduced as the folder name, since it is not one.
	if strings.Contains(msg, "folder name") {
		t.Errorf("message calls %q the overlay folder name, but the folder is %q:\n%s", "cluster01", folder, msg)
	}
}
