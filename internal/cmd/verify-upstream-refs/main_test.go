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

// singleLineGoMod exercises the go.mod forms that are not a parenthesized
// require block. "go mod tidy" writes this shape whenever a module has a
// lone requirement, and "go get" can append standalone require lines to a
// file that already has a block, so both must resolve.
const singleLineGoMod = "" +
	"module example.com/sample\n\n" +
	"go 1.24\n\n" +
	"require github.com/some-org/lonely-dep v1.3.4\n\n" +
	"require (\n" +
	"\tgithub.com/some-org/block-dep v1.5.0\n" +
	"\tk8s.io/api v0.37.0\n" +
	")\n\n" +
	"require github.com/some-org/pseudo-dep v0.0.0-20260827164301-abcdef123456 // indirect\n"

func TestModuleVersionForRepoSingleLineRequire(t *testing.T) {
	withTempGoMod(t, singleLineGoMod, func() {
		cases := []struct {
			name, repo, want string
		}{
			{"standalone require line", "some-org/lonely-dep", "v1.3.4"},
			{"standalone require with trailing // indirect", "some-org/pseudo-dep", "abcdef123456"},
			{"block require alongside standalone ones", "some-org/block-dep", "v1.5.0"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := moduleVersionForRepo(tc.repo)
				if err != nil {
					t.Fatalf("moduleVersionForRepo(%q): unexpected error: %v", tc.repo, err)
				}
				if got != tc.want {
					t.Errorf("moduleVersionForRepo(%q) = %q, want %q", tc.repo, got, tc.want)
				}
			})
		}
	})
}

// A require line inside a replace directive's right-hand side must not be
// mistaken for the requirement itself.
const replaceGoMod = "" +
	"module example.com/sample\n\n" +
	"go 1.24\n\n" +
	"require github.com/some-org/dep v1.0.0\n\n" +
	"replace github.com/some-org/dep => github.com/fork-org/dep v9.9.9\n"

func TestModuleVersionForRepoIgnoresReplaceTarget(t *testing.T) {
	withTempGoMod(t, replaceGoMod, func() {
		got, err := moduleVersionForRepo("some-org/dep")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "v1.0.0"; got != want {
			t.Errorf("moduleVersionForRepo() = %q, want %q (the requirement, not the replace target)", got, want)
		}
		if _, err := moduleVersionForRepo("fork-org/dep"); err == nil {
			t.Error("expected an error: a replace target is not itself a requirement")
		}
	})
}

// tagForRepo scopes an explicit -tag to the repo it can actually name.
func TestTagForRepoScopesExplicitTag(t *testing.T) {
	cases := []struct {
		name, repo, userTag, want string
	}{
		{"default repo takes the explicit tag", runtime.DefaultRepo, "v1.36.3", "v1.36.3"},
		{"non-default repo ignores it", "ovn-kubernetes/ovn-kubernetes", "v1.36.3", ""},
		{"no tag is a no-op for the default repo", runtime.DefaultRepo, "", ""},
		{"no tag is a no-op for a non-default repo", "containernetworking/cni", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tagForRepo(tc.repo, tc.userTag); got != tc.want {
				t.Errorf("tagForRepo(%q, %q) = %q, want %q", tc.repo, tc.userTag, got, tc.want)
			}
		})
	}
}

// A pseudo-version is a legal ValidatedAt for a non-default repo
// (runtime.ValidateValidatedAt accepts it), but go.mod resolution reduces it
// to its trailing commit hash. Comparing the two forms literally reports a
// ref as permanently stale even though both name the same commit.
func TestSameVersionMatchesPseudoVersionAgainstItsCommit(t *testing.T) {
	cases := []struct {
		name, recorded, resolved string
		want                     bool
	}{
		{"identical strings", "abcdef123456", "abcdef123456", true},
		{"pseudo-version vs its own commit", "v0.0.0-20260827164301-e63fce3cf15d", "e63fce3cf15d", true},
		{"release-built pseudo-version vs its commit", "v1.1.2-0.20180830191138-d8f796af33cc", "d8f796af33cc", true},
		{"pseudo-version vs a different commit", "v0.0.0-20260827164301-e63fce3cf15d", "0123456789ab", false},
		{"plain tag vs a commit", "v1.4.2", "e63fce3cf15d", false},
		{"genuinely stale tag", "v1.36.2", "v1.36.3", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameVersion(tc.recorded, tc.resolved); got != tc.want {
				t.Errorf("sameVersion(%q, %q) = %v, want %v", tc.recorded, tc.resolved, got, tc.want)
			}
		})
	}
}

