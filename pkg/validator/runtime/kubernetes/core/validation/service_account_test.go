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

func TestServiceAccount_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		serviceAccountNameInvalidCheck{},
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
		if len(c.Kinds()) == 0 {
			t.Errorf("check %T should declare Kinds", c)
		}
	}
}
