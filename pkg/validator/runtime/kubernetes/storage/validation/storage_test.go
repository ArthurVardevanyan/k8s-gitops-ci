package validation

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// docCase is one complete manifest and the findings it must produce. These
// rules span three kinds with unrelated spec shapes, so a case carries the
// whole document rather than a fragment.
type docCase struct {
	name     string
	doc      string
	want     int
	contains string
}

func runDocCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []docCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := run([]byte(tc.doc), "test.yaml")
			if len(findings) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
			}
			if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
				t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
			}
		})
	}
}

func TestPvAccessModesInvalid(t *testing.T) {
	runDocCases(t, newPvAccessModesInvalidCheck().Run, []docCase{
		{name: "PVAccessModesInvalidCheck", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWritonce\n  capacity:\n    storage: 1Gi\n  storageClassName: standard\n  hostPath:\n    path: /tmp", want: 1},
		{name: "PVAccessModesInvalidValid", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 1Gi\n  storageClassName: standard\n  hostPath:\n    path: /tmp", want: 0},
	})
}

func TestPvCapacityInvalid(t *testing.T) {
	runDocCases(t, newPvCapacityInvalidCheck().Run, []docCase{
		{name: "PVCapacityInvalidCheck", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  storageClassName: standard\n  hostPath:\n    path: /tmp", want: 1},
		{name: "PVCapacityInvalidValid", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 10Gi\n  storageClassName: standard\n  hostPath:\n    path: /tmp", want: 0},
	})
}

func TestPvcAccessModesInvalid(t *testing.T) {
	runDocCases(t, newPvcAccessModesInvalidCheck().Run, []docCase{
		{name: "PVCAccessModesInvalidCheck", doc: "kind: PersistentVolumeClaim\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWritonce\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: standard", want: 1},
		{name: "PVCAccessModesInvalidValid", doc: "kind: PersistentVolumeClaim\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: standard", want: 0},
	})
}

func TestPvcVolumeModeInvalid(t *testing.T) {
	runDocCases(t, newPvcVolumeModeInvalidCheck().Run, []docCase{
		{name: "PVCVolumeModeInvalidCheck", doc: "kind: PersistentVolumeClaim\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  resources:\n    requests:\n      storage: 1Gi\n  volumeMode: InvalidMode", want: 1},
		{name: "PVCVolumeModeInvalidValid", doc: "kind: PersistentVolumeClaim\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  resources:\n    requests:\n      storage: 1Gi\n  volumeMode: Filesystem", want: 0},
	})
}

// TestPvVolumeModeInvalid mirrors TestPvcVolumeModeInvalid. Upstream applies
// the same supportedVolumeModes set to both kinds; the PersistentVolume half
// was missing, so an invalid mode on a PV went unreported.
func TestPvVolumeModeInvalid(t *testing.T) {
	runDocCases(t, newPvVolumeModeInvalidCheck().Run, []docCase{
		{name: "PVVolumeModeInvalid", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 1Gi\n  hostPath:\n    path: /tmp\n  volumeMode: InvalidMode", want: 1, contains: "volumeMode: Unsupported value"},
		{name: "PVVolumeModeFilesystem", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 1Gi\n  hostPath:\n    path: /tmp\n  volumeMode: Filesystem", want: 0},
		{name: "PVVolumeModeBlock", doc: "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 1Gi\n  hostPath:\n    path: /tmp\n  volumeMode: Block", want: 0},
		{
			// Defaulting is guarded on nil, so an absent volumeMode is filled
			// in with Filesystem and must not be reported.
			name: "PVVolumeModeAbsent",
			doc:  "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 1Gi\n  hostPath:\n    path: /tmp",
			want: 0,
		},
		{
			// An explicit "" survives defaulting as a non-nil pointer to the
			// empty string, so the API server does reject it.
			name:     "PVVolumeModeExplicitlyEmpty",
			doc:      "kind: PersistentVolume\nmetadata:\n  name: test\nspec:\n  accessModes:\n  - ReadWriteOnce\n  capacity:\n    storage: 1Gi\n  hostPath:\n    path: /tmp\n  volumeMode: \"\"",
			want:     1,
			contains: "volumeMode: Unsupported value",
		},
		{name: "PVVolumeModeWrongKindIsStillParsed", doc: "kind: PersistentVolumeClaim\nmetadata:\n  name: test\nspec:\n  volumeMode: Filesystem", want: 0},
	})
}

func TestScProvisionerInvalid(t *testing.T) {
	runDocCases(t, newScProvisionerInvalidCheck().Run, []docCase{
		{name: "SCProvisionerInvalidCheck", doc: "kind: StorageClass\nmetadata:\n  name: test\nreclaimPolicy: Delete", want: 1},
		{name: "SCProvisionerInvalidValid", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nreclaimPolicy: Delete", want: 0},
	})
}

func TestScReclaimPolicyInvalid(t *testing.T) {
	runDocCases(t, newScReclaimPolicyInvalidCheck().Run, []docCase{
		{name: "SCReclaimPolicyInvalidCheck", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nreclaimPolicy: InvalidReclaim", want: 1},
		{name: "SCReclaimPolicyInvalidValid", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nreclaimPolicy: Delete", want: 0},
		// Upstream validateReclaimPolicy wraps its NotSupported branch in a
		// len(...) > 0 guard, so the API server accepts an empty value even
		// though the pointer survives defaulting. Reporting it blocked a
		// manifest the cluster allows.
		{name: "SCReclaimPolicyExplicitEmptyIsAcceptedUpstream", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nreclaimPolicy: \"\"", want: 0},
	})
}

func TestScVolumeBindingModeInvalid(t *testing.T) {
	runDocCases(t, newScVolumeBindingModeInvalidCheck().Run, []docCase{
		// The sibling enum in the same file has no such guard, so an empty
		// volumeBindingMode is still a real rejection.
		{name: "SCVolumeBindingModeExplicitEmptyIsRejectedUpstream", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nvolumeBindingMode: \"\"", want: 1},
		{name: "SCVolumeBindingModeInvalidCheck", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nvolumeBindingMode: InvalidMode", want: 1},
		{name: "SCVolumeBindingModeInvalidValid", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nvolumeBindingMode: WaitForFirstConsumer", want: 0},
	})
}

func TestScAllowedTopologyRangeInvalid(t *testing.T) {
	runDocCases(t, newScAllowedTopologyRangeInvalidCheck().Run, []docCase{
		{name: "SCAllowedTopologyRangeInvalidCheck", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nallowedTopologies:\n- matchLabelExpressions:\n  - key: \"\"\n    operator: In\n    values:\n    - us-east-1a", want: 1},
		{name: "SCAllowedTopologyRangeInvalidValid", doc: "kind: StorageClass\nmetadata:\n  name: test\nprovisioner: valid.provisioner\nallowedTopologies:\n- matchLabelExpressions:\n  - key: topology.kubernetes.io/zone\n    operator: In\n    values:\n    - us-east-1a", want: 0},
	})
}
