package validation

import (
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// podDisruptionBudgetSpecWrapper holds policy/v1.PodDisruptionBudgetSpec fields we need to validate.
type podDisruptionBudgetSpecWrapper struct {
	MinAvailable   interface{}       `json:"minAvailable"`
	MaxUnavailable interface{}       `json:"maxUnavailable"`
	Selector       interface{}       `json:"selector"`
	TemplateLabels map[string]string `json:"labels"`
}

// selectorInvalidCheck validates that the PDB selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type selectorInvalidCheck struct{}

func (c selectorInvalidCheck) ID() string            { return "policy/selector-invalid" }
func (c selectorInvalidCheck) Title() string         { return "PDB Selector Must Be A Valid Label Selector" }
func (c selectorInvalidCheck) Category() string      { return "policy" }
func (c selectorInvalidCheck) Blocking() bool        { return true }
func (c selectorInvalidCheck) RenderSensitive() bool { return true }
func (c selectorInvalidCheck) DocSkipper() []string  { return []string{"PodDisruptionBudget"} }

func (c selectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pdb struct {
		Kind string                         `json:"kind"`
		Spec podDisruptionBudgetSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &pdb); err != nil {
		return nil
	}
	if pdb.Kind != "PodDisruptionBudget" {
		return nil
	}

	selector := extractSelectorString(pdb.Spec.Selector)
	if selector == "" {
		return nil
	}
	if _, err := labels.Parse(selector); err != nil {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("selector").String(),
				Message: "selector: invalid label selector",
				Kind:    pdb.Kind,
				Extra:   map[string]string{"selector": selector},
			},
		}}
	}
	return nil
}

// minAvailableInvalidCheck validates that minAvailable >= 0.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type minAvailableInvalidCheck struct{}

func (c minAvailableInvalidCheck) ID() string            { return "policy/min-available-invalid" }
func (c minAvailableInvalidCheck) Title() string         { return "PDB minAvailable Must Be >= 0" }
func (c minAvailableInvalidCheck) Category() string      { return "policy" }
func (c minAvailableInvalidCheck) Blocking() bool        { return true }
func (c minAvailableInvalidCheck) RenderSensitive() bool { return true }
func (c minAvailableInvalidCheck) DocSkipper() []string  { return []string{"PodDisruptionBudget"} }

func (c minAvailableInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pdb struct {
		Kind string                         `json:"kind"`
		Spec podDisruptionBudgetSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &pdb); err != nil {
		return nil
	}
	if pdb.Kind != "PodDisruptionBudget" {
		return nil
	}
	av, ok := intOrStringFromInterface(pdb.Spec.MinAvailable)
	if !ok {
		return nil
	}
	if av.IntVal >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("minAvailable").String(),
			Message: "minAvailable: must be >= 0",
			Kind:    pdb.Kind,
			Extra:   map[string]string{"minAvailable": strconv.Itoa(int(av.IntVal))},
		},
	}}
}

// maxUnavailableInvalidCheck validates that maxUnavailable >= 0.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
//
//nolint:dupl // structurally identical to minAvailableInvalidCheck for a different field
type maxUnavailableInvalidCheck struct{}

func (c maxUnavailableInvalidCheck) ID() string { return "policy/max-unavailable-invalid" }

func (c maxUnavailableInvalidCheck) Title() string {
	return "PDB maxUnavailable Must Be >= 0"
}
func (c maxUnavailableInvalidCheck) Category() string      { return "policy" }
func (c maxUnavailableInvalidCheck) Blocking() bool        { return true }
func (c maxUnavailableInvalidCheck) RenderSensitive() bool { return true }
func (c maxUnavailableInvalidCheck) DocSkipper() []string {
	return []string{"PodDisruptionBudget"}
}

func (c maxUnavailableInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pdb struct {
		Kind string                         `json:"kind"`
		Spec podDisruptionBudgetSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &pdb); err != nil {
		return nil
	}
	if pdb.Kind != "PodDisruptionBudget" {
		return nil
	}
	av, ok := intOrStringFromInterface(pdb.Spec.MaxUnavailable)
	if !ok {
		return nil
	}
	if av.IntVal >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("maxUnavailable").String(),
			Message: "maxUnavailable: must be >= 0",
			Kind:    pdb.Kind,
			Extra:   map[string]string{"maxUnavailable": strconv.Itoa(int(av.IntVal))},
		},
	}}
}

// minAndMaxSpecifiedCheck validates that minAvailable and maxUnavailable
// cannot both be specified.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type minAndMaxSpecifiedCheck struct{}

