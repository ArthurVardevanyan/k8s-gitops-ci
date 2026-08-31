package apps

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestStatefulSetPodManagementPolicyInvalid_Check_OrderedReady(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  podManagementPolicy: OrderedReady
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newStatefulSetPodManagementPolicyInvalidCheck()
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
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newStatefulSetPodManagementPolicyInvalidCheck()
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
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newStatefulSetPodManagementPolicyInvalidCheck()
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
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newStatefulSetPodManagementPolicyInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid podManagementPolicy, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "kubernetes/apps/statefulset-pod-management-policy-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "Random" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestStatefulSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{newStatefulSetReplicasInvalidCheck(), "kubernetes/apps/statefulset-replicas-invalid", "apps"},
		{newStatefulSetPodManagementPolicyInvalidCheck(), "kubernetes/apps/statefulset-pod-management-policy-invalid", "apps"},
		{newStatefulSetUpdateStrategyInvalidCheck(), "kubernetes/apps/statefulset-update-strategy-invalid", "apps"},
	}

	for _, tc := range tests {
		t.Run(tc.wantID, func(t *testing.T) {
			if got := tc.check.ID(); got != tc.wantID {
				t.Errorf("ID() = %q, want %q", got, tc.wantID)
			}
			if got := runtime.CategoryOf(tc.check.ID()); got != tc.wantCat {
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
    matchLabels:
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
	if !ruleIDs["kubernetes/apps/statefulset-replicas-invalid"] {
		t.Error("expected statefulset-replicas-invalid finding")
	}
	if !ruleIDs["kubernetes/apps/statefulset-pod-management-policy-invalid"] {
		t.Error("expected statefulset-pod-management-policy-invalid finding")
	}
	if !ruleIDs["kubernetes/apps/statefulset-update-strategy-invalid"] {
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
    matchLabels:
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

// TestStatefulSetPodManagementPolicyInvalid_Check_ExplicitlyEmpty pins that an
// explicitly-empty podManagementPolicy is accepted.
//
// SetDefaults_StatefulSet guards the field on len()==0, which cannot tell an
// absent value from an explicit "", so both are replaced with OrderedReady
// before validation runs. The rule's own UpstreamRef.Note already said the
// empty case is skipped; the code disagreed, making every StatefulSet written
// with an empty value fail an unsuppressible check.
func TestStatefulSetPodManagementPolicyInvalid_Check_ExplicitlyEmpty(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  podManagementPolicy: ""
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newStatefulSetPodManagementPolicyInvalidCheck()
	if findings := check.Run(data, "test.yaml"); len(findings) != 0 {
		t.Errorf("an explicitly-empty podManagementPolicy is defaulted to OrderedReady and must not be reported, got %d: %v",
			len(findings), findings)
	}
}
