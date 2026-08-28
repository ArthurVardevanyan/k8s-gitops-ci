package validation

import (
	"fmt"

	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var priorityClassKinds = []string{"PriorityClass"}

// nameInvalidCheck validates that the PriorityClass name is a valid DNS subdomain.
// Source: k8s.io/kubernetes/pkg/apis/scheduling/validation/validation.go
type nameInvalidCheck struct{}

func (c nameInvalidCheck) ID() string            { return "scheduling/priorityclass-name-invalid" }
func (c nameInvalidCheck) Title() string         { return "PriorityClass Name Must Be Valid DNS Subdomain" }
func (c nameInvalidCheck) Category() string      { return "scheduling" }
func (c nameInvalidCheck) Blocking() bool        { return true }
func (c nameInvalidCheck) RenderSensitive() bool { return true }
func (c nameInvalidCheck) DocSkipper() []string  { return priorityClassKinds }

func (c nameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pc schedulingv1.PriorityClass
	if yaml.Unmarshal(data, &pc) != nil {
		return nil
	}

	var findings []runtime.Finding
	name := pc.GetName()

	if len(name) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: "name: required",
				Kind:    "PriorityClass",
				Name:    name,
			},
		})
		return findings
	}

	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: fmt.Sprintf("name: invalid value %q: %s", name, errs[0]),
				Kind:    "PriorityClass",
				Name:    name,
			},
		})
	}

	return findings
}

// valueInvalidCheck validates that the PriorityClass value is a valid integer.
// Source: k8s.io/kubernetes/pkg/apis/scheduling/validation/validation.go
type valueInvalidCheck struct{}

func (c valueInvalidCheck) ID() string { return "scheduling/priorityclass-value-invalid" }

func (c valueInvalidCheck) Title() string         { return "PriorityClass Value Must Be Valid Integer" }
func (c valueInvalidCheck) Category() string      { return "scheduling" }
func (c valueInvalidCheck) Blocking() bool        { return true }
func (c valueInvalidCheck) RenderSensitive() bool { return true }
func (c valueInvalidCheck) DocSkipper() []string  { return priorityClassKinds }

func (c valueInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pc schedulingv1.PriorityClass
	if yaml.Unmarshal(data, &pc) != nil {
		return nil
	}

	var findings []runtime.Finding
	value := pc.Value

	// Value must be >= 0
	if value < 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("value").String(),
				Message: fmt.Sprintf("value: invalid value %d: must be >= 0", value),
				Kind:    "PriorityClass",
				Name:    pc.GetName(),
			},
		})
	}

	return findings
}

// globalDefaultInvalidCheck validates that globalDefault=true with value=0 is invalid.
// Source: k8s.io/kubernetes/pkg/apis/scheduling/validation/validation.go
type globalDefaultInvalidCheck struct{}

func (c globalDefaultInvalidCheck) ID() string {
	return "scheduling/priorityclass-global-default-invalid"
}

func (c globalDefaultInvalidCheck) Title() string {
	return "PriorityClass globalDefault Must Not Be True With Zero Value"
}
func (c globalDefaultInvalidCheck) Category() string      { return "scheduling" }
func (c globalDefaultInvalidCheck) Blocking() bool        { return true }
func (c globalDefaultInvalidCheck) RenderSensitive() bool { return true }
func (c globalDefaultInvalidCheck) DocSkipper() []string  { return priorityClassKinds }

func (c globalDefaultInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pc schedulingv1.PriorityClass
	if yaml.Unmarshal(data, &pc) != nil {
		return nil
	}

	var findings []runtime.Finding

	if pc.GlobalDefault && pc.Value == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("globalDefault").String(),
				Message: "globalDefault must not be true when value is 0",
				Kind:    "PriorityClass",
				Name:    pc.GetName(),
			},
		})
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		nameInvalidCheck{},
		valueInvalidCheck{},
		globalDefaultInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