func (c minAndMaxSpecifiedCheck) ID() string            { return "policy/min-and-max-specified" }
func (c minAndMaxSpecifiedCheck) Title() string         { return "PDB Must Specify Only One Disruption Target" }
func (c minAndMaxSpecifiedCheck) Category() string      { return "policy" }
func (c minAndMaxSpecifiedCheck) Blocking() bool        { return true }
func (c minAndMaxSpecifiedCheck) RenderSensitive() bool { return true }
func (c minAndMaxSpecifiedCheck) DocSkipper() []string  { return []string{"PodDisruptionBudget"} }

func (c minAndMaxSpecifiedCheck) Run(data []byte, source string) []runtime.Finding {
	var pdb struct {
		Kind string                         `json:"kind"`
		Spec podDisruptionBudgetSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &pdb); err != nil {
		return nil
	}
	if pdb.Kind != "PodDisruptionBudget" {
		return nil
	}
	_, minOk := intOrStringFromInterface(pdb.Spec.MinAvailable)
	_, maxOk := intOrStringFromInterface(pdb.Spec.MaxUnavailable)
	if !minOk || !maxOk {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").String(),
			Message: "minAvailable and maxUnavailable cannot both be specified",
			Kind:    pdb.Kind,
		},
	}}
}

// selectorAndPodTemplateHashInvalidCheck validates that the PDB selector
// does not match the pod template labels. If the selector matches the
// pod template labels, the PDB will be unable to select any existing pods
// because the selector targets pods that no longer exist.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type selectorAndPodTemplateHashInvalidCheck struct{}

func (c selectorAndPodTemplateHashInvalidCheck) ID() string {
	return "policy/selector-and-pod-template-hash-invalid"
}

func (c selectorAndPodTemplateHashInvalidCheck) Title() string {
	return "PDB Selector Must Not Match Pod Template Labels"
}
func (c selectorAndPodTemplateHashInvalidCheck) Category() string      { return "policy" }
func (c selectorAndPodTemplateHashInvalidCheck) Blocking() bool        { return true }
func (c selectorAndPodTemplateHashInvalidCheck) RenderSensitive() bool { return true }
func (c selectorAndPodTemplateHashInvalidCheck) DocSkipper() []string {
	return []string{"PodDisruptionBudget"}
}

func (c selectorAndPodTemplateHashInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pdb struct {
		Kind string                         `json:"kind"`
		Spec podDisruptionBudgetSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &pdb); err != nil {
		return nil
	}
	if pdb.Kind != "PodDisruptionBudget" {
		return nil
	}

	selectorStr := extractSelectorString(pdb.Spec.Selector)
	if selectorStr == "" {
		return nil
	}

	selector, err := labels.Parse(selectorStr)
	if err != nil {
		// Skip if selector is invalid — that's caught by selector-invalid check
		return nil
	}

	templateLabels := pdb.Spec.TemplateLabels
	if len(templateLabels) == 0 {
		return nil
	}

	// Check if the PDB selector could select pods with these template labels.
	// If the selector matches the pod template's labels, the PDB cannot
	// select any existing pods since the template labels define new pods.
	if selector.Matches(labels.Set(templateLabels)) {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("selector").String(),
				Message: "selector should not match pod template labels",
				Kind:    pdb.Kind,
				Extra:   map[string]string{"selector": selectorStr},
			},
		}}
	}
	return nil
}

// intOrStringFromInterface extracts an intstr.IntOrString from the
// minAvailable/maxUnavailable interface{} value. PDB accepts either a
// positive integer or a valid percentage string.
type intOrStringValue struct {
	IntVal int32
}

func intOrStringFromInterface(v interface{}) (result intOrStringValue, ok bool) {
	if v == nil {
		return result, false
	}
	switch val := v.(type) {
	case float64:
		return intOrStringValue{IntVal: int32(val)}, true
	case string:
		// Percentage strings like "50%" are valid — treat as non-negative int
		// by returning a zero IntVal (we only check for negative).
		return intOrStringValue{IntVal: 0}, true
	default:
		return result, false
	}
}

// extractSelectorString normalizes the PDB selector field to a label
// selector string. The selector can be:
//   - a string (e.g. "app=myapp")
//   - an object with matchLabels (e.g. {"matchLabels":{"app":"myapp"}})
func extractSelectorString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}:
		if ml, ok := val["matchLabels"].(map[string]interface{}); ok {
			labelMap := make(map[string]string)
			for k, v := range ml {
				labelMap[k] = fmt.Sprintf("%v", v)
			}
			return labels.FormatLabels(labels.Set(labelMap))
		}
	case map[string]string:
		return labels.FormatLabels(labels.Set(val))
	}
	return ""
}

// Register registers all PodDisruptionBudget validation checks with the
// check registry.
func Register() {
	checks := []runtime.Check{
		selectorInvalidCheck{},
		minAvailableInvalidCheck{},
		maxUnavailableInvalidCheck{},
		minAndMaxSpecifiedCheck{},
		selectorAndPodTemplateHashInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
