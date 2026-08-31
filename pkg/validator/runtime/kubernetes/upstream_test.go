package kubernetes

import (
	"regexp"
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
// which runs as step 5/10 of `task ci`. Both halves are enforced: this test
// covers the citations offline, and CI re-derives every digest from the
// upstream tag.
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

// checkIDPattern matches a check ID as it appears in note prose.
//
// IDs are "<family>/<category>/<rule>", so this must be greedy across all
// three segments: a two-segment pattern would match the "family/category"
// prefix of a three-segment ID and then report that truncation as an
// unresolvable deferral, which is exactly what it did when the family prefix
// was introduced.
var checkIDPattern = regexp.MustCompile(`[a-z0-9]+(?:-[a-z0-9]+)*(?:/[a-z0-9]+(?:-[a-z0-9]+)*){2}`)

// coveredByTargets returns every check ID a note defers a branch to.
//
// A deferral is written "covered by X" but frequently names several checks
// ("covered by a/b, c/d and e/f"), so this reads to the end of the sentence
// rather than taking the first ID after the phrase. Matching only the first
// would let every later entry in a list go unchecked while the test still
// reported a pass - which is how the defects this test exists to catch got in.
func coveredByTargets(note string) []string {
	var out []string
	for _, sentence := range strings.Split(note, ". ") {
		i := strings.Index(sentence, "covered by ")
		if i < 0 {
			continue
		}
		out = append(out, checkIDPattern.FindAllString(sentence[i:], -1)...)
	}
	return out
}

// TestCoveredByClaimsResolve enforces the one part of a note's prose that can
// be checked mechanically.
//
// A note is required prose, but nothing verifies it. `task verify:upstream-refs`
// re-derives the digest of the cited function, which proves the function has
// not changed - it says nothing about whether the note describes what the
// check actually implements. Notes drift from the code silently, and every
// such drift found so far has concealed a real gap: a note claiming a branch
// was "covered by" a sibling that did not in fact cover it, and a note
// describing an upstream Required branch that does not exist.
//
// The "covered by X" form is the one claim with enough structure to test. If
// a note defers a branch to another check, that check must at least exist and
// cite the same upstream function - otherwise the deferral is to nowhere, and
// the branch is unvalidated while the note says it is handled.
//
// This is deliberately a weak guarantee, and it is worth being precise about
// its limit: it proves the deferral target exists and reads the same upstream
// function. It cannot prove the target implements the specific branch that was
// deferred to it. A note that defers the hostPort branch to a check which
// reads the same function but only validates containerPort would still pass
// here. The rest of a note's accuracy is not mechanically checkable and is
// verified by reading it against upstream.
func TestCoveredByClaimsResolve(t *testing.T) {
	refs := runtime.AllRefs()
	if len(refs) == 0 {
		t.Fatal("no upstream refs registered; the walk cannot prove anything")
	}

	var claims int
	for id, ref := range refs {
		for _, target := range coveredByTargets(ref.Note) {
			claims++

			targetRef, ok := refs[target]
			if !ok {
				t.Errorf("check %q defers a branch to %q, which is not a registered check: "+
					"the branch is unvalidated while the note claims it is handled", id, target)
				continue
			}
			if target == id {
				t.Errorf("check %q defers a branch to itself", id)
				continue
			}
			// Same upstream function, or the deferral crosses to a rule
			// reading different code and cannot cover the branch at all.
			if !sharesFunction(ref, targetRef) {
				t.Errorf("check %q defers a branch to %q, but %q cites %s %v while %q cites %s %v; "+
					"a check reading a different function cannot cover this one's branches",
					id, target, id, ref.Path, ref.Functions, target, targetRef.Path, targetRef.Functions)
			}
		}
	}

	if claims == 0 {
		t.Fatal("no \"covered by\" claims found; if the phrasing changed, this test silently stopped checking anything")
	}
}

func sharesFunction(a, b runtime.UpstreamRef) bool {
	if a.Path != b.Path {
		return false
	}
	for _, af := range a.Functions {
		for _, bf := range b.Functions {
			if af == bf {
				return true
			}
		}
	}
	return false
}

// enumBackedChecks maps each check whose accepted values come from a
// package-level upstream set to the name of that set.
//
// The distinction this table encodes is not cosmetic. Upstream expresses an
// enum in one of two shapes, and only one of them is covered by the primary
// digest:
//
//	switch policy {                        // members live IN the function
//	case core.PullAlways, core.PullNever:  // -> the function digest covers them
//
//	if !supportedServiceType.Has(t) {      // members live in a package-level set
//	                                       // -> the function digest does NOT cover them
//
// For the second shape the function body can be byte-identical across releases
// while the accepted values change underneath it. A check that copies those
// members would keep rejecting a value the API server now accepts, and
// `task verify:upstream-refs` would stay green the whole time, because the
// function it digests never moved. Citing the set closes that gap: the set is
// verified exactly like the function, so an upstream addition fails CI here
// instead of surfacing as a false rejection in a consumer's pipeline.
//
// Checks absent from this table are absent on purpose: their upstream rule
// decides acceptance inside the function (a switch, or a set built locally,
// as validateMountPropagation does), so the primary digest already covers it.
// Citing the set that only formats the error message - `supportedPullPolicies`
// is the example - would imply a guarantee the citation does not provide.
//
// This is a maintained table, not a derivation. It cannot notice a *new* check
// that copies a set without adding a row, because whether a check hard-codes an
// enum is a property of its own source, not of the registry. It does prevent
// the failure that matters in practice: an existing set citation being dropped
// or pointed at the wrong file.
var enumBackedChecks = map[string]string{
	"kubernetes/service/type-invalid":                                    "supportedServiceType",
	"kubernetes/service/session-affinity-invalid":                        "supportedSessionAffinityType",
	"kubernetes/ingress/path-type-invalid":                               "supportedPathTypes",
	"kubernetes/persistent-volume/access-modes-invalid":                  "supportedAccessModes",
	"kubernetes/persistent-volume-claim/access-modes-invalid":            "supportedAccessModes",
	"kubernetes/persistent-volume/volume-mode-invalid":                   "supportedVolumeModes",
	"kubernetes/persistent-volume-claim/volume-mode-invalid":             "supportedVolumeModes",
	"kubernetes/storage-class/reclaim-policy-invalid":                    "supportedReclaimPolicy",
	"kubernetes/storage-class/volume-binding-mode-invalid":               "supportedVolumeBindingModes",
	"kubernetes/admissionregistration/failure-policy-invalid":            "supportedFailurePolicies",
	"kubernetes/admissionregistration/validating-failure-policy-invalid": "supportedFailurePolicies",
	"kubernetes/container/port-protocol-invalid":                         "supportedPortProtocols",
}

func TestEnumBackedChecksCiteTheirUpstreamSet(t *testing.T) {
	refs := runtime.AllRefs()
	for id, setName := range enumBackedChecks {
		ref, ok := refs[id]
		if !ok {
			t.Errorf("enumBackedChecks names %q, which is not a registered check; "+
				"remove the row or fix the ID", id)
			continue
		}
		var found bool
		for _, a := range ref.Additional {
			for _, fn := range a.Functions {
				if fn == setName {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("check %q takes its accepted values from the upstream set %q, but does not "+
				"cite it in Additional: an upstream change to the set alone would leave this "+
				"check's digests unchanged while it rejects a value the API server accepts",
				id, setName)
		}
	}
}
