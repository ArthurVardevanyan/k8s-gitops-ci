package validation

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// deploymentSelectorMustMatchCheck verifies that .spec.selector matches
// .spec.template.metadata.labels.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go:100-150
type deploymentSelectorMustMatchCheck struct{}

func (c deploymentSelectorMustMatchCheck) ID() string {
	return "apps/deployment-selector-must-match"
}

func (c deploymentSelectorMustMatchCheck) Title() string {
	return "Selector Must Match Template Labels"
}

func (c deploymentSelectorMustMatchCheck) Category() string {
	return "apps"
}

func (c deploymentSelectorMustMatchCheck) Blocking() bool {
	return true
}

func (c deploymentSelectorMustMatchCheck) RenderSensitive() bool {
	return true
}

func (c deploymentSelectorMustMatchCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentSelectorMustMatchCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	selector, _, err := unstructured.NestedStringMap(selectorMap, "spec", "selector")
	if err != nil {
		return nil
	}

	templateLabels, _, err := unstructured.NestedStringMap(selectorMap, "spec", "template", "metadata", "labels")
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
					Kind:    "Deployment",
					Name:    name,
				},
			}}
		}
	}

	return nil
}

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

func (c deploymentSelectorInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	selectorMap2, found, _ := unstructured.NestedMap(selectorMap, "spec", "selector")
	if !found {
		return nil
	}

	// Check matchLabels keys (only string values in the selector map)
	for key, val := range selectorMap2 {
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
					Kind:    "Deployment",
					Name:    name,
				},
			}}
		}
	}

	// Check matchExpressions keys
	matchExpressionsList, found, err := unstructured.NestedSlice(selectorMap, "spec", "selector", "matchExpressions")
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
					Kind:    "Deployment",
					Name:    name,
				},
			}}
		}
	}

	return nil
}

// deploymentStrategyUndefinedCheck verifies strategy is defined.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentStrategyUndefinedCheck struct{}

func (c deploymentStrategyUndefinedCheck) ID() string {
	return "apps/deployment-strategy-undefined"
}

func (c deploymentStrategyUndefinedCheck) Title() string {
	return "Strategy Is Required"
}

func (c deploymentStrategyUndefinedCheck) Category() string {
	return "apps"
}

func (c deploymentStrategyUndefinedCheck) Blocking() bool {
	return true
}

func (c deploymentStrategyUndefinedCheck) RenderSensitive() bool {
	return true
}

func (c deploymentStrategyUndefinedCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentStrategyUndefinedCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	strategyMap, hasStrategy, _ := unstructured.NestedMap(selectorMap, "spec", "strategy")
	if hasStrategy {
		_ = strategyMap
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("strategy").String(),
			Message: "strategy is required",
			Kind:    "Deployment",
			Name:    name,
		},
	}}
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

func (c deploymentStrategyTypeInvalidCheck) DocSkipper() []string {
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

func (c deploymentReplicasInvalidCheck) DocSkipper() []string {
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

func (c deploymentMinReadySecondsInvalidCheck) DocSkipper() []string {
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

// deploymentMaxUnavailableInvalidCheck verifies maxUnavailable is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentMaxUnavailableInvalidCheck struct{}

func (c deploymentMaxUnavailableInvalidCheck) ID() string {
	return "apps/deployment-max-unavailable-invalid"
}

func (c deploymentMaxUnavailableInvalidCheck) Title() string {
	return "MaxUnavailable Must Be Valid"
}

func (c deploymentMaxUnavailableInvalidCheck) Category() string {
	return "apps"
}

func (c deploymentMaxUnavailableInvalidCheck) Blocking() bool {
	return true
}

func (c deploymentMaxUnavailableInvalidCheck) RenderSensitive() bool {
	return true
}

func (c deploymentMaxUnavailableInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentMaxUnavailableInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	ruMap, found, _ := unstructured.NestedMap(selectorMap, "spec", "strategy", "rollingUpdate")
	if !found {
		return nil
	}

	maxUnavailable, found := ruMap["maxUnavailable"]
	if !found {
		return nil
	}

	var maxUnavailableValue intstr.IntOrString
	switch v := maxUnavailable.(type) {
	case float64:
		maxUnavailableValue = intstr.FromInt(int(v))
	case string:
		maxUnavailableValue = intstr.FromString(v)
	default:
		return nil
	}

	if err := validateIntOrPositiveIntOrPercentage(maxUnavailableValue); err != nil {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("strategy").Child("rollingUpdate").Child("maxUnavailable").String(),
				Message: fmt.Sprintf("invalid maxUnavailable value: %s", err.Error()),
				Kind:    "Deployment",
				Name:    name,
				Value:   formatIntOrString(maxUnavailableValue),
			},
		}}
	}

	return nil
}

