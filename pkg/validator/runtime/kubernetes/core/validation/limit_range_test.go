package validation

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestLimitRangeTypeInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: InvalidType
    max:
      cpu: "1"
`)
	check := limitRangeTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/limitrange-type-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestLimitRangeTypeInvalid_Check_ValidTypes(t *testing.T) {
	validTypes := []corev1.LimitType{
		corev1.LimitTypePod,
		corev1.LimitTypeContainer,
		corev1.LimitTypePersistentVolumeClaim,
	}
	for _, typ := range validTypes {
		data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: ` + string(typ) + `
    max:
      cpu: "1"
`)
		check := limitRangeTypeInvalidCheck{}
		findings := check.Run(data, "test.yaml")
		if len(findings) != 0 {
			t.Errorf("expected no findings for type %q, got %d: %v", typ, len(findings), findings)
		}
	}
}

func TestLimitRangeTypeInvalid_Check_EmptyType(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - max:
      cpu: "1"
`)
	check := limitRangeTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty type, got %d", len(findings))
	}
}

func TestLimitRangeMaxInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    max:
      cpu: "-1"
      memory: -1Gi
`)
	check := limitRangeMaxInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
}

func TestLimitRangeMaxInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    max:
      cpu: "1"
      memory: 1Gi
`)
	check := limitRangeMaxInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestLimitRangeMinInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    min:
      cpu: "-1"
`)
	check := limitRangeMinInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestLimitRangeMinInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    min:
      cpu: "100m"
`)
	check := limitRangeMinInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

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
	check := limitRangeMaxMinInvalidCheck{}
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
	check := limitRangeMaxMinInvalidCheck{}
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
	check := limitRangeMaxMinInvalidCheck{}
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
	check := limitRangeMaxMinInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no min, got %d", len(findings))
	}
}

func TestLimitRangeNameInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
spec:
  limits:
  - type: Container
`)
	check := limitRangeNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/limitrange-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestLimitRangeNameInvalid_Check_WithName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test-lr
spec:
  limits:
  - type: Container
`)
	check := limitRangeNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestLimitRangeDefaultInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    default:
      cpu: "-1"
`)
	check := limitRangeDefaultInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestLimitRangeDefaultInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    default:
      cpu: "100m"
`)
	check := limitRangeDefaultInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestLimitRangeDefaultRequestInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    defaultRequest:
      cpu: "-1"
`)
	check := limitRangeDefaultRequestInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestLimitRangeDefaultRequestInvalid_Check_Clean(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: LimitRange
metadata:
  name: test
spec:
  limits:
  - type: Container
    defaultRequest:
      cpu: "100m"
`)
	check := limitRangeDefaultRequestInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestLimitRange_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		limitRangeTypeInvalidCheck{},
		limitRangeMaxInvalidCheck{},
		limitRangeMinInvalidCheck{},
		limitRangeMaxMinInvalidCheck{},
		limitRangeNameInvalidCheck{},
		limitRangeDefaultInvalidCheck{},
		limitRangeDefaultRequestInvalidCheck{},
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
