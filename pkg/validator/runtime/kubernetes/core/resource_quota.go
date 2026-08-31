package core

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var resourceQuotaKinds = []string{"ResourceQuota"}

type resourceQuotaHardInvalidCheck struct{ runtime.Meta }

func newResourceQuotaHardInvalidCheck() resourceQuotaHardInvalidCheck {
	return resourceQuotaHardInvalidCheck{runtime.Meta{
		RuleID:    "core/resourcequota-hard-invalid",
		RuleTitle: "ResourceQuota Hard Values Must Be Valid Resources",
		AppliesTo: resourceQuotaKinds,
	}}
}

func (c resourceQuotaHardInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "ResourceQuota" {
		return nil
	}
	var rq corev1.ResourceQuota
	if err := yaml.Unmarshal(data, &rq); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for name := range rq.Spec.Hard {
		if errors := validation.IsQualifiedName(string(name)); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("hard").Key(string(name)).String(),
					Message: fmt.Sprintf("hard[%q]: invalid resource name: %s", name, errors[0]),
					Kind:    "ResourceQuota",
					Name:    rq.GetName(),
				},
			})
		}
	}
	return findings
}

type resourceQuotaHardNegativeCheck struct{ runtime.Meta }

func newResourceQuotaHardNegativeCheck() resourceQuotaHardNegativeCheck {
	return resourceQuotaHardNegativeCheck{runtime.Meta{
		RuleID:    "core/resourcequota-hard-negative",
		RuleTitle: "ResourceQuota Hard Values Must Not Be Negative",
		AppliesTo: resourceQuotaKinds,
	}}
}

func (c resourceQuotaHardNegativeCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "ResourceQuota" {
		return nil
	}
	var rq corev1.ResourceQuota
	if err := yaml.Unmarshal(data, &rq); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for name, val := range rq.Spec.Hard {
		if val.Sign() < 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("hard").Key(string(name)).String(),
					Message: fmt.Sprintf("hard[%q]: %s must not be negative", name, val.String()),
					Kind:    "ResourceQuota",
					Name:    rq.GetName(),
					Value:   val.String(),
				},
			})
		}
	}
	return findings
}
