package runtime

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"golang.org/x/mod/module"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// RefKind distinguishes how a runtime check relates to the upstream code it
// cites.
type RefKind string

const (
	// RefKindRewrite is a check that reimplements upstream's validation
	// logic in this repository (the common case: upstream's own
	// pkg/apis/*/validation packages are not importable as a library). A
	// rewrite can drift silently from what it ports, so it must carry a
	// Digest over the cited functions' normalized source, verified by
	// `task verify:upstream-refs` against ValidatedAt. This is the default
	// when Kind is left empty, matching every ref that predates this field.
	RefKindRewrite RefKind = "rewrite"
	// RefKindImport is a check that calls the cited code directly (an
	// importable dependency such as ovn-kubernetes's netconf parser).
	// There is nothing to drift silently here: go.mod pins the version and
	// the compiler verifies the call, so a Digest would only assert a
	// verification that isn't actually happening. Functions/Path/Note are
	// still required - the citation still has to name the exact upstream
	// code a reviewer should read - but Digest and ValidatedAt are not.
	RefKindImport RefKind = "import"
)

// DefaultRepo is the upstream repository assumed when Repo is left empty,
// preserving every ref that predates this field. Exported so
// verify-upstream-refs can special-case it (its version is derived from
// k8s.io/api in go.mod, an existing staging-module convention that does not
// generalize to other repositories).
const DefaultRepo = defaultRepo

// defaultRepo is the unexported form used internally; see DefaultRepo.
const defaultRepo = "kubernetes/kubernetes"

// UpstreamRef identifies the exact upstream code a runtime check ports or
// imports.
//
// Every runtime check must supply one. The family is always-blocking and
// non-exemptable, which is only defensible if the API server (or, for a
// non-Kubernetes-core dependency like ovn-kubernetes, the controller that
// owns the resource) really would reject the manifest - so a check has to be
// able to point at the specific upstream code it reproduces or calls. A
// citation naming only a file is not enough:
// "k8s.io/kubernetes/pkg/apis/core/validation/validation.go" is equally true
// of a faithful port and of an invented rule, and that is exactly how a
// large number of fabricated checks once passed review here.
//
// Line numbers are deliberately absent. They drift on every upstream release,
// and every incorrect citation in this repository's history was a stale line
// range. Function names are stable and mechanically verifiable.
//
// See docs/CI.md for the standard and `task verify:upstream-refs` for the
// tooling that proves the cited functions exist and (for Kind ==
// RefKindRewrite) are unchanged.
type UpstreamRef struct {
	// Repo is "owner/name" of the upstream GitHub repository the citation
	// points into. Empty means kubernetes/kubernetes, preserving every ref
	// that predates this field.
	Repo string `json:"repo,omitempty"`
	// Kind distinguishes a reimplementation (RefKindRewrite, the default)
	// from a direct import (RefKindImport). See the constants' doc comments.
	Kind RefKind `json:"kind,omitempty"`
	// Path is the file, relative to the root of Repo, that contains the
	// cited code. Using repo-relative paths lets refs into staging modules
	// (apimachinery, apiextensions-apiserver) use the same form as refs
	// into pkg/apis/*.
	Path string `json:"path"`
	// Functions are the upstream function(s) this check ports or calls.
	Functions []string `json:"functions"`
	// Digest is "sha256:<hex>" over the normalized source of Functions
	// (comments and formatting stripped), taken at ValidatedAt. Required
	// for RefKindRewrite; must be empty for RefKindImport, where the
	// compiler and go.mod already pin the exact code and a digest would
	// only assert a verification that isn't happening.
	Digest string `json:"digest,omitempty"`
	// ValidatedAt is the upstream tag or commit the digest was taken at,
	// e.g. "v1.36.3" for kubernetes/kubernetes, or a commit SHA / Go
	// pseudo-version for any other Repo. It records the version a human
	// last validated a RefKindRewrite port against; it is NOT a claim that
	// the check is correct for every cluster version - see the
	// version-skew section in docs/CI.md. Not required for RefKindImport.
	ValidatedAt string `json:"validatedAt,omitempty"`
	// Note records which upstream branch this check ports/imports and
	// every deliberate divergence from it, with the reason.
	//
	// It is required. A citation proves the cited function exists (and,
	// for a rewrite, has not changed); it says nothing about how much of
	// that function the check actually implements. Most rewrite ports are
	// partial by design - skipping a Required branch that defaulting makes
	// unreachable, or a feature-gated branch this tool cannot evaluate -
	// and without the note a reviewer cannot tell a deliberate subset from
	// an incomplete port.
	Note string `json:"note"`
	// Additional cites supporting functions in other files that the ported
	// rule depends on, each verified exactly like the primary ref.
	//
	// A rule is not always contained in one function. Where the API server
	// prepares its input before calling the function that reports the
	// error, the preparation is part of the rule: porting only the callee
	// reproduces its logic against different data. That is not
	// hypothetical - container/volume-mount-name-undefined rejected almost
	// every real StatefulSet because it ported ValidateVolumeMounts while
	// upstream's caller first synthesizes a volume for each
	// volumeClaimTemplate. The digest over the cited function was correct
	// and verified throughout.
	//
	// Cite that supporting code here so a change to it is caught the same
	// way, rather than describing it in Note where nothing checks it.
	// Nesting is one level deep; an entry here may not carry its own
	// Additional.
	Additional []UpstreamRef `json:"additional,omitempty"`
}

