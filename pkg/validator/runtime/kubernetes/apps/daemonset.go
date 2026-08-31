package apps

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// daemonSetSelectorInvalidCheck verifies selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetSelectorInvalidCheck struct{ runtime.Meta }

func newDaemonSetSelectorInvalidCheck() daemonSetSelectorInvalidCheck {
	return daemonSetSelectorInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/apps/daemonset-selector-invalid",
		RuleTitle: "Selector Must Be A Valid Label Selector",
		AppliesTo: []string{"DaemonSet"},
	}}
}

func (c daemonSetSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "DaemonSet")

	return selectorInvalidFindings(c, obj, "DaemonSet", name)
}

// daemonSetUpdateStrategyInvalidCheck verifies updateStrategy type is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetUpdateStrategyInvalidCheck struct{ runtime.Meta }

func newDaemonSetUpdateStrategyInvalidCheck() daemonSetUpdateStrategyInvalidCheck {
	return daemonSetUpdateStrategyInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/apps/daemonset-update-strategy-invalid",
		RuleTitle: "UpdateStrategy Type Must Be Valid",
		AppliesTo: []string{"DaemonSet"},
	}}
}

func (c daemonSetUpdateStrategyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "DaemonSet")

	return enumFieldFindings(
		c, obj, "DaemonSet", name, "updateStrategy",
		[]string{"spec", "updateStrategy", "type"},
		[]string{"RollingUpdate", "OnDelete"},
	)
}

// daemonSetMinReadySecondsInvalidCheck verifies minReadySeconds >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetMinReadySecondsInvalidCheck struct{ runtime.Meta }

func newDaemonSetMinReadySecondsInvalidCheck() daemonSetMinReadySecondsInvalidCheck {
	return daemonSetMinReadySecondsInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/apps/daemonset-min-ready-seconds-invalid",
		RuleTitle: "MinReadySeconds Must Be >= 0",
		AppliesTo: []string{"DaemonSet"},
	}}
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
