package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// accessModesInvalidFindings validates that accessModes contains only valid
// values. PersistentVolume and PersistentVolumeClaim share this validation.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
func accessModesInvalidFindings(c runtime.Check, modes []corev1.PersistentVolumeAccessMode, kind, name string) []runtime.Finding {
	var findings []runtime.Finding
	validModes := map[corev1.PersistentVolumeAccessMode]bool{
		corev1.ReadOnlyMany:     true,
		corev1.ReadWriteMany:    true,
		corev1.ReadWriteOnce:    true,
		corev1.ReadWriteOncePod: true,
	}

	for i, mode := range modes {
		if !validModes[mode] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("accessModes").Index(i).String(),
					Message: fmt.Sprintf("accessModes: Unsupported value: %q", string(mode)),
					Kind:    kind,
					Name:    name,
				},
			})
		}
	}

	return findings
}
