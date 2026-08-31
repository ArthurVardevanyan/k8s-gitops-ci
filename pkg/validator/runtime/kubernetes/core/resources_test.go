package core

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestResourceRequestsGreaterThanLimits(t *testing.T) {
	runContainerCases(t, newResourceRequestsGreaterThanLimitsCheck().Run, []containerCase{
		{name: "RequestsExceedLimits", spec: "  containers:\n  - name: c\n    image: nginx\n    resources:\n      requests:\n        cpu: \"500m\"\n        memory: \"512Mi\"\n      limits:\n        cpu: \"200m\"\n        memory: \"256Mi\"\n", want: 2},
		{name: "Equal", spec: "  containers:\n  - name: c\n    image: nginx\n    resources:\n      requests:\n        cpu: \"100m\"\n      limits:\n        cpu: \"100m\"\n", want: 0},
		{name: "RequestsLessThanLimits", spec: "  containers:\n  - name: c\n    image: nginx\n    resources:\n      requests:\n        cpu: \"100m\"\n      limits:\n        cpu: \"500m\"\n", want: 0},
		{name: "NoLimits", spec: "  containers:\n  - name: c\n    image: nginx\n    resources:\n      requests:\n        cpu: \"500m\"\n", want: 0},
		{name: "MultipleContainers", spec: "  containers:\n  - name: c1\n    image: nginx\n    resources:\n      requests:\n        cpu: \"500m\"\n      limits:\n        cpu: \"200m\"\n  - name: c2\n    image: redis\n    resources:\n      requests:\n        memory: \"1Gi\"\n      limits:\n        memory: \"512Mi\"\n", want: 2},
	})
}

func TestResourceQuantityNegative(t *testing.T) {
	runContainerCases(t, newResourceQuantityNegativeCheck().Run, []containerCase{
		{name: "NegativeInRequests", spec: "  containers:\n  - name: c\n    image: nginx\n    resources:\n      requests:\n        cpu: \"-100m\"\n", want: 1},
		{name: "NoNegative", spec: "  containers:\n  - name: c\n    image: nginx\n    resources:\n      requests:\n        cpu: \"100m\"\n        memory: \"128Mi\"\n      limits:\n        cpu: \"200m\"\n        memory: \"256Mi\"\n", want: 0},
	})
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
