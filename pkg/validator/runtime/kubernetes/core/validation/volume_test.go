package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- volume-name tests ---

func TestVolumeName_Check_ValidName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: my-volume
    emptyDir: {}
`)
	check := volumeNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid volume name, got %d: %v", len(findings), findings)
	}
}

func TestVolumeName_Check_UppercaseName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: MyVolume
    emptyDir: {}
`)
	check := volumeNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for uppercase volume name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/volume-name" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestVolumeName_Check_EmptyName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: ""
    emptyDir: {}
`)
	check := volumeNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty volume name (skipped), got %d", len(findings))
	}
}

func TestVolumeName_Check_InvalidChars(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: my_volume!
    emptyDir: {}
`)
	check := volumeNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid volume name, got %d", len(findings))
	}
}

// --- duplicate-volume-names tests ---

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
	check := duplicateVolumeNamesCheck{}
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
	check := duplicateVolumeNamesCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unique volume names, got %d", len(findings))
	}
}

// --- volume-name-unique tests ---

func TestVolumeNameUnique_Check_Duplicate(t *testing.T) {
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
	check := volumeNameUniqueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate volume name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/volume-name-unique" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "shared" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestVolumeNameUnique_Check_UniqueNames(t *testing.T) {
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
	check := volumeNameUniqueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unique volume names, got %d", len(findings))
	}
}

// --- emptydir-size-limit tests ---

func TestEmptydirSizeLimit_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    emptyDir:
      sizeLimit: 1Gi
`)
	check := emptydirSizeLimitCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid sizeLimit, got %d", len(findings))
	}
}

func TestEmptydirSizeLimit_Check_NoSizeLimit(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    emptyDir: {}
`)
	check := emptydirSizeLimitCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no sizeLimit, got %d", len(findings))
	}
}

// --- persistentvolumeclaim-not-found tests ---

func TestPvcVolume_Check_EmptyClaimName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    persistentVolumeClaim:
      claimName: ""
`)
	check := pvcVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty claimName, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/persistentvolumeclaim-not-found" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPvcVolume_Check_ValidClaimName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    persistentVolumeClaim:
      claimName: my-claim
`)
	check := pvcVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid claimName, got %d", len(findings))
	}
}

// --- secret-not-found tests ---

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
	check := secretVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty secretName, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/secret-not-found" {
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
	check := secretVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid secretName, got %d", len(findings))
	}
}

// --- configmap-not-found tests ---

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
	check := configmapVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty configMap name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/configmap-not-found" {
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
	check := configmapVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid configMap name, got %d", len(findings))
	}
}

// --- downward-api-not-found tests ---

func TestDownwardAPIVolume_Check_EmptyItems(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    downwardAPI:
      items: []
`)
	check := downwardAPIVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty downwardAPI items, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/downward-api-not-found" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDownwardAPIVolume_Check_ValidItems(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    downwardAPI:
      items:
      - path: "labels"
        fieldRef:
          fieldPath: metadata.labels
`)
	check := downwardAPIVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid downwardAPI items, got %d", len(findings))
	}
}

// --- projected-not-found tests ---

func TestProjectedVolume_Check_EmptySources(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    projected:
      sources: []
`)
	check := projectedVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty projected sources, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/projected-not-found" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestProjectedVolume_Check_ValidSources(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    projected:
      sources:
      - secret:
          name: my-secret
`)
	check := projectedVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid projected sources, got %d", len(findings))
	}
}

// --- volume-type-undefined tests ---

func TestVolumeTypeUndefined_Check_NoSource(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: emptyvol
`)
	check := volumeTypeUndefinedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for volume with no type, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/volume-type-undefined" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestVolumeTypeUndefined_Check_ValidSource(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    emptyDir: {}
`)
	check := volumeTypeUndefinedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for volume with valid source, got %d", len(findings))
	}
}

// --- hostpath-not-found tests ---

func TestHostPathVolume_Check_EmptyPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    hostPath:
      path: ""
`)
	check := hostPathVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty hostPath path, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/hostpath-not-found" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestHostPathVolume_Check_ValidPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    hostPath:
      path: /tmp/data
`)
	check := hostPathVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid hostPath path, got %d", len(findings))
	}
}

// --- nfs-invalid tests ---

func TestNfsVolume_Check_EmptyServerAndPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    nfs:
      server: ""
      path: ""
`)
	check := nfsVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty nfs server and path, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/nfs-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestNfsVolume_Check_EmptyServer(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    nfs:
      server: ""
      path: /data
`)
	check := nfsVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty nfs server, got %d: %v", len(findings), findings)
	}
}

func TestNfsVolume_Check_EmptyPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    nfs:
      server: nfs-server
      path: ""
`)
	check := nfsVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty nfs path, got %d: %v", len(findings), findings)
	}
}

func TestNfsVolume_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    nfs:
      server: nfs-server
      path: /data
`)
	check := nfsVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid NFS, got %d", len(findings))
	}
}

// --- csi-invalid tests ---

func TestCsiVolume_Check_EmptyDriver(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    csi:
      driver: ""
`)
	check := csiVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty CSI driver, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/csi-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCsiVolume_Check_ValidDriver(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    csi:
      driver: ebs.csi.aws.com
`)
	check := csiVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid CSI driver, got %d", len(findings))
	}
}

// --- cinder-invalid tests ---

func TestCinderVolume_Check_EmptyVolumeID(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    cinder:
      volumeID: ""
`)
	check := cinderVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty cinder volumeID, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/cinder-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCinderVolume_Check_ValidVolumeID(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    cinder:
      volumeID: my-cinder-vol
`)
	check := cinderVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid cinder volumeID, got %d", len(findings))
	}
}

