package kubernetes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// TestExplicitlyEmptyDefaultedFields pins how the family treats a field that
// is present in the manifest but set to its empty value.
//
// This is the failure mode that has produced every false positive found in
// this family so far: a rule ported faithfully from upstream validation, then
// evaluated against the manifest as written rather than against the object the
// API server actually validates. Defaulting runs first, so for some fields an
// explicit "" never reaches the validation code being ported.
//
// Whether it reaches it is decided by one thing, and it is not obvious from
// the validation source — it is the shape of the guard in the defaulting
// function:
//
//	if len(obj.Spec.PodManagementPolicy) == 0 {   // value: "" IS defaulted
//	if obj.ReclaimPolicy == nil {                 // pointer: "" is NOT defaulted
//
// A value-typed field cannot distinguish "absent" from "explicitly empty", so
// both are defaulted and a rule that rejects "" is a false positive. A
// pointer-typed field can: an explicit "" unmarshals to a non-nil pointer to
// the empty string, defaulting is skipped, and the API server really does
// reject it — so a rule that rejects "" is correct and must be kept.
//
// Both directions are asserted here. Relaxing a pointer-typed rule to accept
// "" would be just as wrong as leaving a value-typed one strict; it would
// silently stop reporting a manifest the cluster rejects.
//
// Each fixture sets every defaulted field of its kind to the empty value at
// once, so a new check that reads one of those fields is covered without
// adding a case.
func TestExplicitlyEmptyDefaultedFields(t *testing.T) {
	cases := []struct {
		file string
		// want is the set of rule IDs that must fire, and exactly those.
		// Empty means every defaulted field of this kind is value-typed and
		// the manifest must be accepted whole.
		want []string
		why  string
	}{
		{file: "pod.yaml", why: "restartPolicy, dnsPolicy, schedulerName, serviceAccountName, imagePullPolicy, terminationMessagePath/Policy are all value-typed"},
		{file: "sts.yaml", why: "podManagementPolicy and updateStrategy.type are value-typed (len()==0 / ==\"\" guards in SetDefaults_StatefulSet)"},
		{file: "deploy.yaml", why: "strategy.type is value-typed"},
		{file: "ds.yaml", why: "updateStrategy.type is value-typed"},
		{file: "svc.yaml", why: "type and sessionAffinity are value-typed"},
		{file: "job.yaml", why: "completionMode and podReplacementPolicy are pointer-typed but unchecked"},
		{file: "cronjob.yaml", why: "concurrencyPolicy is value-typed"},
		{file: "hpa.yaml", why: "minReplicas is pointer-typed and absent, not empty"},
		{file: "csidriver.yaml", why: "volumeLifecycleModes is value-typed (len()==0)"},
		{file: "rb.yaml", why: "SetDefaults_RoleBinding guards roleRef.apiGroup on len()==0"},
		{file: "crb.yaml", why: "SetDefaults_ClusterRoleBinding guards roleRef.apiGroup on len()==0"},

		// Pointer-typed: the API server rejects these, so the checks must fire.
		{
			file: "netpol.yaml",
			want: []string{"network-policy/protocol-invalid"},
			why:  "SetDefaults_NetworkPolicyPort guards Protocol on == nil",
		},
		{
			file: "pv.yaml",
			want: []string{"persistent-volume-claim/volume-mode-invalid"},
			why:  "SetDefaults_PersistentVolume guards VolumeMode on == nil",
		},
		{
			file: "pvc.yaml",
			want: []string{"persistent-volume-claim/volume-mode-invalid"},
			why:  "SetDefaults_PersistentVolumeClaimSpec guards VolumeMode on == nil",
		},
		{
			file: "sc.yaml",
			want: []string{"storage-class/reclaim-policy-invalid", "storage-class/volume-binding-mode-invalid"},
			why:  "SetDefaults_StorageClass guards both on == nil",
		},
		{
			file: "vwc.yaml",
			want: []string{"admissionregistration/validating-failure-policy-invalid"},
			why:  "SetDefaults_ValidatingWebhook guards FailurePolicy on == nil",
		},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("testdata", "defaulting", tc.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var got []string
			for _, c := range check.All() {
				if c.Section() != "runtime-validation" {
					continue
				}
				dc, ok := c.(interface {
					CheckDoc(data []byte, source string) []check.Finding
				})
				if !ok {
					continue
				}
				for _, f := range dc.CheckDoc(data, tc.file) {
					got = append(got, f.CheckID)
				}
			}
			sort.Strings(got)
			got = dedupeStrings(got)

			want := append([]string(nil), tc.want...)
			sort.Strings(want)

			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("findings on explicitly-empty defaulted fields:\n got: %v\nwant: %v\nwhy: %s",
					got, want, tc.why)
			}
		})
	}
}

func dedupeStrings(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || in[i-1] != s {
			out = append(out, s)
		}
	}
	return out
}
