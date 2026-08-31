package networking

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var serviceKinds = []string{"Service"}

// typeInvalidCheck validates that service type is one of the allowed values.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type typeInvalidCheck struct{ runtime.Meta }

func newTypeInvalidCheck() typeInvalidCheck {
	return typeInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/service/type-invalid",
		RuleTitle: "Service Type Must Be Valid",
		AppliesTo: serviceKinds,
	}}
}

func (c typeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	path := field.NewPath("spec").Child("type")

	validTypes := map[corev1.ServiceType]bool{
		corev1.ServiceTypeClusterIP:    true,
		corev1.ServiceTypeNodePort:     true,
		corev1.ServiceTypeLoadBalancer: true,
		corev1.ServiceTypeExternalName: true,
	}

	if !validTypes[svc.Spec.Type] && string(svc.Spec.Type) != "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    path.String(),
				Message: fmt.Sprintf("type: Unsupported value: %q", string(svc.Spec.Type)),
				Kind:    "Service",
				Name:    svc.GetName(),
			},
		})
	}

	return findings
}

// sessionAffinityInvalidCheck validates that sessionAffinity is one of ClientIP, None.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type sessionAffinityInvalidCheck struct{ runtime.Meta }

func newSessionAffinityInvalidCheck() sessionAffinityInvalidCheck {
	return sessionAffinityInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/service/session-affinity-invalid",
		RuleTitle: "Service SessionAffinity Must Be Valid",
		AppliesTo: serviceKinds,
	}}
}

func (c sessionAffinityInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	path := field.NewPath("spec").Child("sessionAffinity")

	validAffinity := map[corev1.ServiceAffinity]bool{
		corev1.ServiceAffinityClientIP: true,
		corev1.ServiceAffinityNone:     true,
	}

	if !validAffinity[svc.Spec.SessionAffinity] && string(svc.Spec.SessionAffinity) != "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    path.String(),
				Message: fmt.Sprintf("sessionAffinity: Unsupported value: %q", string(svc.Spec.SessionAffinity)),
				Kind:    "Service",
				Name:    svc.GetName(),
			},
		})
	}

	return findings
}
