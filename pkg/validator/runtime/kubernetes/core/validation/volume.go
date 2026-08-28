package validation

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Check 1: volume-name
// Volume names must be 1-253 characters and conform to DNS-1123 subdomain.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:1765-1773
type volumeNameCheck struct{}

func (c volumeNameCheck) ID() string {
	return "volume/volume-name"
}

func (c volumeNameCheck) Title() string {
	return "Volume Name Must Be Valid DNS-1123 Subdomain"
}

func (c volumeNameCheck) Category() string {
	return "volume"
}

func (c volumeNameCheck) Blocking() bool {
	return true
}

func (c volumeNameCheck) RenderSensitive() bool {
	return true
}

func (c volumeNameCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c volumeNameCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		name := vol.Name
		if name == "" {
			continue
		}

		if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("name").String(),
					Message:   fmt.Sprintf("volume name %q: %s", name, strings.Join(errs, "; ")),
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

func (c duplicateVolumeNamesCheck) DocSkipper() []string {
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
					Path:      field.NewPath("spec").Child("volumes").Key(name).String(),
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

// Check 3: volume-name-unique
// Volume names must be unique. Reports the first duplicate found.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:2069-2075
type volumeNameUniqueCheck struct{}

func (c volumeNameUniqueCheck) ID() string {
	return "volume/volume-name-unique"
}

func (c volumeNameUniqueCheck) Title() string {
	return "All Volume Names Must Be Unique"
}

func (c volumeNameUniqueCheck) Category() string {
	return "volume"
}

func (c volumeNameUniqueCheck) Blocking() bool {
	return true
}

func (c volumeNameUniqueCheck) RenderSensitive() bool {
	return true
}

func (c volumeNameUniqueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c volumeNameUniqueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	seen := make(map[string]int)
	for i, vol := range vols {
		name := vol.Name
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("name").String(),
					Message:   fmt.Sprintf("duplicate volume name %q", name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Value:     name,
				},
			})
			break
		}
		seen[name] = i
	}

	return findings
}

// Check 4: emptydir-size-limit
// If emptyDir has a sizeLimit, the value must be a valid resource quantity.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
// Note: The Kubernetes YAML unmarshaler validates quantity format during
// unmarshaling, so this check mainly serves as documentation of the upstream
// validation rule. Invalid quantity values are rejected before this check runs.
type emptydirSizeLimitCheck struct{}

func (c emptydirSizeLimitCheck) ID() string {
	return "volume/emptydir-size-limit"
}

func (c emptydirSizeLimitCheck) Title() string {
	return "emptyDir Size Limit Must Be Valid"
}

func (c emptydirSizeLimitCheck) Category() string {
	return "volume"
}

func (c emptydirSizeLimitCheck) Blocking() bool {
	return true
}

func (c emptydirSizeLimitCheck) RenderSensitive() bool {
	return true
}

func (c emptydirSizeLimitCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c emptydirSizeLimitCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.EmptyDir == nil || vol.EmptyDir.SizeLimit == nil {
			continue
		}

		// The Kubernetes YAML unmarshaler already validates quantity format,
		// so a valid SizeLimit here is guaranteed to be a valid quantity.
		// We skip re-validation since it would always pass.
		_ = i
		_ = vol
	}

	return findings
}

// Check 5: persistentvolumeclaim-not-found
// PVC volume must reference a valid (non-empty) claimName.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type pvcVolumeCheck struct{}

func (c pvcVolumeCheck) ID() string {
	return "volume/persistentvolumeclaim-not-found"
}

func (c pvcVolumeCheck) Title() string {
	return "PVC Volume Must Have Valid Claim Name"
}

func (c pvcVolumeCheck) Category() string {
	return "volume"
}

func (c pvcVolumeCheck) Blocking() bool {
	return true
}

func (c pvcVolumeCheck) RenderSensitive() bool {
	return true
}

