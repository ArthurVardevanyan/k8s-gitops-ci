package exempt

import (
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
func Accepts(annotations map[string]string, id, value string) bool {
	if annotations == nil {
		return false
	}
	return annotations[Key(id)] == value
}

// SelectorMatches reports whether a selector matches a scalar finding for id.
func SelectorMatches(sel Selector, s Scalar, id string) bool {
	if sel.Check != id {
		return false
	}
	if sel.File != "" && !strings.Contains(s.File, sel.File) {
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
	if sel.Value != "" && sel.Value != s.Value {
		return false
	}
	if sel.Match != "" && !strings.Contains(s.Value, sel.Match) {
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
	if Accepts(annotations, id, s.Value) {
		return true, Applied{CheckID: id, File: s.File, Path: s.Path, Value: s.Value, Token: s.Token, Kind: s.Kind, Name: s.Name}
	}
	for _, sel := range selectors {
		if SelectorMatches(sel, s, id) {
			return true, Applied{CheckID: id, File: s.File, Path: s.Path, Value: s.Value, Token: s.Token, Kind: s.Kind, Name: s.Name}
		}
	}
	return false, Applied{}
}

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
		// allow selectors with bracket index to match numeric index
		if idxRe.MatchString(sp) && idxRe.MatchString(fp) {
			continue
		}
		return false
	}
	return true
}

var pathIndexRe = regexp.MustCompile(`\[[0-9]+\]`)
var idxRe = regexp.MustCompile(`^[0-9]+$`)

func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, ".")
	p = strings.ReplaceAll(p, ".", "/")
	p = strings.ReplaceAll(p, "[]", "/*")
	p = pathIndexRe.ReplaceAllString(p, "/*")
	parts := strings.Split(p, "/")
	for i := range parts {
		if parts[i] == "*" {
			parts[i] = "*"
		}
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}
