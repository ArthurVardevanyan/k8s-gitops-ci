package validation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var pvKinds = []string{"PersistentVolume"}

// accessModesInvalidCheck validates that accessModes contains only valid values.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvAccessModesInvalidCheck struct{}

func (c pvAccessModesInvalidCheck) ID() string {
	return "persistent-volume/access-modes-invalid"
}

func (c pvAccessModesInvalidCheck) Title() string {
	return "PersistentVolume Access Modes Must Be Valid"
}

func (c pvAccessModesInvalidCheck) Category() string {
	return "persistent-volume"
}

func (c pvAccessModesInvalidCheck) Blocking() bool {
	return true
}

func (c pvAccessModesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvAccessModesInvalidCheck) Kinds() []string {
	return pvKinds
}

func (c pvAccessModesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}

	return accessModesInvalidFindings(c, pv.Spec.AccessModes, "PersistentVolume", pv.GetName())
}

// capacityInvalidCheck validates that capacity specifies at least one resource.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvCapacityInvalidCheck struct{}

func (c pvCapacityInvalidCheck) ID() string {
	return "persistent-volume/capacity-invalid"
}

func (c pvCapacityInvalidCheck) Title() string {
	return "PersistentVolume Capacity Must Specify At Least One Resource"
}

func (c pvCapacityInvalidCheck) Category() string {
	return "persistent-volume"
}

func (c pvCapacityInvalidCheck) Blocking() bool {
	return true
}

func (c pvCapacityInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvCapacityInvalidCheck) Kinds() []string {
	return pvKinds
}

func (c pvCapacityInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if len(pv.Spec.Capacity) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("capacity").String(),
				Message: "capacity must specify at least one resource",
				Kind:    "PersistentVolume",
				Name:    pv.GetName(),
			},
		})
	}

	return findings
}
