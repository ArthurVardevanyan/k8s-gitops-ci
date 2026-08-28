package kubernetes

import (
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// upstreamPathPrefixes are the locations a runtime check may cite. Anything
// outside these is not API-server validation code, so a check citing it is
// not the 1:1 port this family requires.
var upstreamPathPrefixes = []string{
	"pkg/apis/",
	"staging/src/k8s.io/apimachinery/pkg/api/validation/",
	"staging/src/k8s.io/apiextensions-apiserver/pkg/apis/",
	"staging/src/k8s.io/apiserver/pkg/util/webhook/",
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

		var allowed bool
		for _, prefix := range upstreamPathPrefixes {
			if strings.HasPrefix(ref.Path, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			t.Errorf("check %q cites %q, which is not a Kubernetes API validation location (allowed prefixes: %s)",
				c.ID(), ref.Path, strings.Join(upstreamPathPrefixes, ", "))
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
			},
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
