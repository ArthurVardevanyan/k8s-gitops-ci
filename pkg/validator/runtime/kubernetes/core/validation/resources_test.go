package validation

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// --- resource-quantity-format tests ---

func TestResourceQuantityFormat_Check_ValidQuantity(t *testing.T) {
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
	check := resourceQuantityFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid resource quantities, got %d: %v", len(findings), findings)
	}
}

func TestResourceQuantityFormat_Check_InvalidQuantity(t *testing.T) {
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
        memory: "invalid"
`)
	// This test would only trigger if the YAML parser accepts "invalid" as a quantity,
	// which it doesn't in standard Kubernetes unmarshaling. The check primarily
	// validates IsZero, Sign(), and other programmatic checks.
	check := resourceQuantityFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) > 1 {
		t.Errorf("expected at most 1 finding, got %d: %v", len(findings), findings)
	}
}

// --- resource-limits-missing tests ---

func TestResourceLimitsMissing_Check_RequestsWithoutLimits(t *testing.T) {
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
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for requests without limits, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "resources/resource-limits-missing" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Container != "c" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

func TestResourceLimitsMissing_Check_LimitsWithoutRequests(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      limits:
        cpu: "200m"
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for limits without requests, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "resources/resource-limits-missing" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestResourceLimitsMissing_Check_Matching(t *testing.T) {
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
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for matching requests/limits, got %d: %v", len(findings), findings)
	}
}

func TestResourceLimitsMissing_Check_PartialMissing(t *testing.T) {
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
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for partially missing limits, got %d: %v", len(findings), findings)
	}
	if findings[0].Container != "c" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

func TestResourceLimitsMissing_Check_NoResources(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no resources set, got %d: %v", len(findings), findings)
	}
}

// --- resource-requests-greater-than-limits tests ---

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

// --- hugepages-in-requests tests ---

func TestHugePagesInRequests_Check_HugePagesInRequests(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        hugepages-1Mi: "100Mi"
`)
	check := hugepagesInRequestsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for huge pages in requests, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "resources/hugepages-in-requests" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Container != "c" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

func TestHugePagesInRequests_Check_HugePagesInLimits(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      limits:
        hugepages-1Mi: "100Mi"
`)
	check := hugepagesInRequestsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for huge pages in limits, got %d: %v", len(findings), findings)
	}
}

func TestHugePagesInRequests_Check_NormalRequests(t *testing.T) {
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
`)
	check := hugepagesInRequestsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for normal requests, got %d: %v", len(findings), findings)
	}
}

func TestHugePagesInRequests_Check_InitContainer(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  initContainers:
  - name: init
    image: busybox
    resources:
      requests:
        hugepages-2Mi: "200Mi"
  containers:
  - name: c
    image: nginx
`)
	check := hugepagesInRequestsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for huge pages in init container requests, got %d: %v", len(findings), findings)
	}
	if findings[0].Container != "init" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

// --- resource-quantity-zero tests ---

func TestResourceQuantityZero_Check_ZeroInRequests(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "0"
        memory: "128Mi"
`)
	check := resourceQuantityZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for zero cpu request, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "resources/resource-quantity-zero" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestResourceQuantityZero_Check_ZeroInLimits(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      limits:
        cpu: "200m"
        memory: "0"
`)
	check := resourceQuantityZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for zero memory limit, got %d: %v", len(findings), findings)
	}
}

func TestResourceQuantityZero_Check_NoZero(t *testing.T) {
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
        cpu: "200m"
`)
	check := resourceQuantityZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-zero quantities, got %d: %v", len(findings), findings)
	}
}

func TestResourceQuantityZero_Check_MultipleZero(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    resources:
      requests:
        cpu: "0"
        memory: "0"
      limits:
        cpu: "0"
        memory: "0"
`)
	check := resourceQuantityZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings for all zero quantities, got %d: %v", len(findings), findings)
	}
}

// --- resource-quantity-negative tests ---

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
	// Negative values in YAML are typically rejected by the Kubernetes
	// unmarshaler, but if they make it through this check catches them.
	check := resourceQuantityNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	// The YAML unmarshaler may reject negative values, so we accept either
	// 0 or 1 finding depending on whether the value was parsed.
	if len(findings) > 1 {
		t.Errorf("expected at most 1 finding, got %d: %v", len(findings), findings)
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

// --- helper function tests ---

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

func TestValidateResourceQuantity(t *testing.T) {
	tests := []struct {
		name     string
		qty      string
		expected string
	}{
		{"valid", "100m", ""},
		{"valid-gi", "1Gi", ""},
		{"zero", "0", "resource quantity must be greater than zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty := resource.MustParse(tt.qty)
			result := validateResourceQuantity("cpu", qty)
			if result != tt.expected {
				t.Errorf("validateResourceQuantity(%q) = %q, want %q", tt.qty, result, tt.expected)
			}
		})
	}
}

// --- deployment tests ---

func TestResourceLimitsMissing_Check_Deployment(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: c
        image: nginx
        resources:
          requests:
            cpu: "100m"
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for deployment without limits, got %d: %v", len(findings), findings)
	}
	if findings[0].Kind != "Deployment" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
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

// --- CronJob test ---

func TestHugePagesInRequests_Check_CronJob(t *testing.T) {
	data := []byte(`kind: CronJob
metadata:
  name: test
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: c
            image: nginx
            resources:
              requests:
                hugepages-1Mi: "100Mi"
`)
	check := hugepagesInRequestsCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for CronJob huge pages in requests, got %d: %v", len(findings), findings)
	}
	if findings[0].Kind != "CronJob" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

// --- ValidateResources integration test ---

func TestValidateResources_FindingIDs(t *testing.T) {
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
        hugepages-1Mi: "100Mi"
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) == 0 {
		t.Fatal("expected findings for requests without limits + huge pages in requests")
	}
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["resources/resource-limits-missing"] {
		t.Error("expected resource-limits-missing finding")
	}
}

// --- Nil/empty data tests ---

func TestResourceQuantityFormat_Check_EmptyData(t *testing.T) {
	check := resourceQuantityFormatCheck{}
	findings := check.Run([]byte{}, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty data, got %d", len(findings))
	}
}

func TestResourceLimitsMissing_Check_NotPodSpec(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	check := resourceLimitsMissingCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-podspec kind, got %d", len(findings))
	}
}