var (
	// digestPattern matches a "sha256:<64 hex chars>" digest.
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	// k8sTagPattern matches a Kubernetes release tag.
	k8sTagPattern = regexp.MustCompile(`^v1\.\d+\.\d+$`)
	// semverTagPattern matches an ordinary "v<major>.<minor>.<patch>" release
	// tag for a non-default repo, which - unlike kubernetes/kubernetes - is
	// not constrained to major version 1 (moduleVersionForRepo returns
	// whatever tag the dependency's go.mod requirement pins, verbatim), and
	// is not constrained to a bare release either: a real Go module tag may
	// carry a semver prerelease (v1.2.3-rc.1) and/or build metadata
	// (v2.0.0+incompatible, the marker cmd/go itself appends to a
	// pre-modules v2+ import-path-unqualified tag).
	semverTagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	// commitSHAPattern matches a bare (short or full) git commit hash.
	commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	// repoPattern matches a GitHub "owner/name" repository slug.
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	// identPattern matches a Go identifier.
	identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// pathPattern rejects a ":line" suffix, pinning the no-line-numbers rule.
	pathLineSuffix = regexp.MustCompile(`:\d+(-\d+)?$`)
)

// repo returns the effective upstream repository: Repo, or defaultRepo if
// unset.
func (r UpstreamRef) repo() string {
	return r.EffectiveRepo()
}

// EffectiveRepo returns the upstream repository this ref cites: Repo, or
// defaultRepo ("kubernetes/kubernetes") if unset. Exported so
// verify-upstream-refs can resolve which repository (and therefore which
// go.mod requirement) to fetch a ref's source from.
func (r UpstreamRef) EffectiveRepo() string {
	if r.Repo == "" {
		return defaultRepo
	}
	return r.Repo
}

// kind returns the effective ref kind: Kind, or RefKindRewrite if unset -
// preserving every ref that predates this field.
func (r UpstreamRef) kind() RefKind {
	return r.EffectiveKind()
}

// EffectiveKind returns this ref's kind: Kind, or RefKindRewrite if unset -
// preserving every ref that predates this field. Exported so
// verify-upstream-refs can decide whether a ref's digest should be compared
// (RefKindRewrite) or only its existence confirmed (RefKindImport).
func (r UpstreamRef) EffectiveKind() RefKind {
	if r.Kind == "" {
		return RefKindRewrite
	}
	return r.Kind
}

