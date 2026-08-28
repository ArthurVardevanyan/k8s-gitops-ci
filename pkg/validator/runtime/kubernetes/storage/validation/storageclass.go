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
type scProvisionerInvalidCheck struct{ runtime.Meta }

func newScProvisionerInvalidCheck() scProvisionerInvalidCheck {
	return scProvisionerInvalidCheck{runtime.Meta{
		RuleID:    "storage-class/provisioner-invalid",
		RuleTitle: "StorageClass Provisioner Must Be Specified",
		AppliesTo: scKinds,
	}}
}

func (c scProvisionerInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var sc storagev1.StorageClass
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil
	}
	if sc.Kind != "StorageClass" {
		return nil
	}

	var findings []runtime.Finding
	provisionerPath := field.NewPath("provisioner")

	if sc.Provisioner == "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
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
	if sc.Kind != "StorageClass" {
		return nil
	}

	var findings []runtime.Finding

	val, ok := value(sc)
	if ok && !valid[val] {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
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
type scReclaimPolicyInvalidCheck struct{ runtime.Meta }

func newScReclaimPolicyInvalidCheck() scReclaimPolicyInvalidCheck {
	return scReclaimPolicyInvalidCheck{runtime.Meta{
		RuleID:    "storage-class/reclaim-policy-invalid",
		RuleTitle: "StorageClass Reclaim Policy Must Be Valid",
		AppliesTo: scKinds,
	}}
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
type scVolumeBindingModeInvalidCheck struct{ runtime.Meta }

func newScVolumeBindingModeInvalidCheck() scVolumeBindingModeInvalidCheck {
	return scVolumeBindingModeInvalidCheck{runtime.Meta{
		RuleID:    "storage-class/volume-binding-mode-invalid",
		RuleTitle: "StorageClass Volume Binding Mode Must Be Valid",
		AppliesTo: scKinds,
	}}
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
type scAllowedTopologyRangeInvalidCheck struct{ runtime.Meta }

func newScAllowedTopologyRangeInvalidCheck() scAllowedTopologyRangeInvalidCheck {
	return scAllowedTopologyRangeInvalidCheck{runtime.Meta{
		RuleID:    "storage-class/allowed-topology-range-invalid",
		RuleTitle: "StorageClass Allowed Topologies Must Have Valid Label Selectors",
		AppliesTo: scKinds,
	}}
}

func (c scAllowedTopologyRangeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var sc storagev1.StorageClass
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil
	}
	if sc.Kind != "StorageClass" {
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
