package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// volumeModeInvalidFindings validates that volumeMode is one of Filesystem or
// Block. PersistentVolume and PersistentVolumeClaim share this validation, in
// two different upstream functions that apply the same supportedVolumeModes
// set: ValidatePersistentVolumeSpec and ValidatePersistentVolumeClaimSpec.
//
// A nil mode is not reported. Defaulting is guarded on nil, so an absent
// volumeMode is filled in with Filesystem before validation; an explicitly
// empty one survives as a non-nil pointer to "" and is rejected here, which
// is what the API server does.
//
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
func volumeModeInvalidFindings(c runtime.Check, mode *corev1.PersistentVolumeMode, kind, name string) []runtime.Finding {
	if mode == nil {
		return nil
	}

	validModes := map[corev1.PersistentVolumeMode]bool{
		corev1.PersistentVolumeFilesystem: true,
		corev1.PersistentVolumeBlock:      true,
	}
	if validModes[*mode] {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("volumeMode").String(),
			Message: fmt.Sprintf("volumeMode: Unsupported value: %q", string(*mode)),
			Kind:    kind,
			Name:    name,
			Value:   string(*mode),
		},
	}}
}
