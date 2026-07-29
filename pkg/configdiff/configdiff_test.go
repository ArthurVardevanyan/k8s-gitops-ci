package configdiff

import (
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

func TestDetectAffectedApps_NoConfigFiles(t *testing.T) {
	got := DetectAffectedApps([]string{"app/foo.yaml"}, "", "", nil)
	if len(got) != 0 {
		t.Errorf("expected none: %v", got)
	}
}

func TestEnvironmentPrefixes_DefaultsEmpty(t *testing.T) {
	if len(EnvironmentPrefixes) != 0 {
		t.Error("expected empty map default")
	}
}

func TestDetectTemplateChanges(t *testing.T) {
	old := convention.ScaffoldDir
	convention.ScaffoldDir = ".scafctl"
	defer func() { convention.ScaffoldDir = old }()
	got := DetectTemplateChanges([]string{".scafctl/templates/app/base.yaml"})
	if len(got) != 1 || got[0] != "app" {
		t.Errorf("unexpected template apps: %v", got)
	}
}

func TestDedup(t *testing.T) {
	got := dedup([]string{"a", "b", "a", "c"})
	if len(got) != 3 {
		t.Errorf("unexpected dedup: %v", got)
	}
}
