package validation

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var ingressKinds = []string{"Ingress"}

// pathTypeInvalidCheck validates that pathType is one of the allowed values.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type pathTypeInvalidCheck struct{ runtime.Meta }

func newPathTypeInvalidCheck() pathTypeInvalidCheck {
	return pathTypeInvalidCheck{runtime.Meta{
		RuleID:    "ingress/path-type-invalid",
		RuleTitle: "Ingress PathType Must Be Valid",
		AppliesTo: ingressKinds,
	}}
}

func (c pathTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(data, &ing); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range ing.Spec.Rules {
		rulePath := field.NewPath("spec").Child("rules").Index(i)

		if rule.HTTP == nil {
			continue
		}

		for j, path := range rule.HTTP.Paths {
			pathPath := rulePath.Child("http").Child("paths").Index(j)

			if path.PathType == nil || string(*path.PathType) == "" {
				continue
			}

			validPathTypes := map[networkingv1.PathType]bool{
				networkingv1.PathTypeExact:                  true,
				networkingv1.PathTypePrefix:                 true,
				networkingv1.PathTypeImplementationSpecific: true,
			}

			if !validPathTypes[*path.PathType] {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Finding: check.Finding{
						Path:    pathPath.Child("pathType").String(),
						Message: fmt.Sprintf("pathType: Unsupported value: %q", string(*path.PathType)),
						Kind:    "Ingress",
						Name:    ing.GetName(),
					},
				})
			}
		}
	}

	return findings
}
