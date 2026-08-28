package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestDeploymentSelectorInvalid_Check_ValidKeys(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    app: myapp
    "app.kubernetes.io/name": myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid selector keys, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentSelectorInvalid_Check_InvalidKey(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    "invalid key with spaces": myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid selector key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/deployment-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDeploymentSelectorInvalid_Check_InvalidMatchExpressionKey(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    app: myapp
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
	check := deploymentSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid matchExpressions key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/deployment-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDeploymentSelectorInvalid_Check_NoSelector(t *testing.T) {
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
	check := deploymentSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when selector is absent, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentSelectorInvalid_Check_NotDeployment(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
spec:
  selector:
    app: myapp
`)
	check := deploymentSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-Deployment kind, got %d: %v", len(findings), findings)
	}
}

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
	check := deploymentStrategyTypeInvalidCheck{}
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
	check := deploymentStrategyTypeInvalidCheck{}
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
	check := deploymentStrategyTypeInvalidCheck{}
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
	check := deploymentStrategyTypeInvalidCheck{}
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

func TestDeploymentStrategyTypeInvalid_Check_NotDeployment(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := deploymentStrategyTypeInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-Deployment kind, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentReplicasInvalid_Check_Positive(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for positive replicas, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentReplicasInvalid_Check_Zero(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 0
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for zero replicas, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentReplicasInvalid_Check_Negative(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: -1
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for negative replicas, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/deployment-replicas-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "-1" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestDeploymentReplicasInvalid_Check_NotDeployment(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := deploymentReplicasInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-Deployment kind, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentMinReadySecondsInvalid_Check_Valid(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  minReadySeconds: 10
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid minReadySeconds, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentMinReadySecondsInvalid_Check_Zero(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  minReadySeconds: 0
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for zero minReadySeconds, got %d: %v", len(findings), findings)
	}
}

func TestDeploymentMinReadySecondsInvalid_Check_Negative(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  minReadySeconds: -5
  template:
    metadata:
      labels:
        app: myapp
`)
	check := deploymentMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for negative minReadySeconds, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/deployment-min-ready-seconds-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "-5" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestDeploymentMinReadySecondsInvalid_Check_NotDeployment(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := deploymentMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-Deployment kind, got %d: %v", len(findings), findings)
	}
}

func TestDeployment_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{deploymentSelectorInvalidCheck{}, "apps/deployment-selector-invalid", "apps"},
		{deploymentStrategyTypeInvalidCheck{}, "apps/deployment-strategy-type-invalid", "apps"},
		{deploymentReplicasInvalidCheck{}, "apps/deployment-replicas-invalid", "apps"},
		{deploymentMinReadySecondsInvalidCheck{}, "apps/deployment-min-ready-seconds-invalid", "apps"},
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

func TestValidateDeployment_NonDeployment(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
spec:
  selector:
    app: myapp
`)
	findings := runKindChecks(data, "Deployment")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateDeployment_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := runKindChecks(data, "Deployment")
	if len(findings) > 0 {
		t.Errorf("expected nil or empty for invalid YAML, got %v", findings)
	}
}

func TestRegister(t *testing.T) {
	Register()
}
