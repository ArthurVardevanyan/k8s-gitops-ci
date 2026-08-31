package autoscaling

import (
	"testing"
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

// scaleUp and scaleDown are HPAScalingRules objects, so the finding must
// point at spec.behavior.<direction>.stabilizationWindowSeconds. An indexed
// path is not a field an HPA manifest has.
func TestHPABehaviorFindingPathHasNoIndex(t *testing.T) {
	for _, direction := range []string{"scaleUp", "scaleDown"} {
		t.Run(direction, func(t *testing.T) {
			data := []byte("apiVersion: autoscaling/v2\nkind: HorizontalPodAutoscaler\n" +
				"metadata:\n  name: test\nspec:\n  maxReplicas: 5\n  behavior:\n    " +
				direction + ":\n      stabilizationWindowSeconds: -1\n")

			c := newScaleUpInvalidCheck().Run
			if direction == "scaleDown" {
				c = newScaleDownInvalidCheck().Run
			}
			got := c(data, "test.yaml")
			if len(got) != 1 {
				t.Fatalf("expected 1 finding, got %d: %v", len(got), got)
			}
			want := "spec.behavior." + direction + ".stabilizationWindowSeconds"
			if got[0].Path != want {
				t.Errorf("finding Path = %q, want %q", got[0].Path, want)
			}
		})
	}
}
