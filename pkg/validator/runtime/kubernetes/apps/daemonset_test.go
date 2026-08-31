package apps

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestDaemonSet_Check_IDAndMetadata(t *testing.T) {
	tests := []struct {
		check   runtime.Check
		wantID  string
		wantCat string
	}{
		{newDaemonSetSelectorInvalidCheck(), "kubernetes/apps/daemonset-selector-invalid", "apps"},
		{newDaemonSetUpdateStrategyInvalidCheck(), "kubernetes/apps/daemonset-update-strategy-invalid", "apps"},
		{newDaemonSetMinReadySecondsInvalidCheck(), "kubernetes/apps/daemonset-min-ready-seconds-invalid", "apps"},
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
	if !ruleIDs["kubernetes/apps/daemonset-min-ready-seconds-invalid"] {
		t.Error("expected daemonset-min-ready-seconds-invalid finding")
	}
	if !ruleIDs["kubernetes/apps/daemonset-update-strategy-invalid"] {
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
