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
		Value: f.Value, Path: f.Path, File: f.File, Kind: f.Kind,
		Name: f.Name, Namespace: f.Namespace, Token: f.Token,
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

var registry sync.Map

// Register adds a check to the global registry.
func Register(c Check) {
	if c.ID() == "" {
		panic("check id must not be empty")
	}
	if _, loaded := registry.LoadOrStore(c.ID(), c); loaded {
		panic("duplicate check id: " + c.ID())
	}
	exempt.RegisterExemptable(c.ID())
}

// ByID returns a registered check by id.
func ByID(id string) (Check, bool) {
	v, ok := registry.Load(id)
	if !ok {
		return nil, false
	}
	return v.(Check), true
}

// All returns all registered checks sorted by id.
func All() []Check {
	var out []Check
	registry.Range(func(_, v any) bool {
		out = append(out, v.(Check))
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// ByScope returns checks matching the given scope.
func ByScope(s Scope) []Check {
	var out []Check
	registry.Range(func(_, v any) bool {
		c := v.(Check)
		if c.Scope() == s {
			out = append(out, c)
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
