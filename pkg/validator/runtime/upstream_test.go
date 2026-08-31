package runtime

import (
	"strings"
	"testing"

	"golang.org/x/mod/module"
)

// validDigest is a well-formed (structurally, not cryptographically
// meaningful) sha256 digest for use in table cases that don't otherwise care
// about its value.
var validDigest = "sha256:" + strings.Repeat("a", 64)

func wantErrContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error mentioning %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error mentioning %q, got: %v", substr, err)
	}
}

// TestUpstreamRefDefaultsPreserved proves a ref that predates Repo/Kind - the
// entire existing 105-ref corpus - validates exactly as it did before those
// fields existed: Repo defaults to kubernetes/kubernetes and Kind defaults to
// RefKindRewrite.
func TestUpstreamRefDefaultsPreserved(t *testing.T) {
	ref := UpstreamRef{
		Path:        "pkg/apis/core/validation/validation.go",
		Functions:   []string{"ValidatePodSpec"},
		Digest:      validDigest,
		ValidatedAt: "v1.36.3",
		Note:        "Ports the whole function.",
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("unexpected error for a pre-existing-shaped ref: %v", err)
	}
	if got := ref.EffectiveRepo(); got != DefaultRepo {
		t.Errorf("EffectiveRepo() = %q, want %q", got, DefaultRepo)
	}
	if got := ref.EffectiveKind(); got != RefKindRewrite {
		t.Errorf("EffectiveKind() = %q, want %q", got, RefKindRewrite)
	}
}

