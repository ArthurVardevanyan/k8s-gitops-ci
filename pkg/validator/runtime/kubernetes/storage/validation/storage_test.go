package validation

import (
	"testing"
)

func TestPVAccessModesInvalidCheck(t *testing.T) {
	data := []byte(`kind: PersistentVolume
metadata:
  name: test
spec:
  accessModes:
  - ReadWritonce
  capacity:
    storage: 1Gi
  storageClassName: standard
  hostPath:
    path: /tmp`)
	check := newPvAccessModesInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "persistent-volume/access-modes-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PersistentVolume" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPVAccessModesInvalidValid(t *testing.T) {
	data := []byte(`kind: PersistentVolume
metadata:
  name: test
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  storageClassName: standard
  hostPath:
    path: /tmp`)
	check := newPvAccessModesInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPVCapacityInvalidCheck(t *testing.T) {
	data := []byte(`kind: PersistentVolume
metadata:
  name: test
spec:
  accessModes:
  - ReadWriteOnce
  storageClassName: standard
  hostPath:
    path: /tmp`)
	check := newPvCapacityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "persistent-volume/capacity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PersistentVolume" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPVCapacityInvalidValid(t *testing.T) {
	data := []byte(`kind: PersistentVolume
metadata:
  name: test
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 10Gi
  storageClassName: standard
  hostPath:
    path: /tmp`)
	check := newPvCapacityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPVCapacityInvalidGuardsOnKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	// The check guards on kind itself rather than relying solely on the
	// adapter to gate dispatch. Without that guard it decodes any
	// document into a PersistentVolume and reports its required fields as
	// missing - attributing the finding to a kind the document is not.
	if findings := newPvCapacityInvalidCheck().Run(data, "test.yaml"); len(findings) != 0 {
		t.Fatalf("expected no findings for a non-PersistentVolume document, got %d: %v", len(findings), findings)
	}
}

func TestPVCAccessModesInvalidCheck(t *testing.T) {
	data := []byte(`kind: PersistentVolumeClaim
metadata:
  name: test
spec:
  accessModes:
  - ReadWritonce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard`)
	check := newPvcAccessModesInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "persistent-volume-claim/access-modes-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PersistentVolumeClaim" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPVCAccessModesInvalidValid(t *testing.T) {
	data := []byte(`kind: PersistentVolumeClaim
metadata:
  name: test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard`)
	check := newPvcAccessModesInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPVCVolumeModeInvalidCheck(t *testing.T) {
	data := []byte(`kind: PersistentVolumeClaim
metadata:
  name: test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  volumeMode: InvalidMode`)
	check := newPvcVolumeModeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "persistent-volume-claim/volume-mode-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PersistentVolumeClaim" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPVCVolumeModeInvalidValid(t *testing.T) {
	data := []byte(`kind: PersistentVolumeClaim
metadata:
  name: test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  volumeMode: Filesystem`)
	check := newPvcVolumeModeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestSCProvisionerInvalidCheck(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
reclaimPolicy: Delete`)
	check := newScProvisionerInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "storage-class/provisioner-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "StorageClass" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestSCProvisionerInvalidValid(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
reclaimPolicy: Delete`)
	check := newScProvisionerInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestSCProvisionerInvalidGuardsOnKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	// The check guards on kind itself rather than relying solely on the
	// adapter to gate dispatch. Without that guard it decodes any
	// document into a StorageClass and reports its required fields as
	// missing - attributing the finding to a kind the document is not.
	if findings := newScProvisionerInvalidCheck().Run(data, "test.yaml"); len(findings) != 0 {
		t.Fatalf("expected no findings for a non-StorageClass document, got %d: %v", len(findings), findings)
	}
}

func TestSCReclaimPolicyInvalidCheck(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
reclaimPolicy: InvalidReclaim`)
	check := newScReclaimPolicyInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "storage-class/reclaim-policy-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "StorageClass" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestSCReclaimPolicyInvalidValid(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
reclaimPolicy: Delete`)
	check := newScReclaimPolicyInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestSCVolumeBindingModeInvalidCheck(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
volumeBindingMode: InvalidMode`)
	check := newScVolumeBindingModeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "storage-class/volume-binding-mode-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "StorageClass" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestSCVolumeBindingModeInvalidValid(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
volumeBindingMode: WaitForFirstConsumer`)
	check := newScVolumeBindingModeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestSCAllowedTopologyRangeInvalidCheck(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
allowedTopologies:
- matchLabelExpressions:
  - key: ""
    operator: In
    values:
    - us-east-1a`)
	check := newScAllowedTopologyRangeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "storage-class/allowed-topology-range-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "StorageClass" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestSCAllowedTopologyRangeInvalidValid(t *testing.T) {
	data := []byte(`kind: StorageClass
metadata:
  name: test
provisioner: valid.provisioner
allowedTopologies:
- matchLabelExpressions:
  - key: topology.kubernetes.io/zone
    operator: In
    values:
    - us-east-1a`)
	check := newScAllowedTopologyRangeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}