// Validate reports whether the ref is structurally usable. It does not touch
// the network; `task verify:upstream-refs` performs the upstream check.
func (r UpstreamRef) Validate() error {
	if r.Repo != "" {
		if err := ValidateRepo(r.Repo); err != nil {
			return err
		}
	}
	kind := r.kind()
	if kind != RefKindRewrite && kind != RefKindImport {
		return fmt.Errorf("kind %q must be %q or %q", r.Kind, RefKindRewrite, RefKindImport)
	}
	if r.Path == "" {
		return fmt.Errorf("path is required")
	}
	// Enforced at registration rather than left to the verifier: a ref that
	// cannot be cached unambiguously should fail when it is written, not on
	// the next verification run.
	if err := ValidatePath(r.Path); err != nil {
		return fmt.Errorf("path %w", err)
	}
	if pathLineSuffix.MatchString(r.Path) {
		return fmt.Errorf("path %q must not carry a line number: line numbers drift on every upstream release, cite the function instead", r.Path)
	}
	if len(r.Functions) == 0 {
		return fmt.Errorf("at least one entry in functions is required")
	}
	for _, fn := range r.Functions {
		if !identPattern.MatchString(fn) {
			return fmt.Errorf("function %q is not a Go identifier", fn)
		}
	}
	switch kind {
	case RefKindImport:
		// The compiler and go.mod already pin exactly which code runs, so a
		// digest here would only assert a verification that never happens.
		if r.Digest != "" {
			return fmt.Errorf("digest must be empty for kind %q: the compiler and go.mod already pin the imported code, a digest would assert an unverified claim", RefKindImport)
		}
	case RefKindRewrite:
		if !digestPattern.MatchString(r.Digest) {
			return fmt.Errorf("digest %q must be sha256:<64 hex chars>; run 'task verify:upstream-refs -- --update'", r.Digest)
		}
		if err := ValidateValidatedAt(r.repo(), r.ValidatedAt); err != nil {
			return err
		}
	}

	if strings.TrimSpace(r.Note) == "" {
		return fmt.Errorf("note is required: record which upstream branch is ported/imported and any deliberate divergence, so a partial port is distinguishable from an incomplete one")
	}

	for i, a := range r.Additional {
		if len(a.Additional) > 0 {
			return fmt.Errorf("additional[%d]: nesting is one level deep; cite the supporting function directly", i)
		}
		if err := a.Validate(); err != nil {
			return fmt.Errorf("additional[%d]: %w", i, err)
		}
	}
	return nil
}

// ValidatePath reports whether a slash-separated upstream file path is
// unambiguous: no empty, "." or ".." segments. Such a path is not wrong so
// much as not unique - path handling collapses it, so two different-looking
// citations can denote one file.
//
// Exported and shared with verify-upstream-refs so a ref cannot be
// registered here that the verifier will then refuse to cache. Defining the
// rule twice is what would let the two drift apart.
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("must not be empty")
	}
	for _, seg := range strings.Split(path, "/") {
		switch seg {
		case "":
			return fmt.Errorf("%q must not contain an empty path segment", path)
		case ".", "..":
			return fmt.Errorf("%q must not contain a %q path segment", path, seg)
		}
	}
	return nil
}

// ValidateRepo reports whether repo is a well-formed GitHub "owner/name"
// slug. It is exported so that callers accepting a repository from outside
// the ref tables - verify-upstream-refs' -repo flag, for one - can reject a
// bad value up front and against the same rule the ref tables enforce,
// rather than re-deriving it and drifting.
func ValidateRepo(repo string) error {
	if !repoPattern.MatchString(repo) {
		return fmt.Errorf("repo %q must be an \"owner/name\" GitHub slug", repo)
	}
	// repoPattern allows dots, because real repository names contain them
	// (".github", "go-spew.v1"). That leaves "." and ".." matching as a whole
	// segment, so "../repo" and "owner/.." look like valid slugs. They are
	// not, and because the repo is used to build a cache path they would name
	// a directory some other repo also names.
	for _, seg := range strings.Split(repo, "/") {
		if seg == "." || seg == ".." {
			return fmt.Errorf("repo %q must be an \"owner/name\" GitHub slug: %q is not a valid path segment", repo, seg)
		}
	}
	return nil
}

