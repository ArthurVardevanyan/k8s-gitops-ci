package upstreamref

import (
	"errors"
	"strings"
	"testing"
)

// This package is the trust anchor for every runtime check: a check is
// accepted as a faithful port because the function it cites still hashes to
// the digest recorded next to it. If Digest were to stop noticing a change,
// every stale citation in the tree would keep verifying green and nothing
// else in the pipeline would object.
//
// So the properties asserted here are the ones the citation contract actually
// rests on, in both directions: what must NOT move the digest (comments,
// formatting, declaration order, unrelated code) and what MUST (any change to
// a cited body, and a cited name disappearing).

func mustDigest(t *testing.T, src string, fns ...string) string {
	t.Helper()
	d, err := Digest([]byte(src), fns)
	if err != nil {
		t.Fatalf("Digest(%v): %v", fns, err)
	}
	return d
}

// TestDigestIgnoresCommentsAndFormatting pins the normalization the contract
// depends on. Upstream rewords doc comments and reruns gofmt constantly; if
// those moved the digest, `verify:upstream-refs` would fail on every release
// for reasons that have nothing to do with validation behaviour, and the
// failures would be routinely dismissed - which is how a real change gets
// waved through.
func TestDigestIgnoresCommentsAndFormatting(t *testing.T) {
	plain := `package v

func Validate(x int) error {
	if x < 0 {
		return errBad
	}
	return nil
}
`
	commentedAndReformatted := `package v

// Validate rejects a negative x.
//
// This paragraph did not exist in the other version.
func Validate(x int) error {
	// leading comment
	if x < 0 {
		return errBad /* trailing */
	}

	return nil
}
`
	if a, b := mustDigest(t, plain, "Validate"), mustDigest(t, commentedAndReformatted, "Validate"); a != b {
		t.Errorf("comments/formatting moved the digest:\n plain      = %s\n commented  = %s", a, b)
	}
}

// TestDigestIgnoresSurroundingCode pins that only the cited functions are
// hashed. Upstream validation files are thousands of lines and change
// constantly; hashing the file would make every citation fail on every
// release.
func TestDigestIgnoresSurroundingCode(t *testing.T) {
	before := `package v

func Validate(x int) error { return nil }
`
	after := `package v

func Unrelated() string { return "added later" }

func Validate(x int) error { return nil }

var somethingElse = 42
`
	if a, b := mustDigest(t, before, "Validate"), mustDigest(t, after, "Validate"); a != b {
		t.Errorf("unrelated code moved the digest:\n before = %s\n after  = %s", a, b)
	}
}

// TestDigestIsStableAcrossDeclarationOrder pins the sort. Two functions cited
// together must hash the same however upstream orders them in the file, or a
// pure code move would be indistinguishable from a behaviour change.
func TestDigestIsStableAcrossDeclarationOrder(t *testing.T) {
	ab := `package v

func A() {}
func B() {}
`
	ba := `package v

func B() {}
func A() {}
`
	if a, b := mustDigest(t, ab, "A", "B"), mustDigest(t, ba, "A", "B"); a != b {
		t.Errorf("declaration order moved the digest:\n A,B = %s\n B,A = %s", a, b)
	}
}

// TestDigestIsIndependentOfCitationOrder pins that the order the check lists
// its functions in does not matter, so reordering the Functions slice is not
// mistaken for an upstream change.
func TestDigestIsIndependentOfCitationOrder(t *testing.T) {
	src := `package v

func A() {}
func B() {}
`
	if a, b := mustDigest(t, src, "A", "B"), mustDigest(t, src, "B", "A"); a != b {
		t.Errorf("citation order moved the digest:\n A,B = %s\n B,A = %s", a, b)
	}
}

