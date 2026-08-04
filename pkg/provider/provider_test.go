package provider

import (
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/cluster"
)

func TestNilDefaults(t *testing.T) {
	var p Providers
	if p.ReportMarker() != defaultReportMarker {
		t.Errorf("ReportMarker = %q", p.ReportMarker())
	}
	if p.ReportTitle() != defaultReportTitle {
		t.Errorf("ReportTitle = %q", p.ReportTitle())
	}
	if p.PipelineHeader() != defaultPipelineHeader {
		t.Errorf("PipelineHeader = %q", p.PipelineHeader())
	}
	if len(p.ForeignMarkers()) != 0 {
		t.Error("expected no foreign markers")
	}
	idx, _, enabled, err := p.ProjectIdentity()
	if enabled || err != nil || idx.IDToCluster != nil {
		t.Errorf("expected disabled project identity: enabled=%v err=%v", enabled, err)
	}
}

func TestOverrides(t *testing.T) {
	p := Providers{
		Branding: testBranding{},
	}
	if p.ReportMarker() != "MARK" || p.ReportTitle() != "TITLE" || p.PipelineHeader() != "HEADER" || p.BinaryName() != "BIN" {
		t.Errorf("override branding failed: %+v", p)
	}
}

// TestBinaryNameDefault verifies the generic fallback when Branding is nil.
func TestBinaryNameDefault(t *testing.T) {
	if got := (Providers{}).BinaryName(); got != defaultBinaryName {
		t.Errorf("BinaryName() = %q, want %q", got, defaultBinaryName)
	}
}

type testBranding struct{}

func (testBranding) ReportMarker() string   { return "MARK" }
func (testBranding) ReportTitle() string    { return "TITLE" }
func (testBranding) PipelineHeader() string { return "HEADER" }
func (testBranding) BinaryName() string     { return "BIN" }

type testClusterMetadata struct{}

func (testClusterMetadata) ProjectIdentity() (idx cluster.ProjectIndex, idToCluster map[string]bool, ok bool, err error) {
	return cluster.ProjectIndex{IDToCluster: map[string]string{"x": "y"}}, nil, true, nil
}

func (testClusterMetadata) ChangeGroups() (map[string]int, bool) { return map[string]int{"a": 1}, true }

func TestClusterMetadata(t *testing.T) {
	p := Providers{ClusterMetadata: testClusterMetadata{}}
	idx, _, enabled, err := p.ProjectIdentity()
	if !enabled || err != nil || idx.IDToCluster["x"] != "y" {
		t.Error("ClusterMetadata ProjectIdentity failed")
	}
	groups, ok := p.ChangeGroups()
	if !ok || groups["a"] != 1 {
		t.Error("ClusterMetadata ChangeGroups failed")
	}
}
