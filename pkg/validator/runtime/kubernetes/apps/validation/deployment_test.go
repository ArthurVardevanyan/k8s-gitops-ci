package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestDeploymentStrategyTypeInvalid_Check_ValidRollingUpdate(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newDeploymentStrategyTypeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for RollingUpdate, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentStrategyTypeInvalid_Check_ValidRecreate(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newDeploymentStrategyTypeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Recreate, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentStrategyTypeInvalid_Check_NoStrategy(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newDeploymentStrategyTypeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when strategy is absent, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentStrategyTypeInvalid_Check_InvalidType(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: BlueGreen
  template:
    metadata:
      labels:
        app: myapp
`)
	check := newDeploymentStrategyTypeInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid strategy type, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/deployment-strategy-type-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "BlueGreen" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestDeployment_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{newDeploymentSelectorInvalidCheck(), "apps/deployment-selector-invalid", "apps"},
		{newDeploymentStrategyTypeInvalidCheck(), "apps/deployment-strategy-type-invalid", "apps"},
		{newDeploymentReplicasInvalidCheck(), "apps/deployment-replicas-invalid", "apps"},
		{newDeploymentMinReadySecondsInvalidCheck(), "apps/deployment-min-ready-seconds-invalid", "apps"},
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

// --- ValidateDeployment integration tests ---

func TestValidateDeployment_MultipleViolations(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: -1
  strategy:
    type: InvalidType
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: wrong
`)
	findings := runKindChecks(data, "Deployment")
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["apps/deployment-replicas-invalid"] {
		t.Error("expected deployment-replicas-invalid finding")
	}
	if !ruleIDs["apps/deployment-strategy-type-invalid"] {
		t.Error("expected deployment-strategy-type-invalid finding")
	}
}

func TestValidateDeployment_Clean(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	findings := runKindChecks(data, "Deployment")
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean deployment, got %d: %v", len(findings), findings)
	}
}

func TestRegister(t *testing.T) {
	Register()
}
