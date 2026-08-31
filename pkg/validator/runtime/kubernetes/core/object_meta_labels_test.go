package core

import (
	"strings"
	"testing"
)

func TestObjectMetaLabelsInvalidCheck(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     int
		contains string
	}{
		{
			name: "valid labels",
			manifest: `kind: Deployment
metadata:
  name: test
  labels:
    app.kubernetes.io/name: my-app
    tier: backend
`,
			want: 0,
		},
		{
			name: "no labels",
			manifest: `kind: Deployment
metadata:
  name: test
`,
			want: 0,
		},
		{
			name: "invalid label key",
			manifest: `kind: Deployment
metadata:
  name: test
  labels:
    "not a valid key": value
`,
			want:     1,
			contains: "invalid label key",
		},
		{
			// Label keys are case-sensitive and uppercase is a valid
			// qualified name, so this must NOT be reported.
			name: "uppercase label key is valid",
			manifest: `kind: Deployment
metadata:
  name: test
  labels:
    MyKey: value
`,
			want: 0,
		},
		{
			name: "invalid label value",
			manifest: `kind: Deployment
metadata:
  name: test
  labels:
    app: "value with spaces"
`,
			want:     1,
			contains: "invalid label value",
		},
		{
			// Upstream reports key and value independently, so a label
			// that is bad in both dimensions yields two findings.
			name: "invalid key and value together",
			manifest: `kind: Deployment
metadata:
  name: test
  labels:
    "bad key": "bad value"
`,
			want: 2,
		},
	}

	check := newObjectMetaLabelsInvalidCheck()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := check.Run([]byte(tt.manifest), "test.yaml")
			if len(findings) != tt.want {
				t.Fatalf("expected %d findings, got %d: %v", tt.want, len(findings), findings)
			}
			if tt.contains != "" && !strings.Contains(findings[0].Message, tt.contains) {
				t.Errorf("expected message containing %q, got %q", tt.contains, findings[0].Message)
			}
		})
	}
}

func TestObjectMetaAnnotationsInvalidCheck(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     int
		contains string
	}{
		{
			name: "valid annotations",
			manifest: `kind: Deployment
metadata:
  name: test
  annotations:
    argocd.argoproj.io/sync-wave: "1"
`,
			want: 0,
		},
		{
			// Annotation keys are lowercased before validation upstream,
			// so an uppercase key must be accepted. This is the branch
			// that differs from labels.
			name: "uppercase annotation key is valid",
			manifest: `kind: Deployment
metadata:
  name: test
  annotations:
    MyAnnotation/Key: value
`,
			want: 0,
		},
		{
			name: "invalid annotation key",
			manifest: `kind: Deployment
metadata:
  name: test
  annotations:
    "not a valid key": value
`,
			want:     1,
			contains: "invalid annotation key",
		},
		{
			// An annotation value may be anything, including spaces.
			name: "arbitrary annotation value is valid",
			manifest: `kind: Deployment
metadata:
  name: test
  annotations:
    description: "any value at all, with spaces"
`,
			want: 0,
		},
	}

	check := newObjectMetaAnnotationsInvalidCheck()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := check.Run([]byte(tt.manifest), "test.yaml")
			if len(findings) != tt.want {
				t.Fatalf("expected %d findings, got %d: %v", tt.want, len(findings), findings)
			}
			if tt.contains != "" && !strings.Contains(findings[0].Message, tt.contains) {
				t.Errorf("expected message containing %q, got %q", tt.contains, findings[0].Message)
			}
		})
	}
}

// TestObjectMetaAnnotationsSizeLimit exercises the ValidateAnnotationsSize
// branch, which is reported once for the object rather than per annotation.
func TestObjectMetaAnnotationsSizeLimit(t *testing.T) {
	underLimit := strings.Repeat("b", totalAnnotationSizeLimitB-100)
	manifest := "kind: Deployment\nmetadata:\n  name: test\n  annotations:\n    a: " + underLimit + "\n"
	findings := newObjectMetaAnnotationsInvalidCheck().Run([]byte(manifest), "test.yaml")
	if len(findings) != 0 {
		t.Fatalf("expected no findings just under the limit, got %d", len(findings))
	}

	overLimit := strings.Repeat("b", totalAnnotationSizeLimitB+1)
	manifest = "kind: Deployment\nmetadata:\n  name: test\n  annotations:\n    a: " + overLimit + "\n"
	findings = newObjectMetaAnnotationsInvalidCheck().Run([]byte(manifest), "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding over the limit, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "too long") {
		t.Errorf("expected a too-long message, got %q", findings[0].Message)
	}
}

// TestObjectMetaLabelsKindIndependent documents that these two checks are not
// kind-scoped: unlike the name rules they apply to every object, including
// custom resources.
func TestObjectMetaLabelsKindIndependent(t *testing.T) {
	if kinds := newObjectMetaLabelsInvalidCheck().Kinds(); len(kinds) != 0 {
		t.Errorf("labels check must declare no kinds, got %v", kinds)
	}
	if kinds := newObjectMetaAnnotationsInvalidCheck().Kinds(); len(kinds) != 0 {
		t.Errorf("annotations check must declare no kinds, got %v", kinds)
	}

	manifest := `kind: SomeCustomResource
metadata:
  name: test
  labels:
    "bad key": value
`
	findings := newObjectMetaLabelsInvalidCheck().Run([]byte(manifest), "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected the check to apply to a custom resource, got %d findings", len(findings))
	}
}
