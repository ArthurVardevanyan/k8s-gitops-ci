package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestReplicaSetSelectorInvalid_Check_ValidKeys(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: myapp
      "app.kubernetes.io/name": myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := replicaSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid selector keys, got %d: %v", len(findings), findings)
	}
}

func TestReplicaSetSelectorInvalid_Check_InvalidKey(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  selector:
    matchLabels:
      "invalid key with spaces": myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := replicaSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid selector key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/replicaset-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestReplicaSetSelectorInvalid_Check_InvalidMatchExpressionKey(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  selector:
    matchLabels:
      "invalid key": myapp
    matchExpressions:
    - key: "another invalid key"
      operator: In
      values:
      - val
  template:
    metadata:
      labels:
        app: myapp
`)
	check := replicaSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid matchExpressions key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/replicaset-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestReplicaSetSelectorInvalid_Check_NoSelector(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  template:
    metadata:
      labels:
        app: myapp
`)
	check := replicaSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when selector is absent, got %d: %v", len(findings), findings)
	}
}

func TestReplicaSetSelectorInvalid_Check_NotReplicaSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := replicaSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-ReplicaSet kind, got %d: %v", len(findings), findings)
	}
}

func TestReplicaSetReplicasInvalid_Check_Positive(t *testing.T) {
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
`)
	check := replicaSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for positive replicas, got %d: %v", len(findings), findings)
	}
}

func TestReplicaSetReplicasInvalid_Check_Zero(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test
spec:
  replicas: 0
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := replicaSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for zero replicas, got %d: %v", len(findings), findings)
	}
}

func TestReplicaSetReplicasInvalid_Check_Negative(t *testing.T) {
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
        app: myapp
`)
	check := replicaSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for negative replicas, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/replicaset-replicas-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "ReplicaSet" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
	if findings[0].Value != "-1" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestReplicaSetReplicasInvalid_Check_NotReplicaSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := replicaSetReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-ReplicaSet kind, got %d: %v", len(findings), findings)
	}
}

func TestReplicaSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{replicaSetSelectorInvalidCheck{}, "apps/replicaset-selector-invalid", "apps"},
		{replicaSetReplicasInvalidCheck{}, "apps/replicaset-replicas-invalid", "apps"},
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

func TestValidateReplicaSet_NonReplicaSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	findings := runKindChecks(data, "ReplicaSet")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateReplicaSet_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := runKindChecks(data, "ReplicaSet")
	if len(findings) > 0 {
		t.Errorf("expected nil or empty for invalid YAML, got %v", findings)
	}
}
