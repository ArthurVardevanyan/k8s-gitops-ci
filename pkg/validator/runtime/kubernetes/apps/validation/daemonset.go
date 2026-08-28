package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// daemonSetSelectorInvalidCheck verifies selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetSelectorInvalidCheck struct{}

func (c daemonSetSelectorInvalidCheck) ID() string {
	return "apps/daemonset-selector-invalid"
}

func (c daemonSetSelectorInvalidCheck) Title() string {
	return "Selector Must Be A Valid Label Selector"
}

func (c daemonSetSelectorInvalidCheck) Category() string {
	return "apps"
}

func (c daemonSetSelectorInvalidCheck) Blocking() bool {
	return true
}

func (c daemonSetSelectorInvalidCheck) RenderSensitive() bool {
	return true
}

func (c daemonSetSelectorInvalidCheck) Kinds() []string {
	return []string{"DaemonSet"}
}

func (c daemonSetSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "DaemonSet")

	return selectorInvalidFindings(c, obj, "DaemonSet", name)
}

// daemonSetUpdateStrategyInvalidCheck verifies updateStrategy type is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetUpdateStrategyInvalidCheck struct{}

func (c daemonSetUpdateStrategyInvalidCheck) ID() string {
	return "apps/daemonset-update-strategy-invalid"
}

func (c daemonSetUpdateStrategyInvalidCheck) Title() string {
	return "UpdateStrategy Type Must Be Valid"
}

func (c daemonSetUpdateStrategyInvalidCheck) Category() string {
	return "apps"
}

func (c daemonSetUpdateStrategyInvalidCheck) Blocking() bool {
	return true
}

func (c daemonSetUpdateStrategyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c daemonSetUpdateStrategyInvalidCheck) Kinds() []string {
	return []string{"DaemonSet"}
}

func (c daemonSetUpdateStrategyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "DaemonSet")

	return enumFieldFindings(
		c, obj, "DaemonSet", name, "updateStrategy",
		[]string{"spec", "updateStrategy", "type"},
		[]string{"RollingUpdate", "OnDelete"},
		true,
	)
}

// daemonSetMinReadySecondsInvalidCheck verifies minReadySeconds >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetMinReadySecondsInvalidCheck struct{}

func (c daemonSetMinReadySecondsInvalidCheck) ID() string {
	return "apps/daemonset-min-ready-seconds-invalid"
}

func (c daemonSetMinReadySecondsInvalidCheck) Title() string {
	return "MinReadySeconds Must Be >= 0"
}

func (c daemonSetMinReadySecondsInvalidCheck) Category() string {
	return "apps"
}

func (c daemonSetMinReadySecondsInvalidCheck) Blocking() bool {
	return true
}

func (c daemonSetMinReadySecondsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c daemonSetMinReadySecondsInvalidCheck) Kinds() []string {
	return []string{"DaemonSet"}
}

func (c daemonSetMinReadySecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseKind(data, "DaemonSet")
	if specMap == nil {
		return nil
	}

	minReadySeconds, found, err := unstructured.NestedInt64(specMap, "spec", "minReadySeconds")
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
			Kind:    "DaemonSet",
			Name:    name,
			Value:   fmt.Sprintf("%d", minReadySeconds),
		},
	}}
}

// init registers all daemonset checks.
func init() {
	Register()
}
