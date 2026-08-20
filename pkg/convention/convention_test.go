package convention

import (
	"testing"
)

func TestIsKnownNonManifestFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"Taskfile.yml", true},
		{"repo/Taskfile.yml", true},
		{".golangci.yml", true},
		{".golangci.yaml", true},
		{".goreleaser.yaml", true},
		{".goreleaser.yml", true},
		{".pre-commit-config.yaml", true},
		{"kubernetes/tekton/overlays/operator/deployment.yaml", false},
		{"Taskfile.yml.bak", false},
	}
	for _, c := range cases {
		if got := IsKnownNonManifestFile(c.path); got != c.want {
			t.Errorf("IsKnownNonManifestFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
