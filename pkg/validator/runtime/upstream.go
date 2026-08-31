package runtime

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// UpstreamRef identifies the exact upstream Kubernetes code a runtime check
// ports.
//
// Every runtime check must supply one. The family is always-blocking and
// non-exemptable, which is only defensible if the API server really would
// reject the manifest - so a check has to be able to point at the specific
// upstream function it reproduces. A citation naming only a file is not
// enough: "k8s.io/kubernetes/pkg/apis/core/validation/validation.go" is
// equally true of a faithful port and of an invented rule, and that is
// exactly how a large number of fabricated checks once passed review here.
//
// Line numbers are deliberately absent. They drift on every upstream release,
// and every incorrect citation in this repository's history was a stale line
// range. Function names are stable and mechanically verifiable.
//
// See docs/CI.md for the standard and `task verify:upstream-refs` for the
// tooling that proves the cited functions exist and are unchanged.
type UpstreamRef struct {
	// Path is the file, relative to the root of kubernetes/kubernetes, that
	// contains the ported rule. Using repo-relative paths lets refs into
	// staging modules (apimachinery, apiextensions-apiserver) use the same
	// form as refs into pkg/apis/*.
	Path string `json:"path"`
	// Functions are the upstream function(s) this check ports.
	Functions []string `json:"functions"`
	// Digest is "sha256:<hex>" over the normalized source of Functions
	// (comments and formatting stripped), taken at ValidatedAt.
	Digest string `json:"digest"`
	// ValidatedAt is the kubernetes/kubernetes tag the digest was taken at,
	// e.g. "v1.36.3". It records the version a human last validated this
	// port against. It is NOT a claim that the check is correct for every
	// cluster version - see the version-skew section in docs/CI.md.
	ValidatedAt string `json:"validatedAt"`
	// Note records which upstream branch this check ports and every
	// deliberate divergence from it, with the reason.
	//
	// It is required. A citation proves the cited function exists and has
	// not changed; it says nothing about how much of that function the
	// check actually implements. Most of these ports are partial by design
	// - skipping a Required branch that defaulting makes unreachable, or a
	// feature-gated branch this tool cannot evaluate - and without the note
	// a reviewer cannot tell a deliberate subset from an incomplete port.
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
	// tagPattern matches a Kubernetes release tag.
	tagPattern = regexp.MustCompile(`^v1\.\d+\.\d+$`)
	// identPattern matches a Go identifier.
	identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// pathPattern rejects a ":line" suffix, pinning the no-line-numbers rule.
	pathLineSuffix = regexp.MustCompile(`:\d+(-\d+)?$`)
)

// Validate reports whether the ref is structurally usable. It does not touch
// the network; `task verify:upstream-refs` performs the upstream check.
func (r UpstreamRef) Validate() error {
	if r.Path == "" {
		return fmt.Errorf("path is required")
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
	if !digestPattern.MatchString(r.Digest) {
		return fmt.Errorf("digest %q must be sha256:<64 hex chars>; run 'task verify:upstream-refs -- --update'", r.Digest)
	}
	if !tagPattern.MatchString(r.ValidatedAt) {
		return fmt.Errorf("validatedAt %q must be a Kubernetes release tag such as v1.36.3", r.ValidatedAt)
	}
	if strings.TrimSpace(r.Note) == "" {
		return fmt.Errorf("note is required: record which upstream branch is ported and any deliberate divergence, so a partial port is distinguishable from an incomplete one")
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
