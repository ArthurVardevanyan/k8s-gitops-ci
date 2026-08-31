package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// refFile is a ref map shaped like a real upstream_refs.go: two shared digest
// constants, several entries using each, and a nested Additional ref carrying
// its own digest.
const refFile = "" +
	"var refs = map[string]runtime.UpstreamRef{\n" +
	"\t\"admissionregistration/service-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      mutatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t},\n" +
	"\t\"admissionregistration/failure-policy-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      mutatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t\tAdditional: []runtime.UpstreamRef{{\n" +
	"\t\t\tPath:        p,\n" +
	"\t\t\tDigest:      validatingWebhookDigest,\n" +
	"\t\t}},\n" +
	"\t},\n" +
	"\t\"admissionregistration/validating-service-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      validatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t},\n" +
	"\t\"admissionregistration/validating-timeout-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      validatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t},\n" +
	"}\n"

func TestEntriesReferencing(t *testing.T) {
	tests := []struct {
		name  string
		field string
		ident string
		want  []string
	}{
		{
			// The regression. A lazy scan over the whole file matched from
			// the first entry to the first use of the constant, naming a
			// mutating check as a user of the validating digest and hiding
			// every real user behind that one match.
			name:  "a later constant does not capture earlier entries",
			field: "Digest",
			ident: "validatingWebhookDigest",
			want: []string{
				"admissionregistration/validating-service-invalid",
				"admissionregistration/validating-timeout-invalid",
			},
		},
		{
			name:  "every user of a shared constant is returned",
			field: "Digest",
			ident: "mutatingWebhookDigest",
			want: []string{
				"admissionregistration/service-invalid",
				"admissionregistration/failure-policy-invalid",
			},
		},
		{
			name:  "an unused identifier has no users",
			field: "Digest",
			ident: "someOtherDigest",
			want:  []string{},
		},
		{
			name:  "a different field is not matched",
			field: "ValidatedAt",
			ident: "mutatingWebhookDigest",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entriesReferencing(refFile, tt.field, tt.ident)
			if !slices.Equal(got, tt.want) {
				t.Errorf("entriesReferencing(%q, %q) = %v, want %v", tt.field, tt.ident, got, tt.want)
			}
		})
	}
}

// TestEntriesReferencingIgnoresNestedRefs pins that a supporting citation's
// digest is not attributed to the entry that contains it. The entry for
// failure-policy-invalid nests an Additional ref using validatingWebhookDigest
// while its own digest is the mutating one; counting the nested value would
// make --update compare the primary digest against a supporting citation's.
func TestEntriesReferencingIgnoresNestedRefs(t *testing.T) {
	got := entriesReferencing(refFile, "Digest", "validatingWebhookDigest")
	if slices.Contains(got, "admissionregistration/failure-policy-invalid") {
		t.Errorf("nested Additional digest attributed to the entry: %v", got)
	}
}

// withTempGoMod chdirs into a temp directory containing the given go.mod
// content for the duration of fn, restoring the original working directory
// afterward. moduleVersionForRepo/tagFromGoMod read "go.mod" relative to the
// process's working directory (the same convention scripts/verify-upstream-refs.sh
// establishes by `cd`-ing to the repo root before invoking this binary), so
// this is the only way to exercise them against a controlled go.mod without
// depending on this repository's own, real go.mod staying in a particular shape.
func withTempGoMod(t *testing.T, content string, fn func()) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp go.mod: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restoring cwd: %v", err)
		}
	})
	fn()
}

const sampleGoMod = "" +
	"module example.com/sample\n\n" +
	"go 1.24\n\n" +
	"require (\n" +
	"\tgithub.com/ovn-kubernetes/ovn-kubernetes/go-controller v0.0.0-20260827164301-e63fce3cf15d\n" +
	"\tgithub.com/some-org/some-tagged-dep v1.4.2\n" +
	"\tgithub.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc\n" +
	"\tk8s.io/api v0.37.0\n" +
	")\n"

func TestModuleVersionForRepo(t *testing.T) {
	withTempGoMod(t, sampleGoMod, func() {
		t.Run("pseudo-version resolves to its trailing commit hash", func(t *testing.T) {
			// The module path (github.com/ovn-kubernetes/ovn-kubernetes/go-controller)
			// is a subdirectory of the repo (ovn-kubernetes/ovn-kubernetes) -
			// go-controller is ovn-kubernetes's own submodule layout - so this
			// also proves the subdirectory-prefix match, not just an exact one.
			got, err := moduleVersionForRepo("ovn-kubernetes/ovn-kubernetes")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := "e63fce3cf15d"; got != want {
				t.Errorf("moduleVersionForRepo() = %q, want %q", got, want)
			}
		})

		t.Run("ordinary semver tag is returned as-is", func(t *testing.T) {
			got, err := moduleVersionForRepo("some-org/some-tagged-dep")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := "v1.4.2"; got != want {
				t.Errorf("moduleVersionForRepo() = %q, want %q", got, want)
			}
		})

		t.Run("pseudo-version built on a tagged release resolves to its trailing commit hash", func(t *testing.T) {
			// "v1.1.2-0.20180830191138-d8f796af33cc" (a real entry in this
			// repo's own go.mod) is a different pseudo-version shape than the
			// bare "vX.0.0-<ts>-<hash>" form above: it has a "0." segment
			// between the version core and the timestamp, marking it as
			// built on tagged release v1.1.2 rather than an untagged repo.
			// pseudoVersionCommit previously only matched the bare form.
			got, err := moduleVersionForRepo("davecgh/go-spew")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := "d8f796af33cc"; got != want {
				t.Errorf("moduleVersionForRepo() = %q, want %q", got, want)
			}
		})

		t.Run("repo with no go.mod requirement errors", func(t *testing.T) {
			if _, err := moduleVersionForRepo("nobody/nothing"); err == nil {
				t.Fatal("expected an error for a repo with no go.mod requirement")
			}
		})
	})
}

