package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestPDBSelectorCheck(t *testing.T) {
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": "invalid=invalid[", "minAvailable": 1}
}`)
	check := selectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "policy/selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PodDisruptionBudget" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPDBSelectorValid(t *testing.T) {
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": 1}
}`)
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
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": -1}
}`)
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
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": 1}
}`)
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
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "maxUnavailable": -1}
}`)
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
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "maxUnavailable": 1}
}`)
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
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": 1, "maxUnavailable": 1}
}`)
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
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": 1}
}`)
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

func TestPDBSelectorAndPodTemplateHashCheck(t *testing.T) {
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": 1, "labels": {"app": "test"}}
}`)
	check := selectorAndPodTemplateHashInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "policy/selector-and-pod-template-hash-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "PodDisruptionBudget" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPDBSelectorAndPodTemplateHashValid(t *testing.T) {
	data := []byte(`{
  "kind": "PodDisruptionBudget",
  "metadata": {
    "name": "test"
  },
  "spec": {"selector": {"matchLabels": {"app": "test"}}, "minAvailable": 1, "labels": {"app": "other"}}
}`)
	check := selectorAndPodTemplateHashInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestPDBSelectorAndPodTemplateHashNonMatchingKind(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
`)
	check := selectorAndPodTemplateHashInvalidCheck{}
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
		selectorAndPodTemplateHashInvalidCheck{},
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
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}
