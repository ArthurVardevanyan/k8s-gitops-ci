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

func TestStripNSSelectors(t *testing.T) {
	in := []byte(`match:
  namespaceSelector:
    foo: bar
  resources:
    kinds:
    - Pod
`)
	out := stripNSSelectors(in)
	if strings.Contains(string(out), "namespaceSelector") {
		t.Errorf("namespaceSelector not stripped: %s", out)
	}
}
