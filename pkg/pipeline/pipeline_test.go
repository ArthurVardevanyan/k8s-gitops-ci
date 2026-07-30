package pipeline

import (
	"errors"
	"testing"
)

func TestIsValidPR(t *testing.T) {
	if isValidPR("") {
		t.Error("empty PR invalid")
	}
	if isValidPR("{{ params.pr }}") {
		t.Error("placeholder PR invalid")
	}
	if !isValidPR("123") {
		t.Error("numeric PR valid")
	}
}

func TestResolveBaseRef(t *testing.T) {
	if got := resolveBaseRef("gh-readonly-queue/main/pr-1-abc"); got != "main" {
		t.Errorf("main base ref: %s", got)
	}
	if got := resolveBaseRef(""); got != "origin/main" {
		t.Errorf("default base ref: %s", got)
	}
}

func TestOptionsWorkers(t *testing.T) {
	o := Options{Concurrency: 4}
	if o.Workers() != 4 {
		t.Errorf("expected 4 workers")
	}
}

func TestToValidatorOptions_IncludePrefixes(t *testing.T) {
	opts := Options{IncludePrefixes: []string{"kubernetes/", "tekton/"}}
	vopts := toValidatorOptions(opts)
	if len(vopts.IncludePrefixes) != 2 {
		t.Fatalf("expected 2 include prefixes, got %v", vopts.IncludePrefixes)
	}
}

func TestShouldRunPRChecks_ValidPR(t *testing.T) {
	if !shouldRunPRChecks(Options{PR: "123"}) {
		t.Errorf("expected true for a valid PR")
	}
}

func TestShouldRunPRChecks_ValidPR_LintOnly(t *testing.T) {
	// Regression: title/signed-commit checks must still run in --lint-only
	// mode - they were previously (incorrectly) skipped entirely.
	if !shouldRunPRChecks(Options{PR: "123", LintOnly: true}) {
		t.Errorf("expected PR checks to run in lint-only mode for a valid PR")
	}
}

func TestShouldRunPRChecks_InvalidPR(t *testing.T) {
	if shouldRunPRChecks(Options{PR: ""}) {
		t.Errorf("expected false for an empty PR")
	}
	if shouldRunPRChecks(Options{PR: "{{ params.pr }}"}) {
		t.Errorf("expected false for a placeholder PR")
	}
}

func TestShouldRunPRChecks_MergeQueue(t *testing.T) {
	opts := Options{PR: "123", TargetBranch: "gh-readonly-queue/main/pr-1-abc"}
	if shouldRunPRChecks(opts) {
		t.Errorf("expected false in a merge-queue run")
	}
}

func TestShouldRunChecklistCheck_LintOnly(t *testing.T) {
	// The checklist check remains the one PR check that IS skipped in
	// lint-only mode.
	if shouldRunChecklistCheck(Options{PR: "123", LintOnly: true}) {
		t.Errorf("expected checklist check to be skipped in lint-only mode")
	}
}

func TestShouldRunChecklistCheck_NotLintOnly(t *testing.T) {
	if !shouldRunChecklistCheck(Options{PR: "123"}) {
		t.Errorf("expected checklist check to run outside lint-only mode")
	}
}

func TestShouldRunChecklistCheck_InvalidPR(t *testing.T) {
	if shouldRunChecklistCheck(Options{PR: ""}) {
		t.Errorf("expected false for an invalid PR regardless of lint-only")
	}
}

func TestComposeSections(t *testing.T) {
	res := &Result{TitleErr: errors.New("bad title")}
	sections := composeSections(res, Options{})
	if len(sections) == 0 {
		t.Errorf("expected sections")
	}
	if !sections[0].Error {
		t.Errorf("expected PR checks error")
	}
}
