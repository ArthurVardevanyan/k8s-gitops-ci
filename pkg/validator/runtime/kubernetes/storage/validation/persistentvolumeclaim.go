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
type pvcAccessModesInvalidCheck struct{ runtime.Meta }

func newPvcAccessModesInvalidCheck() pvcAccessModesInvalidCheck {
	return pvcAccessModesInvalidCheck{runtime.Meta{
		RuleID:    "persistent-volume-claim/access-modes-invalid",
		RuleTitle: "PersistentVolumeClaim Access Modes Must Be Valid",
		AppliesTo: pvcKinds,
	}}
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
type pvcVolumeModeInvalidCheck struct{ runtime.Meta }

func newPvcVolumeModeInvalidCheck() pvcVolumeModeInvalidCheck {
	return pvcVolumeModeInvalidCheck{runtime.Meta{
		RuleID:    "persistent-volume-claim/volume-mode-invalid",
		RuleTitle: "PersistentVolumeClaim Volume Mode Must Be Valid",
		AppliesTo: pvcKinds,
	}}
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
