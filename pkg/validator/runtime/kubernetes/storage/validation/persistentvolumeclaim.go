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

func (c pvcAccessModesInvalidCheck) DocSkipper() []string {
	return pvcKinds
}

func (c pvcAccessModesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validModes := map[corev1.PersistentVolumeAccessMode]bool{
		corev1.ReadOnlyMany:     true,
		corev1.ReadWriteMany:    true,
		corev1.ReadWriteOnce:    true,
		corev1.ReadWriteOncePod: true,
	}

	for i, mode := range pvc.Spec.AccessModes {
		if !validModes[mode] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("accessModes").Index(i).String(),
					Message: fmt.Sprintf("accessModes: Unsupported value: %q", string(mode)),
					Kind:    "PersistentVolumeClaim",
					Name:    pvc.GetName(),
				},
			})
		}
	}

	return findings
}

// storageClassInvalidCheck validates that storageClassName is a valid name.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvcStorageClassInvalidCheck struct{}

func (c pvcStorageClassInvalidCheck) ID() string {
	return "persistent-volume-claim/storage-class-invalid"
}

func (c pvcStorageClassInvalidCheck) Title() string {
	return "PersistentVolumeClaim Storage Class Name Must Be Valid"
}

func (c pvcStorageClassInvalidCheck) Category() string {
	return "persistent-volume-claim"
}

func (c pvcStorageClassInvalidCheck) Blocking() bool {
	return true
}

func (c pvcStorageClassInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvcStorageClassInvalidCheck) DocSkipper() []string {
	return pvcKinds
}

func (c pvcStorageClassInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	scPath := field.NewPath("spec").Child("storageClassName")

	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		if errs := validation.IsQualifiedName(*pvc.Spec.StorageClassName); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    scPath.String(),
					Message: fmt.Sprintf("storageClassName: invalid value: %s", errs[0]),
					Kind:    "PersistentVolumeClaim",
					Name:    pvc.GetName(),
					Value:   *pvc.Spec.StorageClassName,
				},
			})
		}
	}

	return findings
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

func (c pvcVolumeModeInvalidCheck) DocSkipper() []string {
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

// resourcesInvalidCheck validates that resources specifies at least one request or limit.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvcResourcesInvalidCheck struct{}

func (c pvcResourcesInvalidCheck) ID() string {
	return "persistent-volume-claim/resources-invalid"
}

func (c pvcResourcesInvalidCheck) Title() string {
	return "PersistentVolumeClaim Resources Must Specify At Least One Resource"
}

func (c pvcResourcesInvalidCheck) Category() string {
	return "persistent-volume-claim"
}

func (c pvcResourcesInvalidCheck) Blocking() bool {
	return true
}

func (c pvcResourcesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvcResourcesInvalidCheck) DocSkipper() []string {
	return pvcKinds
}

func (c pvcResourcesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	resources := pvc.Spec.Resources

	hasRequests := len(resources.Requests) > 0
	hasLimits := len(resources.Limits) > 0

	if !hasRequests && !hasLimits {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("resources").String(),
				Message: "resources must specify at least one resource",
				Kind:    "PersistentVolumeClaim",
				Name:    pvc.GetName(),
			},
		})
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		pvcAccessModesInvalidCheck{},
		pvcStorageClassInvalidCheck{},
		pvcVolumeModeInvalidCheck{},
		pvcResourcesInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
