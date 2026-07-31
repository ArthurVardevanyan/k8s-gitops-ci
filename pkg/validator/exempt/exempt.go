package exempt

import (
	"path/filepath"
	"regexp"
	"strings"
)

// AnnotationPrefix for self-granting exemptions.
const AnnotationPrefix = "gitops-ci.k8s.io/"

// Exemptable check identifiers.
const (
	IDImageChecksum   = "image-checksum"
	IDClusterName     = "cluster-name"
	IDProjectRef      = "project-ref"
	IDClusterIdentity = "cluster-identity" // non-exemptable structural bucket
)

// Scalar is a generic finding value used for exemption matching.
type Scalar struct {
	Value, Path, File, Kind, Name, Namespace, Token string
}

// Selector configures an EXEMPTIONS entry.
type Selector struct {
	Check, File, Kind, Name, Namespace, Match, Value, Path string
}

// Applied records an accepted exemption.
type Applied struct {
	CheckID, File, Path, Value, Token, Kind, Name string
	Direct                                        bool
}

var exemptable = map[string]bool{
	IDImageChecksum: true,
	IDClusterName:   true,
	IDProjectRef:    true,
}

// Exemptable reports whether a check id supports exemptions.
func Exemptable(id string) bool {
	if id == IDClusterIdentity {
		return false
	}
	return exemptable[id]
}

// RegisterExemptable marks a check id as exemptable.
func RegisterExemptable(id string) {
	if id == "" || id == IDClusterIdentity {
		return
	}
	exemptable[id] = true
}

// Known reports whether the id has been registered.
func Known(id string) bool { return exemptable[id] || id == IDClusterIdentity }

// Key returns the annotation key for an exemption id.
func Key(id string) string { return AnnotationPrefix + "exempt-" + id }

// Accepts reports whether annotations grant an exact-value exemption.
// Fails closed: an empty value never matches, even against an empty (but
// present) annotation - otherwise a finding with an empty
// annotationValue() (e.g. both Token/Value unset) would be granted a
// false exemption by any resource with no matching annotation at all.
func Accepts(annotations map[string]string, id, value string) bool {
	if len(annotations) == 0 || value == "" {
		return false
	}
	return annotations[Key(id)] == value
}

// annotationValue returns the value used for exemption matching: Token
// when set (a stable, machine-oriented identifier a check can use instead
// of its human-readable display value), defaulting to Value when Token is
// empty.
func (s Scalar) annotationValue() string {
	if s.Token != "" {
		return s.Token
	}
	return s.Value
}

// SelectorMatches reports whether a selector matches a scalar finding for id.
func SelectorMatches(sel Selector, s Scalar, id string) bool {
	if sel.Check != id {
		return false
	}
	if sel.File != "" && !fileMatches(sel.File, s.File) {
		return false
	}
	if sel.Kind != "" && sel.Kind != s.Kind {
		return false
	}
	if sel.Name != "" && sel.Name != s.Name {
		return false
	}
	if sel.Namespace != "" && sel.Namespace != s.Namespace {
		return false
	}
	if sel.Value != "" && sel.Value != s.annotationValue() {
		return false
	}
	if sel.Match != "" && !strings.Contains(s.annotationValue(), sel.Match) {
		return false
	}
	if sel.Path != "" && !pathMatches(sel.Path, s.Path) {
		return false
	}
	return true
}

// Evaluate checks annotation and selector exemptions for id against scalar s.
func Evaluate(id string, s Scalar, annotations map[string]string, selectors []Selector) (bool, Applied) {
	if !Exemptable(id) {
		return false, Applied{}
	}
	if Accepts(annotations, id, s.annotationValue()) {
		return true, Applied{CheckID: id, File: s.File, Path: s.Path, Value: s.Value, Token: s.Token, Kind: s.Kind, Name: s.Name}
	}
	for _, sel := range selectors {
		if SelectorMatches(sel, s, id) {
			return true, Applied{CheckID: id, File: s.File, Path: s.Path, Value: s.Value, Token: s.Token, Kind: s.Kind, Name: s.Name}
		}
	}
	return false, Applied{}
}

// fileMatches reports whether a selector's File value matches a finding's
// file path: either an exact basename match (want == filepath.Base(file)),
// or a "/"+want path-suffix match. This is intentionally anchored rather
// than a raw substring check - a bare strings.Contains would let
// File: "app" match unrelated paths like "myapp-config.yaml" or
// "app-old/whatever.yaml".
func fileMatches(want, file string) bool {
	if want == "" {
		return true
	}
	if want == filepath.Base(file) {
		return true
	}
	return strings.HasSuffix(file, "/"+want)
}

// pathMatches reports whether selPath (a selector's Path field) matches
// findingPath (a finding's Path field), aligning the selector as a suffix
// of the finding path. A selector segment of "*" (from an empty bracket
// "[]") matches any index; a selector segment with a literal numeric index
// (from "[N]") only matches that exact index in the finding path - it does
// not degrade to "any index the same way". This is what makes array-index
// pinning (e.g. "containers[1].image" matching only index 1, not any
// index) actually possible.
func pathMatches(selPath, findingPath string) bool {
	sel := normalizePath(selPath)
	find := normalizePath(findingPath)
	selParts := strings.Split(sel, "/")
	findParts := strings.Split(find, "/")
	if len(selParts) > len(findParts) {
		return false
	}
	for i := range selParts {
		j := len(findParts) - len(selParts) + i
		sp := selParts[i]
		fp := findParts[j]
		if sp == "*" || sp == fp {
			continue
		}
		return false
	}
	return true
}

// pathIndexRe matches a bracketed array index, capturing the digits (empty
// for "[]", the literal index for "[N]").
var pathIndexRe = regexp.MustCompile(`\[([0-9]*)\]`)

// normalizePath converts a selector/finding path into slash-separated
// segments. "name[N]" becomes "name/N" (preserving the literal index, so
// it can be pinned exactly); "name[]" becomes "name/*" (an explicit,
// intentional wildcard). Dots are treated as segment separators.
func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, ".")
	p = strings.ReplaceAll(p, ".", "/")
	p = pathIndexRe.ReplaceAllStringFunc(p, func(m string) string {
		idx := pathIndexRe.FindStringSubmatch(m)[1]
		if idx == "" {
			return "/*"
		}
		return "/" + idx
	})
	return strings.Trim(p, "/")
}
