package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Check 2: duplicate-volume-names
// All volumes must have unique names within a Pod.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:2069-2075
type duplicateVolumeNamesCheck struct{}

func (c duplicateVolumeNamesCheck) ID() string {
	return "volume/duplicate-volume-names"
}

func (c duplicateVolumeNamesCheck) Title() string {
	return "Duplicate Volume Names Not Allowed"
}

func (c duplicateVolumeNamesCheck) Category() string {
	return "volume"
}

func (c duplicateVolumeNamesCheck) Blocking() bool {
	return true
}

func (c duplicateVolumeNamesCheck) RenderSensitive() bool {
	return true
}

func (c duplicateVolumeNamesCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c duplicateVolumeNamesCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	names := make(map[string]int)
	for _, vol := range vols {
		if vol.Name == "" {
			continue
		}
		names[vol.Name]++
	}

	for name, count := range names {
		if count > 1 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath(info.VolumesPath()).Key(name).String(),
					Message:   fmt.Sprintf("duplicate volume name %q appears %d times", name, count),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Value:     name,
				},
			})
		}
	}

	return findings
}

// Check 6: secret-name-required
// A secret volume must set a non-empty secretName. This is the upstream
// field.Required rule, NOT an existence check: whether the Secret exists in
// the cluster cannot be determined from a manifest, and must not be inferred
// here, since a Secret is frequently created by a separate controller or
// secret manager rather than by the commit under review.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type secretVolumeCheck struct{}

func (c secretVolumeCheck) ID() string {
	return "volume/secret-name-required"
}

func (c secretVolumeCheck) Title() string {
	return "Secret Volume Must Set secretName"
}

func (c secretVolumeCheck) Category() string {
	return "volume"
}

func (c secretVolumeCheck) Blocking() bool {
	return true
}

func (c secretVolumeCheck) RenderSensitive() bool {
	return true
}

func (c secretVolumeCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c secretVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	return volumeReferenceNameFindings(c, data, source, "secret", "secretName", func(vol corev1.Volume) (string, bool) {
		if vol.Secret == nil {
			return "", false
		}
		return vol.Secret.SecretName, true
	})
}

// volumeReferenceNameFindings reports volumes whose referenced object name is
// empty. volumeField and nameField are the JSON field names used in the
// reported path and message; ref returns the referenced name and whether the
// volume uses that volume source at all.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
func volumeReferenceNameFindings(
	c runtime.Check,
	data []byte,
	source string,
	volumeField string,
	nameField string,
	ref func(corev1.Volume) (string, bool),
) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		name, ok := ref(vol)
		if !ok {
			continue
		}

		if name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath(info.VolumesPath()).Index(i).Child(volumeField).Child(nameField).String(),
					Message:   fmt.Sprintf("volume %q: %s.%s: Required value", vol.Name, volumeField, nameField),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 7: configmap-name-required
// A configMap volume must set a non-empty name. As with secret-name-required
// this is the upstream field.Required rule, not an existence check.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type configmapVolumeCheck struct{}

func (c configmapVolumeCheck) ID() string {
	return "volume/configmap-name-required"
}

func (c configmapVolumeCheck) Title() string {
	return "ConfigMap Volume Must Set name"
}

func (c configmapVolumeCheck) Category() string {
	return "volume"
}

func (c configmapVolumeCheck) Blocking() bool {
	return true
}

func (c configmapVolumeCheck) RenderSensitive() bool {
	return true
}

func (c configmapVolumeCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c configmapVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	return volumeReferenceNameFindings(c, data, source, "configMap", "name", func(vol corev1.Volume) (string, bool) {
		if vol.ConfigMap == nil {
			return "", false
		}
		return vol.ConfigMap.Name, true
	})
}
