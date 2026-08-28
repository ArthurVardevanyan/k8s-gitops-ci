package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var namespaceKinds = []string{"Namespace"}

type namespaceNameInvalidCheck struct{}

func (c namespaceNameInvalidCheck) ID() string            { return "core/namespace-name-invalid" }
func (c namespaceNameInvalidCheck) Title() string         { return "Namespace Name Must Be Valid DNS Label" }
func (c namespaceNameInvalidCheck) Category() string      { return "core" }
func (c namespaceNameInvalidCheck) Blocking() bool        { return true }
func (c namespaceNameInvalidCheck) RenderSensitive() bool { return true }
func (c namespaceNameInvalidCheck) Kinds() []string       { return namespaceKinds }

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

func init() {
	checks := []runtime.Check{
		namespaceNameInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
