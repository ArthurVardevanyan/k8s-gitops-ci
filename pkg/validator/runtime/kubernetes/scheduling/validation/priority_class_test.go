package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- name-invalid tests ---

func TestPriorityClassNameInvalid_Check_EmptyName(t *testing.T) {
	data := []byte(`apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: ""
value: 100
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "scheduling/priorityclass-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PriorityClass" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPriorityClassNameInvalid_Check_NoName(t *testing.T) {
	data := []byte(`apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
value: 100
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "scheduling/priorityclass-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPriorityClassNameInvalid_Check_InvalidDNSName(t *testing.T) {
	data := []byte(`apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: "INVALID NAME!"
value: 100
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "scheduling/priorityclass-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPriorityClassNameInvalid_Check_ValidName(t *testing.T) {
	data := []byte(`apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: "high-priority"
value: 100
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPriorityClassNameInvalid_Check_NonPriorityClass(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-PriorityClass, got %d", len(findings))
	}
}

// --- value-invalid tests ---

// --- global-default-invalid tests ---

// --- Check interface implementation verification ---

func TestAllPriorityClassChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		nameInvalidCheck{},
	}

	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if c.Category() == "" {
			t.Errorf("check %T has empty Category", c)
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
