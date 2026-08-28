package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

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
		if len(c.Kinds()) == 0 {
			t.Errorf("check %T should declare Kinds", c)
		}
	}
}
