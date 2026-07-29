package golangci

import (
	"strings"
	"testing"
)

func TestFilterGo(t *testing.T) {
	in := []string{"a.go", "b.yaml", "c_test.go"}
	out := FilterGo(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 go files, got %d", len(out))
	}
}

func TestFindModuleRoot(t *testing.T) {
	root, err := findModuleRoot(t.TempDir())
	if err == nil && root == "" {
		t.Errorf("expected error for missing go.mod")
	}
}

func TestFormatResult_Empty(t *testing.T) {
	r := &Result{}
	if formatResult(r) != "" {
		t.Errorf("expected empty result")
	}
}

func TestFormatResult_WithGoFmt(t *testing.T) {
	r := &Result{GoFmtIssues: []string{"a.go"}}
	out := formatResult(r)
	if out == "" || !contains(out, "gofmt") {
		t.Errorf("expected gofmt output: %s", out)
	}
}

func contains(s, sub string) bool { return strings.Index(s, sub) != -1 }
