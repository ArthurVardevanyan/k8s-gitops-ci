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

	return enumFieldFindings(
		c, obj, "StatefulSet", name, "updateStrategy",
		[]string{"spec", "updateStrategy", "type"},
		[]string{"RollingUpdate", "OnDelete"},
		true,
	)
}

// ValidateStatefulSet runs all statefulset validation checks and returns findings.
func ValidateStatefulSet(data []byte, source string) []runtime.Finding {
	checks := []runtime.Check{
		statefulSetReplicasInvalidCheck{},
		statefulSetPodManagementPolicyInvalidCheck{},
		statefulSetUpdateStrategyInvalidCheck{},
	}
	findings := make([]runtime.Finding, 0, len(checks))
	for _, c := range checks {
		findings = append(findings, c.Run(data, source)...)
	}
	return findings
}

// init registers all statefulset checks.
func init() {
	Register()
}