func TestVersionFor(t *testing.T) {
	t.Run("explicit tag always wins for a non-default repo", func(t *testing.T) {
		// A non-default repo's ValidatedAt format is loose (commit SHA,
		// pseudo-version, or semver tag) and -compute legitimately wants to
		// fetch an arbitrary ref (a branch name, "HEAD", ...) before any
		// ValidatedAt is ever recorded for a brand-new ref, so this is
		// intentionally unvalidated.
		got, err := versionFor("anything/anything", "explicit-override")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "explicit-override" {
			t.Errorf("versionFor() = %q, want %q", got, "explicit-override")
		}
	})

	t.Run("non-default repo without explicit tag resolves via go.mod", func(t *testing.T) {
		withTempGoMod(t, sampleGoMod, func() {
			got, err := versionFor("ovn-kubernetes/ovn-kubernetes", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := "e63fce3cf15d"; got != want {
				t.Errorf("versionFor() = %q, want %q", got, want)
			}
		})
	})

	t.Run("explicit tag wins for the default repo when it's a real Kubernetes release tag", func(t *testing.T) {
		got, err := versionFor(runtime.DefaultRepo, "v1.36.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.36.3" {
			t.Errorf("versionFor() = %q, want %q", got, "v1.36.3")
		}
	})

	t.Run("explicit tag rejected for the default repo when it isn't a release tag", func(t *testing.T) {
		// kubernetes/kubernetes's RefKindRewrite refs can only ever record a
		// v1.<minor>.<patch> tag as ValidatedAt (runtime.ValidateValidatedAt),
		// so an arbitrary -tag override here must be rejected before it can
		// round-trip into a --update'd upstream_refs.go entry that
		// RegisterAll would then panic on at the next binary start.
		if _, err := versionFor(runtime.DefaultRepo, "not-a-release-tag"); err == nil {
			t.Fatal("expected an error for a non-release-tag -tag override against the default repo")
		}
	})
}

func TestCachePathForRejectsEscapes(t *testing.T) {
	dir := t.TempDir()

	t.Run("well-formed inputs stay under cacheDir", func(t *testing.T) {
		got, err := cachePathFor(dir, "kubernetes/kubernetes", "v1.36.3", "pkg/apis/core/validation/validation.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			t.Fatalf("filepath.Abs: %v", err)
		}
		if !strings.HasPrefix(got, absDir+string(filepath.Separator)) {
			t.Errorf("cachePathFor() = %q, want a path under %q", got, absDir)
		}
	})

	t.Run("dot-dot in path is rejected", func(t *testing.T) {
		if _, err := cachePathFor(dir, "kubernetes/kubernetes", "v1.36.3", "../../../../etc/passwd"); err == nil {
			t.Fatal("expected an error for a path escaping cacheDir via ..")
		}
	})

	t.Run("dot-dot in repo is rejected", func(t *testing.T) {
		if _, err := cachePathFor(dir, "../../etc", "v1.36.3", "passwd"); err == nil {
			t.Fatal("expected an error for a repo escaping cacheDir via ..")
		}
	})

	t.Run("dot-dot in version is rejected", func(t *testing.T) {
		// A 2-segment repo ("kubernetes/kubernetes") plus exactly 2 ".."
		// levels in version cancels out and lands back under cacheDir -
		// filepath.Join already handles that shallow case safely on its
		// own, which is not what this guard exists to catch. What it must
		// catch is enough ".." to walk past cacheDir entirely, which a
		// crafted version string can still supply.
		if _, err := cachePathFor(dir, "kubernetes/kubernetes", "../../../etc", "passwd"); err == nil {
			t.Fatal("expected an error for a version escaping cacheDir via ..")
		}
	})

	t.Run("absolute path component is rejected explicitly", func(t *testing.T) {
		// Not asserted via the containment check (filepath.Join happens to
		// neutralize an absolute-looking later argument on its own, since
		// it only preserves a leading "/" specially for its *first*
		// argument) - cachePathFor rejects an absolute component outright
		// instead, so this doesn't depend on that Join behavior continuing
		// to hold.
		if _, err := cachePathFor(dir, "kubernetes/kubernetes", "v1.36.3", "/etc/passwd"); err == nil {
			t.Fatal("expected an error for an absolute path component")
		}
	})

	t.Run("absolute repo component is rejected explicitly", func(t *testing.T) {
		if _, err := cachePathFor(dir, "/etc", "v1.36.3", "passwd"); err == nil {
			t.Fatal("expected an error for an absolute repo component")
		}
	})

	t.Run("absolute version component is rejected explicitly", func(t *testing.T) {
		if _, err := cachePathFor(dir, "kubernetes/kubernetes", "/etc", "passwd"); err == nil {
			t.Fatal("expected an error for an absolute version component")
		}
	})
}
