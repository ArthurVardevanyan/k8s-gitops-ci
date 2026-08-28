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
	check := maxReplicasInvalidCheck{}
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
	check := maxReplicasInvalidCheck{}
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
	check := maxReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMinReplicasInvalidCheck_Negative(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  minReplicas: -1
  maxReplicas: 10`)
	check := minReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "autoscaling/min-replicas-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestMinReplicasInvalidCheck_ExceedsMax(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  minReplicas: 15
  maxReplicas: 10`)
	check := minReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMinReplicasInvalidCheck_Valid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  minReplicas: 2
  maxReplicas: 10`)
	check := minReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestScaleTargetRefInvalidCheck_Invalid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  scaleTargetRef:
    kind: InvalidKind
    name: test`)
	check := scaleTargetRefInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "autoscaling/scale-target-ref-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestScaleTargetRefInvalidCheck_Valid(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  scaleTargetRef:
    kind: Deployment
    name: test`)
	check := scaleTargetRefInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestMetricSpecInvalidCheck_InvalidType(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10
  metrics:
  - type: InvalidType`)
	check := metricSpecInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "autoscaling/metric-spec-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestMetricSpecInvalidCheck_ValidType(t *testing.T) {
	data := []byte(`kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 10
  metrics:
  - type: Utilization
    target:
      type: Utilization
      targetUtilizationPercentage: 80`)
	check := metricSpecInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
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
	check := scaleDownInvalidCheck{}
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
	check := scaleDownInvalidCheck{}
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
	check := scaleUpInvalidCheck{}
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
	check := scaleUpInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestHPANonMatchingKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test`)
	check := maxReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-matching kind, got %d: %v", len(findings), findings)
	}
}

func TestAllHPAChecksImplementCheckInterface(t *testing.T) {
	var _ runtime.Check = maxReplicasInvalidCheck{}
	var _ runtime.Check = minReplicasInvalidCheck{}
	var _ runtime.Check = scaleTargetRefInvalidCheck{}
	var _ runtime.Check = metricSpecInvalidCheck{}
	var _ runtime.Check = scaleDownInvalidCheck{}
	var _ runtime.Check = scaleUpInvalidCheck{}
}
