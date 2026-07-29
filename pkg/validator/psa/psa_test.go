package psa

import (
	"strings"
	"testing"
)

func TestValidateReader_CompleteLabels(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: good
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/audit-version: latest
`
	errs := ValidateReader(strings.NewReader(data), "ns.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateReader_MissingAllLabels(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: bad
`
	errs := ValidateReader(strings.NewReader(data), "ns.yaml")
	if len(errs) != 1 || len(errs[0].MissingLabels) != 6 {
		t.Fatalf("expected 6 missing labels: %v", errs)
	}
}

func TestValidateReader_InvalidLevel(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: ns
  labels:
    pod-security.kubernetes.io/enforce: bad
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/audit-version: latest
`
	errs := ValidateReader(strings.NewReader(data), "ns.yaml")
	if len(errs) != 1 || !strings.Contains(errs[0].MissingLabels[0], "invalid value") {
		t.Errorf("expected invalid level error: %v", errs)
	}
}

func TestFormatComment(t *testing.T) {
	err := ValidationError{File: "ns.yaml", Name: "ns", MissingLabels: []string{"pod-security.kubernetes.io/enforce"}}
	s := FormatComment([]ValidationError{err})
	if !strings.Contains(s, Marker) || !strings.Contains(s, "Namespace") {
		t.Errorf("unexpected comment: %q", s)
	}
}

func TestFormatComment_Empty(t *testing.T) {
	if s := FormatComment(nil); s != "" {
		t.Errorf("expected empty: %q", s)
	}
}
