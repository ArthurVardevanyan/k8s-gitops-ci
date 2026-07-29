package cluster

import "testing"

func TestBuildProjectIndex(t *testing.T) {
	idx := BuildProjectIndex([]ClusterProject{
		{Name: "c1", ProjectID: "p1", ProjectNumber: "n1"},
		{Name: "c2", ProjectID: "p2", ProjectNumber: "n2", NetworkProjectID: "shared"},
	})
	if idx.IDToCluster["p1"] != "c1" || idx.NumberToCluster["n2"] != "c2" {
		t.Errorf("unexpected lookup values: %+v", idx)
	}
	if !idx.SharedProjects["shared"] {
		t.Error("expected shared project")
	}
}
