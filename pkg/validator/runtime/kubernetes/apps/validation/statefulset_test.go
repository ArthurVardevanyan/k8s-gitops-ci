package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestStatefulSetReplicasInvalid_Check_Positive(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  replicas: 3
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for positive replicas, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetReplicasInvalid_Check_Zero(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  replicas: 0
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for zero replicas, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetReplicasInvalid_Check_Negative(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  replicas: -1
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for negative replicas, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/statefulset-replicas-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "StatefulSet" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
	if findings[0].Value != "-1" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestStatefulSetReplicasInvalid_Check_NotStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := statefulSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-StatefulSet kind, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetSelectorMustMatch_Check_Match(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetSelectorMustMatchCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for matching selector, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetSelectorMustMatch_Check_Mismatch(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  selector:
    app: myapp
    tier: frontend
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetSelectorMustMatchCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for mismatched selector, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/statefulset-selector-must-match" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestStatefulSetSelectorMustMatch_Check_MissingSelector(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetSelectorMustMatchCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when selector is absent, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetSelectorMustMatch_Check_NotStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := statefulSetSelectorMustMatchCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-StatefulSet kind, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetPodManagementPolicyInvalid_Check_OrderedReady(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  podManagementPolicy: OrderedReady
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetPodManagementPolicyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for OrderedReady, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetPodManagementPolicyInvalid_Check_Parallel(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  podManagementPolicy: Parallel
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetPodManagementPolicyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Parallel, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetPodManagementPolicyInvalid_Check_Default(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetPodManagementPolicyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when podManagementPolicy is absent, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetPodManagementPolicyInvalid_Check_Invalid(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  podManagementPolicy: Random
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetPodManagementPolicyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid podManagementPolicy, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/statefulset-pod-management-policy-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "Random" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestStatefulSetPodManagementPolicyInvalid_Check_NotStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := statefulSetPodManagementPolicyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-StatefulSet kind, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetUpdateStrategyInvalid_Check_RollingUpdate(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for RollingUpdate, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetUpdateStrategyInvalid_Check_OnDelete(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  updateStrategy:
    type: OnDelete
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for OnDelete, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetUpdateStrategyInvalid_Check_NoUpdateStrategy(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when updateStrategy is absent, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetUpdateStrategyInvalid_Check_InvalidType(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  updateStrategy:
    type: BlueGreen
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid updateStrategy, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/statefulset-update-strategy-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "BlueGreen" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestStatefulSetUpdateStrategyInvalid_Check_NotStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := statefulSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-StatefulSet kind, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetServiceNameInvalid_Check_Valid(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  serviceName: my-service
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetServiceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid serviceName, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetServiceNameInvalid_Check_Invalid(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  serviceName: invalid name with spaces
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetServiceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid serviceName, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/statefulset-service-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestStatefulSetServiceNameInvalid_Check_Empty(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  serviceName: ""
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetServiceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when serviceName is empty, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetServiceNameInvalid_Check_NotStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := statefulSetServiceNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-StatefulSet kind, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetVolumeClaimTemplatesEmpty_Check_WithTemplates(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      resources:
        requests:
          storage: 1Gi
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetVolumeClaimTemplatesEmptyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when volumeClaimTemplates has items, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetVolumeClaimTemplatesEmpty_Check_EmptyArray(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  volumeClaimTemplates: []
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetVolumeClaimTemplatesEmptyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty volumeClaimTemplates, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/statefulset-volume-claim-templates-empty" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestStatefulSetVolumeClaimTemplatesEmpty_Check_NoField(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := statefulSetVolumeClaimTemplatesEmptyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when volumeClaimTemplates field is absent, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSetVolumeClaimTemplatesEmpty_Check_NotStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := statefulSetVolumeClaimTemplatesEmptyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-StatefulSet kind, got %d: %v", len(findings), findings)
	}
}

func TestStatefulSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{statefulSetReplicasInvalidCheck{}, "apps/statefulset-replicas-invalid", "apps"},
		{statefulSetSelectorMustMatchCheck{}, "apps/statefulset-selector-must-match", "apps"},
		{statefulSetPodManagementPolicyInvalidCheck{}, "apps/statefulset-pod-management-policy-invalid", "apps"},
		{statefulSetUpdateStrategyInvalidCheck{}, "apps/statefulset-update-strategy-invalid", "apps"},
		{statefulSetServiceNameInvalidCheck{}, "apps/statefulset-service-name-invalid", "apps"},
		{statefulSetVolumeClaimTemplatesEmptyCheck{}, "apps/statefulset-volume-claim-templates-empty", "apps"},
	}

	for _, tc := range tests {
		t.Run(tc.wantID, func(t *testing.T) {
			if got := tc.check.ID(); got != tc.wantID {
				t.Errorf("ID() = %q, want %q", got, tc.wantID)
			}
			if got := tc.check.Category(); got != tc.wantCat {
				t.Errorf("Category() = %q, want %q", got, tc.wantCat)
			}
			if !tc.check.Blocking() {
				t.Errorf("%s should be blocking", tc.wantID)
			}
			if !tc.check.RenderSensitive() {
				t.Errorf("%s should render sensitive", tc.wantID)
			}
			if len(tc.check.DocSkipper()) == 0 {
				t.Errorf("%s should have DocSkipper", tc.wantID)
			}
		})
	}
}

// --- ValidateStatefulSet integration tests ---

func TestValidateStatefulSet_MultipleViolations(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  replicas: -1
  podManagementPolicy: Random
  updateStrategy:
    type: InvalidType
  serviceName: invalid name
  volumeClaimTemplates: []
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: wrong
`)
	findings := ValidateStatefulSet(data, "test.yaml")
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["apps/statefulset-replicas-invalid"] {
		t.Error("expected statefulset-replicas-invalid finding")
	}
	if !ruleIDs["apps/statefulset-pod-management-policy-invalid"] {
		t.Error("expected statefulset-pod-management-policy-invalid finding")
	}
	if !ruleIDs["apps/statefulset-update-strategy-invalid"] {
		t.Error("expected statefulset-update-strategy-invalid finding")
	}
	if !ruleIDs["apps/statefulset-service-name-invalid"] {
		t.Error("expected statefulset-service-name-invalid finding")
	}
	if !ruleIDs["apps/statefulset-volume-claim-templates-empty"] {
		t.Error("expected statefulset-volume-claim-templates-empty finding")
	}
	if !ruleIDs["apps/statefulset-selector-must-match"] {
		t.Error("expected statefulset-selector-must-match finding")
	}
}

func TestValidateStatefulSet_Clean(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  replicas: 3
  podManagementPolicy: OrderedReady
  updateStrategy:
    type: RollingUpdate
  serviceName: my-service
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      resources:
        requests:
          storage: 1Gi
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	findings := ValidateStatefulSet(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean statefulset, got %d: %v", len(findings), findings)
	}
}

func TestValidateStatefulSet_NonStatefulSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	findings := ValidateStatefulSet(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateStatefulSet_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := ValidateStatefulSet(data, "test.yaml")
	if len(findings) > 0 {
		t.Errorf("expected nil or empty for invalid YAML, got %v", findings)
	}
}
