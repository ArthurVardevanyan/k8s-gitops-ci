package core

import (
	"testing"
)

func TestDuplicateVolumeNames_Check_Duplicate(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: shared
    emptyDir: {}
  - name: shared
    emptyDir: {}
`)
	check := newDuplicateVolumeNamesCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate volume names, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/duplicate-volume-names" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDuplicateVolumeNames_Check_UniqueNames(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol1
    emptyDir: {}
  - name: vol2
    emptyDir: {}
`)
	check := newDuplicateVolumeNamesCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unique volume names, got %d", len(findings))
	}
}

func TestSecretVolume_Check_EmptySecretName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    secret:
      secretName: ""
`)
	check := newSecretVolumeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty secretName, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/secret-name-required" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestSecretVolume_Check_ValidSecretName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    secret:
      secretName: my-secret
`)
	check := newSecretVolumeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid secretName, got %d", len(findings))
	}
}

func TestConfigmapVolume_Check_EmptyName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    configMap:
      name: ""
`)
	check := newConfigmapVolumeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty configMap name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/configmap-name-required" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestConfigmapVolume_Check_ValidName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    configMap:
      name: my-config
`)
	check := newConfigmapVolumeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid configMap name, got %d", len(findings))
	}
}
