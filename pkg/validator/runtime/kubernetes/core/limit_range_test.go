package core

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestLimitRangeMaxMinInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    min:
      cpu: "1"
    max:
      cpu: "500m"
`)
	check := newLimitRangeMaxMinInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLimitRangeMaxMinInvalid_Check_MaxEqualMin(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    min:
      cpu: "1"
    max:
      cpu: "1"
`)
	check := newLimitRangeMaxMinInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when max=min, got %d", len(findings))
	}
}

func TestLimitRangeMaxMinInvalid_Check_MaxGreaterThanMin(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    min:
      cpu: "1"
    max:
      cpu: "4"
`)
	check := newLimitRangeMaxMinInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when max>min, got %d", len(findings))
	}
}

func TestLimitRangeMaxMinInvalid_Check_NoMin(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    max:
      cpu: "500m"
`)
	check := newLimitRangeMaxMinInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no min, got %d", len(findings))
	}
}

func TestLimitRangeResourceQuantitySign(t *testing.T) {
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
