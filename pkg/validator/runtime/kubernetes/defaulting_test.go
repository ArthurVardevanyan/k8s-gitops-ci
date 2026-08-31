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
// the empty string and defaulting is skipped, so the value does reach
// validation.
//
// Reaching validation is not the same as being rejected by it, and treating
// the two as equivalent is how a false positive got in here. Once "" arrives,
// the ported function decides:
//
//	if !supportedVolumeBindingModes.Has(*mode) {   // "" IS rejected
//	if len(string(*reclaimPolicy)) > 0 { ... }     // "" is NOT rejected
//
// Both fields are pointer-typed on the same kind and defaulted by the same
// function, and they still differ, because validateReclaimPolicy wraps its
// NotSupported branch in a length guard and validateVolumeBindingMode does
// not. So a fixture entry has to answer two questions, not one: does the
// empty value survive defaulting, and does the cited function then reject it?
//
// Both directions are asserted here. Relaxing a rule whose upstream function
// really does reject "" would silently stop reporting a manifest the cluster
// rejects; keeping one whose function accepts "" blocks a manifest the
// cluster allows.
//
// Each fixture sets every defaulted field of its kind to the empty value at
// once, so a new check that reads one of those fields is covered without
// adding a case.
func TestExplicitlyEmptyDefaultedFields(t *testing.T) {
	cases := []struct {
		file string
		// kind is the fixture's root kind, declared rather than parsed out of
		// the document. The engine filters checks by kind before running them
		// (see evaluateDoc), and a test that skips that filter observes
		// cross-kind leakage that cannot happen in production - a check whose
		// Run does not re-guard on kind will happily read a structurally
		// similar spec belonging to another kind.
		kind string
		// want is the set of rule IDs that must fire, and exactly those.
		// Empty means every defaulted field of this kind is value-typed and
		// the manifest must be accepted whole.
		want []string
		why  string
	}{
		{file: "pod.yaml", kind: "Pod", why: "restartPolicy, dnsPolicy, schedulerName, serviceAccountName, imagePullPolicy, terminationMessagePath/Policy are all value-typed"},
		{file: "sts.yaml", kind: "StatefulSet", why: "podManagementPolicy and updateStrategy.type are value-typed (len()==0 / ==\"\" guards in SetDefaults_StatefulSet)"},
		{file: "deploy.yaml", kind: "Deployment", why: "strategy.type is value-typed"},
		{file: "ds.yaml", kind: "DaemonSet", why: "updateStrategy.type is value-typed"},
		{file: "svc.yaml", kind: "Service", why: "type and sessionAffinity are value-typed"},
		{file: "job.yaml", kind: "Job", why: "completionMode and podReplacementPolicy are pointer-typed but unchecked"},
		{file: "cronjob.yaml", kind: "CronJob", why: "concurrencyPolicy is value-typed"},
		{file: "hpa.yaml", kind: "HorizontalPodAutoscaler", why: "minReplicas is pointer-typed and absent, not empty"},
		{file: "csidriver.yaml", kind: "CSIDriver", why: "volumeLifecycleModes is value-typed (len()==0)"},
		{file: "rb.yaml", kind: "RoleBinding", why: "SetDefaults_RoleBinding guards roleRef.apiGroup on len()==0"},
		{file: "crb.yaml", kind: "ClusterRoleBinding", why: "SetDefaults_ClusterRoleBinding guards roleRef.apiGroup on len()==0"},

		// Pointer-typed: the API server rejects these, so the checks must fire.
		{
			file: "netpol.yaml",
			kind: "NetworkPolicy",
			want: []string{"network-policy/protocol-invalid"},
			why:  "SetDefaults_NetworkPolicyPort guards Protocol on == nil",
		},
		{
			file: "pv.yaml",
			kind: "PersistentVolume",
			want: []string{"persistent-volume/volume-mode-invalid"},
			why:  "SetDefaults_PersistentVolume guards VolumeMode on == nil",
		},
		{
			file: "pvc.yaml",
			kind: "PersistentVolumeClaim",
			want: []string{"persistent-volume-claim/volume-mode-invalid"},
			why:  "SetDefaults_PersistentVolumeClaimSpec guards VolumeMode on == nil",
		},
		{
			file: "sc.yaml",
			kind: "StorageClass",
			want: []string{"storage-class/volume-binding-mode-invalid"},
			// reclaimPolicy is deliberately absent. Surviving defaulting is
			// only half the question: upstream validateReclaimPolicy then
			// wraps its NotSupported branch in a len(...) > 0 guard, so the
			// API server accepts an empty value and reporting it would be a
			// false positive. validateVolumeBindingMode has no such guard,
			// which is why its sibling stays.
			why: "SetDefaults_StorageClass guards both on == nil, but only volumeBindingMode is then rejected empty",
		},
		{
			file: "vwc.yaml",
			kind: "ValidatingWebhookConfiguration",
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

			got := runtimeFindingsForKind(t, data, tc.kind, tc.file)
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

// runtimeFindingsForKind runs every registered runtime check that the engine
// would dispatch for kind, and returns the rule IDs that fired, sorted and
// deduplicated.
//
// The kind filter is the part that matters. evaluateDoc in pkg/validator
// consults check.DocSkipper before calling CheckDoc, and most runtime checks
// rely on that entirely rather than re-checking the kind themselves - reading
// a typed struct out of the document is enough once dispatch guarantees the
// kind. A test that calls CheckDoc directly drops that guarantee and sees
// findings production cannot produce: the PersistentVolumeClaim volume-mode
// rule, for instance, will happily decode a PersistentVolume, whose spec has
// an identically-shaped volumeMode field.
func runtimeFindingsForKind(t *testing.T, doc []byte, kind, source string) []string {
	t.Helper()

	var got []string
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		skipper, ok := c.(interface{ SkipDoc(string) bool })
		if !ok {
			t.Fatalf("check %q does not implement SkipDoc, so the dispatcher cannot filter it", c.ID())
		}
		if skipper.SkipDoc(kind) {
			continue
		}
		dc, ok := c.(interface {
			CheckDoc(data []byte, source string) []check.Finding
		})
		if !ok {
			t.Fatalf("check %q is not a DocCheck", c.ID())
		}
		for _, f := range dc.CheckDoc(doc, source) {
			got = append(got, f.CheckID)
		}
	}
	sort.Strings(got)
	return dedupeStrings(got)
}
