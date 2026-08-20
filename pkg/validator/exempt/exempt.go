package exempt

import (
	"path/filepath"
	"regexp"
	"strings"
)

// AnnotationPrefix for self-granting exemptions.
const AnnotationPrefix = "gitops-ci.k8s.io/"

// Check identifiers used by the shared engine. Most are exemptable (see
// the exemptable map below); a couple are deliberately not:
// IDImageFQDN (an unqualified image reference is almost always a mistake,
// and the framework's own escape hatches - annotation exact-value and
// EXEMPTIONS selectors - don't fit it well; a genuine structural exception,
// e.g. an OpenShift ImageStream-triggered bare reference, should get a
// targeted skip in the check itself instead) and IDClusterIdentity (a
// non-exemptable structural bucket, see Exemptable).
const (
	IDImageChecksum   = "image-checksum"
	IDImageFQDN       = "image-fqdn"
	IDClusterName     = "cluster-name"
	IDProjectRef      = "project-ref"
	IDClusterIdentity = "cluster-identity" // non-exemptable structural bucket
	IDLargeFile       = "large-file"
)

// Scalar is a generic finding value used for exemption matching.
type Scalar struct {
	Value, Path, File, Kind, Name, Namespace, Token string

	// MatchAliases holds additional stable values that should also count
	// as a match for an annotation or a Value/Match selector, alongside
	// annotationValue() (Token when set, else Value). This is purely
	// additive - it never changes what annotationValue() itself resolves
	// to, so a check that deliberately excludes its own human-readable
	// Value from matching by setting a Token (see annotationValue) is
	// unaffected unless it explicitly also sets an alias. Used by
	// image-checksum for a tag/digest-independent "registry/repo" key, so
	// an annotation naming just the repo exempts every tag/digest of it,
	// while an annotation naming the exact tagged/digested reference still
	// matches too.
	MatchAliases []string

	// ExemptAnnotationVals holds values that the exemption annotation
	// should match against, independently of Value/Token. Unlike
	// MatchAliases (which are checked alongside Value for the same
	// finding), each ExemptAnnotationVals entry is an alternate
	// "what this finding represents" that the annotation should be
	// allowed to match. Used by image-checksum so an annotation like
	// "cuda,nvidia/driver" exempts individual images.
	ExemptAnnotationVals []string
}

// Selector configures an EXEMPTIONS entry.
type Selector struct {
	Check, File, Kind, Name, Namespace, Match, Value, Path string

	// Dir, when set, requires the finding's file path to begin with
	// dir+"/" (i.e. dir is a directory at the repository root, such as
	// ".tekton"). Unlike File (basename/path-suffix matching), this is a
	// root-anchored path-prefix match - it intentionally does NOT match
	// a same-named directory nested elsewhere in the repo (e.g.
	// "apps/foo/.tekton/x.yaml" does not match Dir: ".tekton").
	Dir string
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
	IDLargeFile:     true,
}

// Exemptable reports whether a check id supports exemptions.
//
// IDImageFQDN is deliberately hardcoded to false here, the same way
// IDClusterIdentity is: check.Register unconditionally calls
// RegisterExemptable(c.ID()) for every registered check (so that, absent
// this guard, image-fqdn would still end up selector-exemptable purely by
// virtue of being registered) - see the doc comment on the ID constants
// above for why image-fqdn is meant to stay non-exemptable.
func Exemptable(id string) bool {
	if id == IDClusterIdentity || id == IDImageFQDN {
		return false
	}
	return exemptable[id]
}

// RegisterExemptable marks a check id as exemptable. A no-op for
// IDClusterIdentity/IDImageFQDN even if called, since Exemptable hardcodes
// both to false regardless of this map.
func RegisterExemptable(id string) {
	if id == "" || id == IDClusterIdentity || id == IDImageFQDN {
		return
	}
	exemptable[id] = true
}

// Known reports whether the id has been registered.
func Known(id string) bool {
	return exemptable[id] || id == IDClusterIdentity || id == IDImageFQDN
}

// Key returns the annotation key for an exemption id.
func Key(id string) string { return AnnotationPrefix + "exempt-" + id }

