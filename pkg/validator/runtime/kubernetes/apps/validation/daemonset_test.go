package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestDaemonSetSelectorInvalid_Check_ValidKeys(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
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
	check := daemonSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid selector keys, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetSelectorInvalid_Check_InvalidKey(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
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
	check := daemonSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid selector key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/daemonset-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDaemonSetSelectorInvalid_Check_InvalidMatchExpressionKey(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
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
	check := daemonSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid matchExpressions key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/daemonset-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDaemonSetSelectorInvalid_Check_NoSelector(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when selector is absent, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetSelectorInvalid_Check_NotDaemonSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := daemonSetSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-DaemonSet kind, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetUpdateStrategyInvalid_Check_RollingUpdate(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for RollingUpdate, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetUpdateStrategyInvalid_Check_OnDelete(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  updateStrategy:
    type: OnDelete
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for OnDelete, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetUpdateStrategyInvalid_Check_NoUpdateStrategy(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
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
	check := daemonSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when updateStrategy is absent, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetUpdateStrategyInvalid_Check_InvalidType(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  updateStrategy:
    type: BlueGreen
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid updateStrategy, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/daemonset-update-strategy-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "BlueGreen" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestDaemonSetUpdateStrategyInvalid_Check_NotDaemonSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := daemonSetUpdateStrategyInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-DaemonSet kind, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetMinReadySecondsInvalid_Check_Valid(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  minReadySeconds: 10
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid minReadySeconds, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetMinReadySecondsInvalid_Check_Zero(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  minReadySeconds: 0
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for zero minReadySeconds, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSetMinReadySecondsInvalid_Check_Negative(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  minReadySeconds: -5
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	check := daemonSetMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for negative minReadySeconds, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "apps/daemonset-min-ready-seconds-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "DaemonSet" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
	if findings[0].Value != "-5" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestDaemonSetMinReadySecondsInvalid_Check_NotDaemonSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	check := daemonSetMinReadySecondsInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-DaemonSet kind, got %d: %v", len(findings), findings)
	}
}

func TestDaemonSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{daemonSetSelectorInvalidCheck{}, "apps/daemonset-selector-invalid", "apps"},
		{daemonSetUpdateStrategyInvalidCheck{}, "apps/daemonset-update-strategy-invalid", "apps"},
		{daemonSetMinReadySecondsInvalidCheck{}, "apps/daemonset-min-ready-seconds-invalid", "apps"},
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

// --- ValidateDaemonSet integration tests ---

func TestValidateDaemonSet_MultipleViolations(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  minReadySeconds: -5
  updateStrategy:
    type: InvalidType
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: wrong
`)
	findings := runKindChecks(data, "DaemonSet")
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["apps/daemonset-min-ready-seconds-invalid"] {
		t.Error("expected daemonset-min-ready-seconds-invalid finding")
	}
	if !ruleIDs["apps/daemonset-update-strategy-invalid"] {
		t.Error("expected daemonset-update-strategy-invalid finding")
	}
}

func TestValidateDaemonSet_Clean(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
spec:
  minReadySeconds: 10
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
`)
	findings := runKindChecks(data, "DaemonSet")
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean daemonset, got %d: %v", len(findings), findings)
	}
}

func TestValidateDaemonSet_NonDaemonSet(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: test
`)
	findings := runKindChecks(data, "DaemonSet")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateDaemonSet_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := runKindChecks(data, "DaemonSet")
	if len(findings) > 0 {
		t.Errorf("expected nil or empty for invalid YAML, got %v", findings)
	}
}
