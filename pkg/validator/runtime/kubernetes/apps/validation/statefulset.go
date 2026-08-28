package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// statefulSetReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetReplicasInvalidCheck struct{}

func (c statefulSetReplicasInvalidCheck) ID() string {
	return "apps/statefulset-replicas-invalid"
}

func (c statefulSetReplicasInvalidCheck) Title() string {
	return "Replicas Must Be >= 0"
}

func (c statefulSetReplicasInvalidCheck) Category() string {
	return "apps"
}

func (c statefulSetReplicasInvalidCheck) Blocking() bool {
	return true
}

func (c statefulSetReplicasInvalidCheck) RenderSensitive() bool {
	return true
}

func (c statefulSetReplicasInvalidCheck) Kinds() []string {
	return []string{"StatefulSet"}
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
		Category:  c.Category(),
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
type statefulSetPodManagementPolicyInvalidCheck struct{}

func (c statefulSetPodManagementPolicyInvalidCheck) ID() string {
	return "apps/statefulset-pod-management-policy-invalid"
}

func (c statefulSetPodManagementPolicyInvalidCheck) Title() string {
	return "PodManagementPolicy Must Be Valid"
}

func (c statefulSetPodManagementPolicyInvalidCheck) Category() string {
	return "apps"
}

func (c statefulSetPodManagementPolicyInvalidCheck) Blocking() bool {
	return true
}

func (c statefulSetPodManagementPolicyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c statefulSetPodManagementPolicyInvalidCheck) Kinds() []string {
	return []string{"StatefulSet"}
}

func (c statefulSetPodManagementPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "StatefulSet")

	return enumFieldFindings(
		c, obj, "StatefulSet", name, "podManagementPolicy",
		[]string{"spec", "podManagementPolicy"},
		[]string{"OrderedReady", "Parallel"},
		false,
	)
}

// statefulSetUpdateStrategyInvalidCheck verifies updateStrategy type is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetUpdateStrategyInvalidCheck struct{}

func (c statefulSetUpdateStrategyInvalidCheck) ID() string {
	return "apps/statefulset-update-strategy-invalid"
}

func (c statefulSetUpdateStrategyInvalidCheck) Title() string {
	return "UpdateStrategy Type Must Be Valid"
}

func (c statefulSetUpdateStrategyInvalidCheck) Category() string {
	return "apps"
}

func (c statefulSetUpdateStrategyInvalidCheck) Blocking() bool {
	return true
}

func (c statefulSetUpdateStrategyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c statefulSetUpdateStrategyInvalidCheck) Kinds() []string {
	return []string{"StatefulSet"}
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
		true,
	)
}

// init registers all statefulset checks.
func init() {
	Register()
}
