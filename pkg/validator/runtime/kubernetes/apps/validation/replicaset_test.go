package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestReplicaSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{newReplicaSetSelectorInvalidCheck(), "apps/replicaset-selector-invalid", "apps"},
		{newReplicaSetReplicasInvalidCheck(), "apps/replicaset-replicas-invalid", "apps"},
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

// --- ValidateReplicaSet integration tests ---

func TestValidateReplicaSet_MultipleViolations(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  replicas: -1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: wrong
    spec:
      restartPolicy: BadPolicy
      containers:
      - name: c
        image: nginx
`)
	findings := runKindChecks(data, "ReplicaSet")
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["apps/replicaset-replicas-invalid"] {
		t.Error("expected replicaset-replicas-invalid finding")
	}
}

func TestValidateReplicaSet_Clean(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      restartPolicy: Always
      containers:
      - name: c
        image: nginx
`)
	findings := runKindChecks(data, "ReplicaSet")
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean replicaset, got %d: %v", len(findings), findings)
	}
}
