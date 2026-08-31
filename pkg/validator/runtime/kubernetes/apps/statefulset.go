package apps

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// statefulSetReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetReplicasInvalidCheck struct{ runtime.Meta }

func newStatefulSetReplicasInvalidCheck() statefulSetReplicasInvalidCheck {
	return statefulSetReplicasInvalidCheck{runtime.Meta{
		RuleID:    "apps/statefulset-replicas-invalid",
		RuleTitle: "Replicas Must Be >= 0",
		AppliesTo: []string{"StatefulSet"},
	}}
}

func (c statefulSetReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseKind(data, "StatefulSet")
	if specMap == nil {
		return nil
	}

	replicas, found, err := unstructured.NestedInt64(specMap, "spec", "replicas")
	if err != nil || !found {
		return nil
	}

	if replicas >= 0 {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("replicas").String(),
			Message: fmt.Sprintf("replicas: must be >= 0, got %d", replicas),
			Kind:    "StatefulSet",
			Name:    name,
			Value:   fmt.Sprintf("%d", replicas),
		},
	}}
}

// statefulSetPodManagementPolicyInvalidCheck verifies podManagementPolicy is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetPodManagementPolicyInvalidCheck struct{ runtime.Meta }

func newStatefulSetPodManagementPolicyInvalidCheck() statefulSetPodManagementPolicyInvalidCheck {
	return statefulSetPodManagementPolicyInvalidCheck{runtime.Meta{
		RuleID:    "apps/statefulset-pod-management-policy-invalid",
		RuleTitle: "PodManagementPolicy Must Be Valid",
		AppliesTo: []string{"StatefulSet"},
	}}
}

func (c statefulSetPodManagementPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "StatefulSet")

	return enumFieldFindings(
		c, obj, "StatefulSet", name, "podManagementPolicy",
		[]string{"spec", "podManagementPolicy"},
		[]string{"OrderedReady", "Parallel"},
	)
}

// statefulSetUpdateStrategyInvalidCheck verifies updateStrategy type is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetUpdateStrategyInvalidCheck struct{ runtime.Meta }

func newStatefulSetUpdateStrategyInvalidCheck() statefulSetUpdateStrategyInvalidCheck {
	return statefulSetUpdateStrategyInvalidCheck{runtime.Meta{
		RuleID:    "apps/statefulset-update-strategy-invalid",
		RuleTitle: "UpdateStrategy Type Must Be Valid",
		AppliesTo: []string{"StatefulSet"},
	}}
}

func (c statefulSetUpdateStrategyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "StatefulSet")

	// "Recreate" is accepted because Kubernetes 1.37 adds it behind the
	// AllowStatefulSetRecreateStrategy gate. This tool cannot see a target
	// cluster's feature gates, so it takes the permissive branch: a cluster
	// with the gate on must not have a valid manifest blocked by an
	// always-blocking, non-exemptable check. A cluster with the gate off
	// rejects it at apply time instead, which is the safe direction to err.
	return enumFieldFindings(
		c, obj, "StatefulSet", name, "updateStrategy",
		[]string{"spec", "updateStrategy", "type"},
		[]string{"RollingUpdate", "OnDelete", "Recreate"},
	)
}

// init registers all statefulset checks.
func init() {
	Register()
}
