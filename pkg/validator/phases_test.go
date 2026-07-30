package validator

import "testing"

func TestToIDSet_Empty(t *testing.T) {
	if got := toIDSet(nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
	if got := toIDSet([]string{}); got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

func TestToIDSet_Populated(t *testing.T) {
	set := toIDSet([]string{"sync-options", "golangci"})
	if !set["sync-options"] || !set["golangci"] {
		t.Fatalf("expected both ids present, got %v", set)
	}
	if set["kyverno"] {
		t.Errorf("expected kyverno absent")
	}
}

func TestStepEnabled_DefaultOnStepEnabledByDefault(t *testing.T) {
	if !stepEnabled("sync-options", nil, nil) {
		t.Errorf("default-on step should be enabled with no lists set")
	}
}

func TestStepEnabled_DefaultOnStepDisabledViaDisabledChecks(t *testing.T) {
	disabled := toIDSet([]string{"sync-options"})
	if stepEnabled("sync-options", disabled, nil) {
		t.Errorf("expected sync-options disabled")
	}
	// An unrelated ID must remain enabled.
	if !stepEnabled(stepGolangci, disabled, nil) {
		t.Errorf("expected golangci to remain enabled")
	}
}

func TestStepEnabled_DefaultOffStepDisabledByDefault(t *testing.T) {
	if stepEnabled(stepKyverno, nil, nil) {
		t.Errorf("kyverno should default to disabled")
	}
}

func TestStepEnabled_DefaultOffStepEnabledViaEnabledChecks(t *testing.T) {
	enabled := toIDSet([]string{stepKyverno})
	if !stepEnabled(stepKyverno, nil, enabled) {
		t.Errorf("expected kyverno enabled once listed in EnabledChecks")
	}
}

func TestStepEnabled_EnabledChecksHasNoEffectOnDefaultOnSteps(t *testing.T) {
	// Listing a default-on step in EnabledChecks is a no-op - it's already
	// enabled, and only DisabledChecks can turn it off.
	enabled := toIDSet([]string{stepGolangci})
	if !stepEnabled(stepGolangci, nil, enabled) {
		t.Errorf("expected golangci enabled")
	}
	disabled := toIDSet([]string{stepGolangci})
	if stepEnabled(stepGolangci, disabled, enabled) {
		t.Errorf("DisabledChecks must win over an unrelated EnabledChecks entry for a default-on step")
	}
}