// mixedCaseGoMod uses module paths whose owner segment is not lowercase.
// These are ordinary on GitHub - this repository's own module path is
// github.com/ArthurVardevanyan/k8s-gitops-ci - and repoPattern accepts
// [A-Za-z], so a ref can name one in any case and still validate.
const mixedCaseGoMod = "" +
	"module example.com/sample\n\n" +
	"go 1.24\n\n" +
	"require (\n" +
	"\tgithub.com/Masterminds/semver/v3 v3.2.1\n" +
	"\tgithub.com/SomeOrg/SomeRepo v1.0.0\n" +
	")\n"

func TestModuleVersionForRepoIsCaseInsensitive(t *testing.T) {
	withTempGoMod(t, mixedCaseGoMod, func() {
		cases := []struct {
			name, repo, want string
		}{
			{"exact case", "Masterminds/semver", "v3.2.1"},
			{"lowercased owner", "masterminds/semver", "v3.2.1"},
			{"uppercased owner", "MASTERMINDS/semver", "v3.2.1"},
			{"exact case, no subdirectory", "SomeOrg/SomeRepo", "v1.0.0"},
			{"lowercased owner and name", "someorg/somerepo", "v1.0.0"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := moduleVersionForRepo(tc.repo)
				if err != nil {
					t.Fatalf("moduleVersionForRepo(%q): unexpected error: %v", tc.repo, err)
				}
				if got != tc.want {
					t.Errorf("moduleVersionForRepo(%q) = %q, want %q", tc.repo, got, tc.want)
				}
			})
		}

		// Case-insensitivity must not turn into prefix sloppiness: a repo
		// that merely shares a prefix with a required module is still not a
		// match, or "some-org/some" would resolve "some-org/something".
		t.Run("a shared prefix is still not a match", func(t *testing.T) {
			if _, err := moduleVersionForRepo("SomeOrg/Some"); err == nil {
				t.Error("expected an error: SomeOrg/Some is not SomeOrg/SomeRepo")
			}
		})
	})
}

// expectedCitations underpins the run's accounting guard, which is what
// turns a skipped citation into a loud failure instead of a clean report.
func TestExpectedCitations(t *testing.T) {
	cases := []struct {
		name string
		refs map[string]runtime.UpstreamRef
		want int
	}{
		{"no refs", map[string]runtime.UpstreamRef{}, 0},
		{
			"a ref with no Additional counts once",
			map[string]runtime.UpstreamRef{"a": {}},
			1,
		},
		{
			"each Additional counts separately",
			map[string]runtime.UpstreamRef{
				"a": {Additional: []runtime.UpstreamRef{{}, {}}},
			},
			3,
		},
		{
			"counted across refs, including import parents",
			map[string]runtime.UpstreamRef{
				"a": {Additional: []runtime.UpstreamRef{{}}},
				"b": {Kind: runtime.RefKindImport, Additional: []runtime.UpstreamRef{{}, {}}},
				"c": {},
			},
			6,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expectedCitations(tc.refs); got != tc.want {
				t.Errorf("expectedCitations() = %d, want %d", got, tc.want)
			}
		})
	}
}

// hyphenPrereleaseGoMod pins a dependency at a pseudo-version whose base
// prerelease identifier contains a hyphen ("beta-1"). SemVer allows it and
// Go builds pseudo-versions on such tags, so go.mod can legitimately
// contain one.
const hyphenPrereleaseGoMod = "" +
	"module example.com/sample\n\n" +
	"go 1.24\n\n" +
	"require (\n" +
	"\tgithub.com/some-org/hyphen-dep v1.2.3-beta-1.0.20260101000000-abcdefabcdef\n" +
	"\tgithub.com/some-org/dashy-dep/v2 v2.0.0-pre-release-x.0.20260101000000-abcdefabcdef\n" +
	")\n"

