package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- name-invalid tests ---

func TestCRDNameInvalid_Check_EmptyName(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ""
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: v1
    served: true
    storage: true
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "CustomResourceDefinition" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestCRDNameInvalid_Check_NoName(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: v1
    served: true
    storage: true
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "apiextensions/crd-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDNameInvalid_Check_NoSlash(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "test"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: v1
    served: true
    storage: true
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "apiextensions/crd-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDNameInvalid_Check_ValidName(t *testing.T) {
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
  - name: v1
    served: true
    storage: true
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestCRDNameInvalid_Check_InvalidGroup(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.INVALID"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
  versions:
  - name: v1
    served: true
    storage: true
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "apiextensions/crd-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDNameInvalid_Check_NonCRD(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := nameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-CRD, got %d", len(findings))
	}
}

// --- version-invalid tests ---

func TestCRDVersionInvalid_Check_EmptyVersionName(t *testing.T) {
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
  - name: ""
    served: true
    storage: true
`)
	check := versionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-version-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDVersionInvalid_Check_InvalidVersionName(t *testing.T) {
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
  - name: "invalid"
    served: true
    storage: true
`)
	check := versionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-version-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDVersionInvalid_Check_ValidVersionName(t *testing.T) {
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
`)
	check := versionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestCRDVersionInvalid_Check_ValidBetaVersion(t *testing.T) {
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
  - name: "v1beta1"
    served: true
    storage: false
  - name: "v1"
    served: true
    storage: true
`)
	check := versionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestCRDVersionInvalid_Check_NonCRD(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := versionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-CRD, got %d", len(findings))
	}
}

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
	check := storageVersionInvalidCheck{}
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
	check := storageVersionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-storage-version-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
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
	check := storageVersionInvalidCheck{}
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
	check := storageVersionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-CRD, got %d", len(findings))
	}
}

// --- served-version-invalid tests ---

func TestCRDServedVersionInvalid_Check_NonCRD(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := servedVersionInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-CRD, got %d", len(findings))
	}
}

// --- short-name-invalid tests ---

func TestCRDShortNameInvalid_Check_EmptyShortName(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
    shortNames:
    - ""
  versions:
  - name: "v1"
    served: true
    storage: true
`)
	check := shortNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-short-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDShortNameInvalid_Check_LongShortName(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
    shortNames:
    - "this-short-name-is-way-too-long-and-exceeds-sixty-three-characters"
  versions:
  - name: "v1"
    served: true
    storage: true
`)
	check := shortNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-short-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDShortNameInvalid_Check_InvalidShortName(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
    shortNames:
    - "INVALID NAME!"
  versions:
  - name: "v1"
    served: true
    storage: true
`)
	check := shortNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apiextensions/crd-short-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCRDShortNameInvalid_Check_ValidShortName(t *testing.T) {
	data := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "tests.test.example.com"
spec:
  group: test.example.com
  names:
    kind: Test
    plural: tests
    shortNames:
    - "t"
    - "tests"
  versions:
  - name: "v1"
    served: true
    storage: true
`)
	check := shortNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestCRDShortNameInvalid_Check_NonCRD(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := shortNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-CRD, got %d", len(findings))
	}
}

// --- Check interface implementation verification ---

func TestAllCRDChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		nameInvalidCheck{},
		versionInvalidCheck{},
		storageVersionInvalidCheck{},
		servedVersionInvalidCheck{},
		shortNameInvalidCheck{},
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