// --- gce-pd-invalid tests ---

func TestGcePDVolume_Check_EmptyPDName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    gcePersistentDisk:
      pdName: ""
`)
	check := gcePDVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty GCE PD name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/gce-pd-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestGcePDVolume_Check_ValidPDName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    gcePersistentDisk:
      pdName: my-gce-pd
`)
	check := gcePDVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid GCE PD name, got %d", len(findings))
	}
}

// --- azure-disk-invalid tests ---

func TestAzureDiskVolume_Check_EmptyDiskName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    azureDisk:
      diskName: ""
`)
	check := azureDiskVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty Azure Disk name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/azure-disk-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestAzureDiskVolume_Check_ValidDiskName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    azureDisk:
      diskName: my-azure-disk
`)
	check := azureDiskVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid Azure Disk name, got %d", len(findings))
	}
}

// --- azure-file-invalid tests ---

func TestAzureFileVolume_Check_EmptyShareName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    azureFile:
      shareName: ""
`)
	check := azureFileVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty Azure File share name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/azure-file-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestAzureFileVolume_Check_ValidShareName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    azureFile:
      shareName: my-share
`)
	check := azureFileVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid Azure File share name, got %d", len(findings))
	}
}

// --- glusterfs-invalid tests ---

func TestGlusterfsVolume_Check_EmptyEndpoints(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    glusterfs:
      endpoints: ""
`)
	check := glusterfsVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty glusterfs endpoints, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/glusterfs-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestGlusterfsVolume_Check_ValidEndpoints(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    glusterfs:
      endpoints: glusterfs-endpoints
      path: vol
`)
	check := glusterfsVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid glusterfs endpoints, got %d", len(findings))
	}
}

// --- iscsi-invalid tests ---

func TestIscsiVolume_Check_EmptyTargetPortalAndIQN(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    iscsi:
      targetPortal: ""
      iqn: ""
`)
	check := iscsiVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty iSCSI targetPortal and IQN, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/iscsi-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestIscsiVolume_Check_EmptyTargetPortal(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    iscsi:
      targetPortal: ""
      iqn: my-iqn
`)
	check := iscsiVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty iSCSI targetPortal, got %d: %v", len(findings), findings)
	}
}

func TestIscsiVolume_Check_EmptyIQN(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    iscsi:
      targetPortal: 10.0.0.1
      iqn: ""
`)
	check := iscsiVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty iSCSI IQN, got %d: %v", len(findings), findings)
	}
}

func TestIscsiVolume_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    iscsi:
      targetPortal: 10.0.0.1
      iqn: my-iqn
`)
	check := iscsiVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid iSCSI, got %d", len(findings))
	}
}

// --- rbd-invalid tests ---

func TestRbdVolume_Check_EmptyMonitors(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    rbd:
      monitors: []
`)
	check := rbdVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty RBD monitors, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "volume/rbd-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestRbdVolume_Check_ValidMonitors(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol
    rbd:
      monitors:
      - 10.0.0.1:6789
`)
	check := rbdVolumeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid RBD monitors, got %d", len(findings))
	}
}

// --- ValidateVolumeContext integration tests ---

func TestValidateVolumeContext_MultipleViolations(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: invalid!vol
    emptyDir: {}
  - name: dup
    emptyDir: {}
  - name: dup
    emptyDir: {}
  - name: emptyvol
  - name: hpt
    hostPath:
      path: ""
`)
	findings := ValidateVolumeContext(data, "test.yaml")
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["volume/volume-name"] {
		t.Error("missing volume-name finding")
	}
	if !ruleIDs["volume/duplicate-volume-names"] {
		t.Error("missing duplicate-volume-names finding")
	}
	if !ruleIDs["volume/volume-type-undefined"] {
		t.Error("missing volume-type-undefined finding")
	}
	if !ruleIDs["volume/hostpath-not-found"] {
		t.Error("missing hostpath-not-found finding")
	}
}

func TestValidateVolumeContext_Clean(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  volumes:
  - name: vol1
    emptyDir: {}
  - name: vol2
    secret:
      secretName: my-secret
  - name: vol3
    configMap:
      name: my-config
`)
	findings := ValidateVolumeContext(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean pod, got %d: %v", len(findings), findings)
	}
}

func TestValidateVolumeContext_NonWorkload(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	findings := ValidateVolumeContext(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateVolumeContext_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := ValidateVolumeContext(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

// --- Check interface implementation verification ---

func TestVolumeChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		volumeNameCheck{},
		duplicateVolumeNamesCheck{},
		volumeNameUniqueCheck{},
		emptydirSizeLimitCheck{},
		pvcVolumeCheck{},
		secretVolumeCheck{},
		configmapVolumeCheck{},
		downwardAPIVolumeCheck{},
		projectedVolumeCheck{},
		volumeTypeUndefinedCheck{},
		hostPathVolumeCheck{},
		nfsVolumeCheck{},
		csiVolumeCheck{},
		cinderVolumeCheck{},
		gcePDVolumeCheck{},
		azureDiskVolumeCheck{},
		azureFileVolumeCheck{},
		glusterfsVolumeCheck{},
		iscsiVolumeCheck{},
		rbdVolumeCheck{},
	}

	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if c.Category() == "" {
			t.Errorf("check %T has empty Category", c)
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}

// Note: TestRegister in security_context_test.go verifies that Register() can
// be called without panicking (including all new volume checks).
