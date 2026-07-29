package kubeconform

import (
	"strings"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.KubernetesVersion != "1.29.0" || !opts.Strict || !opts.UseSchemas {
		t.Errorf("unexpected defaults: %+v", opts)
	}
}

func TestDeduplicate(t *testing.T) {
	r := &Result{Details: []FileResult{
		{Filename: "a.yaml", Errors: []string{"err1"}},
		{Filename: "b.yaml", Errors: []string{"err1"}},
	}}
	ded := r.Deduplicate()
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}

func TestResultSummary(t *testing.T) {
	r := &Result{Valid: 1, Invalid: 0, Errors: 0, Skipped: 0}
	if !strings.Contains(r.Summary(), "1 valid") {
		t.Errorf("summary missing valid: %s", r.Summary())
	}
}

func TestValidateFiles_EmptyList(t *testing.T) {
	r, err := ValidateFiles([]string{}, DefaultOptions())
	if err != nil || r.Valid != 0 {
		t.Errorf("expected empty result: %+v err %v", r, err)
	}
}


