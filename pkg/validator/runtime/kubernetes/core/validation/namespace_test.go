package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestNamespaceNameInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: valid-name
`)
	check := namespaceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid name, got %d", len(findings))
	}
}

func TestNamespaceNameInvalid_Check_InvalidName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: INVALID_NAME
`)
	check := namespaceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/namespace-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestNamespaceNameInvalid_Check_EmptyName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
`)
	check := namespaceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/namespace-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestNamespaceNameInvalid_Check_DashName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: -invalid
`)
	check := namespaceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for name starting with dash, got %d", len(findings))
	}
}

func TestNamespaceFinalizersInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: test
  finalizers:
  - kubernetes
  - unknown-finalizer
`)
	check := namespaceFinalizersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/namespace-finalizers-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestNamespaceFinalizersInvalid_Check_KubernetesOnly(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: test
  finalizers:
  - kubernetes
`)
	check := namespaceFinalizersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for kubernetes finalizer, got %d", len(findings))
	}
}

func TestNamespaceFinalizersInvalid_Check_EmptyFinalizers(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: test
`)
	check := namespaceFinalizersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty finalizers, got %d", len(findings))
	}
}

func TestNamespace_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		namespaceNameInvalidCheck{},
		namespaceFinalizersInvalidCheck{},
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
