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
	MinAvailable   interface{} `json:"minAvailable"`
	MaxUnavailable interface{} `json:"maxUnavailable"`
	Selector       interface{} `json:"selector"`
}

// selectorInvalidCheck validates that the PDB selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type selectorInvalidCheck struct{}

func (c selectorInvalidCheck) ID() string            { return "policy/selector-invalid" }
func (c selectorInvalidCheck) Title() string         { return "PDB Selector Must Be A Valid Label Selector" }
func (c selectorInvalidCheck) Category() string      { return "policy" }
func (c selectorInvalidCheck) Blocking() bool        { return true }
func (c selectorInvalidCheck) RenderSensitive() bool { return true }
func (c selectorInvalidCheck) Kinds() []string       { return []string{"PodDisruptionBudget"} }

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

// pdbNonNegativeFindings validates that the named PodDisruptionBudget spec
// field, an IntOrString, is not negative. minAvailable and maxUnavailable
// share this logic.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
func pdbNonNegativeFindings(c runtime.Check, data []byte, fieldName string, value func(podDisruptionBudgetSpecWrapper) interface{}) []runtime.Finding {
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
	av, ok := intOrStringFromInterface(value(pdb.Spec))
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
			Path:    field.NewPath("spec").Child(fieldName).String(),
			Message: fieldName + ": must be >= 0",
			Kind:    pdb.Kind,
			Extra:   map[string]string{fieldName: strconv.Itoa(int(av.IntVal))},
		},
	}}
}

// minAvailableInvalidCheck validates that minAvailable >= 0.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type minAvailableInvalidCheck struct{}

func (c minAvailableInvalidCheck) ID() string            { return "policy/min-available-invalid" }
func (c minAvailableInvalidCheck) Title() string         { return "PDB minAvailable Must Be >= 0" }
func (c minAvailableInvalidCheck) Category() string      { return "policy" }
func (c minAvailableInvalidCheck) Blocking() bool        { return true }
func (c minAvailableInvalidCheck) RenderSensitive() bool { return true }
func (c minAvailableInvalidCheck) Kinds() []string       { return []string{"PodDisruptionBudget"} }

func (c minAvailableInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return pdbNonNegativeFindings(c, data, "minAvailable", func(spec podDisruptionBudgetSpecWrapper) interface{} {
		return spec.MinAvailable
	})
}

// maxUnavailableInvalidCheck validates that maxUnavailable >= 0.
// Source: k8s.io/kubernetes/pkg/apis/policy/validation/validation.go
type maxUnavailableInvalidCheck struct{}

func (c maxUnavailableInvalidCheck) ID() string { return "policy/max-unavailable-invalid" }

func (c maxUnavailableInvalidCheck) Title() string {
	return "PDB maxUnavailable Must Be >= 0"
}
func (c maxUnavailableInvalidCheck) Category() string      { return "policy" }
func (c maxUnavailableInvalidCheck) Blocking() bool        { return true }
func (c maxUnavailableInvalidCheck) RenderSensitive() bool { return true }
func (c maxUnavailableInvalidCheck) Kinds() []string {
	return []string{"PodDisruptionBudget"}
}

func (c maxUnavailableInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return pdbNonNegativeFindings(c, data, "maxUnavailable", func(spec podDisruptionBudgetSpecWrapper) interface{} {
		return spec.MaxUnavailable
	})
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
func (c minAndMaxSpecifiedCheck) Kinds() []string       { return []string{"PodDisruptionBudget"} }

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
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
func Register() {
	checks := []runtime.Check{
		selectorInvalidCheck{},
		minAvailableInvalidCheck{},
		maxUnavailableInvalidCheck{},
		minAndMaxSpecifiedCheck{},
	}

	runtime.RegisterAll(checks, upstreamRefs)
}
