package validation

import (
	"fmt"
	"regexp"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var crdKinds = []string{"CustomResourceDefinition"}

// nameInvalidCheck validates that the CRD name is in plural.group format.
// Source: k8s.io/kubernetes/pkg/apis/apiextensions/validation/validation.go
type nameInvalidCheck struct{}

func (c nameInvalidCheck) ID() string            { return "apiextensions/crd-name-invalid" }
func (c nameInvalidCheck) Title() string         { return "CRD Name Must Be In Plural.Group Format" }
func (c nameInvalidCheck) Category() string      { return "apiextensions" }
func (c nameInvalidCheck) Blocking() bool        { return true }
func (c nameInvalidCheck) RenderSensitive() bool { return true }
func (c nameInvalidCheck) DocSkipper() []string  { return crdKinds }

func (c nameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crd apiextensionsv1.CustomResourceDefinition
	if yaml.Unmarshal(data, &crd) != nil {
		return nil
	}
	if crd.Kind != "CustomResourceDefinition" {
		return nil
	}

	var findings []runtime.Finding
	name := crd.GetName()

	if len(name) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: "name: required",
				Kind:    "CustomResourceDefinition",
				Name:    name,
			},
		})
		return findings
	}

	// Name must contain a "." (plural.group format)
	if !strings.Contains(name, ".") {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: fmt.Sprintf("name: invalid value %q: must be in plural.group format (contains .)", name),
				Kind:    "CustomResourceDefinition",
				Name:    name,
			},
		})
		return findings
	}

	parts := strings.SplitN(name, ".", 2)
	resourcePart := parts[0]
	groupPart := parts[1]

	var allErrs []string
	if len(resourcePart) == 0 {
		allErrs = append(allErrs, "resource name must not be empty")
	} else {
		if errs := validation.IsDNS1123Subdomain(resourcePart); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if len(groupPart) == 0 {
		allErrs = append(allErrs, "group must not be empty")
	} else {
		if errs := validation.IsDNS1123Subdomain(groupPart); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if len(allErrs) > 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("name").String(),
				Message: fmt.Sprintf("name: invalid value %q: %s", name, strings.Join(allErrs, "; ")),
				Kind:    "CustomResourceDefinition",
				Name:    name,
			},
		})
	}

	return findings
}

// versionInvalidCheck validates that CRD version names are valid.
// Source: k8s.io/kubernetes/pkg/apis/apiextensions/validation/validation.go
type versionInvalidCheck struct{}

func (c versionInvalidCheck) ID() string            { return "apiextensions/crd-version-invalid" }
func (c versionInvalidCheck) Title() string         { return "CRD Version Names Must Be Valid" }
func (c versionInvalidCheck) Category() string      { return "apiextensions" }
func (c versionInvalidCheck) Blocking() bool        { return true }
func (c versionInvalidCheck) RenderSensitive() bool { return true }
func (c versionInvalidCheck) DocSkipper() []string  { return crdKinds }

func (c versionInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crd apiextensionsv1.CustomResourceDefinition
	if yaml.Unmarshal(data, &crd) != nil {
		return nil
	}
	if crd.Kind != "CustomResourceDefinition" {
		return nil
	}

	var findings []runtime.Finding
	versionPath := field.NewPath("spec").Child("versions")

	for i, version := range crd.Spec.Versions {
		versionName := version.Name
		path := versionPath.Index(i).Child("name")

		if versionName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: "version name: required",
					Kind:    "CustomResourceDefinition",
					Name:    crd.GetName(),
				},
			})
			continue
		}

		if !isValidVersionName(versionName) {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("version name: invalid value %q: must be v1, v2, v1beta1, v1alpha1, etc.", versionName),
					Kind:    "CustomResourceDefinition",
					Name:    crd.GetName(),
				},
			})
		}
	}

	return findings
}

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
func (c storageVersionInvalidCheck) DocSkipper() []string  { return crdKinds }

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

// servedVersionInvalidCheck validates that the served flag is valid.
// Source: k8s.io/kubernetes/pkg/apis/apiextensions/validation/validation.go
type servedVersionInvalidCheck struct{}

func (c servedVersionInvalidCheck) ID() string            { return "apiextensions/crd-served-version-invalid" }
func (c servedVersionInvalidCheck) Title() string         { return "CRD Served Flag Must Be Valid" }
func (c servedVersionInvalidCheck) Category() string      { return "apiextensions" }
func (c servedVersionInvalidCheck) Blocking() bool        { return true }
func (c servedVersionInvalidCheck) RenderSensitive() bool { return true }
func (c servedVersionInvalidCheck) DocSkipper() []string  { return crdKinds }

func (c servedVersionInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crd apiextensionsv1.CustomResourceDefinition
	if yaml.Unmarshal(data, &crd) != nil {
		return nil
	}
	if crd.Kind != "CustomResourceDefinition" {
		return nil
	}

	var findings []runtime.Finding

	return findings
}

// shortNameInvalidCheck validates that shortNames are valid.
// Source: k8s.io/kubernetes/pkg/apis/apiextensions/validation/validation.go
type shortNameInvalidCheck struct{}

func (c shortNameInvalidCheck) ID() string            { return "apiextensions/crd-short-name-invalid" }
func (c shortNameInvalidCheck) Title() string         { return "CRD ShortNames Must Be Valid" }
func (c shortNameInvalidCheck) Category() string      { return "apiextensions" }
func (c shortNameInvalidCheck) Blocking() bool        { return true }
func (c shortNameInvalidCheck) RenderSensitive() bool { return true }
func (c shortNameInvalidCheck) DocSkipper() []string  { return crdKinds }

func (c shortNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crd apiextensionsv1.CustomResourceDefinition
	if yaml.Unmarshal(data, &crd) != nil {
		return nil
	}
	if crd.Kind != "CustomResourceDefinition" {
		return nil
	}

	var findings []runtime.Finding
	shortNamePath := field.NewPath("spec").Child("names").Child("shortNames")

	for j, shortName := range crd.Spec.Names.ShortNames {
		path := shortNamePath.Index(j)

		if len(shortName) == 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: "shortNames: invalid value: must not be empty",
					Kind:    "CustomResourceDefinition",
					Name:    crd.GetName(),
				},
			})
			continue
		}

		if len(shortName) > 63 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("shortNames: invalid value %q: must be <= 63 characters", shortName),
					Kind:    "CustomResourceDefinition",
					Name:    crd.GetName(),
				},
			})
			continue
		}

		if errs := validation.IsDNS1123Label(shortName); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("shortNames: invalid value %q: %s", shortName, errs[0]),
					Kind:    "CustomResourceDefinition",
					Name:    crd.GetName(),
				},
			})
		}
	}

	return findings
}

// isValidVersionName checks if a version name follows the valid pattern (v1, v2, v1beta1, v1alpha1, etc.)
func isValidVersionName(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Must start with 'v'
	if name[0] != 'v' {
		return false
	}

	rest := name[1:]
	if len(rest) == 0 {
		return false
	}

	// Match: v1, v2, v1beta1, v1alpha1, v1beta2, v1alpha2, v10, v10beta1, etc.
	// Pattern: v + digit (not starting with 0) + optional (alpha|beta) + digit (not starting with 0)
	pattern := `^v(0|[1-9][0-9]*)$|^v(0|[1-9][0-9]*)(alpha|beta)(0|[1-9][0-9]*)$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

func init() {
	checks := []runtime.Check{
		nameInvalidCheck{},
		versionInvalidCheck{},
		storageVersionInvalidCheck{},
		servedVersionInvalidCheck{},
		shortNameInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
