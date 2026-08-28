package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
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

func (c statefulSetReplicasInvalidCheck) DocSkipper() []string {
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

// statefulSetSelectorMustMatchCheck verifies selector matches template labels.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetSelectorMustMatchCheck struct{}

func (c statefulSetSelectorMustMatchCheck) ID() string {
	return "apps/statefulset-selector-must-match"
}

func (c statefulSetSelectorMustMatchCheck) Title() string {
	return "Selector Must Match Template Labels"
}

func (c statefulSetSelectorMustMatchCheck) Category() string {
	return "apps"
}

func (c statefulSetSelectorMustMatchCheck) Blocking() bool {
	return true
}

func (c statefulSetSelectorMustMatchCheck) RenderSensitive() bool {
	return true
}

func (c statefulSetSelectorMustMatchCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c statefulSetSelectorMustMatchCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseStatefulSet(data)
	if specMap == nil {
		return nil
	}

	selector, _, err := unstructured.NestedStringMap(specMap, "spec", "selector")
	if err != nil {
		return nil
	}

	templateLabels, _, err := unstructured.NestedStringMap(specMap, "spec", "template", "metadata", "labels")
	if err != nil {
		return nil
	}

	for key, value := range selector {
		templateValue, exists := templateLabels[key]
		if !exists || templateValue != value {
			return []runtime.Finding{{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("selector").Key(key).String(),
					Message: fmt.Sprintf("selector[%q=%q] does not match template labels", key, value),
					Kind:    "StatefulSet",
					Name:    name,
				},
			}}
		}
	}

	return nil
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

func (c statefulSetPodManagementPolicyInvalidCheck) DocSkipper() []string {
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

func (c statefulSetUpdateStrategyInvalidCheck) DocSkipper() []string {
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

// statefulSetServiceNameInvalidCheck verifies serviceName is a valid DNS subdomain.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetServiceNameInvalidCheck struct{}

func (c statefulSetServiceNameInvalidCheck) ID() string {
	return "apps/statefulset-service-name-invalid"
}

func (c statefulSetServiceNameInvalidCheck) Title() string {
	return "ServiceName Must Be A Valid DNS Subdomain"
}

func (c statefulSetServiceNameInvalidCheck) Category() string {
	return "apps"
}

func (c statefulSetServiceNameInvalidCheck) Blocking() bool {
	return true
}

func (c statefulSetServiceNameInvalidCheck) RenderSensitive() bool {
	return true
}

func (c statefulSetServiceNameInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c statefulSetServiceNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseStatefulSet(data)
	if specMap == nil {
		return nil
	}

	serviceName, found, _ := unstructured.NestedString(specMap, "spec", "serviceName")
	if !found || serviceName == "" {
		return nil
	}

	if errs := validation.IsDNS1123Subdomain(serviceName); len(errs) > 0 {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("serviceName").String(),
				Message: fmt.Sprintf("serviceName: invalid value: %s", serviceName),
				Kind:    "StatefulSet",
				Name:    name,
				Value:   serviceName,
			},
		}}
	}

	return nil
}

// statefulSetVolumeClaimTemplatesEmptyCheck verifies volumeClaimTemplates is not empty when required.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type statefulSetVolumeClaimTemplatesEmptyCheck struct{}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) ID() string {
	return "apps/statefulset-volume-claim-templates-empty"
}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) Title() string {
	return "VolumeClaimTemplates Must Not Be Empty"
}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) Category() string {
	return "apps"
}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) Blocking() bool {
	return true
}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) RenderSensitive() bool {
	return true
}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c statefulSetVolumeClaimTemplatesEmptyCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseStatefulSet(data)
	if specMap == nil {
		return nil
	}

	volumeClaimTemplates, found, _ := unstructured.NestedSlice(specMap, "spec", "volumeClaimTemplates")
	if !found {
		return nil
	}

	if len(volumeClaimTemplates) > 0 {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("volumeClaimTemplates").String(),
			Message: "volumeClaimTemplates must not be empty",
			Kind:    "StatefulSet",
			Name:    name,
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
		statefulSetSelectorMustMatchCheck{},
		statefulSetPodManagementPolicyInvalidCheck{},
		statefulSetUpdateStrategyInvalidCheck{},
		statefulSetServiceNameInvalidCheck{},
		statefulSetVolumeClaimTemplatesEmptyCheck{},
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
