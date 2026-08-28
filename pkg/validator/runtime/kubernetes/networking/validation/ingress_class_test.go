package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- name-invalid tests ---

func TestIngressClassNameInvalid_Check_EmptyName(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: ""
spec:
  controller: k8s.io/ingress-class
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "networking/ingressclass-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "IngressClass" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestIngressClassNameInvalid_Check_NoName(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
spec:
  controller: k8s.io/ingress-class
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "networking/ingressclass-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestIngressClassNameInvalid_Check_InvalidDNSName(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "INVALID NAME!"
spec:
  controller: k8s.io/ingress-class
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "networking/ingressclass-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestIngressClassNameInvalid_Check_ValidName(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx-ingress"
spec:
  controller: k8s.io/ingress-class
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestIngressClassNameInvalid_Check_NonIngressClass(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-IngressClass, got %d", len(findings))
	}
}

// --- controller-invalid tests ---

func TestIngressClassControllerInvalid_Check_InvalidDomain(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx"
spec:
  controller: "invalid controller"
`)
	check := controllerInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "networking/ingressclass-controller-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestIngressClassControllerInvalid_Check_ValidDomain(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx"
spec:
  controller: "k8s.io/ingress-nginx"
`)
	check := controllerInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestIngressClassControllerInvalid_Check_EmptyController(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx"
spec:
  controller: ""
`)
	check := controllerInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty controller, got %d", len(findings))
	}
}

func TestIngressClassControllerInvalid_Check_NonIngressClass(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := controllerInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-IngressClass, got %d", len(findings))
	}
}

// --- parameters-invalid tests ---

func TestIngressClassParametersInvalid_Check_InvalidName(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx"
spec:
  controller: "k8s.io/ingress-nginx"
  parameters:
    apiGroup: internal.k8s.io
    kind: IngressParameters
    name: "INVALID NAME!"
`)
	check := parametersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "networking/ingressclass-parameters-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestIngressClassParametersInvalid_Check_ValidName(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx"
spec:
  controller: "k8s.io/ingress-nginx"
  parameters:
    apiGroup: internal.k8s.io
    kind: IngressParameters
    name: "valid-name"
`)
	check := parametersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestIngressClassParametersInvalid_Check_NilParameters(t *testing.T) {
	data := []byte(`apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: "nginx"
spec:
  controller: "k8s.io/ingress-nginx"
`)
	check := parametersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil parameters, got %d", len(findings))
	}
}

func TestIngressClassParametersInvalid_Check_NonIngressClass(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := parametersInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-IngressClass, got %d", len(findings))
	}
}

// --- Check interface implementation verification ---

func TestAllIngressClassChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		nameInvalidCheck{},
		controllerInvalidCheck{},
		parametersInvalidCheck{},
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
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}
