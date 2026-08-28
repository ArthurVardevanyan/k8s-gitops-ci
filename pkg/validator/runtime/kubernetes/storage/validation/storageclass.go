package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var scKinds = []string{"StorageClass"}

// provisionerInvalidCheck validates that provisioner is specified and non-empty.
// Source: k8s.io/kubernetes/pkg/apis/storage/validation/validation.go
type scProvisionerInvalidCheck struct{}

func (c scProvisionerInvalidCheck) ID() string {
	return "storage-class/provisioner-invalid"
}

func (c scProvisionerInvalidCheck) Title() string {
	return "StorageClass Provisioner Must Be Specified"
}

func (c scProvisionerInvalidCheck) Category() string {
	return "storage-class"
}

func (c scProvisionerInvalidCheck) Blocking() bool {
	return true
}

func (c scProvisionerInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scProvisionerInvalidCheck) Kinds() []string {
	return scKinds
}

func (c scProvisionerInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var sc storagev1.StorageClass
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	provisionerPath := field.NewPath("provisioner")

	if sc.Provisioner == "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    provisionerPath.String(),
				Message: "provisioner: required value",
				Kind:    "StorageClass",
				Name:    sc.GetName(),
			},
		})
	}

	return findings
}

// scEnumFieldFindings validates that an optional enum-typed StorageClass field
// holds one of the recognized values. reclaimPolicy and volumeBindingMode
// share this logic.
// Source: k8s.io/kubernetes/pkg/apis/storage/validation/validation.go
func scEnumFieldFindings(
	c runtime.Check,
	data []byte,
	fieldName string,
	value func(storagev1.StorageClass) (string, bool),
	valid map[string]bool,
) []runtime.Finding {
	var sc storagev1.StorageClass
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	val, ok := value(sc)
	if ok && !valid[val] {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath(fieldName).String(),
				Message: fmt.Sprintf("%s: Unsupported value: %q", fieldName, val),
				Kind:    "StorageClass",
				Name:    sc.GetName(),
				Value:   val,
			},
		})
	}

	return findings
}

// reclaimPolicyInvalidCheck validates that reclaimPolicy is one of Delete or Retain.
// Source: k8s.io/kubernetes/pkg/apis/storage/validation/validation.go
type scReclaimPolicyInvalidCheck struct{}

func (c scReclaimPolicyInvalidCheck) ID() string {
	return "storage-class/reclaim-policy-invalid"
}

func (c scReclaimPolicyInvalidCheck) Title() string {
	return "StorageClass Reclaim Policy Must Be Valid"
}

func (c scReclaimPolicyInvalidCheck) Category() string {
	return "storage-class"
}

func (c scReclaimPolicyInvalidCheck) Blocking() bool {
	return true
}

func (c scReclaimPolicyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scReclaimPolicyInvalidCheck) Kinds() []string {
	return scKinds
}

func (c scReclaimPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return scEnumFieldFindings(c, data, "reclaimPolicy", func(sc storagev1.StorageClass) (string, bool) {
		if sc.ReclaimPolicy == nil {
			return "", false
		}
		return string(*sc.ReclaimPolicy), true
	}, map[string]bool{
		string(corev1.PersistentVolumeReclaimDelete): true,
		string(corev1.PersistentVolumeReclaimRetain): true,
	})
}

// volumeBindingModeInvalidCheck validates that volumeBindingMode is one of
// WaitForFirstConsumer or Immediate.
// Source: k8s.io/kubernetes/pkg/apis/storage/validation/validation.go
type scVolumeBindingModeInvalidCheck struct{}

func (c scVolumeBindingModeInvalidCheck) ID() string {
	return "storage-class/volume-binding-mode-invalid"
}

func (c scVolumeBindingModeInvalidCheck) Title() string {
	return "StorageClass Volume Binding Mode Must Be Valid"
}

func (c scVolumeBindingModeInvalidCheck) Category() string {
	return "storage-class"
}

func (c scVolumeBindingModeInvalidCheck) Blocking() bool {
	return true
}

func (c scVolumeBindingModeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scVolumeBindingModeInvalidCheck) Kinds() []string {
	return scKinds
}

func (c scVolumeBindingModeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return scEnumFieldFindings(c, data, "volumeBindingMode", func(sc storagev1.StorageClass) (string, bool) {
		if sc.VolumeBindingMode == nil {
			return "", false
		}
		return string(*sc.VolumeBindingMode), true
	}, map[string]bool{
		string(storagev1.VolumeBindingImmediate):            true,
		string(storagev1.VolumeBindingWaitForFirstConsumer): true,
	})
}

// allowedTopologyRangeInvalidCheck validates that allowedTopologies have
// valid label selectors (key must be a valid label key).
// Source: k8s.io/kubernetes/pkg/apis/storage/validation/validation.go
type scAllowedTopologyRangeInvalidCheck struct{}

func (c scAllowedTopologyRangeInvalidCheck) ID() string {
	return "storage-class/allowed-topology-range-invalid"
}

func (c scAllowedTopologyRangeInvalidCheck) Title() string {
	return "StorageClass Allowed Topologies Must Have Valid Label Selectors"
}

func (c scAllowedTopologyRangeInvalidCheck) Category() string {
	return "storage-class"
}

func (c scAllowedTopologyRangeInvalidCheck) Blocking() bool {
	return true
}

func (c scAllowedTopologyRangeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scAllowedTopologyRangeInvalidCheck) Kinds() []string {
	return scKinds
}

func (c scAllowedTopologyRangeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var sc storagev1.StorageClass
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, topology := range sc.AllowedTopologies {
		topologyPath := field.NewPath("allowedTopologies").Index(i)

		for j, expr := range topology.MatchLabelExpressions {
			exprPath := topologyPath.Child("matchLabelExpressions").Index(j)

			if expr.Key == "" {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    exprPath.Child("key").String(),
						Message: "allowedTopologies: invalid label selector — key is required",
						Kind:    "StorageClass",
						Name:    sc.GetName(),
					},
				})
			} else if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    exprPath.Child("key").String(),
						Message: fmt.Sprintf("allowedTopologies: invalid label selector: %s", errs[0]),
						Kind:    "StorageClass",
						Name:    sc.GetName(),
						Value:   expr.Key,
					},
				})
			}
		}
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		scProvisionerInvalidCheck{},
		scReclaimPolicyInvalidCheck{},
		scVolumeBindingModeInvalidCheck{},
		scAllowedTopologyRangeInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
