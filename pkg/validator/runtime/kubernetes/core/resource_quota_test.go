package core

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
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
	check := newResourceQuotaHardInvalidCheck()
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
	check := newResourceQuotaHardInvalidCheck()
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
	check := newResourceQuotaHardNegativeCheck()
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
	check := newResourceQuotaHardNegativeCheck()
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
	check := newResourceQuotaHardNegativeCheck()
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
	hardInvalid := newResourceQuotaHardInvalidCheck()
	negCheck := newResourceQuotaHardNegativeCheck()
	hf := hardInvalid.Run(data, "test.yaml")
	nf := negCheck.Run(data, "test.yaml")
	if len(hf) != 0 {
		t.Errorf("expected no hard-invalid finding for valid resource name, got %d", len(hf))
	}
	if len(nf) != 1 {
		t.Errorf("expected 1 hard-negative finding, got %d", len(nf))
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
	check := newResourceQuotaHardNegativeCheck()
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
