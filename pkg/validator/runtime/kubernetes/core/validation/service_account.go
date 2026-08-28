package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var serviceAccountKinds = []string{"ServiceAccount"}

type serviceAccountNameInvalidCheck struct{}

func (c serviceAccountNameInvalidCheck) ID() string { return "core/serviceaccount-name-invalid" }

func (c serviceAccountNameInvalidCheck) Title() string         { return "ServiceAccount Name Is Required" }
func (c serviceAccountNameInvalidCheck) Category() string      { return "core" }
func (c serviceAccountNameInvalidCheck) Blocking() bool        { return true }
func (c serviceAccountNameInvalidCheck) RenderSensitive() bool { return true }
func (c serviceAccountNameInvalidCheck) DocSkipper() []string  { return serviceAccountKinds }

func (c serviceAccountNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "ServiceAccount" {
		return nil
	}
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal(data, &sa); err != nil {
		return nil
	}
	if sa.GetName() == "" {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "metadata.name",
				Message: "serviceaccount: metadata.name is required",
				Kind:    "ServiceAccount",
			},
		}}
	}
	return nil
}

type serviceAccountSecretsInvalidCheck struct{}

func (c serviceAccountSecretsInvalidCheck) ID() string { return "core/serviceaccount-secrets-invalid" }

func (c serviceAccountSecretsInvalidCheck) Title() string {
	return "ServiceAccount Secrets Must Reference Valid Service Accounts"
}
func (c serviceAccountSecretsInvalidCheck) Category() string      { return "core" }
func (c serviceAccountSecretsInvalidCheck) Blocking() bool        { return true }
func (c serviceAccountSecretsInvalidCheck) RenderSensitive() bool { return true }
func (c serviceAccountSecretsInvalidCheck) DocSkipper() []string  { return serviceAccountKinds }

func (c serviceAccountSecretsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "ServiceAccount" {
		return nil
	}
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal(data, &sa); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for i, secret := range sa.Secrets {
		if errors := validation.IsDNS1123Subdomain(secret.Name); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("secrets").Index(i).Child("name").String(),
					Message: fmt.Sprintf("secrets[%d].name: %s", i, errors[0]),
					Kind:    "ServiceAccount",
					Name:    sa.GetName(),
				},
			})
		}
	}
	return findings
}

type serviceAccountAutomountInvalidCheck struct{}

func (c serviceAccountAutomountInvalidCheck) ID() string {
	return "core/serviceaccount-automount-invalid"
}

func (c serviceAccountAutomountInvalidCheck) Title() string {
	return "ServiceAccount AutomountServiceAccountToken Must Be Boolean"
}
func (c serviceAccountAutomountInvalidCheck) Category() string      { return "core" }
func (c serviceAccountAutomountInvalidCheck) Blocking() bool        { return true }
func (c serviceAccountAutomountInvalidCheck) RenderSensitive() bool { return true }
func (c serviceAccountAutomountInvalidCheck) DocSkipper() []string  { return serviceAccountKinds }

func (c serviceAccountAutomountInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "ServiceAccount" {
		return nil
	}
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal(data, &sa); err != nil {
		return nil
	}
	if sa.AutomountServiceAccountToken == nil {
		return nil
	}
	return nil
}

type serviceAccountImagePullSecretsInvalidCheck struct{}

func (c serviceAccountImagePullSecretsInvalidCheck) ID() string {
	return "core/serviceaccount-image-pull-secrets-invalid"
}

func (c serviceAccountImagePullSecretsInvalidCheck) Title() string {
	return "ServiceAccount ImagePullSecrets Must Be Valid"
}
func (c serviceAccountImagePullSecretsInvalidCheck) Category() string      { return "core" }
func (c serviceAccountImagePullSecretsInvalidCheck) Blocking() bool        { return true }
func (c serviceAccountImagePullSecretsInvalidCheck) RenderSensitive() bool { return true }
func (c serviceAccountImagePullSecretsInvalidCheck) DocSkipper() []string  { return serviceAccountKinds }

func (c serviceAccountImagePullSecretsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "ServiceAccount" {
		return nil
	}
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal(data, &sa); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for i, ips := range sa.ImagePullSecrets {
		if ips.Name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("imagePullSecrets").Index(i).Child("name").String(),
					Message: fmt.Sprintf("imagePullSecrets[%d].name: name is required", i),
					Kind:    "ServiceAccount",
					Name:    sa.GetName(),
				},
			})
		} else if errors := validation.IsDNS1123Subdomain(ips.Name); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("imagePullSecrets").Index(i).Child("name").String(),
					Message: fmt.Sprintf("imagePullSecrets[%d].name: %s", i, errors[0]),
					Kind:    "ServiceAccount",
					Name:    sa.GetName(),
				},
			})
		}
	}
	return findings
}

func init() {
	checks := []runtime.Check{
		serviceAccountNameInvalidCheck{},
		serviceAccountSecretsInvalidCheck{},
		serviceAccountAutomountInvalidCheck{},
		serviceAccountImagePullSecretsInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
