package validator

import "testing"

func TestBuiltinExemptSelectors_Default(t *testing.T) {
	sels := builtinExemptSelectors()
	if len(sels) != 2 {
		t.Fatalf("expected 2 built-in selectors, got %d: %+v", len(sels), sels)
	}
	want := map[string]bool{"sync-options": false, "namespace": false}
	for _, s := range sels {
		if s.Kind != "PipelineRun" {
			t.Errorf("selector %+v: expected Kind=PipelineRun", s)
		}
		if s.Dir != ".tekton" {
			t.Errorf("selector %+v: expected Dir=.tekton", s)
		}
		if _, ok := want[s.Check]; !ok {
			t.Errorf("unexpected selector Check %q", s.Check)
		}
		want[s.Check] = true
	}
	for check, seen := range want {
		if !seen {
			t.Errorf("expected a built-in selector for check %q", check)
		}
	}
}

func TestBuiltinExemptSelectors_DisabledWhenEmpty(t *testing.T) {
	orig := TektonPACDir
	defer func() { TektonPACDir = orig }()

	TektonPACDir = ""
	if sels := builtinExemptSelectors(); sels != nil {
		t.Errorf("expected no built-in selectors when TektonPACDir is empty, got %+v", sels)
	}
}

func TestBuiltinExemptSelectors_CustomDir(t *testing.T) {
	orig := TektonPACDir
	defer func() { TektonPACDir = orig }()

	TektonPACDir = "/custom-pac-dir/"
	sels := builtinExemptSelectors()
	for _, s := range sels {
		if s.Dir != "custom-pac-dir" {
			t.Errorf("expected trimmed custom dir, got %q", s.Dir)
		}
	}
}
