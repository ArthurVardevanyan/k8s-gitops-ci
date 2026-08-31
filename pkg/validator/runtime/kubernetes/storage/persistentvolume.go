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
type pvAccessModesInvalidCheck struct{ runtime.Meta }

func newPvAccessModesInvalidCheck() pvAccessModesInvalidCheck {
	return pvAccessModesInvalidCheck{runtime.Meta{
		RuleID:    "persistent-volume/access-modes-invalid",
		RuleTitle: "PersistentVolume Access Modes Must Be Valid",
		AppliesTo: pvKinds,
	}}
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
type pvCapacityInvalidCheck struct{ runtime.Meta }

func newPvCapacityInvalidCheck() pvCapacityInvalidCheck {
	return pvCapacityInvalidCheck{runtime.Meta{
		RuleID:    "persistent-volume/capacity-invalid",
		RuleTitle: "PersistentVolume Capacity Must Specify At Least One Resource",
		AppliesTo: pvKinds,
	}}
}

func (c pvCapacityInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}
	if pv.Kind != "PersistentVolume" {
		return nil
	}

	var findings []runtime.Finding

	if len(pv.Spec.Capacity) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
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

// pvVolumeModeInvalidCheck validates that volumeMode is one of Filesystem or
// Block. PersistentVolumeClaim has had this rule from the start; upstream
// applies the same supportedVolumeModes set to a PersistentVolume in
// ValidatePersistentVolumeSpec, so the kind is validated the same way.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvVolumeModeInvalidCheck struct{ runtime.Meta }

func newPvVolumeModeInvalidCheck() pvVolumeModeInvalidCheck {
	return pvVolumeModeInvalidCheck{runtime.Meta{
		RuleID:    "persistent-volume/volume-mode-invalid",
		RuleTitle: "PersistentVolume Volume Mode Must Be Valid",
		AppliesTo: pvKinds,
	}}
}

func (c pvVolumeModeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}

	return volumeModeInvalidFindings(c, pv.Spec.VolumeMode, "PersistentVolume", pv.GetName())
}
