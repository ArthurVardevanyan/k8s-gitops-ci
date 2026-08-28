package validation

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var crdKinds = []string{"CustomResourceDefinition"}

// storageVersionInvalidCheck validates that exactly one version has storage=true.
// Source: k8s.io/kubernetes/pkg/apis/apiextensions/validation/validation.go
type storageVersionInvalidCheck struct{}

func (c storageVersionInvalidCheck) ID() string { return "apiextensions/crd-storage-version-invalid" }

func (c storageVersionInvalidCheck) Title() string {
	return "CRD Must Have Exactly One Storage Version"
}
func (c storageVersionInvalidCheck) Category() string      { return "apiextensions" }
func (c storageVersionInvalidCheck) Blocking() bool        { return true }
func (c storageVersionInvalidCheck) RenderSensitive() bool { return true }
func (c storageVersionInvalidCheck) Kinds() []string       { return crdKinds }

func (c storageVersionInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crd apiextensionsv1.CustomResourceDefinition
	if yaml.Unmarshal(data, &crd) != nil {
		return nil
	}
	if crd.Kind != "CustomResourceDefinition" {
		return nil
	}

	var findings []runtime.Finding

	storageCount := 0
	storageName := ""
	versionPath := field.NewPath("spec").Child("versions")

	for i, v := range crd.Spec.Versions {
		if v.Storage {
			storageCount++
			storageName = v.Name
			if storageCount > 1 {
				path := versionPath.Index(i).Child("storage")
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    path.String(),
						Message: fmt.Sprintf("storage: must not have more than one version with storage=true (found %q and %q)", storageName, v.Name),
						Kind:    "CustomResourceDefinition",
						Name:    crd.GetName(),
					},
				})
			}
		}
	}

	if storageCount == 0 && len(crd.Spec.Versions) > 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("versions").String(),
				Message: "exactly one version must have storage=true",
				Kind:    "CustomResourceDefinition",
				Name:    crd.GetName(),
			},
		})
	}

	return findings
}