// ValidateValidatedAt checks a RefKindRewrite ref's ValidatedAt against the
// version forms valid for repo: a Kubernetes release tag for
// kubernetes/kubernetes, or an ordinary semver tag, a commit SHA, or a Go
// pseudo-version for any other repository - moduleVersionForRepo resolves a
// non-default repo's version straight from its go.mod requirement, which may
// be any of the three depending on whether the dependency is tagged at all,
// and tagged dependencies are not confined to kubernetes/kubernetes's major
// version 1 convention.
//
// Exported so verify-upstream-refs can reject an explicit -tag override
// against the default repo before writing it into a --update'd
// upstream_refs.go entry - an override that isn't a real Kubernetes release
// tag would otherwise round-trip into a ValidatedAt that immediately fails
// this same check the next time RegisterAll runs.
func ValidateValidatedAt(repo, validatedAt string) error {
	if repo == defaultRepo {
		if !k8sTagPattern.MatchString(validatedAt) {
			return fmt.Errorf("validatedAt %q must be a Kubernetes release tag such as v1.36.3", validatedAt)
		}
		return nil
	}
	// module.IsPseudoVersion is Go's own definition of the format. A local
	// regex for it was wrong twice (bare form only, then rejecting hyphens
	// in prerelease identifiers). Both times semverTagPattern happened to
	// accept the string anyway - a pseudo-version is syntactically valid
	// SemVer - so validation never visibly broke and the bug survived here
	// while the same mistake caused real failures in verify-upstream-refs,
	// which has to extract the commit rather than just accept the string.
	if semverTagPattern.MatchString(validatedAt) ||
		module.IsPseudoVersion(validatedAt) ||
		commitSHAPattern.MatchString(validatedAt) {
		return nil
	}
	return fmt.Errorf("validatedAt %q must be a semver release tag, a Go pseudo-version, or a commit SHA for repo %q", validatedAt, repo)
}

// refs holds the UpstreamRef for every registered runtime check, keyed by
// check ID. It is populated by RegisterAll and read by the enforcement test
// and by the refs dump that `task verify:upstream-refs` consumes.
var (
	refsMu sync.RWMutex
	refs   = map[string]UpstreamRef{}
)

// RegisterAll registers every check in checks, requiring each to have an
// UpstreamRef in upstreamRefs keyed by its check ID.
//
// It panics on a missing or malformed ref. That is deliberate: registration
// happens from init(), so a check added without a citation fails immediately
// on any `go test` or binary start rather than shipping unnoticed. Panicking
// here is the enforcement mechanism for the 1:1-with-upstream standard.
func RegisterAll(checks []Check, upstreamRefs map[string]UpstreamRef) {
	for _, c := range checks {
		ref, ok := upstreamRefs[c.ID()]
		if !ok {
			panic(fmt.Sprintf(
				"runtime check %q has no UpstreamRef: every runtime check must cite the upstream Kubernetes function it ports (see docs/CI.md)",
				c.ID(),
			))
		}
		if err := ref.Validate(); err != nil {
			panic(fmt.Sprintf("runtime check %q has an invalid UpstreamRef: %v", c.ID(), err))
		}

		refsMu.Lock()
		refs[c.ID()] = ref
		refsMu.Unlock()

		check.Register(CheckToRegistered(c))
	}
}

// Ref returns the UpstreamRef registered for a check ID.
func Ref(id string) (UpstreamRef, bool) {
	refsMu.RLock()
	defer refsMu.RUnlock()
	r, ok := refs[id]
	return r, ok
}

// AllRefs returns every registered UpstreamRef keyed by check ID.
func AllRefs() map[string]UpstreamRef {
	refsMu.RLock()
	defer refsMu.RUnlock()
	out := make(map[string]UpstreamRef, len(refs))
	for k, v := range refs {
		out[k] = v
	}
	return out
}

// RefIDs returns the sorted set of check IDs that have a registered ref.
func RefIDs() []string {
	refsMu.RLock()
	ids := make([]string, 0, len(refs))
	for k := range refs {
		ids = append(ids, k)
	}
	refsMu.RUnlock()

	sort.Strings(ids)
	return ids
}