// A pseudo-version must resolve to its commit, whatever its prerelease
// identifiers look like. Failing to extract it does not degrade gracefully:
// the full pseudo-version string is then used as a git ref to fetch by, and
// no such ref exists upstream.
func TestModuleVersionForRepoHyphenatedPrerelease(t *testing.T) {
	withTempGoMod(t, hyphenPrereleaseGoMod, func() {
		cases := []struct{ name, repo, want string }{
			{"hyphen in the prerelease identifier", "some-org/hyphen-dep", "abcdefabcdef"},
			{"several hyphens", "some-org/dashy-dep", "abcdefabcdef"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := moduleVersionForRepo(tc.repo)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Errorf("moduleVersionForRepo(%q) = %q, want the commit %q", tc.repo, got, tc.want)
				}
			})
		}
	})
}

// sameVersion must likewise normalize these, or a correctly recorded ref is
// reported stale forever.
func TestSameVersionHandlesHyphenatedPrerelease(t *testing.T) {
	if !sameVersion("v1.2.3-beta-1.0.20260101000000-abcdefabcdef", "abcdefabcdef") {
		t.Error("sameVersion() did not match a hyphenated-prerelease pseudo-version against its own commit")
	}
}

// cachePathFor's diagnostics must be reproducible. Reporting whichever
// invalid component a map iteration happened to yield first makes a failure
// describe itself differently on consecutive identical runs.
func TestCachePathForReportsComponentsInAFixedOrder(t *testing.T) {
	dir := t.TempDir()
	// Every component is invalid, so only the ordering decides which is named.
	var first string
	for i := 0; i < 50; i++ {
		_, err := cachePathFor(dir, "/abs-repo", "/abs-version", "/abs-path")
		if err == nil {
			t.Fatal("expected an error when every component is absolute")
		}
		if i == 0 {
			first = err.Error()
			continue
		}
		if err.Error() != first {
			t.Fatalf("error text varies between identical calls:\n  %q\n  %q", first, err.Error())
		}
	}
	if !strings.Contains(first, "repo") {
		t.Errorf("expected the first-declared component (repo) to be reported, got %q", first)
	}
}

// filepath.Join cleans its result, so a component that is empty or carries a
// "", "." or ".." segment names a file some other component triple also
// names. These never escape the cache dir, so the escape check cannot see
// them; the result is a wrong cache hit rather than a failed lookup.
func TestCachePathForRejectsCollapsingComponents(t *testing.T) {
	dir := t.TempDir()
	cases := []struct{ name, repo, version, path string }{
		{"empty repo", "", "v1.0.0", "pkg/a.go"},
		{"empty version", "some-org/dep", "", "pkg/a.go"},
		{"empty path", "some-org/dep", "v1.0.0", ""},
		{"dotdot in repo", "some-org/dep/sub/..", "v1.0.0", "pkg/a.go"},
		{"dotdot in path", "some-org/dep", "v1.0.0", "pkg/sub/../a.go"},
		{"dot in repo", "some-org/./dep", "v1.0.0", "pkg/a.go"},
		{"dot in path", "some-org/dep", "v1.0.0", "./pkg/a.go"},
		{"dotdot in version", "some-org/dep", "v1.0.0/..", "pkg/a.go"},
		{"double slash in repo", "some-org//dep", "v1.0.0", "pkg/a.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := cachePathFor(dir, tc.repo, tc.version, tc.path); err == nil {
				t.Error("expected an error for a component filepath.Join would rewrite")
			}
		})
	}
}

// The property behind the case list above, checked over a cross product
// rather than hand-picked aliases: no two distinct accepted triples may
// resolve to the same path. Enumerating aliases is what previously let the
// "/" case through - a version like "release/v1" rewrites nothing, so it
// passed every spelling check, yet still crossed the component boundary.
func TestCachePathForIsInjective(t *testing.T) {
	dir := t.TempDir()
	repos := []string{"some-org/dep", "some-org/dep2", "other-org/dep"}
	versions := []string{"v1.0.0", "v1.0.0/a", "release/v1", "v1", "v1.0.0-rc.1"}
	paths := []string{"pkg/a.go", "a/b", "b", "pkg/sub/a.go", "v1.0.0/pkg/a.go"}

	type triple struct{ repo, version, path string }
	seen := map[string]triple{}
	for _, r := range repos {
		for _, v := range versions {
			for _, pth := range paths {
				got, err := cachePathFor(dir, r, v, pth)
				if err != nil {
					continue // rejected, which is an acceptable outcome
				}
				cur := triple{r, v, pth}
				if prev, dup := seen[got]; dup {
					t.Errorf("collision: %+v and %+v both resolve to %q", prev, cur, got)
					continue
				}
				seen[got] = cur
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no triple was accepted; the test is proving nothing")
	}
}
