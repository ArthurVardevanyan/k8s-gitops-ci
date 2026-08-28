package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestMaxReplicasInvalidCheck_Invalid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 0`)
	check := newMaxReplicasInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "autoscaling/max-replicas-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "HorizontalPodAutoscaler" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestMaxReplicasInvalidCheck_Valid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10`)
	check := newMaxReplicasInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestMaxReplicasInvalidCheck_Negative(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: -1`)
	check := newMaxReplicasInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestScaleDownInvalidCheck_Negative(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10
  behavior:
    scaleDown:
      stabilizationWindowSeconds: -1`)
	check := newScaleDownInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "autoscaling/scale-down-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestScaleDownInvalidCheck_Valid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 60`)
	check := newScaleDownInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestScaleUpInvalidCheck_Negative(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10
  behavior:
    scaleUp:
      stabilizationWindowSeconds: -1`)
	check := newScaleUpInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "autoscaling/scale-up-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestScaleUpInvalidCheck_Valid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60`)
	check := newScaleUpInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestAllHPAChecksImplementCheckInterface(t *testing.T) {
	var _ runtime.Check = newMaxReplicasInvalidCheck()
	var _ runtime.Check = newScaleDownInvalidCheck()
	var _ runtime.Check = newScaleUpInvalidCheck()
}
