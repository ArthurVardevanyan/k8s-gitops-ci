package validation

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// The apps checks are the same handful of rules applied to different
// workloads: three kinds validate a selector, three validate replicas, two
// validate minReadySeconds, two validate an update strategy. Each rule was
// covered by a near-identical function per workload, differing only in the
// kind name and the constructor.
//
// These tables cover each rule once across every workload that has it, so a
// rule gains coverage when a workload is added to it rather than needing a
// new copy of the same test.

// workloadDoc builds an apps/v1 workload with the given spec body. The
// template is included because these checks decode the whole spec.
func workloadDoc(kind, specBody string) []byte {
	return []byte("apiVersion: apps/v1\nkind: " + kind + "\nmetadata:\n  name: test\nspec:\n" +
		specBody +
		"  template:\n    metadata:\n      labels:\n        app: myapp\n" +
		"    spec:\n      containers:\n      - name: c\n        image: nginx\n")
}

const (
	validSelector = "  selector:\n    matchLabels:\n      app: myapp\n" +
		"      \"app.kubernetes.io/name\": myapp\n"
	invalidSelectorKey = "  selector:\n    matchLabels:\n      \"invalid key with spaces\": myapp\n"
	invalidMatchExpr   = "  selector:\n    matchExpressions:\n    - key: \"another invalid key\"\n" +
		"      operator: In\n      values:\n      - val\n"
)

func TestWorkloadSelectorInvalid(t *testing.T) {
	checks := map[string]runtime.Check{
		"Deployment": newDeploymentSelectorInvalidCheck(),
		"DaemonSet":  newDaemonSetSelectorInvalidCheck(),
		"ReplicaSet": newReplicaSetSelectorInvalidCheck(),
	}
	cases := []struct {
		name string
		spec string
		want int
	}{
		{"valid keys", validSelector, 0},
		{"no selector", "", 0},
		{"invalid matchLabels key", invalidSelectorKey, 1},
		{"invalid matchExpressions key", invalidMatchExpr, 1},
	}

	for kind, c := range checks {
		for _, tt := range cases {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				got := c.Run(workloadDoc(kind, tt.spec), "test.yaml")
				if len(got) != tt.want {
					t.Fatalf("expected %d finding(s), got %d: %v", tt.want, len(got), got)
				}
				if tt.want > 0 && !strings.HasSuffix(got[0].RuleID, "selector-invalid") {
					t.Errorf("unexpected rule ID: %s", got[0].RuleID)
				}
			})
		}
	}
}

func TestWorkloadReplicasInvalid(t *testing.T) {
	checks := map[string]runtime.Check{
		"Deployment":  newDeploymentReplicasInvalidCheck(),
		"StatefulSet": newStatefulSetReplicasInvalidCheck(),
		"ReplicaSet":  newReplicaSetReplicasInvalidCheck(),
	}
	cases := []struct {
		name string
		spec string
		want int
	}{
		{"positive", "  replicas: 3\n", 0},
		{"zero", "  replicas: 0\n", 0},
		{"absent", "", 0},
		{"negative", "  replicas: -1\n", 1},
	}

	for kind, c := range checks {
		for _, tt := range cases {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				got := c.Run(workloadDoc(kind, validSelector+tt.spec), "test.yaml")
				if len(got) != tt.want {
					t.Fatalf("expected %d finding(s), got %d: %v", tt.want, len(got), got)
				}
				if tt.want > 0 && got[0].Kind != kind {
					t.Errorf("finding Kind = %q, want %q", got[0].Kind, kind)
				}
			})
		}
	}
}

func TestWorkloadMinReadySecondsInvalid(t *testing.T) {
	checks := map[string]runtime.Check{
		"Deployment": newDeploymentMinReadySecondsInvalidCheck(),
		"DaemonSet":  newDaemonSetMinReadySecondsInvalidCheck(),
	}
	cases := []struct {
		name string
		spec string
		want int
	}{
		{"positive", "  minReadySeconds: 10\n", 0},
		{"zero", "  minReadySeconds: 0\n", 0},
		{"absent", "", 0},
		{"negative", "  minReadySeconds: -1\n", 1},
	}

	for kind, c := range checks {
		for _, tt := range cases {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				got := c.Run(workloadDoc(kind, validSelector+tt.spec), "test.yaml")
				if len(got) != tt.want {
					t.Fatalf("expected %d finding(s), got %d: %v", tt.want, len(got), got)
				}
			})
		}
	}
}

// StatefulSet and DaemonSet share the updateStrategy enum, but not its
// members: Recreate is valid only for a StatefulSet, behind the
// AllowStatefulSetRecreateStrategy gate, and the permissive branch is taken
// because this tool cannot see a cluster's gates.
func TestWorkloadUpdateStrategyInvalid(t *testing.T) {
	cases := []struct {
		kind  string
		check runtime.Check
		value string
		want  int
	}{
		{"StatefulSet", newStatefulSetUpdateStrategyInvalidCheck(), "RollingUpdate", 0},
		{"StatefulSet", newStatefulSetUpdateStrategyInvalidCheck(), "OnDelete", 0},
		{"StatefulSet", newStatefulSetUpdateStrategyInvalidCheck(), "Recreate", 0},
		{"StatefulSet", newStatefulSetUpdateStrategyInvalidCheck(), "", 0},
		{"StatefulSet", newStatefulSetUpdateStrategyInvalidCheck(), "Bogus", 1},
		{"DaemonSet", newDaemonSetUpdateStrategyInvalidCheck(), "RollingUpdate", 0},
		{"DaemonSet", newDaemonSetUpdateStrategyInvalidCheck(), "OnDelete", 0},
		{"DaemonSet", newDaemonSetUpdateStrategyInvalidCheck(), "", 0},
		{"DaemonSet", newDaemonSetUpdateStrategyInvalidCheck(), "Bogus", 1},
	}

	for _, tt := range cases {
		name := tt.kind + "/" + tt.value
		if tt.value == "" {
			name = tt.kind + "/absent"
		}
		t.Run(name, func(t *testing.T) {
			spec := validSelector
			if tt.value != "" {
				spec += "  updateStrategy:\n    type: " + tt.value + "\n"
			}
			got := tt.check.Run(workloadDoc(tt.kind, spec), "test.yaml")
			if len(got) != tt.want {
				t.Fatalf("expected %d finding(s), got %d: %v", tt.want, len(got), got)
			}
		})
	}
}

// Unparseable input must yield nothing rather than an error or a spurious
// finding. Kinds a check does not declare are covered for all 81 checks by
// TestChecksIgnoreKindsTheyDoNotDeclare in the parent package.
func TestAppsChecksIgnoreInvalidYAML(t *testing.T) {
	for _, c := range []runtime.Check{
		newDeploymentSelectorInvalidCheck(), newDeploymentReplicasInvalidCheck(),
		newDeploymentMinReadySecondsInvalidCheck(), newDeploymentStrategyTypeInvalidCheck(),
		newStatefulSetReplicasInvalidCheck(), newStatefulSetUpdateStrategyInvalidCheck(),
		newStatefulSetPodManagementPolicyInvalidCheck(),
		newDaemonSetSelectorInvalidCheck(), newDaemonSetUpdateStrategyInvalidCheck(),
		newDaemonSetMinReadySecondsInvalidCheck(),
		newReplicaSetSelectorInvalidCheck(), newReplicaSetReplicasInvalidCheck(),
	} {
		if got := c.Run([]byte("not valid yaml {{"), "test.yaml"); len(got) != 0 {
			t.Errorf("check %s reported %d finding(s) for unparseable input", c.ID(), len(got))
		}
	}
}
