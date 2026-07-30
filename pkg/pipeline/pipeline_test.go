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
