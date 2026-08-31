package storage

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

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

	return volumeModeInvalidFindings(c, pvc.Spec.VolumeMode, "PersistentVolumeClaim", pvc.GetName())
}
