package cel

import (
	"testing"
)

func TestCompileRules(t *testing.T) {
	// Test that CompileRules doesn't panic with an empty directory.
	rules, err := CompileRules("/nonexistent")
	if err != nil {
		// Expected - directory doesn't exist.
		t.Logf("CompileRules returned error (expected): %v", err)
	}
	if rules == nil {
		t.Fatal("CompileRules returned nil rules")
	}
	if rules.Rules == nil {
		t.Fatal("rules.Rules is nil")
	}
	if rules.byFile == nil {
		t.Fatal("rules.byFile is nil")
	}
}

func TestSplitDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "single document",
			input:    "kind: Deployment\napiVersion: apps/v1\n",
			expected: 1,
		},
		{
			name:     "multi document",
			input:    "kind: Deployment\napiVersion: apps/v1\n---\nkind: Service\napiVersion: v1\n",
			expected: 2,
		},
		{
			name:     "empty",
			input:    "",
			expected: 1,
		},
		{
			name:     "only separator",
			input:    "---\n",
			expected: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			docs := splitDocuments([]byte(tc.input))
			if len(docs) != tc.expected {
				t.Errorf("expected %d documents, got %d", tc.expected, len(docs))
			}
		})
	}
}

func TestExtractResourceMeta(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantKind   string
		wantAPIVer string
	}{
		{
			name: "complete",
			input: map[string]interface{}{
				"kind":       "Deployment",
				"apiVersion": "apps/v1",
				"metadata":   map[string]interface{}{"name": "test"},
			},
			wantKind:   "Deployment",
			wantAPIVer: "apps/v1",
		},
		{
			name: "missing apiVersion",
			input: map[string]interface{}{
				"kind":     "Service",
				"metadata": map[string]interface{}{"name": "test"},
			},
			wantKind:   "Service",
			wantAPIVer: "",
		},
		{
			name: "missing kind",
			input: map[string]interface{}{
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{"name": "test"},
			},
			wantKind:   "",
			wantAPIVer: "v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, apiVersion := extractResourceMeta(tc.input)
			if kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", kind, tc.wantKind)
			}
			if apiVersion != tc.wantAPIVer {
				t.Errorf("apiVersion = %q, want %q", apiVersion, tc.wantAPIVer)
			}
		})
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		wantName string
	}{
		{
			name: "with name",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "test-deployment"},
			},
			wantName: "test-deployment",
		},
		{
			name: "missing name",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{},
			},
			wantName: "<unknown>",
		},
		{
			name:     "missing metadata",
			input:    map[string]interface{}{},
			wantName: "<unknown>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name := extractName(tc.input)
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
		})
	}
}

func TestResultMerge(t *testing.T) {
	r1 := &Result{
		Valid:   5,
		Invalid: 2,
		Errors:  1,
		Details: []FileResult{{SchemaFile: "test.json", Resources: 5}},
	}
	r2 := &Result{
		Valid:   3,
		Invalid: 1,
		Errors:  0,
		Details: []FileResult{{SchemaFile: "test2.json", Resources: 3}},
	}

	r1.Merge(r2)

	if r1.Valid != 8 {
		t.Errorf("Valid = %d, want 8", r1.Valid)
	}
	if r1.Invalid != 3 {
		t.Errorf("Invalid = %d, want 3", r1.Invalid)
	}
	if r1.Errors != 1 {
		t.Errorf("Errors = %d, want 1", r1.Errors)
	}
	if len(r1.Details) != 2 {
		t.Errorf("Details len = %d, want 2", len(r1.Details))
	}
}

func TestResultMergeNil(t *testing.T) {
	r := &Result{Valid: 5}
	r.Merge(nil)
	if r.Valid != 5 {
		t.Errorf("Valid = %d, want 5", r.Valid)
	}
}

func TestResultSummary(t *testing.T) {
	r := &Result{
		Valid:   5,
		Invalid: 2,
		Errors:  1,
		Details: []FileResult{
			{SchemaFile: "test.json", Resources: 5, Failures: 0, Errors: 0},
			{SchemaFile: "fail.json", Resources: 2, Failures: 3, Rules: []Violation{
				{Resource: "Deployment/test", Rule: "size(self) > 0", Message: "must be non-empty", Field: "spec"},
			}},
			{SchemaFile: "error.json", Resources: 0, Failures: 0, Errors: 1},
		},
	}

	summary := r.Summary()
	if summary == "" {
		t.Fatal("Summary is empty")
	}
}

func TestCompileCEL(t *testing.T) {
	// Test that simple CEL expressions compile.
	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "simple comparison",
			source:  "size(self) > 0",
			wantErr: false,
		},
		{
			name:    "ip check",
			source:  "isIP(self)",
			wantErr: false,
		},
		{
			name:    "oldSelf rule (fails to compile - oldSelf not declared)",
			source:  "self == oldSelf",
			wantErr: true,
		},
		{
			name:    "invalid syntax",
			source:  "size(self",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileCEL(tc.source)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBytes(t *testing.T) {
	// Test that ValidateBytes handles empty rules gracefully.
	compiled := &CompiledRules{
		Rules:  make(map[string]map[string][]*Rule),
		byFile: make(map[string][]*Rule),
	}

	data := []byte("kind: Deployment\napiVersion: apps/v1\nmetadata:\n  name: test\n")
	result := ValidateBytes(data, compiled, "test.yaml")

	if result == nil {
		t.Fatal("ValidateBytes returned nil")
	}
	// With no rules compiled, all documents should be counted as valid.
	t.Logf("result: Valid=%d, Invalid=%d, Errors=%d, Details=%d", result.Valid, result.Invalid, result.Errors, len(result.Details))
	if result.Valid != 1 {
		t.Errorf("Valid = %d, want 1", result.Valid)
	}
}
