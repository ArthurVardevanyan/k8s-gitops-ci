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
func (c nameInvalidCheck) DocSkipper() []string  { return ingressClassKinds }

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

// controllerInvalidCheck validates that the IngressClass controller field is a valid domain path.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type controllerInvalidCheck struct{}

func (c controllerInvalidCheck) ID() string { return "networking/ingressclass-controller-invalid" }

func (c controllerInvalidCheck) Title() string         { return "IngressClass Controller Must Be Valid" }
func (c controllerInvalidCheck) Category() string      { return "networking" }
func (c controllerInvalidCheck) Blocking() bool        { return true }
func (c controllerInvalidCheck) RenderSensitive() bool { return true }
func (c controllerInvalidCheck) DocSkipper() []string  { return ingressClassKinds }

func (c controllerInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ic networkingv1.IngressClass
	if yaml.Unmarshal(data, &ic) != nil {
		return nil
	}

	var findings []runtime.Finding
	controller := ic.Spec.Controller

	if controller == "" {
		return nil
	}

	domain := controller
	pathPart := ""
	if idx := strings.Index(controller, "/"); idx > 0 {
		domain = controller[:idx]
		pathPart = controller[idx+1:]
	}

	var allErrs []string
	if errs := validation.IsDNS1123Subdomain(domain); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(pathPart) > 0 {
		if errs := validation.IsDNS1123Subdomain(pathPart); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if len(allErrs) > 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("controller").String(),
				Message: fmt.Sprintf("controller: invalid value %q: %s", controller, strings.Join(allErrs, "; ")),
				Kind:    "IngressClass",
				Name:    ic.GetName(),
			},
		})
	}

	return findings
}

// parametersInvalidCheck validates that the IngressClass parameters reference is valid.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type parametersInvalidCheck struct{}

func (c parametersInvalidCheck) ID() string { return "networking/ingressclass-parameters-invalid" }
func (c parametersInvalidCheck) Title() string {
	return "IngressClass Parameters Reference Must Be Valid"
}
func (c parametersInvalidCheck) Category() string      { return "networking" }
func (c parametersInvalidCheck) Blocking() bool        { return true }
func (c parametersInvalidCheck) RenderSensitive() bool { return true }
func (c parametersInvalidCheck) DocSkipper() []string  { return ingressClassKinds }

func (c parametersInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ic networkingv1.IngressClass
	if err := yaml.Unmarshal(data, &ic); err != nil {
		return nil
	}

	var findings []runtime.Finding
	params := ic.Spec.Parameters

	if params == nil || params.Name == "" {
		return nil
	}

	// Parameter reference must be in namespace/name format
	refName := params.Name
	var namePart string
	var nsPart string

	if strings.Contains(refName, "/") {
		parts := strings.SplitN(refName, "/", 2)
		nsPart = parts[0]
		namePart = parts[1]
	} else {
		namePart = refName
	}

	var allErrs []string
	if len(namePart) == 0 {
		allErrs = append(allErrs, "name is required")
	} else {
		if errs := validation.IsDNS1123Subdomain(namePart); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	nsVal := ""
	if params.Namespace != nil {
		nsVal = *params.Namespace
	}
	if len(nsPart) > 0 || nsVal != "" {
		ns := nsPart
		if ns == "" {
			ns = nsVal
		}
		if errs := validation.IsDNS1123Subdomain(ns); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if len(allErrs) > 0 {
		path := field.NewPath("spec").Child("parameters")
		nsHasValue := params.Namespace != nil && *params.Namespace != ""
		if nsHasValue || strings.Contains(refName, "/") {
			path = path.Child("namespace")
		}
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    path.String(),
				Message: fmt.Sprintf("parameters: invalid value %q: %s", refName, strings.Join(allErrs, "; ")),
				Kind:    "IngressClass",
				Name:    ic.GetName(),
			},
		})
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		nameInvalidCheck{},
		controllerInvalidCheck{},
		parametersInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