// Accepts reports whether annotations grant an exact-value exemption,
// matching either value, any of the optional aliases, or any of the
// exempt annotation values. When exemptAnnotationValues is set, the
// annotation value is split by comma so each entry is checked
// independently (e.g. "img1,img2" exempts individual images). When
// exemptAnnotationValues is empty, the annotation value is compared as a
// single string. Fails closed: an empty value/alias never matches, even
// against an empty (but present) annotation - otherwise a finding with an
// empty annotationValue() (e.g. both Token/Value unset) would be granted
// a false exemption by any resource with no matching annotation at all.
func Accepts(annotations map[string]string, id, value string, aliases, exemptAnnotationValues []string) bool {
	if len(annotations) == 0 {
		return false
	}
	ann := annotations[Key(id)]
	if ann == "" {
		return false
	}
	var entries []string
	if len(exemptAnnotationValues) > 0 {
		entries = splitComma(ann)
	}
	// Build the full list of targets to check (value + aliases + exemptAnnotationValues).
	targets := make([]string, 0, 1+len(aliases)+len(exemptAnnotationValues))
	if value != "" {
		targets = append(targets, value)
	}
	targets = append(targets, aliases...)
	targets = append(targets, exemptAnnotationValues...)
	for _, target := range targets {
		if target == "" {
			continue
		}
		if len(entries) > 0 {
			if contains(entries, target) {
				return true
			}
		} else if ann == target {
			return true
		}
	}
	return false
}

// splitComma splits a comma-separated annotation value into trimmed,
// non-empty entries. Empty entries (from trailing commas or multiple
// consecutive commas) are dropped.
func splitComma(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		entry := strings.TrimSpace(part)
		if entry != "" {
			out = append(out, entry)
		}
	}
	return out
}

// contains reports whether sl contains target.
func contains(sl []string, target string) bool {
	for _, s := range sl {
		if s == target {
			return true
		}
	}
	return false
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

// matchesValue reports whether target equals this scalar's primary match
// value (annotationValue()) or any of its MatchAliases.
func (s Scalar) matchesValue(target string) bool {
	if target == s.annotationValue() {
		return true
	}
	for _, a := range s.MatchAliases {
		if a == target {
			return true
		}
	}
	return false
}

// containsMatch reports whether sub is a substring of this scalar's
// primary match value (annotationValue()) or any of its MatchAliases.
func (s Scalar) containsMatch(sub string) bool {
	if strings.Contains(s.annotationValue(), sub) {
		return true
	}
	for _, a := range s.MatchAliases {
		if strings.Contains(a, sub) {
			return true
		}
	}
	return false
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
	if sel.Value != "" && !s.matchesValue(sel.Value) {
		return false
	}
	if sel.Match != "" && !s.containsMatch(sel.Match) {
		return false
	}
	if sel.Path != "" && !pathMatches(sel.Path, s.Path) {
		return false
	}
	if sel.Dir != "" && !dirMatches(sel.Dir, s.File) {
		return false
	}
	return true
}

// Evaluate checks annotation and selector exemptions for id against scalar s.
func Evaluate(id string, s Scalar, annotations map[string]string, selectors []Selector) (bool, Applied) {
	if !Exemptable(id) {
		return false, Applied{}
	}
	if Accepts(annotations, id, s.annotationValue(), s.MatchAliases, s.ExemptAnnotationVals) {
		return true, Applied{CheckID: id, File: s.File, Path: s.Path, Value: s.Value, Token: s.Token, Kind: s.Kind, Name: s.Name}
	}
	for _, sel := range selectors {
		if SelectorMatches(sel, s, id) {
			return true, Applied{CheckID: id, File: s.File, Path: s.Path, Value: s.Value, Token: s.Token, Kind: s.Kind, Name: s.Name}
		}
	}
	return false, Applied{}
}

// dirMatches reports whether file lives under a top-level repository
// directory named dir - i.e. file == dir, or file has a "dir/" prefix.
// Root-anchored on purpose (see Selector.Dir's doc comment): this must not
// degrade to a substring/suffix check, or a nested same-named directory
// elsewhere in the repo would match too.
func dirMatches(dir, file string) bool {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return false
	}
	file = strings.TrimPrefix(file, "./")
	return file == dir || strings.HasPrefix(file, dir+"/")
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
