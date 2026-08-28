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

// replicaSetSelectorMustMatchCheck verifies selector matches template labels.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetSelectorMustMatchCheck struct{}

func (c replicaSetSelectorMustMatchCheck) ID() string {
	return "apps/replicaset-selector-must-match"
}

func (c replicaSetSelectorMustMatchCheck) Title() string {
	return "Selector Must Match Template Labels"
}

func (c replicaSetSelectorMustMatchCheck) Category() string {
	return "apps"
}

func (c replicaSetSelectorMustMatchCheck) Blocking() bool {
	return true
}

func (c replicaSetSelectorMustMatchCheck) RenderSensitive() bool {
	return true
}

func (c replicaSetSelectorMustMatchCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c replicaSetSelectorMustMatchCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseReplicaSet(data)
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
					Kind:    "ReplicaSet",
					Name:    name,
				},
			}}
		}
	}

	return nil
}

// replicaSetSelectorInvalidCheck verifies selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetSelectorInvalidCheck struct{}

func (c replicaSetSelectorInvalidCheck) ID() string {
	return "apps/replicaset-selector-invalid"
}

func (c replicaSetSelectorInvalidCheck) Title() string {
	return "Selector Must Be A Valid Label Selector"
}

func (c replicaSetSelectorInvalidCheck) Category() string {
	return "apps"
}

func (c replicaSetSelectorInvalidCheck) Blocking() bool {
	return true
}

func (c replicaSetSelectorInvalidCheck) RenderSensitive() bool {
	return true
}

func (c replicaSetSelectorInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c replicaSetSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseReplicaSet(data)
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
					Kind:    "ReplicaSet",
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
					Kind:    "ReplicaSet",
					Name:    name,
				},
			}}
		}
	}

	return nil
}

// replicaSetReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetReplicasInvalidCheck struct{}

func (c replicaSetReplicasInvalidCheck) ID() string {
	return "apps/replicaset-replicas-invalid"
}

func (c replicaSetReplicasInvalidCheck) Title() string {
	return "Replicas Must Be >= 0"
}

func (c replicaSetReplicasInvalidCheck) Category() string {
	return "apps"
}

func (c replicaSetReplicasInvalidCheck) Blocking() bool {
	return true
}

func (c replicaSetReplicasInvalidCheck) RenderSensitive() bool {
	return true
}

func (c replicaSetReplicasInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c replicaSetReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseReplicaSet(data)
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
			Kind:    "ReplicaSet",
			Name:    name,
			Value:   fmt.Sprintf("%d", replicas),
		},
	}}
}

// replicaSetRestartPolicyInvalidCheck verifies restartPolicy is valid.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetRestartPolicyInvalidCheck struct{}

func (c replicaSetRestartPolicyInvalidCheck) ID() string {
	return "apps/replicaset-restart-policy-invalid"
}

func (c replicaSetRestartPolicyInvalidCheck) Title() string {
	return "RestartPolicy Must Be Valid"
}

func (c replicaSetRestartPolicyInvalidCheck) Category() string {
	return "apps"
}

func (c replicaSetRestartPolicyInvalidCheck) Blocking() bool {
	return true
}

func (c replicaSetRestartPolicyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c replicaSetRestartPolicyInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c replicaSetRestartPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseReplicaSet(data)
	if specMap == nil {
		return nil
	}

	// RestartPolicy is on the pod template spec
	restartPolicy, found, _ := unstructured.NestedString(specMap, "spec", "template", "spec", "restartPolicy")
	if !found {
		return nil
	}

	if restartPolicy == "Always" || restartPolicy == "OnFailure" || restartPolicy == "Never" {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("template").Child("spec").Child("restartPolicy").String(),
			Message: fmt.Sprintf("restartPolicy: Unsupported value: %q: supported values: \"Always\", \"OnFailure\", \"Never\"", restartPolicy),
			Kind:    "ReplicaSet",
			Name:    name,
			Value:   restartPolicy,
		},
	}}
}

// parseReplicaSet parses data as a ReplicaSet resource.
func parseReplicaSet(data []byte) (specMap map[string]interface{}, name string) {
	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, ""
	}

	kind := nestedString(obj, "kind")
	if kind != "ReplicaSet" {
		return nil, ""
	}

	name = nestedString(obj, "metadata", "name")
	specMap = obj

	return specMap, name
}

// ValidateReplicaSet runs all replicaset validation checks and returns findings.
func ValidateReplicaSet(data []byte, source string) []runtime.Finding {
	checks := []runtime.Check{
		replicaSetSelectorMustMatchCheck{},
		replicaSetSelectorInvalidCheck{},
		replicaSetReplicasInvalidCheck{},
		replicaSetRestartPolicyInvalidCheck{},
	}
	findings := make([]runtime.Finding, 0, len(checks))
	for _, c := range checks {
		findings = append(findings, c.Run(data, source)...)
	}
	return findings
}

// init registers all replicaset checks.
func init() {
	Register()
}
