package validation

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var ingressClassKinds = []string{"IngressClass"}

// nameInvalidCheck validates that the IngressClass name is not empty.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type nameInvalidCheck struct{}

func (c nameInvalidCheck) ID() string            { return "networking/ingressclass-name-invalid" }
func (c nameInvalidCheck) Title() string         { return "IngressClass Name Must Be Valid" }
func (c nameInvalidCheck) Category() string      { return "networking" }
func (c nameInvalidCheck) Blocking() bool        { return true }
func (c nameInvalidCheck) RenderSensitive() bool { return true }
func (c nameInvalidCheck) Kinds() []string       { return ingressClassKinds }

func (c nameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ic networkingv1.IngressClass
	if err := yaml.Unmarshal(data, &ic); err != nil {
		return nil
	}

	var findings []runtime.Finding
	name := ic.GetName()

	if len(name) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: "name: required",
				Kind:    "IngressClass",
				Name:    name,
			},
		})
		return findings
	}

	// Validate the name as a DNS subdomain
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: fmt.Sprintf("name: invalid value %q: %s", name, strings.Join(errs, "; ")),
				Kind:    "IngressClass",
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
