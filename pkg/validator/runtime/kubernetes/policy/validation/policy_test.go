package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// The selector must be validated as a structured LabelSelector. It was
// previously flattened to a string and parsed with labels.Parse, which has
// no representation for matchExpressions - so every operator/values rule
// was skipped. The old fixture passed a bare string selector, which is not
// a valid PodDisruptionBudget shape at all.
func TestPDBSelectorCheck(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{
			"invalid matchLabels key",
			`  selector:
    matchLabels:
      "invalid key with spaces": myapp
  minAvailable: 1`,
		},
		{
			// Unreachable through a stringified selector: this is the case
			// the previous implementation silently accepted.
			"invalid matchExpressions key",
			`  selector:
    matchExpressions:
    - key: "invalid key with spaces"
      operator: In
      values:
      - val
  minAvailable: 1`,
		},
		{
			"In operator with no values",
			`  selector:
    matchExpressions:
    - key: app
      operator: In
      values: []
  minAvailable: 1`,
		},
		{
			"unknown operator",
			`  selector:
    matchExpressions:
    - key: app
      operator: Sometimes
      values:
      - val
  minAvailable: 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("kind: PodDisruptionBudget\nmetadata:\n  name: test\nspec:\n" + tt.spec + "\n")
			findings := selectorInvalidCheck{}.Run(data, "test.yaml")
			if len(findings) == 0 {
				t.Fatalf("expected at least 1 finding, got none")
			}
			if findings[0].RuleID != "policy/selector-invalid" {
				t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
			}
			if findings[0].Kind != "PodDisruptionBudget" {
				t.Errorf("unexpected kind: %s", findings[0].Kind)
			}
		})
	}
}

func TestPDBSelectorValid(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  minAvailable: 1
`)
	check := selectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPDBSelectorNonMatchingKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := selectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-matching kind, got %d: %v", len(findings), findings)
	}
}

func TestPDBMinAvailableCheck(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  minAvailable: -1
`)
	check := minAvailableInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "policy/min-available-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PodDisruptionBudget" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPDBMinAvailableValid(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  minAvailable: 1
`)
	check := minAvailableInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPDBMinAvailableNonMatchingKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := minAvailableInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-matching kind, got %d: %v", len(findings), findings)
	}
}

func TestPDBMaxUnavailableCheck(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  maxUnavailable: -1
`)
	check := maxUnavailableInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "policy/max-unavailable-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PodDisruptionBudget" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPDBMaxUnavailableValid(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  maxUnavailable: 1
`)
	check := maxUnavailableInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPDBMaxUnavailableNonMatchingKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := maxUnavailableInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-matching kind, got %d: %v", len(findings), findings)
	}
}

func TestPDBMinAndMaxSpecifiedCheck(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  minAvailable: 1
  maxUnavailable: 1
`)
	check := minAndMaxSpecifiedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "policy/min-and-max-specified" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PodDisruptionBudget" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPDBMinAndMaxSpecifiedValid(t *testing.T) {
	data := []byte(`kind: "PodDisruptionBudget"
metadata:
  name: "test"
spec:
  selector:
    matchLabels:
      app: "test"
  minAvailable: 1
`)
	check := minAndMaxSpecifiedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPDBMinAndMaxSpecifiedNonMatchingKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := minAndMaxSpecifiedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-matching kind, got %d: %v", len(findings), findings)
	}
}

func TestAllChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		selectorInvalidCheck{},
		minAvailableInvalidCheck{},
		maxUnavailableInvalidCheck{},
		minAndMaxSpecifiedCheck{},
	}
	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if c.Category() == "" {
			t.Errorf("check %T has empty Category", c)
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.Kinds()) == 0 {
			t.Errorf("check %T should declare Kinds", c)
		}
	}
}
