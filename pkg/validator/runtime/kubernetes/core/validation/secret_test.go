package validation

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestSecretTypeInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: invalid/type
data:
  key: dmFsdWU=
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/secret-type-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "invalid/type" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestSecretTypeInvalid_Check_Opaque(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: Opaque
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Opaque, got %d", len(findings))
	}
}

func TestSecretTypeInvalid_Check_TLS(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: kubernetes.io/tls
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for TLS, got %d", len(findings))
	}
}

func TestSecretTypeInvalid_Check_DockerConfigJson(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: kubernetes.io/dockerconfigjson
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for dockerconfigjson, got %d", len(findings))
	}
}

func TestSecretTypeInvalid_Check_EmptyType(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  key: dmFsdWU=
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty type, got %d", len(findings))
	}
}

func TestSecretTypeInvalid_Check_ServiceAccountToken(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: kubernetes.io/service-account-token
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for service-account-token, got %d", len(findings))
	}
}

func TestSecretTypeInvalid_Check_SSHAuth(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: kubernetes.io/ssh-auth
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for ssh-auth, got %d", len(findings))
	}
}

func TestSecretTypeInvalid_Check_BasicAuth(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
type: kubernetes.io/basic-auth
`)
	check := secretTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for basic-auth, got %d", len(findings))
	}
}

func TestSecretStringDataInvalidKey_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
stringData:
  valid.key: "value"
  invalid key: "bad"
`)
	check := secretStringDataInvalidKeyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Path != "stringData[invalid key]" {
		t.Errorf("unexpected path: %s", findings[0].Path)
	}
}

func TestSecretStringDataInvalidKey_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
stringData:
  app/config: "value"
  ns/name: "value2"
`)
	check := secretStringDataInvalidKeyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestSecretDataInvalidKey_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  valid.key: dmFsdWU=
  invalid key: YmFk
`)
	check := secretDataInvalidKeyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Path != "data[invalid key]" {
		t.Errorf("unexpected path: %s", findings[0].Path)
	}
}

func TestSecretDataInvalidKey_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  app.key: dmFsdWU=
`)
	check := secretDataInvalidKeyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestSecretNameInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
data:
  key: dmFsdWU=
`)
	check := secretNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/secret-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestSecretNameInvalid_Check_WithName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  key: dmFsdWU=
`)
	check := secretNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestSecret_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		secretTypeInvalidCheck{},
		secretStringDataInvalidKeyCheck{},
		secretDataInvalidKeyCheck{},
		secretNameInvalidCheck{},
	}
	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Blocking() != true {
			t.Errorf("check %T should be blocking", c)
		}
		if c.RenderSensitive() != true {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}

func TestValidSecretTypes(t *testing.T) {
	expected := []corev1.SecretType{
		corev1.SecretTypeOpaque,
		corev1.SecretTypeServiceAccountToken,
		corev1.SecretTypeTLS,
		corev1.SecretTypeDockerConfigJson,
		corev1.SecretTypeSSHAuth,
		corev1.SecretTypeBasicAuth,
	}
	for _, st := range expected {
		if !validSecretTypes[st] {
			t.Errorf("expected %q to be a valid secret type", st)
		}
	}
}