// deploymentMaxSurgeInvalidCheck verifies maxSurge is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type deploymentMaxSurgeInvalidCheck struct{}

func (c deploymentMaxSurgeInvalidCheck) ID() string {
	return "apps/deployment-max-surge-invalid"
}

func (c deploymentMaxSurgeInvalidCheck) Title() string {
	return "MaxSurge Must Be Valid"
}

func (c deploymentMaxSurgeInvalidCheck) Category() string {
	return "apps"
}

func (c deploymentMaxSurgeInvalidCheck) Blocking() bool {
	return true
}

func (c deploymentMaxSurgeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c deploymentMaxSurgeInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c deploymentMaxSurgeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	selectorMap, name := parseDeployment(data)
	if selectorMap == nil {
		return nil
	}

	ruMap, found, _ := unstructured.NestedMap(selectorMap, "spec", "strategy", "rollingUpdate")
	if !found {
		return nil
	}

	maxSurge, found := ruMap["maxSurge"]
	if !found {
		return nil
	}

	var maxSurgeValue intstr.IntOrString
	switch v := maxSurge.(type) {
	case float64:
		maxSurgeValue = intstr.FromInt(int(v))
	case string:
		maxSurgeValue = intstr.FromString(v)
	default:
		return nil
	}

	if err := validateIntOrPositiveIntOrPercentage(maxSurgeValue); err != nil {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("strategy").Child("rollingUpdate").Child("maxSurge").String(),
				Message: fmt.Sprintf("invalid maxSurge value: %s", err.Error()),
				Kind:    "Deployment",
				Name:    name,
				Value:   formatIntOrString(maxSurgeValue),
			},
		}}
	}

	return nil
}

// validateIntOrPositiveIntOrPercentage validates that an IntOrString field
// is either a positive integer or a valid percentage.
func validateIntOrPositiveIntOrPercentage(val intstr.IntOrString) error {
	switch val.Type {
	case intstr.Int:
		if val.IntVal <= 0 {
			return fmt.Errorf("must be a positive integer")
		}
	case intstr.String:
		if !strings.HasSuffix(val.StrVal, "%") {
			return fmt.Errorf("must be a positive integer or a percentage")
		}
		pct := strings.TrimSuffix(val.StrVal, "%")
		if _, err := strconv.Atoi(pct); err != nil {
			return fmt.Errorf("invalid percentage: %s", val.StrVal)
		}
		pctVal, _ := strconv.Atoi(pct)
		if pctVal <= 0 {
			return fmt.Errorf("must be a positive integer or a percentage")
		}
	default:
		return fmt.Errorf("must be a positive integer or a percentage")
	}
	return nil
}

// formatIntOrString formats an IntOrString for display.
func formatIntOrString(val intstr.IntOrString) string {
	if val.Type == intstr.String {
		return val.StrVal
	}
	return strconv.FormatInt(int64(val.IntVal), 10)
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
		deploymentSelectorMustMatchCheck{},
		deploymentSelectorInvalidCheck{},
		deploymentStrategyUndefinedCheck{},
		deploymentStrategyTypeInvalidCheck{},
		deploymentReplicasInvalidCheck{},
		deploymentMinReadySecondsInvalidCheck{},
		deploymentMaxUnavailableInvalidCheck{},
		deploymentMaxSurgeInvalidCheck{},
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
