package validation

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- storage-version-invalid tests ---

func TestCRDStorageVersionInvalid_Check_NoStorageVersion(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: "v1"
    served: true
    storage: false
`)
	check := newStorageVersionInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-storage-version-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDStorageVersionInvalid_Check_MultipleStorageVersions(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: "v1"
    served: true
    storage: true
  - name: "v2"
    served: true
    storage: true
`)
	check := newStorageVersionInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-storage-version-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	// The message must name both sides of the conflict. Asserting only the
	// finding count let a bug through where the first storage version was
	// overwritten before the message was built, so it reported the second
	// version twice ("found \"v2\" and \"v2\"") and told the reader nothing
	// about which version it collided with.
	if msg := findings[0].Message; !strings.Contains(msg, `"v1"`) || !strings.Contains(msg, `"v2"`) {
		t.Errorf("message must name both conflicting versions, got: %s", msg)
	}
}

func TestCRDStorageVersionInvalid_Check_ExactlyOneStorage(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: "v1"
    served: true
    storage: true
  - name: "v2"
    served: true
    storage: false
`)
	check := newStorageVersionInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestCRDStorageVersionInvalid_Check_NonCRD(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := newStorageVersionInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-CRD, got %d", len(findings))
	}
}

// --- Check interface implementation verification ---

func TestAllCRDChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		newStorageVersionInvalidCheck(),
	}

	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if runtime.CategoryOf(c.ID()) == "" {
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
