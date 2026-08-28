package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// daemonSetSelectorMustMatchCheck verifies selector matches template labels.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type daemonSetSelectorMustMatchCheck struct{}

func (c daemonSetSelectorMustMatchCheck) ID() string {
	return "apps/daemonset-selector-must-match"
}

func (c daemonSetSelectorMustMatchCheck) Title() string {
	return "Selector Must Match Template Labels"
}

func (c daemonSetSelectorMustMatchCheck) Category() string {
	return "apps"
}

func (c daemonSetSelectorMustMatchCheck) Blocking() bool {
	return true
}

func (c daemonSetSelectorMustMatchCheck) RenderSensitive() bool {
	return true
}

func (c daemonSetSelectorMustMatchCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c daemonSetSelectorMustMatchCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseDaemonSet(data)
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
					Kind:    "DaemonSet",
					Name:    name,
				},
			}}
		}
	}

	return nil
}

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

func (c daemonSetSelectorInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c daemonSetSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseDaemonSet(data)
	if specMap == nil {
		return nil
	}

	selectorMap, found, _ := unstructured.NestedMap(specMap, "spec", "selector")
	if !found {
		return nil
	}

	// Check matchLabels keys (only string values in the selector map)
	for key, val := range selectorMap {
		if _, ok := val.(string); !ok {
			continue
		}
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return []runtime.Finding{{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("selector").Child("matchLabels").Key(key).String(),
					Message: fmt.Sprintf("invalid label selector key %q: %s", key, strings.Join(errs, ", ")),
					Kind:    "DaemonSet",
					Name:    name,
				},
			}}
		}
	}

	// Check matchExpressions keys
	matchExpressionsList, found, err := unstructured.NestedSlice(specMap, "spec", "selector", "matchExpressions")
	if err != nil || !found {
		return nil
	}

	for _, rawExpr := range matchExpressionsList {
		exprMap, ok := rawExpr.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := exprMap["key"].(string)
		if key == "" {
			continue
		}
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return []runtime.Finding{{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("selector").Child("matchExpressions").Child("key").String(),
					Message: fmt.Sprintf("invalid label selector key %q: %s", key, strings.Join(errs, ", ")),
					Kind:    "DaemonSet",
					Name:    name,
				},
			}}
		}
	}

	return nil
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

func (c daemonSetUpdateStrategyInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c daemonSetUpdateStrategyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseDaemonSet(data)
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
			Kind:    "DaemonSet",
			Name:    name,
			Value:   updateStrategyType,
		},
	}}
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

func (c daemonSetMinReadySecondsInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c daemonSetMinReadySecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseDaemonSet(data)
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

// parseDaemonSet parses data as a DaemonSet resource.
func parseDaemonSet(data []byte) (specMap map[string]interface{}, name string) {
	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, ""
	}

	kind := nestedString(obj, "kind")
	if kind != "DaemonSet" {
		return nil, ""
	}

	name = nestedString(obj, "metadata", "name")
	specMap = obj

	return specMap, name
}

// ValidateDaemonSet runs all daemonset validation checks and returns findings.
func ValidateDaemonSet(data []byte, source string) []runtime.Finding {
	checks := []runtime.Check{
		daemonSetSelectorMustMatchCheck{},
		daemonSetSelectorInvalidCheck{},
		daemonSetUpdateStrategyInvalidCheck{},
		daemonSetMinReadySecondsInvalidCheck{},
	}
	findings := make([]runtime.Finding, 0, len(checks))
	for _, c := range checks {
		findings = append(findings, c.Run(data, source)...)
	}
	return findings
}

// init registers all daemonset checks.
func init() {
	Register()
}
