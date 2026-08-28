package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var namespaceKinds = []string{"Namespace"}

var wellKnownFinalizers = map[string]bool{
	"kubernetes": true,
}

type namespaceNameInvalidCheck struct{}

func (c namespaceNameInvalidCheck) ID() string            { return "core/namespace-name-invalid" }
func (c namespaceNameInvalidCheck) Title() string         { return "Namespace Name Must Be Valid DNS Label" }
func (c namespaceNameInvalidCheck) Category() string      { return "core" }
func (c namespaceNameInvalidCheck) Blocking() bool        { return true }
func (c namespaceNameInvalidCheck) RenderSensitive() bool { return true }
func (c namespaceNameInvalidCheck) DocSkipper() []string  { return namespaceKinds }

func (c namespaceNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "Namespace" {
		return nil
	}
	var ns corev1.Namespace
	if err := yaml.Unmarshal(data, &ns); err != nil {
		return nil
	}
	name := ns.GetName()
	if name == "" {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "metadata.name",
				Message: "namespace: metadata.name is required",
				Kind:    "Namespace",
			},
		}}
	}
	if errors := validation.IsDNS1123Label(name); len(errors) > 0 {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "metadata.name",
				Message: fmt.Sprintf("namespace %q: %s", name, errors[0]),
				Kind:    "Namespace",
				Name:    name,
			},
		}}
	}
	return nil
}

type namespaceFinalizersInvalidCheck struct{}

func (c namespaceFinalizersInvalidCheck) ID() string { return "core/namespace-finalizers-invalid" }
func (c namespaceFinalizersInvalidCheck) Title() string {
	return "Namespace Finalizers Must Be Well-Known"
}
func (c namespaceFinalizersInvalidCheck) Category() string      { return "core" }
func (c namespaceFinalizersInvalidCheck) Blocking() bool        { return true }
func (c namespaceFinalizersInvalidCheck) RenderSensitive() bool { return true }
func (c namespaceFinalizersInvalidCheck) DocSkipper() []string  { return namespaceKinds }

func (c namespaceFinalizersInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "Namespace" {
		return nil
	}
	var ns corev1.Namespace
	if err := yaml.Unmarshal(data, &ns); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for i, finalizer := range ns.Finalizers {
		if !wellKnownFinalizers[finalizer] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("finalizers").Index(i).String(),
					Message: fmt.Sprintf("finalizers[%d]: unknown finalizer %q", i, finalizer),
					Kind:    "Namespace",
					Name:    ns.GetName(),
					Value:   finalizer,
				},
			})
		}
	}
	return findings
}

func init() {
	checks := []runtime.Check{
		namespaceNameInvalidCheck{},
		namespaceFinalizersInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
