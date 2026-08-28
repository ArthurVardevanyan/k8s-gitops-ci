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
type storageVersionInvalidCheck struct{ runtime.Meta }

func newStorageVersionInvalidCheck() storageVersionInvalidCheck {
	return storageVersionInvalidCheck{runtime.Meta{
		RuleID:    "apiextensions/crd-storage-version-invalid",
		RuleTitle: "CRD Must Have Exactly One Storage Version",
		AppliesTo: crdKinds,
	}}
}

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
			if storageCount == 1 {
				// Remember the first storage version so a later duplicate
				// can name both sides of the conflict. Assigning on every
				// iteration instead reported the same version twice
				// ("found \"v2\" and \"v2\""), hiding which other version
				// it collided with.
				storageName = v.Name
				continue
			}
			path := versionPath.Index(i).Child("storage")
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("storage: must not have more than one version with storage=true (found %q and %q)", storageName, v.Name),
					Kind:    "CustomResourceDefinition",
					Name:    crd.GetName(),
				},
			})
		}
	}

	if storageCount == 0 && len(crd.Spec.Versions) > 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
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
