package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var pvcKinds = []string{"PersistentVolumeClaim"}

// accessModesInvalidCheck validates that accessModes contains only valid values.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvcAccessModesInvalidCheck struct{}

func (c pvcAccessModesInvalidCheck) ID() string {
	return "persistent-volume-claim/access-modes-invalid"
}

func (c pvcAccessModesInvalidCheck) Title() string {
	return "PersistentVolumeClaim Access Modes Must Be Valid"
}

func (c pvcAccessModesInvalidCheck) Category() string {
	return "persistent-volume-claim"
}

func (c pvcAccessModesInvalidCheck) Blocking() bool {
	return true
}

func (c pvcAccessModesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvcAccessModesInvalidCheck) Kinds() []string {
	return pvcKinds
}

func (c pvcAccessModesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil
	}

	return accessModesInvalidFindings(c, pvc.Spec.AccessModes, "PersistentVolumeClaim", pvc.GetName())
}

// volumeModeInvalidCheck validates that volumeMode is one of Filesystem or Block.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvcVolumeModeInvalidCheck struct{}

func (c pvcVolumeModeInvalidCheck) ID() string {
	return "persistent-volume-claim/volume-mode-invalid"
}

func (c pvcVolumeModeInvalidCheck) Title() string {
	return "PersistentVolumeClaim Volume Mode Must Be Valid"
}

func (c pvcVolumeModeInvalidCheck) Category() string {
	return "persistent-volume-claim"
}

func (c pvcVolumeModeInvalidCheck) Blocking() bool {
	return true
}

func (c pvcVolumeModeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvcVolumeModeInvalidCheck) Kinds() []string {
	return pvcKinds
}

func (c pvcVolumeModeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validModes := map[corev1.PersistentVolumeMode]bool{
		corev1.PersistentVolumeFilesystem: true,
		corev1.PersistentVolumeBlock:      true,
	}

	if pvc.Spec.VolumeMode != nil && !validModes[*pvc.Spec.VolumeMode] {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("volumeMode").String(),
				Message: fmt.Sprintf("volumeMode: Unsupported value: %q", string(*pvc.Spec.VolumeMode)),
				Kind:    "PersistentVolumeClaim",
				Name:    pvc.GetName(),
				Value:   string(*pvc.Spec.VolumeMode),
			},
		})
	}

	return findings
}