// TestDigestDetectsBodyChanges is the property the whole mechanism exists for.
// Each case is a change a reviewer would want CI to stop on, and each is
// small enough that a coarser scheme might miss it.
func TestDigestDetectsBodyChanges(t *testing.T) {
	base := `package v

func Validate(x int) error {
	if x < 0 {
		return errBad
	}
	return nil
}
`
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "the comparison is inverted",
			src: `package v

func Validate(x int) error {
	if x > 0 {
		return errBad
	}
	return nil
}
`,
		},
		{
			name: "a boundary moves",
			src: `package v

func Validate(x int) error {
	if x < 1 {
		return errBad
	}
	return nil
}
`,
		},
		{
			name: "a branch is removed",
			src: `package v

func Validate(x int) error {
	return nil
}
`,
		},
		{
			name: "the signature changes",
			src: `package v

func Validate(x int64) error {
	if x < 0 {
		return errBad
	}
	return nil
}
`,
		},
	}

	want := mustDigest(t, base, "Validate")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustDigest(t, tt.src, "Validate"); got == want {
				t.Errorf("digest unchanged after %s: a real upstream change would verify green", tt.name)
			}
		})
	}
}

// TestDigestCoversVarDeclarations pins the second citable form. Several rules
// cite a package-level set (supportedServiceType and friends) precisely
// because acceptance is decided there rather than in the function body, so a
// change confined to the set must move the digest.
func TestDigestCoversVarDeclarations(t *testing.T) {
	two := `package v

var supported = sets.New("a", "b")
`
	three := `package v

var supported = sets.New("a", "b", "c")
`
	if a, b := mustDigest(t, two, "supported"), mustDigest(t, three, "supported"); a == b {
		t.Error("adding a member to a cited set did not move the digest; " +
			"a newly-accepted upstream value would keep being rejected with CI green")
	}

	// The alias form, e.g. `var ValidateNamespaceName = NameIsDNSLabel`.
	alias := `package v

var ValidateName = NameIsDNSSubdomain
`
	repointed := `package v

var ValidateName = NameIsDNSLabel
`
	if a, b := mustDigest(t, alias, "ValidateName"), mustDigest(t, repointed, "ValidateName"); a == b {
		t.Error("repointing a cited alias did not move the digest")
	}
}

// TestMissingFunctionIsReported pins the strongest signal the tool can give:
// the cited function is gone, so the check cannot be the port it claims. It
// must be an error rather than an empty digest, or a citation to a deleted
// function would silently hash to a constant and verify forever.
func TestMissingFunctionIsReported(t *testing.T) {
	src := `package v

func Present() {}
`
	_, err := Digest([]byte(src), []string{"Present", "Gone", "AlsoGone"})
	if err == nil {
		t.Fatal("expected an error for absent functions, got none")
	}
	var missing *MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected *MissingError, got %T: %v", err, err)
	}
	// Sorted, so the message is stable across runs.
	if len(missing.Functions) != 2 || missing.Functions[0] != "AlsoGone" || missing.Functions[1] != "Gone" {
		t.Errorf("missing = %v, want [AlsoGone Gone]", missing.Functions)
	}
	if !strings.Contains(err.Error(), "AlsoGone, Gone") {
		t.Errorf("error message does not name the absent functions: %q", err.Error())
	}
}

// TestMethodsAreNotMatched pins that a method is not mistaken for the
// package-level function of the same name. Citing `Validate` must not
// silently resolve to some type's `Validate` method.
func TestMethodsAreNotMatched(t *testing.T) {
	src := `package v

type T struct{}

func (T) Validate() {}
`
	if _, err := Digest([]byte(src), []string{"Validate"}); err == nil {
		t.Error("a method satisfied a citation to a package-level function")
	}
}

// TestUnparseableSourceIsAnError pins that a fetch returning something that is
// not Go - an HTML error page from a bad path, most likely - fails loudly
// rather than hashing whatever arrived.
func TestUnparseableSourceIsAnError(t *testing.T) {
	if _, err := Digest([]byte("<!DOCTYPE html><html>404</html>"), []string{"Validate"}); err == nil {
		t.Error("expected a parse error for non-Go source, got none")
	}
}

// TestRenderIsReadable pins that Render returns the source a mismatch is
// explained with. A digest pair alone cannot tell a reviewer what upstream
// changed, which is the whole reason Render is exported.
func TestRenderIsReadable(t *testing.T) {
	src := `package v

// dropped by the parser
func Validate(x int) error { return nil }
`
	out, err := Render([]byte(src), []string{"Validate"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "func Validate(x int) error") {
		t.Errorf("rendered output does not contain the function signature: %q", out)
	}
	if strings.Contains(out, "dropped by the parser") {
		t.Errorf("comments leaked into the rendered output, so the digest is not comment-stable: %q", out)
	}
}
