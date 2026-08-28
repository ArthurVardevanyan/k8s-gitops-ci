package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestServiceAccountNameInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
`)
	check := serviceAccountNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/serviceaccount-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestServiceAccountNameInvalid_Check_WithName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
`)
	check := serviceAccountNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestServiceAccountSecretsInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
secrets:
- name: valid-secret
- name: invalid name
- name: another.valid
`)
	check := serviceAccountSecretsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].Path != "secrets[1].name" {
		t.Errorf("unexpected path: %s", findings[0].Path)
	}
}

func TestServiceAccountSecretsInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
secrets:
- name: secret1
- name: secret2
`)
	check := serviceAccountSecretsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestServiceAccountAutomountInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
automountServiceAccountToken: true
`)
	check := serviceAccountAutomountInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid boolean, got %d", len(findings))
	}
}

func TestServiceAccountAutomountInvalid_Check_Nil(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
`)
	check := serviceAccountAutomountInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when nil, got %d", len(findings))
	}
}

func TestServiceAccountImagePullSecretsInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
imagePullSecrets:
- name: valid-secret
- name: invalid secret
- name: ""
`)
	check := serviceAccountImagePullSecretsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (invalid name + empty name), got %d: %v", len(findings), findings)
	}
}

func TestServiceAccountImagePullSecretsInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
imagePullSecrets:
- name: secret1
- name: secret2
`)
	check := serviceAccountImagePullSecretsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestServiceAccountImagePullSecretsInvalid_Check_Empty(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
`)
	check := serviceAccountImagePullSecretsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no imagePullSecrets, got %d", len(findings))
	}
}

func TestServiceAccount_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		serviceAccountNameInvalidCheck{},
		serviceAccountSecretsInvalidCheck{},
		serviceAccountAutomountInvalidCheck{},
		serviceAccountImagePullSecretsInvalidCheck{},
	}
	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Category() != "core" {
			t.Errorf("check %T has wrong category: %s", c, c.Category())
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}
