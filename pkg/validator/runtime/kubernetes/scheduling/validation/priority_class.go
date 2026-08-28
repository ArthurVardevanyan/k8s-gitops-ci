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
func (c nameInvalidCheck) Kinds() []string       { return priorityClassKinds }

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

func init() {
	checks := []runtime.Check{
		nameInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