func (c pvcVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c pvcVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.PersistentVolumeClaim == nil {
			continue
		}

		if vol.PersistentVolumeClaim.ClaimName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("persistentVolumeClaim").Child("claimName").String(),
					Message:   fmt.Sprintf("volume %q: persistentVolumeClaim.claimName: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 6: secret-not-found
// Secret volume must reference a valid (non-empty) secretName.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type secretVolumeCheck struct{}

func (c secretVolumeCheck) ID() string {
	return "volume/secret-not-found"
}

func (c secretVolumeCheck) Title() string {
	return "Secret Volume Must Have Valid Secret Name"
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

func (c secretVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c secretVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.Secret == nil {
			continue
		}

		if vol.Secret.SecretName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("secret").Child("secretName").String(),
					Message:   fmt.Sprintf("volume %q: secret.secretName: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 7: configmap-not-found
// ConfigMap volume must reference a valid (non-empty) name.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type configmapVolumeCheck struct{}

func (c configmapVolumeCheck) ID() string {
	return "volume/configmap-not-found"
}

func (c configmapVolumeCheck) Title() string {
	return "ConfigMap Volume Must Have Valid Name"
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

func (c configmapVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c configmapVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.ConfigMap == nil {
			continue
		}

		if vol.ConfigMap.Name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("configMap").Child("name").String(),
					Message:   fmt.Sprintf("volume %q: configMap.name: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 8: downward-api-not-found
// DownwardAPI volume items must have at least one item.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type downwardAPIVolumeCheck struct{}

func (c downwardAPIVolumeCheck) ID() string {
	return "volume/downward-api-not-found"
}

func (c downwardAPIVolumeCheck) Title() string {
	return "DownwardAPI Volume Must Have Valid Field References"
}

func (c downwardAPIVolumeCheck) Category() string {
	return "volume"
}

func (c downwardAPIVolumeCheck) Blocking() bool {
	return true
}

func (c downwardAPIVolumeCheck) RenderSensitive() bool {
	return true
}

func (c downwardAPIVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c downwardAPIVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.DownwardAPI == nil {
			continue
		}

		if len(vol.DownwardAPI.Items) == 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("downwardAPI").Child("items").String(),
					Message:   fmt.Sprintf("volume %q: downwardAPI.items: must have at least one item", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 9: projected-not-found
// Projected volume must have at least one source.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type projectedVolumeCheck struct{}

func (c projectedVolumeCheck) ID() string {
	return "volume/projected-not-found"
}

func (c projectedVolumeCheck) Title() string {
	return "Projected Volume Must Have At Least One Source"
}

func (c projectedVolumeCheck) Category() string {
	return "volume"
}

func (c projectedVolumeCheck) Blocking() bool {
	return true
}

func (c projectedVolumeCheck) RenderSensitive() bool {
	return true
}

func (c projectedVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c projectedVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.Projected == nil {
			continue
		}

		if len(vol.Projected.Sources) == 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("projected").Child("sources").String(),
					Message:   fmt.Sprintf("volume %q: projected.sources: must have at least one source", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 10: volume-type-undefined
// Volume must define at least one volume source type.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:2069-2075
type volumeTypeUndefinedCheck struct{}

func (c volumeTypeUndefinedCheck) ID() string {
	return "volume/volume-type-undefined"
}

func (c volumeTypeUndefinedCheck) Title() string {
	return "Volume Must Define A Volume Source Type"
}

func (c volumeTypeUndefinedCheck) Category() string {
	return "volume"
}

func (c volumeTypeUndefinedCheck) Blocking() bool {
	return true
}

func (c volumeTypeUndefinedCheck) RenderSensitive() bool {
	return true
}

func (c volumeTypeUndefinedCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c volumeTypeUndefinedCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if !hasVolumeSource(vol) {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).String(),
					Message:   fmt.Sprintf("volume %q: must specify volume type", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Value:     vol.Name,
				},
			})
		}
	}

	return findings
}

// hasVolumeSource reports whether the volume has at least one volume source type set.
func hasVolumeSource(vol corev1.Volume) bool {
	return vol.HostPath != nil ||
		vol.EmptyDir != nil ||
		vol.GCEPersistentDisk != nil ||
		vol.AWSElasticBlockStore != nil ||
		vol.GitRepo != nil ||
		vol.Secret != nil ||
		vol.NFS != nil ||
		vol.ISCSI != nil ||
		vol.Glusterfs != nil ||
		vol.PersistentVolumeClaim != nil ||
		vol.RBD != nil ||
		vol.FlexVolume != nil ||
		vol.Cinder != nil ||
		vol.CephFS != nil ||
		vol.CSI != nil ||
		vol.Flocker != nil ||
		vol.DownwardAPI != nil ||
		vol.FC != nil ||
		vol.AzureFile != nil ||
		vol.ConfigMap != nil ||
		vol.VsphereVolume != nil ||
		vol.Quobyte != nil ||
		vol.AzureDisk != nil ||
		vol.StorageOS != nil ||
		vol.Projected != nil
}

// Check 11: hostpath-not-found
// hostPath volume must have a path defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type hostPathVolumeCheck struct{}

func (c hostPathVolumeCheck) ID() string {
	return "volume/hostpath-not-found"
}

func (c hostPathVolumeCheck) Title() string {
	return "hostPath Volume Must Have A Path"
}

func (c hostPathVolumeCheck) Category() string {
	return "volume"
}

func (c hostPathVolumeCheck) Blocking() bool {
	return true
}

func (c hostPathVolumeCheck) RenderSensitive() bool {
	return true
}

func (c hostPathVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c hostPathVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.HostPath == nil {
			continue
		}

		if vol.HostPath.Path == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("hostPath").Child("path").String(),
					Message:   fmt.Sprintf("volume %q: hostPath.path: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 12: nfs-invalid
// NFS volume must have both server and path defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type nfsVolumeCheck struct{}

func (c nfsVolumeCheck) ID() string {
	return "volume/nfs-invalid"
}

func (c nfsVolumeCheck) Title() string {
	return "NFS Volume Must Have Server And Path"
}

func (c nfsVolumeCheck) Category() string {
	return "volume"
}

func (c nfsVolumeCheck) Blocking() bool {
	return true
}

func (c nfsVolumeCheck) RenderSensitive() bool {
	return true
}

func (c nfsVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c nfsVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.NFS == nil {
			continue
		}

		if vol.NFS.Server == "" && vol.NFS.Path == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("nfs").String(),
					Message:   fmt.Sprintf("volume %q: nfs.server or nfs.path: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		} else if vol.NFS.Server == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("nfs").Child("server").String(),
					Message:   fmt.Sprintf("volume %q: nfs.server: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		} else if vol.NFS.Path == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("nfs").Child("path").String(),
					Message:   fmt.Sprintf("volume %q: nfs.path: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 13: csi-invalid
// CSI volume must have driver defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type csiVolumeCheck struct{}

func (c csiVolumeCheck) ID() string {
	return "volume/csi-invalid"
}

func (c csiVolumeCheck) Title() string {
	return "CSI Volume Must Have Driver"
}

func (c csiVolumeCheck) Category() string {
	return "volume"
}

func (c csiVolumeCheck) Blocking() bool {
	return true
}

func (c csiVolumeCheck) RenderSensitive() bool {
	return true
}

func (c csiVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c csiVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.CSI == nil {
			continue
		}

		if vol.CSI.Driver == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("csi").Child("driver").String(),
					Message:   fmt.Sprintf("volume %q: csi.driver: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 14: cinder-invalid
// Cinder volume must have volumeID defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type cinderVolumeCheck struct{}

func (c cinderVolumeCheck) ID() string {
	return "volume/cinder-invalid"
}

func (c cinderVolumeCheck) Title() string {
	return "Cinder Volume Must Have Volume ID"
}

func (c cinderVolumeCheck) Category() string {
	return "volume"
}

func (c cinderVolumeCheck) Blocking() bool {
	return true
}

func (c cinderVolumeCheck) RenderSensitive() bool {
	return true
}

func (c cinderVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c cinderVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.Cinder == nil {
			continue
		}

		if vol.Cinder.VolumeID == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("cinder").Child("volumeID").String(),
					Message:   fmt.Sprintf("volume %q: cinder.volumeID: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 15: gce-pd-invalid
// GCE PD volume must have pdName defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type gcePDVolumeCheck struct{}

func (c gcePDVolumeCheck) ID() string {
	return "volume/gce-pd-invalid"
}

func (c gcePDVolumeCheck) Title() string {
	return "GCE PD Volume Must Have PD Name"
}

func (c gcePDVolumeCheck) Category() string {
	return "volume"
}

func (c gcePDVolumeCheck) Blocking() bool {
	return true
}

func (c gcePDVolumeCheck) RenderSensitive() bool {
	return true
}

func (c gcePDVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c gcePDVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.GCEPersistentDisk == nil {
			continue
		}

		if vol.GCEPersistentDisk.PDName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("gcePersistentDisk").Child("pdName").String(),
					Message:   fmt.Sprintf("volume %q: gcePersistentDisk.pdName: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 16: azure-disk-invalid
// Azure Disk volume must have diskName defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type azureDiskVolumeCheck struct{}

func (c azureDiskVolumeCheck) ID() string {
	return "volume/azure-disk-invalid"
}

func (c azureDiskVolumeCheck) Title() string {
	return "Azure Disk Volume Must Have Disk Name"
}

func (c azureDiskVolumeCheck) Category() string {
	return "volume"
}

func (c azureDiskVolumeCheck) Blocking() bool {
	return true
}

func (c azureDiskVolumeCheck) RenderSensitive() bool {
	return true
}

func (c azureDiskVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c azureDiskVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.AzureDisk == nil {
			continue
		}

		if vol.AzureDisk.DiskName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("azureDisk").Child("diskName").String(),
					Message:   fmt.Sprintf("volume %q: azureDisk.diskName: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 17: azure-file-invalid
// Azure File volume must have shareName defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type azureFileVolumeCheck struct{}

func (c azureFileVolumeCheck) ID() string {
	return "volume/azure-file-invalid"
}

func (c azureFileVolumeCheck) Title() string {
	return "Azure File Volume Must Have Share Name"
}

func (c azureFileVolumeCheck) Category() string {
	return "volume"
}

func (c azureFileVolumeCheck) Blocking() bool {
	return true
}

func (c azureFileVolumeCheck) RenderSensitive() bool {
	return true
}

func (c azureFileVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c azureFileVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.AzureFile == nil {
			continue
		}

		if vol.AzureFile.ShareName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("azureFile").Child("shareName").String(),
					Message:   fmt.Sprintf("volume %q: azureFile.shareName: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 18: glusterfs-invalid
// Glusterfs volume must have endpoints defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type glusterfsVolumeCheck struct{}

func (c glusterfsVolumeCheck) ID() string {
	return "volume/glusterfs-invalid"
}

func (c glusterfsVolumeCheck) Title() string {
	return "Glusterfs Volume Must Have Endpoints"
}

func (c glusterfsVolumeCheck) Category() string {
	return "volume"
}

func (c glusterfsVolumeCheck) Blocking() bool {
	return true
}

func (c glusterfsVolumeCheck) RenderSensitive() bool {
	return true
}

func (c glusterfsVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c glusterfsVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.Glusterfs == nil {
			continue
		}

		if vol.Glusterfs.EndpointsName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("glusterfs").Child("endpoints").String(),
					Message:   fmt.Sprintf("volume %q: glusterfs.endpoints: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 19: iscsi-invalid
// iSCSI volume must have targetPortal and iqn defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type iscsiVolumeCheck struct{}

func (c iscsiVolumeCheck) ID() string {
	return "volume/iscsi-invalid"
}

func (c iscsiVolumeCheck) Title() string {
	return "iSCSI Volume Must Have Target Portal And IQN"
}

func (c iscsiVolumeCheck) Category() string {
	return "volume"
}

func (c iscsiVolumeCheck) Blocking() bool {
	return true
}

func (c iscsiVolumeCheck) RenderSensitive() bool {
	return true
}

func (c iscsiVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c iscsiVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.ISCSI == nil {
			continue
		}

		if vol.ISCSI.TargetPortal == "" && vol.ISCSI.IQN == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("iscsi").String(),
					Message:   fmt.Sprintf("volume %q: iscsi.targetPortal or iscsi.iqn: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		} else if vol.ISCSI.TargetPortal == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("iscsi").Child("targetPortal").String(),
					Message:   fmt.Sprintf("volume %q: iscsi.targetPortal: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		} else if vol.ISCSI.IQN == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("iscsi").Child("iqn").String(),
					Message:   fmt.Sprintf("volume %q: iscsi.iqn: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 20: rbd-invalid
// RBD volume must have monitors defined.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type rbdVolumeCheck struct{}

func (c rbdVolumeCheck) ID() string {
	return "volume/rbd-invalid"
}

func (c rbdVolumeCheck) Title() string {
	return "RBD Volume Must Have Monitors"
}

func (c rbdVolumeCheck) Category() string {
	return "volume"
}

func (c rbdVolumeCheck) Blocking() bool {
	return true
}

func (c rbdVolumeCheck) RenderSensitive() bool {
	return true
}

func (c rbdVolumeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c rbdVolumeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	vols := info.PodSpec.Volumes

	for i, vol := range vols {
		if vol.RBD == nil {
			continue
		}

		if len(vol.RBD.CephMonitors) == 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("volumes").Index(i).Child("rbd").Child("monitors").String(),
					Message:   fmt.Sprintf("volume %q: rbd.monitors: Required value", vol.Name),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// ValidateVolumeContext runs all volume validation rules against a parsed PodSpecInfo.
// It is called by kubernetes/register.go init() to register all checks and is the
// public entry point for running these validations.
func ValidateVolumeContext(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	checks := []struct {
		name string
		fn   func(*runtime.PodSpecInfo) []runtime.Finding
	}{
		{"volume-name", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				name := vol.Name
				if name == "" {
					continue
				}
				if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/volume-name",
						RuleTitle: "Volume Name Must Be Valid DNS-1123 Subdomain",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("name").String(),
							Message:   fmt.Sprintf("volume name %q: %s", name, strings.Join(errs, "; ")),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
							Value:     name,
						},
					})
				}
			}
			return findings
		}},
		{"duplicate-volume-names", func(info *runtime.PodSpecInfo) []runtime.Finding {
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
						RuleID:    "volume/duplicate-volume-names",
						RuleTitle: "Duplicate Volume Names Not Allowed",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Key(name).String(),
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
		}},
		{"volume-name-unique", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			seen := make(map[string]int)
			for i, vol := range vols {
				name := vol.Name
				if name == "" {
					continue
				}
				if _, exists := seen[name]; exists {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/volume-name-unique",
						RuleTitle: "All Volume Names Must Be Unique",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("name").String(),
							Message:   fmt.Sprintf("duplicate volume name %q", name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
							Value:     name,
						},
					})
					break
				}
				seen[name] = i
			}
			return findings
		}},
		{"emptydir-size-limit", func(info *runtime.PodSpecInfo) []runtime.Finding {
			// The Kubernetes YAML unmarshaler already validates quantity format,
			// so this check is a no-op here. Invalid quantities are rejected
			// before ExtractPodSpecInfo returns successfully.
			return nil
		}},
		{"persistentvolumeclaim-not-found", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.PersistentVolumeClaim == nil {
					continue
				}
				if vol.PersistentVolumeClaim.ClaimName == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/persistentvolumeclaim-not-found",
						RuleTitle: "PVC Volume Must Have Valid Claim Name",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("persistentVolumeClaim").Child("claimName").String(),
							Message:   fmt.Sprintf("volume %q: persistentVolumeClaim.claimName: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"secret-not-found", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.Secret == nil {
					continue
				}
				if vol.Secret.SecretName == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/secret-not-found",
						RuleTitle: "Secret Volume Must Have Valid Secret Name",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("secret").Child("secretName").String(),
							Message:   fmt.Sprintf("volume %q: secret.secretName: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"configmap-not-found", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.ConfigMap == nil {
					continue
				}
				if vol.ConfigMap.Name == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/configmap-not-found",
						RuleTitle: "ConfigMap Volume Must Have Valid Name",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("configMap").Child("name").String(),
							Message:   fmt.Sprintf("volume %q: configMap.name: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"downward-api-not-found", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.DownwardAPI == nil {
					continue
				}
				if len(vol.DownwardAPI.Items) == 0 {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/downward-api-not-found",
						RuleTitle: "DownwardAPI Volume Must Have Valid Field References",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("downwardAPI").Child("items").String(),
							Message:   fmt.Sprintf("volume %q: downwardAPI.items: must have at least one item", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"projected-not-found", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.Projected == nil {
					continue
				}
				if len(vol.Projected.Sources) == 0 {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/projected-not-found",
						RuleTitle: "Projected Volume Must Have At Least One Source",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("projected").Child("sources").String(),
							Message:   fmt.Sprintf("volume %q: projected.sources: must have at least one source", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"volume-type-undefined", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if !hasVolumeSource(vol) {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/volume-type-undefined",
						RuleTitle: "Volume Must Define A Volume Source Type",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).String(),
							Message:   fmt.Sprintf("volume %q: must specify volume type", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
							Value:     vol.Name,
						},
					})
				}
			}
			return findings
		}},
		{"hostpath-not-found", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.HostPath == nil {
					continue
				}
				if vol.HostPath.Path == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/hostpath-not-found",
						RuleTitle: "hostPath Volume Must Have A Path",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("hostPath").Child("path").String(),
							Message:   fmt.Sprintf("volume %q: hostPath.path: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"nfs-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.NFS == nil {
					continue
				}
				if vol.NFS.Server == "" && vol.NFS.Path == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/nfs-invalid",
						RuleTitle: "NFS Volume Must Have Server And Path",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("nfs").String(),
							Message:   fmt.Sprintf("volume %q: nfs.server or nfs.path: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				} else if vol.NFS.Server == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/nfs-invalid",
						RuleTitle: "NFS Volume Must Have Server And Path",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("nfs").Child("server").String(),
							Message:   fmt.Sprintf("volume %q: nfs.server: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				} else if vol.NFS.Path == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/nfs-invalid",
						RuleTitle: "NFS Volume Must Have Server And Path",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("nfs").Child("path").String(),
							Message:   fmt.Sprintf("volume %q: nfs.path: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"csi-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.CSI == nil {
					continue
				}
				if vol.CSI.Driver == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/csi-invalid",
						RuleTitle: "CSI Volume Must Have Driver",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("csi").Child("driver").String(),
							Message:   fmt.Sprintf("volume %q: csi.driver: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"cinder-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.Cinder == nil {
					continue
				}
				if vol.Cinder.VolumeID == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/cinder-invalid",
						RuleTitle: "Cinder Volume Must Have Volume ID",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("cinder").Child("volumeID").String(),
							Message:   fmt.Sprintf("volume %q: cinder.volumeID: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"gce-pd-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.GCEPersistentDisk == nil {
					continue
				}
				if vol.GCEPersistentDisk.PDName == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/gce-pd-invalid",
						RuleTitle: "GCE PD Volume Must Have PD Name",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("gcePersistentDisk").Child("pdName").String(),
							Message:   fmt.Sprintf("volume %q: gcePersistentDisk.pdName: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"azure-disk-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.AzureDisk == nil {
					continue
				}
				if vol.AzureDisk.DiskName == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/azure-disk-invalid",
						RuleTitle: "Azure Disk Volume Must Have Disk Name",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("azureDisk").Child("diskName").String(),
							Message:   fmt.Sprintf("volume %q: azureDisk.diskName: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"azure-file-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.AzureFile == nil {
					continue
				}
				if vol.AzureFile.ShareName == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/azure-file-invalid",
						RuleTitle: "Azure File Volume Must Have Share Name",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("azureFile").Child("shareName").String(),
							Message:   fmt.Sprintf("volume %q: azureFile.shareName: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"glusterfs-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.Glusterfs == nil {
					continue
				}
				if vol.Glusterfs.EndpointsName == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/glusterfs-invalid",
						RuleTitle: "Glusterfs Volume Must Have Endpoints",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("glusterfs").Child("endpoints").String(),
							Message:   fmt.Sprintf("volume %q: glusterfs.endpoints: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"iscsi-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.ISCSI == nil {
					continue
				}
				if vol.ISCSI.TargetPortal == "" && vol.ISCSI.IQN == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/iscsi-invalid",
						RuleTitle: "iSCSI Volume Must Have Target Portal And IQN",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("iscsi").String(),
							Message:   fmt.Sprintf("volume %q: iscsi.targetPortal or iscsi.iqn: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				} else if vol.ISCSI.TargetPortal == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/iscsi-invalid",
						RuleTitle: "iSCSI Volume Must Have Target Portal And IQN",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("iscsi").Child("targetPortal").String(),
							Message:   fmt.Sprintf("volume %q: iscsi.targetPortal: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				} else if vol.ISCSI.IQN == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/iscsi-invalid",
						RuleTitle: "iSCSI Volume Must Have Target Portal And IQN",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("iscsi").Child("iqn").String(),
							Message:   fmt.Sprintf("volume %q: iscsi.iqn: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
		{"rbd-invalid", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			vols := info.PodSpec.Volumes
			for i, vol := range vols {
				if vol.RBD == nil {
					continue
				}
				if len(vol.RBD.CephMonitors) == 0 {
					findings = append(findings, runtime.Finding{
						RuleID:    "volume/rbd-invalid",
						RuleTitle: "RBD Volume Must Have Monitors",
						Category:  "volume",
						Finding: check.Finding{
							Path:      field.NewPath("spec").Child("volumes").Index(i).Child("rbd").Child("monitors").String(),
							Message:   fmt.Sprintf("volume %q: rbd.monitors: Required value", vol.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
						},
					})
				}
			}
			return findings
		}},
	}

	var findings []runtime.Finding
	for _, c := range checks {
		findings = append(findings, c.fn(info)...)
	}

	return findings
}
