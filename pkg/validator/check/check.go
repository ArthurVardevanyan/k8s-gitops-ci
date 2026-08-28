package check

import (
	"sort"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// Scope identifies the validation scope.
type Scope int

const (
	ScopeDoc Scope = iota
	ScopeOverlay
	ScopeFile
	ScopeRepo
)

// CountMode controls deduplication counting.
type CountMode int

const (
	CountRows CountMode = iota
	CountOccurrences
)

// Finding is a generic validator finding.
type Finding struct {
	CheckID, File, Path, Value, Token, Kind, Name, Namespace string
	Annotations                                              map[string]string
	Message                                                  string
	Count                                                    int
	Container                                                string
	Extra                                                    map[string]string
	ForcedDirect                                             bool

	// MatchAliases holds additional stable values that should also count
	// as an exemption match for this finding, alongside its Value/Token
	// (see exempt.Scalar.MatchAliases). Purely additive.
	MatchAliases []string

	// ExemptAnnotationValues holds values that the exemption annotation
	// should match against, independently of Value/Token. Unlike
	// MatchAliases (which are checked alongside Value for the same
	// finding), each ExemptAnnotationValues entry is an alternate
	// "what this finding represents" that the annotation should be
	// allowed to match. Used by image-checksum so an annotation like
	// "cuda,nvidia/driver" exempts individual images.
	ExemptAnnotationValues []string
}

// Get returns an extra value by key.
func (f Finding) Get(key string) string {
	if f.Extra == nil {
		return ""
	}
	return f.Extra[key]
}

// Scalar returns the exempt.Scalar for this finding.
func (f Finding) Scalar() exempt.Scalar {
	return exempt.Scalar{
		Value:                f.Value,
		Path:                 f.Path,
		File:                 f.File,
		Kind:                 f.Kind,
		Name:                 f.Name,
		Namespace:            f.Namespace,
		Token:                f.Token,
		MatchAliases:         f.MatchAliases,
		ExemptAnnotationVals: f.ExemptAnnotationValues,
	}
}

// Result collects findings and accepted exemptions.
type Result struct {
	Findings []Finding
	Exempted []exempt.Applied
}

// Check is the base interface for all checks.
type Check interface {
	ID() string
	Title() string
	Section() string
	Blocking() bool
	Scope() Scope
}

// DocCheck validates parsed YAML documents.
type DocCheck interface {
	Check
	CheckDoc(data []byte, source string) []Finding
}

// RenderSensitive is implemented by a DocCheck whose verdict depends on the
// kustomize/AVP-rendered output rather than the raw committed source. A
// check that returns true is evaluated against the rendered overlay stream
// (the source of truth), and its raw-source pass is suppressed for any file
// that participates in at least one successfully-rendered overlay - so a
// placeholder/image/podspec value injected or replaced by a base+overlay+
// component merge (e.g. `image: <PATCHED_BY_KUSTOMIZE>` replaced by an
// overlay `images:`/JSON-patch) is judged on the final rendered result, not
// the intermediate raw fragment. Files that never appear in any rendered
// overlay (e.g. a brand-new component not yet referenced by any
// kustomization.yaml) still fall back to the raw-source pass, so violations
// in not-yet-wired-up manifests are never silently skipped. A DocCheck that
// doesn't implement this interface runs only on raw source, exactly as
// before.
type RenderSensitive interface {
	RenderSensitive() bool
}

// RenderedDocCheck is implemented by a RenderSensitive DocCheck that needs
// to behave differently when validating already-rendered output vs. raw
// source - e.g. the placeholder check enables AVP-scheme scanning
// (<path:...>, <vault:...>) only on rendered input, where a surviving AVP
// reference is a genuine unresolved-secret failure rather than the intended
// committed state. When a check implements this interface the rendered pass
// calls CheckRenderedDoc; the raw pass always calls CheckDoc. A
// RenderSensitive DocCheck that doesn't implement it uses CheckDoc for both.
type RenderedDocCheck interface {
	CheckRenderedDoc(data []byte, source string) []Finding
}

// IsRenderSensitive reports whether c opts into rendered-output evaluation.
func IsRenderSensitive(c Check) bool {
	rs, ok := c.(RenderSensitive)
	return ok && rs.RenderSensitive()
}

// DocSkipper is implemented by a DocCheck that wants to opt certain
// documents out of validation based on their kind - e.g. a placeholder
// check skipping CustomResourceDefinition documents, whose embedded
// OpenAPI schemas can legitimately contain placeholder-shaped tokens (such
// as example values) that aren't real unresolved secrets. Checked once per
// unique document by the doc-check dispatcher before CheckDoc is called;
// a DocCheck that doesn't implement this interface is never skipped.
type DocSkipper interface {
	SkipDoc(kind string) bool
}

// OverlayCheck validates overlays.
type OverlayCheck interface {
	Check
	CheckOverlay(overlayPath, cluster string) []Finding
}

// FileCheck validates a single file.
type FileCheck interface {
	Check
	CheckFile(path string) []Finding
}

// RepoCheck validates a repository.
type RepoCheck interface {
	Check
	CheckRepo(root string) []Finding
}

// Column describes a report column.
type Column struct {
	Header string
	Cell   func(Finding) string
}

// TableSpec drives generic rendering of check results.
type TableSpec struct {
	Title             string
	Preamble          string
	Columns           []Column
	SourceKey         func(Finding) (string, string)
	SingleFileOverlay bool
	CountMode         CountMode
	DedupKey          func(Finding) string
	ResourceKey       func(Finding) string
}

// ColumnedCheck exposes a TableSpec for report rendering.
type ColumnedCheck interface {
	Check
	Table() TableSpec
}

// NonExemptable is implemented by a Check that must never be suppressible
// via an exemption annotation or EXEMPTIONS=(...) selector. Register skips
// exempt.RegisterExemptable for these, so exempt.Known reports false and no
// selector can ever match them.
//
// Runtime-validation checks (pkg/validator/runtime) implement this: they
// describe manifests the API server itself would reject, so "exempting" one
// would only hide a failure that reappears at apply time.
type NonExemptable interface {
	NonExemptable() bool
}

var registry sync.Map

// Register adds a check to the global registry.
func Register(c Check) {
	if c.ID() == "" {
		panic("check id must not be empty")
	}
	if _, loaded := registry.LoadOrStore(c.ID(), c); loaded {
		panic("duplicate check id: " + c.ID())
	}
	if ne, ok := c.(NonExemptable); ok && ne.NonExemptable() {
		return
	}
	exempt.RegisterExemptable(c.ID())
}

// ByID returns a registered check by id.
func ByID(id string) (Check, bool) {
	v, ok := registry.Load(id)
	if !ok {
		return nil, false
	}
	c, ok := v.(Check)
	return c, ok
}

// All returns all registered checks sorted by id.
func All() []Check {
	var out []Check
	registry.Range(func(_, v any) bool {
		if c, ok := v.(Check); ok {
			out = append(out, c)
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// ByScope returns checks matching the given scope.
func ByScope(s Scope) []Check {
	var out []Check
	registry.Range(func(_, v any) bool {
		c, ok := v.(Check)
		if !ok {
			return true
		}
		if c.Scope() == s {
			out = append(out, c)
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// PartitionByRenderSensitivity splits checks into those that opt into
// rendered-output evaluation (see RenderSensitive) and those that run only
// on raw source. Both slices preserve the input order.
func PartitionByRenderSensitivity(checks []Check) (rendered, raw []Check) {
	for _, c := range checks {
		if IsRenderSensitive(c) {
			rendered = append(rendered, c)
		} else {
			raw = append(raw, c)
		}
	}
	return rendered, raw
}
