package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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

		t.Run("repo with no go.mod requirement errors", func(t *testing.T) {
			if _, err := moduleVersionForRepo("nobody/nothing"); err == nil {
				t.Fatal("expected an error for a repo with no go.mod requirement")
			}
		})
	})
}

func TestVersionFor(t *testing.T) {
	t.Run("explicit tag always wins", func(t *testing.T) {
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
}
