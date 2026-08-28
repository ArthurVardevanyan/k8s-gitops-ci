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

func (c pvAccessModesInvalidCheck) DocSkipper() []string {
	return pvKinds
}

func (c pvAccessModesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validModes := map[corev1.PersistentVolumeAccessMode]bool{
		corev1.ReadOnlyMany:     true,
		corev1.ReadWriteMany:    true,
		corev1.ReadWriteOnce:    true,
		corev1.ReadWriteOncePod: true,
	}

	for i, mode := range pv.Spec.AccessModes {
		if !validModes[mode] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("accessModes").Index(i).String(),
					Message: fmt.Sprintf("accessModes: Unsupported value: %q", string(mode)),
					Kind:    "PersistentVolume",
					Name:    pv.GetName(),
				},
			})
		}
	}

	return findings
}

// storageClassInvalidCheck validates that storageClassName is a valid name.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvStorageClassInvalidCheck struct{}

func (c pvStorageClassInvalidCheck) ID() string {
	return "persistent-volume/storage-class-invalid"
}

func (c pvStorageClassInvalidCheck) Title() string {
	return "PersistentVolume Storage Class Name Must Be Valid"
}

func (c pvStorageClassInvalidCheck) Category() string {
	return "persistent-volume"
}

func (c pvStorageClassInvalidCheck) Blocking() bool {
	return true
}

func (c pvStorageClassInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvStorageClassInvalidCheck) DocSkipper() []string {
	return pvKinds
}

func (c pvStorageClassInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}

	var findings []runtime.Finding
	scPath := field.NewPath("spec").Child("storageClassName")

	if pv.Spec.StorageClassName != "" {
		if errs := validation.IsQualifiedName(pv.Spec.StorageClassName); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    scPath.String(),
					Message: fmt.Sprintf("storageClassName: invalid value: %s", errs[0]),
					Kind:    "PersistentVolume",
					Name:    pv.GetName(),
					Value:   pv.Spec.StorageClassName,
				},
			})
		}
	}

	return findings
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

func (c pvCapacityInvalidCheck) DocSkipper() []string {
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

// claimRefInvalidCheck validates that claimRef has valid namespace and name.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvClaimRefInvalidCheck struct{}

func (c pvClaimRefInvalidCheck) ID() string {
	return "persistent-volume/persistent-volume-claim-ref-invalid"
}

func (c pvClaimRefInvalidCheck) Title() string {
	return "PersistentVolume ClaimRef Must Have Valid Namespace And Name"
}

func (c pvClaimRefInvalidCheck) Category() string {
	return "persistent-volume"
}

func (c pvClaimRefInvalidCheck) Blocking() bool {
	return true
}

func (c pvClaimRefInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pvClaimRefInvalidCheck) DocSkipper() []string {
	return pvKinds
}

func (c pvClaimRefInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil
	}

	var findings []runtime.Finding
	claimRef := pv.Spec.ClaimRef

	if claimRef != nil {
		claimRefPath := field.NewPath("spec").Child("claimRef")

		if claimRef.Namespace == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    claimRefPath.Child("namespace").String(),
					Message: "claimRef: invalid value — namespace is required",
					Kind:    "PersistentVolume",
					Name:    pv.GetName(),
				},
			})
		}

		if claimRef.Name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    claimRefPath.Child("name").String(),
					Message: "claimRef: invalid value — name is required",
					Kind:    "PersistentVolume",
					Name:    pv.GetName(),
				},
			})
		}
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		pvAccessModesInvalidCheck{},
		pvStorageClassInvalidCheck{},
		pvCapacityInvalidCheck{},
		pvClaimRefInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
