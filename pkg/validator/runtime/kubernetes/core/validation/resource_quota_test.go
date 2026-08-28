package validation

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestResourceQuotaHardInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    invalid/resource/name: "100"
    cpu: "10"
`)
	check := resourceQuotaHardInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/resourcequota-hard-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestResourceQuotaHardInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    cpu: "10"
    memory: 1Gi
    pods: "5"
`)
	check := resourceQuotaHardInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestResourceQuotaHardNegative_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    cpu: "-5"
    memory: 1Gi
`)
	check := resourceQuotaHardNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/resourcequota-hard-negative" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestResourceQuotaHardNegative_Check_Zero(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    cpu: "0"
`)
	check := resourceQuotaHardNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for zero, got %d", len(findings))
	}
}

func TestResourceQuotaHardNegative_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    cpu: "10"
    memory: 1Gi
`)
	check := resourceQuotaHardNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestResourceQuotaNameInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
spec:
  hard:
    cpu: "10"
`)
	check := resourceQuotaNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/resourcequota-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestResourceQuotaNameInvalid_Check_WithName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test-quota
spec:
  hard:
    cpu: "10"
`)
	check := resourceQuotaNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestResourceQuotaHardInvalid_Check_NegativeResource(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    cpu: "-5"
`)
	hardInvalid := resourceQuotaHardInvalidCheck{}
	negCheck := resourceQuotaHardNegativeCheck{}
	hf := hardInvalid.Run(data, "test.yaml")
	nf := negCheck.Run(data, "test.yaml")
	if len(hf) != 0 {
		t.Errorf("expected no hard-invalid finding for valid resource name, got %d", len(hf))
	}
	if len(nf) != 1 {
		t.Errorf("expected 1 hard-negative finding, got %d", len(nf))
	}
}

func TestResourceQuota_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		resourceQuotaHardInvalidCheck{},
		resourceQuotaHardNegativeCheck{},
		resourceQuotaNameInvalidCheck{},
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

func TestResourceQuotaHardNegative_Check_Multiple(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: test
spec:
  hard:
    cpu: "-5"
    memory: -1Gi
    pods: "10"
`)
	check := resourceQuotaHardNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
}

func TestResourceQuantitySign(t *testing.T) {
	neg := resource.MustParse("-5")
	if neg.Sign() >= 0 {
		t.Error("expected negative resource to have sign < 0")
	}
	zero := resource.MustParse("0")
	if zero.Sign() != 0 {
		t.Error("expected zero resource to have sign == 0")
	}
	pos := resource.MustParse("5")
	if pos.Sign() <= 0 {
		t.Error("expected positive resource to have sign > 0")
	}
}
