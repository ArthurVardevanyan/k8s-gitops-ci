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

// TestStatefulSetUpdateStrategyInvalid_Check_Recreate pins the deliberate
// divergence recorded in upstreamRefs: Kubernetes 1.37 adds a Recreate
// strategy behind the AllowStatefulSetRecreateStrategy gate. This tool cannot
// see a cluster's feature gates, so it accepts Recreate rather than block a
// valid manifest with a non-exemptable check.
func TestStatefulSetUpdateStrategyInvalid_Check_Recreate(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  updateStrategy:
    type: Recreate
  selector:
    app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	findings := statefulSetUpdateStrategyInvalidCheck{}.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Recreate, got %d: %v", len(findings), findings)
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

func TestStatefulSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{statefulSetReplicasInvalidCheck{}, "apps/statefulset-replicas-invalid", "apps"},
		{statefulSetPodManagementPolicyInvalidCheck{}, "apps/statefulset-pod-management-policy-invalid", "apps"},
		{statefulSetUpdateStrategyInvalidCheck{}, "apps/statefulset-update-strategy-invalid", "apps"},
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
			if len(tc.check.Kinds()) == 0 {
				t.Errorf("%s should declare Kinds", tc.wantID)
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
	findings := runKindChecks(data, "StatefulSet")
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
	findings := runKindChecks(data, "StatefulSet")
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
	findings := runKindChecks(data, "StatefulSet")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateStatefulSet_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := runKindChecks(data, "StatefulSet")
	if len(findings) > 0 {
		t.Errorf("expected nil or empty for invalid YAML, got %v", findings)
	}
}
