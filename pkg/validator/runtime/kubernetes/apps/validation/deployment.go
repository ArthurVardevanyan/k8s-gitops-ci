package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// deploymentSelectorInvalidCheck verifies the selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentSelectorInvalidCheck struct{}

func (c deploymentSelectorInvalidCheck) ID() string {
	return "apps/deployment-selector-invalid"
}

func (c deploymentSelectorInvalidCheck) Title() string {
	return "Selector Must Be A Valid Label Selector"
}

func (c deploymentSelectorInvalidCheck) Category() string {
	return "apps"
}

func (c deploymentSelectorInvalidCheck) Blocking() bool {
	return true
}

func (c deploymentSelectorInvalidCheck) RenderSensitive() bool {
	return true
}

func (c deploymentSelectorInvalidCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseDeployment(data)

	return selectorInvalidFindings(c, obj, "Deployment", name)
}

// deploymentStrategyTypeInvalidCheck verifies strategy type is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentStrategyTypeInvalidCheck struct{}

func (c deploymentStrategyTypeInvalidCheck) ID() string {
	return "apps/deployment-strategy-type-invalid"
}

func (c deploymentStrategyTypeInvalidCheck) Title() string {
	return "Strategy Type Must Be Valid"
}

func (c deploymentStrategyTypeInvalidCheck) Category() string {
	return "apps"
}

func (c deploymentStrategyTypeInvalidCheck) Blocking() bool {
	return true
}

func (c deploymentStrategyTypeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c deploymentStrategyTypeInvalidCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentStrategyTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	strategyMap, found, _ := unstructured.NestedMap(selectorMap, "spec", "strategy")
	if !found {
		return nil
	}

	strategyType, _, _ := unstructured.NestedString(strategyMap, "type")
	if strategyType == "" || strategyType == "RollingUpdate" || strategyType == "Recreate" {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("strategy").Child("type").String(),
			Message: fmt.Sprintf("strategy: Unsupported value: %q: supported values: \"RollingUpdate\", \"Recreate\"", strategyType),
			Kind:    "Deployment",
			Name:    name,
			Value:   strategyType,
		},
	}}
}

// deploymentReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentReplicasInvalidCheck struct{}

func (c deploymentReplicasInvalidCheck) ID() string {
	return "apps/deployment-replicas-invalid"
}

func (c deploymentReplicasInvalidCheck) Title() string {
	return "Replicas Must Be >= 0"
}

func (c deploymentReplicasInvalidCheck) Category() string {
	return "apps"
}

func (c deploymentReplicasInvalidCheck) Blocking() bool {
	return true
}

func (c deploymentReplicasInvalidCheck) RenderSensitive() bool {
	return true
}

func (c deploymentReplicasInvalidCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	replicas, found, err := unstructured.NestedInt64(selectorMap, "spec", "replicas")
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
			Kind:    "Deployment",
			Name:    name,
			Value:   fmt.Sprintf("%d", replicas),
		},
	}}
}

// deploymentMinReadySecondsInvalidCheck verifies minReadySeconds >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentMinReadySecondsInvalidCheck struct{}

func (c deploymentMinReadySecondsInvalidCheck) ID() string {
	return "apps/deployment-min-ready-seconds-invalid"
}

func (c deploymentMinReadySecondsInvalidCheck) Title() string {
	return "MinReadySeconds Must Be >= 0"
}

func (c deploymentMinReadySecondsInvalidCheck) Category() string {
	return "apps"
}

func (c deploymentMinReadySecondsInvalidCheck) Blocking() bool {
	return true
}

func (c deploymentMinReadySecondsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c deploymentMinReadySecondsInvalidCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentMinReadySecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	minReadySeconds, found, err := unstructured.NestedInt64(selectorMap, "spec", "minReadySeconds")
	if err != nil || !found {
		return nil
	}

	if minReadySeconds >= 0 {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("minReadySeconds").String(),
			Message: fmt.Sprintf("minReadySeconds: must be >= 0, got %d", minReadySeconds),
			Kind:    "Deployment",
			Name:    name,
			Value:   fmt.Sprintf("%d", minReadySeconds),
		},
	}}
}

// parseDeployment parses data as a Deployment resource, returning
// raw spec map and metadata name. Returns nil if not a Deployment.
func parseDeployment(data []byte) (specMap map[string]interface{}, name string) {
	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, ""
	}

	kind := nestedString(obj, "kind")
	if kind != "Deployment" {
		return nil, ""
	}

	name = nestedString(obj, "metadata", "name")
	specMap = obj

	return specMap, name
}

// nestedString returns a string value from the map at the given path.
func nestedString(obj map[string]interface{}, path ...string) string {
	if obj == nil {
		return ""
	}
	val, found, _ := unstructured.NestedString(obj, path...)
	if !found {
		return ""
	}
	return val
}

// ValidateDeployment runs all deployment validation checks and returns findings.
func ValidateDeployment(data []byte, source string) []runtime.Finding {
	checks := []runtime.Check{
		deploymentSelectorInvalidCheck{},
		deploymentStrategyTypeInvalidCheck{},
		deploymentReplicasInvalidCheck{},
		deploymentMinReadySecondsInvalidCheck{},
	}
	findings := make([]runtime.Finding, 0, len(checks))
	for _, c := range checks {
		findings = append(findings, c.Run(data, source)...)
	}
	return findings
}

// init registers all deployment checks.
func init() {
	Register()
}