func TestUpstreamRefImportKind(t *testing.T) {
	t.Run("valid import ref accepted without digest or validatedAt", func(t *testing.T) {
		ref := UpstreamRef{
			Repo:      "ovn-kubernetes/ovn-kubernetes",
			Kind:      RefKindImport,
			Path:      "go-controller/pkg/config/network.go",
			Functions: []string{"ParseNetConf"},
			Note:      "Called directly; go.mod pins the exact commit.",
		}
		if err := ref.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("digest set on an import ref is rejected", func(t *testing.T) {
		ref := UpstreamRef{
			Repo:      "ovn-kubernetes/ovn-kubernetes",
			Kind:      RefKindImport,
			Path:      "go-controller/pkg/config/network.go",
			Functions: []string{"ParseNetConf"},
			Digest:    validDigest,
			Note:      "Called directly; go.mod pins the exact commit.",
		}
		wantErrContains(t, ref.Validate(), "digest must be empty")
	})
}

func TestUpstreamRefRewriteKindNonDefaultRepo(t *testing.T) {
	t.Run("commit SHA accepted as validatedAt for a non-default repo", func(t *testing.T) {
		ref := UpstreamRef{
			Repo:        "ovn-kubernetes/ovn-kubernetes",
			Kind:        RefKindRewrite,
			Path:        "go-controller/pkg/util/multi_network.go",
			Functions:   []string{"ValidateNetConf"},
			Digest:      validDigest,
			ValidatedAt: "e63fce3cf15d",
			Note:        "Ports the semantic rule set.",
		}
		if err := ref.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Go pseudo-version accepted as validatedAt for a non-default repo", func(t *testing.T) {
		ref := UpstreamRef{
			Repo:        "ovn-kubernetes/ovn-kubernetes",
			Kind:        RefKindRewrite,
			Path:        "go-controller/pkg/util/multi_network.go",
			Functions:   []string{"ValidateNetConf"},
			Digest:      validDigest,
			ValidatedAt: "v0.0.0-20260827164301-e63fce3cf15d",
			Note:        "Ports the semantic rule set.",
		}
		if err := ref.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Go pseudo-version built on a tagged release accepted as validatedAt", func(t *testing.T) {
		// "v1.1.2-0.20180830191138-d8f796af33cc" (a real entry in this
		// repo's own go.mod, github.com/davecgh/go-spew) is a different
		// pseudo-version shape than the bare form above: it has a "0."
		// segment marking it as built on tagged release v1.1.2 rather than
		// an untagged repo. The former hand-rolled pattern matched only
		// the bare form; this is now delegated to x/mod.
		ref := UpstreamRef{
			Repo:        "davecgh/go-spew",
			Kind:        RefKindRewrite,
			Path:        "spew/dump.go",
			Functions:   []string{"Dump"},
			Digest:      validDigest,
			ValidatedAt: "v1.1.2-0.20180830191138-d8f796af33cc",
			Note:        "n/a",
		}
		if err := ref.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ordinary semver tag accepted as validatedAt for a non-default repo", func(t *testing.T) {
		// moduleVersionForRepo returns a tagged dependency's version
		// verbatim, and unlike kubernetes/kubernetes a non-default repo is
		// not confined to major version 1 - v0.x.y and v2.x.y are both
		// legitimate release tags a real dependency might be pinned to, as
		// are a semver prerelease suffix and Go's "+incompatible" build-
		// metadata marker (appended to a pre-modules v2+ tag whose import
		// path isn't version-qualified).
		for _, tag := range []string{"v0.5.2", "v2.1.0", "v1.3.0", "v1.2.3-rc.1", "v2.0.0+incompatible", "v1.2.3-rc.1+build.5"} {
			ref := UpstreamRef{
				Repo:        "containernetworking/cni",
				Kind:        RefKindRewrite,
				Path:        "libcni/conf.go",
				Functions:   []string{"ConfFromBytes"},
				Digest:      validDigest,
				ValidatedAt: tag,
				Note:        "n/a",
			}
			if err := ref.Validate(); err != nil {
				t.Errorf("tag %q: unexpected error: %v", tag, err)
			}
		}
	})

	t.Run("invalid version string rejected as validatedAt for a non-default repo", func(t *testing.T) {
		// Not every non-default repo would reject a v1.x.y tag (some may
		// tag releases that way), so this only proves the acceptance list
		// covers commit SHAs, pseudo-versions and ordinary semver tags -
		// not that some stricter subset of those is required.
		ref := UpstreamRef{
			Repo:        "ovn-kubernetes/ovn-kubernetes",
			Kind:        RefKindRewrite,
			Path:        "go-controller/pkg/util/multi_network.go",
			Functions:   []string{"ValidateNetConf"},
			Digest:      validDigest,
			ValidatedAt: "not-a-valid-version-at-all",
			Note:        "Ports the semantic rule set.",
		}
		wantErrContains(t, ref.Validate(), "validatedAt")
	})

	t.Run("kubernetes/kubernetes still requires a release tag", func(t *testing.T) {
		ref := UpstreamRef{
			Kind:        RefKindRewrite,
			Path:        "pkg/apis/core/validation/validation.go",
			Functions:   []string{"ValidatePodSpec"},
			Digest:      validDigest,
			ValidatedAt: "e63fce3cf15d",
			Note:        "Ports the whole function.",
		}
		wantErrContains(t, ref.Validate(), "release tag")
	})
}

func TestUpstreamRefKindAndRepoRejected(t *testing.T) {
	t.Run("unknown kind rejected", func(t *testing.T) {
		ref := UpstreamRef{
			Kind:        RefKind("rewrite-ish"),
			Path:        "pkg/apis/core/validation/validation.go",
			Functions:   []string{"ValidatePodSpec"},
			Digest:      validDigest,
			ValidatedAt: "v1.36.3",
			Note:        "n/a",
		}
		wantErrContains(t, ref.Validate(), "must be")
	})

	t.Run("malformed repo slug rejected", func(t *testing.T) {
		ref := UpstreamRef{
			Repo:        "not-an-owner-slash-name",
			Path:        "pkg/apis/core/validation/validation.go",
			Functions:   []string{"ValidatePodSpec"},
			Digest:      validDigest,
			ValidatedAt: "v1.36.3",
			Note:        "n/a",
		}
		wantErrContains(t, ref.Validate(), "owner/name")
	})
}

func TestUpstreamRefAdditionalOwnRepoAndKind(t *testing.T) {
	// An Additional citation may point at a different repo/kind than its
	// parent - e.g. a rewrite whose supporting citation is a direct import.
	ref := UpstreamRef{
		Path:        "pkg/apis/core/validation/validation.go",
		Functions:   []string{"ValidatePodSpec"},
		Digest:      validDigest,
		ValidatedAt: "v1.36.3",
		Note:        "Ports the whole function.",
		Additional: []UpstreamRef{
			{
				Repo:      "ovn-kubernetes/ovn-kubernetes",
				Kind:      RefKindImport,
				Path:      "go-controller/pkg/config/network.go",
				Functions: []string{"ParseNetConf"},
				Note:      "Supporting citation, called directly.",
			},
		},
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPseudoVersionAgreesWithGoModule pins ValidateValidatedAt's
// pseudo-version handling to golang.org/x/mod's, which is the definition Go
// itself uses. Hand-written approximations of this format have now been
// wrong twice - first by matching only the bare form, then by rejecting
// prerelease identifiers containing a hyphen, which SemVer explicitly
// allows and real tags such as "v1.2.3-beta-1" use.
func TestPseudoVersionAgreesWithGoModule(t *testing.T) {
	pseudoVersions := []string{
		"v0.0.0-20260827164301-e63fce3cf15d",
		"v1.1.2-0.20180830191138-d8f796af33cc",
		"v1.2.3-alpha.1.0.20260101000000-abcdefabcdef",
		"v1.2.3-beta-1.0.20260101000000-abcdefabcdef",
		"v1.2.3-rc-2.0.20260101000000-abcdefabcdef",
		"v2.0.0-pre-release-x.0.20260101000000-abcdefabcdef",
	}
	for _, v := range pseudoVersions {
		t.Run(v, func(t *testing.T) {
			if !module.IsPseudoVersion(v) {
				t.Fatalf("test bug: %q is not a pseudo-version per x/mod", v)
			}
			ref := UpstreamRef{
				Repo:        "some-org/some-dep",
				Kind:        RefKindRewrite,
				Path:        "pkg/thing.go",
				Functions:   []string{"Thing"},
				Digest:      validDigest,
				ValidatedAt: v,
				Note:        "n/a",
			}
			if err := ref.Validate(); err != nil {
				t.Errorf("Validate() rejected a valid Go pseudo-version %q: %v", v, err)
			}
		})
	}
}
