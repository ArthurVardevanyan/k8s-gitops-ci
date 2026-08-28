package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

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
	return runtime.HasPodSpecKinds()
}

func (c statefulSetReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseStatefulSet(data)
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
	return runtime.HasPodSpecKinds()
}

func (c statefulSetPodManagementPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseStatefulSet(data)
	if specMap == nil {
		return nil
	}

	podManagementPolicy, found, _ := unstructured.NestedString(specMap, "spec", "podManagementPolicy")
	if !found {
		return nil
	}

	if podManagementPolicy == "OrderedReady" || podManagementPolicy == "Parallel" {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("podManagementPolicy").String(),
			Message: fmt.Sprintf("podManagementPolicy: Unsupported value: %q: supported values: \"OrderedReady\", \"Parallel\"", podManagementPolicy),
			Kind:    "StatefulSet",
			Name:    name,
			Value:   podManagementPolicy,
		},
	}}
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
	return runtime.HasPodSpecKinds()
}

func (c statefulSetUpdateStrategyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseStatefulSet(data)
	if specMap == nil {
		return nil
	}

	strategyMap, found, _ := unstructured.NestedMap(specMap, "spec", "updateStrategy")
	if !found {
		return nil
	}

	updateStrategyType, _, _ := unstructured.NestedString(strategyMap, "type")
	if updateStrategyType == "" || updateStrategyType == "RollingUpdate" || updateStrategyType == "OnDelete" {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("updateStrategy").Child("type").String(),
			Message: fmt.Sprintf("updateStrategy: Unsupported value: %q: supported values: \"RollingUpdate\", \"OnDelete\"", updateStrategyType),
			Kind:    "StatefulSet",
			Name:    name,
			Value:   updateStrategyType,
		},
	}}
}

// parseStatefulSet parses data as a StatefulSet resource.
func parseStatefulSet(data []byte) (specMap map[string]interface{}, name string) {
	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, ""
	}

	kind := nestedString(obj, "kind")
	if kind != "StatefulSet" {
		return nil, ""
	}

	name = nestedString(obj, "metadata", "name")
	specMap = obj

	return specMap, name
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
