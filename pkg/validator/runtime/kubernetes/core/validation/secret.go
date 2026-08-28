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

var secretKinds = []string{"Secret"}

var validSecretTypes = map[corev1.SecretType]bool{
	corev1.SecretTypeOpaque:              true,
	corev1.SecretTypeServiceAccountToken: true,
	corev1.SecretTypeTLS:                 true,
	corev1.SecretTypeDockerConfigJson:    true,
	corev1.SecretTypeSSHAuth:             true,
	corev1.SecretTypeBasicAuth:           true,
	corev1.SecretType("kubernetes.io/openshift.io/service-account-token"): true,
}

type secretTypeInvalidCheck struct{}

func (c secretTypeInvalidCheck) ID() string            { return "core/secret-type-invalid" }
func (c secretTypeInvalidCheck) Title() string         { return "Secret Type Must Be Valid" }
func (c secretTypeInvalidCheck) Category() string      { return "core" }
func (c secretTypeInvalidCheck) Blocking() bool        { return true }
func (c secretTypeInvalidCheck) RenderSensitive() bool { return true }
func (c secretTypeInvalidCheck) DocSkipper() []string  { return secretKinds }

func (c secretTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "Secret" {
		return nil
	}
	var s corev1.Secret
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil
	}
	if s.Type == "" || validSecretTypes[s.Type] {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    "type",
			Message: fmt.Sprintf("secret %q: type: Unsupported value: %s: supported values: Opaque, kubernetes.io/service-account-token, kubernetes.io/tls, kubernetes.io/dockerconfigjson, kubernetes.io/ssh-auth, kubernetes.io/basic-auth, kubernetes.io/openshift.io/service-account-token", s.GetName(), string(s.Type)),
			Kind:    "Secret",
			Name:    s.GetName(),
			Value:   string(s.Type),
		},
	}}
}

type secretStringDataInvalidKeyCheck struct{}

func (c secretStringDataInvalidKeyCheck) ID() string { return "core/secret-stringdata-invalid-key" }
func (c secretStringDataInvalidKeyCheck) Title() string {
	return "Secret StringData Keys Must Be Valid"
}
func (c secretStringDataInvalidKeyCheck) Category() string      { return "core" }
func (c secretStringDataInvalidKeyCheck) Blocking() bool        { return true }
func (c secretStringDataInvalidKeyCheck) RenderSensitive() bool { return true }
func (c secretStringDataInvalidKeyCheck) DocSkipper() []string  { return secretKinds }

func (c secretStringDataInvalidKeyCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "Secret" {
		return nil
	}
	var s corev1.Secret
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for k := range s.StringData {
		if errors := validation.IsQualifiedName(k); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("stringData").Key(k).String(),
					Message: fmt.Sprintf("stringData: invalid key: %s", errors[0]),
					Kind:    "Secret",
					Name:    s.GetName(),
				},
			})
		}
	}
	return findings
}

type secretDataInvalidKeyCheck struct{}

func (c secretDataInvalidKeyCheck) ID() string            { return "core/secret-data-invalid-key" }
func (c secretDataInvalidKeyCheck) Title() string         { return "Secret Data Keys Must Be Valid" }
func (c secretDataInvalidKeyCheck) Category() string      { return "core" }
func (c secretDataInvalidKeyCheck) Blocking() bool        { return true }
func (c secretDataInvalidKeyCheck) RenderSensitive() bool { return true }
func (c secretDataInvalidKeyCheck) DocSkipper() []string  { return secretKinds }

func (c secretDataInvalidKeyCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "Secret" {
		return nil
	}
	var s corev1.Secret
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for k := range s.Data {
		if errors := validation.IsQualifiedName(k); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("data").Key(k).String(),
					Message: fmt.Sprintf("data: invalid key: %s", errors[0]),
					Kind:    "Secret",
					Name:    s.GetName(),
				},
			})
		}
	}
	return findings
}

type secretNameInvalidCheck struct{}

func (c secretNameInvalidCheck) ID() string            { return "core/secret-name-invalid" }
func (c secretNameInvalidCheck) Title() string         { return "Secret Name Is Required" }
func (c secretNameInvalidCheck) Category() string      { return "core" }
func (c secretNameInvalidCheck) Blocking() bool        { return true }
func (c secretNameInvalidCheck) RenderSensitive() bool { return true }
func (c secretNameInvalidCheck) DocSkipper() []string  { return secretKinds }

func (c secretNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "Secret" {
		return nil
	}
	var s corev1.Secret
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil
	}
	if s.GetName() == "" {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "metadata.name",
				Message: "secret: metadata.name is required",
				Kind:    "Secret",
			},
		}}
	}
	return nil
}

func init() {
	checks := []runtime.Check{
		secretTypeInvalidCheck{},
		secretStringDataInvalidKeyCheck{},
		secretDataInvalidKeyCheck{},
		secretNameInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
