package syncopts

import (
	"strings"
	"testing"
)

func TestValidateReader_Builtin(t *testing.T) {
	data := `kind: Deployment
apiVersion: apps/v1
metadata:
  name: d
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for builtin: %v", errs)
	}
}

func TestValidateReader_CRD_NoAnnotation(t *testing.T) {
	data := `kind: ArgoCD
apiVersion: argoproj.io/v1alpha1
metadata:
  name: cd
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

func TestValidateReader_CRD_Annotation(t *testing.T) {
	data := `kind: ArgoCD
apiVersion: argoproj.io/v1alpha1
metadata:
  name: cd
  annotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors with annotation: %v", errs)
	}
}

func TestDeduplicatedError_String(t *testing.T) {
	d := DeduplicatedError{APIVersion: "argoproj.io/v1alpha1", Kind: "ArgoCD", Name: "cd", Count: 2}
	if !strings.Contains(d.String(), "missing argocd.argoproj.io/sync-options") {
		t.Errorf("unexpected string: %q", d.String())
	}
}

func TestExtractGroup(t *testing.T) {
	cases := map[string]string{
		"v1": "", "apps/v1": "apps", "argoproj.io/v1alpha1": "argoproj.io",
	}
	for in, want := range cases {
		if got := extractGroup(in); got != want {
			t.Errorf("extractGroup(%q) = %q", in, got)
		}
	}
}
