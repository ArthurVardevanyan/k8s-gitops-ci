package validation

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestResourceRequestsGreaterThanLimits_Check_RequestsExceedLimits(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "200m"
        memory: "256Mi"
`)
	check := resourceRequestsGreaterThanLimitsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (cpu + memory), got %d: %v", len(findings), findings)
	}
	for i, f := range findings {
		if f.RuleID != "resources/resource-requests-greater-than-limits" {
			t.Errorf("finding %d: unexpected rule ID: %s", i, f.RuleID)
		}
		if f.Container != "c" {
			t.Errorf("finding %d: unexpected container: %s", i, f.Container)
		}
	}
}

func TestResourceRequestsGreaterThanLimits_Check_Equal(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "100m"
      limits:
        cpu: "100m"
`)
	check := resourceRequestsGreaterThanLimitsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when requests equal limits, got %d: %v", len(findings), findings)
	}
}

func TestResourceRequestsGreaterThanLimits_Check_RequestsLessThanLimits(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "100m"
      limits:
        cpu: "500m"
`)
	check := resourceRequestsGreaterThanLimitsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when requests less than limits, got %d: %v", len(findings), findings)
	}
}

func TestResourceRequestsGreaterThanLimits_Check_NoLimits(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "500m"
`)
	check := resourceRequestsGreaterThanLimitsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no limits, got %d: %v", len(findings), findings)
	}
}

func TestResourceQuantityNegative_Check_NegativeInRequests(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "-100m"
`)
	// resource.Quantity parses negative values fine ("-100m" is a valid
	// quantity), so this must produce exactly one finding. The assertion
	// previously accepted 0 or 1 on the theory that the unmarshaler might
	// reject it, which made the test pass whether or not the check worked -
	// including if it silently stopped detecting negative quantities.
	check := resourceQuantityNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding for a negative quantity, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "resources/resource-quantity-negative" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestResourceQuantityNegative_Check_NoNegative(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "200m"
        memory: "256Mi"
`)
	check := resourceQuantityNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-negative quantities, got %d: %v", len(findings), findings)
	}
}

func TestIsHugePageResource(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"hugepages-1Mi", "hugepages-1Mi", true},
		{"hugepages-2Mi", "hugepages-2Mi", true},
		{"cpu", "cpu", false},
		{"memory", "memory", false},
		{"ephemeral-storage", "ephemeral-storage", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHugePageResource(corev1.ResourceName(tt.input))
			if result != tt.expected {
				t.Errorf("isHugePageResource(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResourceRequestsGreaterThanLimits_Check_MultipleContainers(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c1
    image: nginx
    resources:
      requests:
        cpu: "500m"
      limits:
        cpu: "200m"
  - name: c2
    image: redis
    resources:
      requests:
        memory: "1Gi"
      limits:
        memory: "512Mi"
`)
	check := resourceRequestsGreaterThanLimitsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (one per container), got %d: %v", len(findings), findings)
	}
	containerNames := map[string]bool{
		findings[0].Container: true,
		findings[1].Container: true,
	}
	if !containerNames["c1"] || !containerNames["c2"] {
		t.Errorf("unexpected containers: %v", containerNames)
	}
}
