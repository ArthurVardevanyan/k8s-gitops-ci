package apps

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// deploymentSelectorInvalidCheck verifies the selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentSelectorInvalidCheck struct{ runtime.Meta }

func newDeploymentSelectorInvalidCheck() deploymentSelectorInvalidCheck {
	return deploymentSelectorInvalidCheck{runtime.Meta{
		RuleID:    "apps/deployment-selector-invalid",
		RuleTitle: "Selector Must Be A Valid Label Selector",
		AppliesTo: []string{"Deployment"},
	}}
}

func (c deploymentSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "Deployment")

	return selectorInvalidFindings(c, obj, "Deployment", name)
}

// deploymentStrategyTypeInvalidCheck verifies strategy type is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentStrategyTypeInvalidCheck struct{ runtime.Meta }

func newDeploymentStrategyTypeInvalidCheck() deploymentStrategyTypeInvalidCheck {
	return deploymentStrategyTypeInvalidCheck{runtime.Meta{
		RuleID:    "apps/deployment-strategy-type-invalid",
		RuleTitle: "Strategy Type Must Be Valid",
		AppliesTo: []string{"Deployment"},
	}}
}

func (c deploymentStrategyTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "Deployment")

	return enumFieldFindings(
		c, obj, "Deployment", name, "strategy",
		[]string{"spec", "strategy", "type"},
		[]string{"RollingUpdate", "Recreate"},
	)
}

// deploymentReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentReplicasInvalidCheck struct{ runtime.Meta }

func newDeploymentReplicasInvalidCheck() deploymentReplicasInvalidCheck {
	return deploymentReplicasInvalidCheck{runtime.Meta{
		RuleID:    "apps/deployment-replicas-invalid",
		RuleTitle: "Replicas Must Be >= 0",
		AppliesTo: []string{"Deployment"},
	}}
}

func (c deploymentReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseKind(data, "Deployment")
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
type deploymentMinReadySecondsInvalidCheck struct{ runtime.Meta }

func newDeploymentMinReadySecondsInvalidCheck() deploymentMinReadySecondsInvalidCheck {
	return deploymentMinReadySecondsInvalidCheck{runtime.Meta{
		RuleID:    "apps/deployment-min-ready-seconds-invalid",
		RuleTitle: "MinReadySeconds Must Be >= 0",
		AppliesTo: []string{"Deployment"},
	}}
}

func (c deploymentMinReadySecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseKind(data, "Deployment")
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
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("minReadySeconds").String(),
			Message: fmt.Sprintf("minReadySeconds: must be >= 0, got %d", minReadySeconds),
			Kind:    "Deployment",
			Name:    name,
			Value:   fmt.Sprintf("%d", minReadySeconds),
		},
	}}
}

// init registers all deployment checks.
func init() {
	Register()
}
