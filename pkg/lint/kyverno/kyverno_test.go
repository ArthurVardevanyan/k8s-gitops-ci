package kyverno

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeduplicate(t *testing.T) {
	violations := []Violation{
		{Policy: "p1", Rule: "r1", Message: "m1", Resource: "res1", File: "a.yaml", Severity: "high"},
		{Policy: "p1", Rule: "r1", Message: "m1", Resource: "res2", File: "b.yaml", Severity: "high"},
		{Policy: "p2", Rule: "r2", Message: "m2", Resource: "res3", File: "c.yaml", Severity: "low"},
	}
	ded := Deduplicate(violations, 2)
	if len(ded) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(ded))
	}
	if ded[0].Count != 2 || len(ded[0].Resources) != 2 {
		t.Errorf("unexpected first group: %+v", ded[0])
	}
}

func TestFormatComment_Empty(t *testing.T) {
	if FormatComment(nil) != "" {
		t.Errorf("expected empty string for empty violations")
	}
}

func TestFormatComment_Rendered(t *testing.T) {
	v := []DeduplicatedViolation{{Policy: "p", Rule: "r", Message: "m", Severity: "high", Count: 1}}
	out := FormatComment(v)
	if !strings.Contains(out, Marker) || !strings.Contains(out, "high") {
		t.Errorf("unexpected comment: %s", out)
	}
}

func TestResultSummary(t *testing.T) {
	r := &Result{Pass: 1, Fail: 2, Warn: 3, Error: 4, Skip: 5}
	if r.Summary() != "1 pass, 2 fail, 3 warn, 4 error, 5 skip" {
		t.Errorf("unexpected summary: %s", r.Summary())
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 3) != "hel" {
		t.Errorf("unexpected truncate")
	}
}

func TestCollectYAML(t *testing.T) {
	d := t.TempDir()
	_ = []byte("")
	f1, _ := os.Create(filepath.Join(d, "a.yaml"))
	_ = f1.Close()
	f2, _ := os.Create(filepath.Join(d, "b.yml"))
	_ = f2.Close()
	files := CollectYAML(d)
	if len(files) != 2 {
		t.Errorf("expected 2 yaml files, got %d", len(files))
	}
}

func TestMergeResults(t *testing.T) {
	base := &Result{Pass: 1}
	mergeResults(base, &Result{Fail: 2})
	if base.Pass != 1 || base.Fail != 2 {
		t.Errorf("unexpected merged: %+v", base)
	}
}

func TestFindResourceFile_Match(t *testing.T) {
	files := []string{"/tmp/build/myapp/prod.yaml", "/tmp/build/myapp/dev.yaml"}
	if got := findResourceFile("prod", files); got != files[0] {
		t.Errorf("findResourceFile = %q, want %q", got, files[0])
	}
}

func TestFindResourceFile_NoMatch(t *testing.T) {
	files := []string{"/tmp/build/myapp/prod.yaml"}
	if got := findResourceFile("nonexistent", files); got != "" {
		t.Errorf("findResourceFile = %q, want empty", got)
	}
}

func TestFindResourceFile_EmptyName(t *testing.T) {
	if got := findResourceFile("", []string{"/tmp/a.yaml"}); got != "" {
		t.Errorf("findResourceFile = %q, want empty", got)
	}
}

func TestParseOutput_FileAttributionViaResourceName(t *testing.T) {
	out := []byte(`{"kind":"ClusterReport","results":[
		{"policy":"require-labels","rule":"check-labels","status":"fail","message":"missing label",
		 "resources":[{"kind":"Deployment","name":"foo","namespace":"bar"}]}
	]}`)
	res, err := parseOutput(out, []string{"/tmp/build/myapp/prod-foo.yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(res.Violations))
	}
	if res.Violations[0].File != "/tmp/build/myapp/prod-foo.yaml" {
		t.Errorf("File = %q, want the matched source file, not the resource Kind", res.Violations[0].File)
	}
	if res.Violations[0].Resource != "Deployment/foo" {
		t.Errorf("Resource = %q, want %q", res.Violations[0].Resource, "Deployment/foo")
	}
}

func TestParseOutput_ExcludedRuleDropped(t *testing.T) {
	orig := ExcludedRules
	defer func() { ExcludedRules = orig }()
	ExcludedRules = map[string][]string{"excluded-policy": nil}

	out := []byte(`{"kind":"ClusterReport","results":[
		{"policy":"excluded-policy","rule":"any-rule","status":"fail","message":"m",
		 "resources":[{"kind":"Pod","name":"foo"}]},
		{"policy":"kept-policy","rule":"any-rule","status":"fail","message":"m",
		 "resources":[{"kind":"Pod","name":"bar"}]}
	]}`)
	res, err := parseOutput(out, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Violations) != 1 || res.Violations[0].Policy != "kept-policy" {
		t.Fatalf("expected only the non-excluded policy's violation to survive, got %+v", res.Violations)
	}
	// The excluded result's pass/fail counters must not be tallied either.
	if res.Fail != 1 {
		t.Errorf("Fail = %d, want 1 (excluded result shouldn't be counted)", res.Fail)
	}
}

func TestIsExcludedRule(t *testing.T) {
	orig := ExcludedRules
	defer func() { ExcludedRules = orig }()
	ExcludedRules = map[string][]string{
		"policy-with-all-rules-excluded": nil,
		"policy-with-one-rule-excluded":  {"specific-rule"},
	}
	if !isExcludedRule("policy-with-all-rules-excluded", "any-rule") {
		t.Error("expected an empty rule slice to exclude every rule under that policy")
	}
	if !isExcludedRule("policy-with-one-rule-excluded", "specific-rule") {
		t.Error("expected the specifically-listed rule to be excluded")
	}
	if isExcludedRule("policy-with-one-rule-excluded", "other-rule") {
		t.Error("expected a different rule under the same policy to remain included")
	}
	if isExcludedRule("unconfigured-policy", "any-rule") {
		t.Error("expected an unconfigured policy to never be excluded")
	}
}
