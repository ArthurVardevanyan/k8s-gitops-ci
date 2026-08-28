package kubernetes

import (
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// upstreamRoots are the modules a runtime check may cite from.
//
// A root alone is not sufficient: "pkg/apis/" also contains types.go,
// defaults.go, conversion.go and register.go, none of which are validation
// code, so a prefix test would accept a citation to any of them and still
// claim to have proven the path is "a real API-validation location". The
// path must additionally sit in a /validation/ directory - see
// isUpstreamValidationPath.
var upstreamRoots = []string{
	"pkg/apis/",
	"staging/src/k8s.io/apimachinery/pkg/api/validation/",
	// meta/v1 validation is where the shared object-metadata path above
	// delegates the label rules to (ValidateLabels, ValidateLabelName).
	"staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/",
	"staging/src/k8s.io/apiextensions-apiserver/pkg/apis/",
}

// isUpstreamValidationPath reports whether path is an API-server validation
// source file: under a known root, inside a /validation/ directory, and a Go
// file.
func isUpstreamValidationPath(path string) bool {
	var rooted bool
	for _, root := range upstreamRoots {
		if strings.HasPrefix(path, root) {
			rooted = true
			break
		}
	}
	if !rooted {
		return false
	}
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false
	}
	return strings.Contains(path, "/validation/")
}

// TestEveryRuntimeCheckCitesUpstream is the offline half of the 1:1 standard.
//
// It walks the registry rather than scanning source text, which matters: the
// admissionregistration checks compose their IDs at runtime from a shared
// base ("admissionregistration/" + idPrefix + suffix), so no grep or AST pass
// over the source can enumerate them reliably.
//
// The online half - proving the cited functions actually exist upstream and
// have not changed since they were validated - is `task verify:upstream-refs`,
// which is kept out of `task ci` because it needs network access.
func TestEveryRuntimeCheckCitesUpstream(t *testing.T) {
	var runtimeChecks int
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		runtimeChecks++

		ref, ok := runtime.Ref(c.ID())
		if !ok {
			t.Errorf("check %q has no UpstreamRef; register it via runtime.RegisterAll with an entry in the package's upstream_refs.go", c.ID())
			continue
		}
		if err := ref.Validate(); err != nil {
			t.Errorf("check %q has an invalid UpstreamRef: %v", c.ID(), err)
			continue
		}

		if !isUpstreamValidationPath(ref.Path) {
			t.Errorf("check %q cites %q, which is not a Kubernetes API validation source file "+
				"(must be under one of %s, inside a /validation/ directory, and a non-test .go file)",
				c.ID(), ref.Path, strings.Join(upstreamRoots, ", "))
		}
	}

	if runtimeChecks == 0 {
		t.Fatal("no runtime checks registered; the registry walk cannot prove anything")
	}
	if got := len(runtime.AllRefs()); got != runtimeChecks {
		t.Errorf("registered %d runtime checks but %d upstream refs; every check must have exactly one",
			runtimeChecks, got)
	}
}

// TestUpstreamRefValidateRejectsLineNumbers pins the no-line-numbers rule.
// Every incorrect citation in this repository's history was a stale line
// range - one file cited "validation.go:5200-5210" for a rule that actually
// lived 3000 lines away - so the format forbids them outright.
func TestUpstreamRefValidateRejectsLineNumbers(t *testing.T) {
	tests := []struct {
		name    string
		ref     runtime.UpstreamRef
		wantErr string
	}{
		{
			name: "line range rejected",
			ref: runtime.UpstreamRef{
				Path:        "pkg/apis/core/validation/validation.go:5200-5210",
				Functions:   []string{"ValidatePodSpec"},
				Digest:      "sha256:" + strings.Repeat("a", 64),
				ValidatedAt: "v1.36.3",
			},
			wantErr: "line number",
		},
		{
			name: "single line rejected",
			ref: runtime.UpstreamRef{
				Path:        "pkg/apis/core/validation/validation.go:42",
				Functions:   []string{"ValidatePodSpec"},
				Digest:      "sha256:" + strings.Repeat("a", 64),
				ValidatedAt: "v1.36.3",
			},
			wantErr: "line number",
		},
		{
			name: "missing functions rejected",
			ref: runtime.UpstreamRef{
				Path:        "pkg/apis/core/validation/validation.go",
				Digest:      "sha256:" + strings.Repeat("a", 64),
				ValidatedAt: "v1.36.3",
			},
			wantErr: "functions",
		},
		{
			name: "malformed digest rejected",
			ref: runtime.UpstreamRef{
				Path:        "pkg/apis/core/validation/validation.go",
				Functions:   []string{"ValidatePodSpec"},
				Digest:      "not-a-digest",
				ValidatedAt: "v1.36.3",
			},
			wantErr: "digest",
		},
		{
			name: "missing validated tag rejected",
			ref: runtime.UpstreamRef{
				Path:      "pkg/apis/core/validation/validation.go",
				Functions: []string{"ValidatePodSpec"},
				Digest:    "sha256:" + strings.Repeat("a", 64),
			},
			wantErr: "validatedAt",
		},
		{
			name: "well-formed ref accepted",
			ref: runtime.UpstreamRef{
				Path:        "pkg/apis/core/validation/validation.go",
				Functions:   []string{"ValidatePodSpec"},
				Digest:      "sha256:" + strings.Repeat("a", 64),
				ValidatedAt: "v1.36.3",
				Note:        "Ports the whole function.",
			},
		},
		{
			name: "missing note rejected",
			ref: runtime.UpstreamRef{
				Path:        "pkg/apis/core/validation/validation.go",
				Functions:   []string{"ValidatePodSpec"},
				Digest:      "sha256:" + strings.Repeat("a", 64),
				ValidatedAt: "v1.36.3",
			},
			wantErr: "note is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected ref to be valid, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error mentioning %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestIsUpstreamValidationPath pins the predicate itself. The previous
// prefix-only test accepted every file under pkg/apis/, so it would have
// passed a citation to defaults.go or conversion.go while reporting that it
// had verified the path was API-validation code.
func TestIsUpstreamValidationPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Real citations in use today.
		{"pkg/apis/core/validation/validation.go", true},
		{"pkg/apis/apps/validation/validation.go", true},
		{"staging/src/k8s.io/apimachinery/pkg/api/validation/objectmeta.go", true},
		{"staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go", true},
		{"staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation.go", true},

		// Under a valid root, but not validation code.
		{"pkg/apis/core/types.go", false},
		{"pkg/apis/core/v1/defaults.go", false},
		{"pkg/apis/core/v1/conversion.go", false},
		{"pkg/apis/core/register.go", false},

		// Validation code, but outside any allowed root.
		{"pkg/registry/core/pod/validation/validation.go", false},
		{"plugin/pkg/admission/validation/validation.go", false},

		// Not a Go source file, or a test file.
		{"pkg/apis/core/validation/validation_test.go", false},
		{"pkg/apis/core/validation/README.md", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isUpstreamValidationPath(tt.path); got != tt.want {
				t.Errorf("isUpstreamValidationPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
