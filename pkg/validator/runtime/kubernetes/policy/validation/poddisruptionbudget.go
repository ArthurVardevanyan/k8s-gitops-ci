package validation

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// podDisruptionBudgetSpecWrapper holds policy/v1.PodDisruptionBudgetSpec fields we need to validate.
type podDisruptionBudgetSpecWrapper struct {
	MinAvailable   interface{} `json:"minAvailable"`
	MaxUnavailable interface{} `json:"maxUnavailable"`
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
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Selector *metav1.LabelSelector `json:"selector"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(data, &pdb); err != nil {
		return nil
	}
	if pdb.Kind != "PodDisruptionBudget" {
		return nil
	}
	// Upstream tolerates a nil selector here.
	if pdb.Spec.Selector == nil {
		return nil
	}

	// Call the same apimachinery helper upstream calls, rather than
	// approximating it.
	//
	// This previously flattened the selector to a string and handed it to
	// labels.Parse. A string has no representation for matchExpressions, so
	// every operator/values rule was silently skipped: a selector whose only
	// error was in a matchExpression passed. It also accepted a bare string
	// selector, which is not a valid PodDisruptionBudget shape at all - the
	// test fixture used one, so the check was exercised only on input the
	// API server would already have rejected.
	//
	// AllowInvalidLabelValueInSelector is set because the API server sets it
	// when an object already carries such a value, and this tool cannot see
	// whether the object exists. Taking the permissive branch is the
	// standing policy for non-exemptable checks: a missed finding is
	// recoverable, an unsuppressible false positive is not.
	opts := metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}
	errs := metav1validation.ValidateLabelSelector(pdb.Spec.Selector, opts, field.NewPath("spec").Child("selector"))

	findings := make([]runtime.Finding, 0, len(errs))
	for _, err := range errs {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    err.Field,
				Message: "selector: " + err.ErrorBody(),
				Kind:    pdb.Kind,
				Name:    pdb.Metadata.Name,
			},
		})
	}
	return findings
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
	if err := yaml.Unmarshal(data, &pdb); err != nil {
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
	if err := yaml.Unmarshal(data, &pdb); err != nil {
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
