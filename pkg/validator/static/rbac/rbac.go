package rbac

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadonlyVerbs lists read-only verbs.
var ReadonlyVerbs = map[string]bool{"get": true, "list": true, "watch": true}

// AggregateLabels identifies aggregate-to-view / cluster-reader labels.
var AggregateLabels = []string{
	"rbac.authorization.k8s.io/aggregate-to-view",
	"rbac.authorization.k8s.io/aggregate-to-cluster-reader",
}

const WildcardMarker = "<!-- rbac-wildcard-warning -->"

// ValidationError records an RBAC readonly violation.
type ValidationError struct {
	File, Kind, Resource string
	BadVerbs             []string
	RuleIndex            int
	AggLabel             string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s %q has readonly aggregate label %q but rule[%d] contains non-readonly verbs: [%s]",
		e.File, e.Kind, e.Resource, e.AggLabel, e.RuleIndex, strings.Join(e.BadVerbs, ", "))
}

// DeduplicatedError is a de-duplicated RBAC readonly error.
type DeduplicatedError struct {
	Kind, Resource string
	RuleIndex      int
	BadVerbs       []string
	AggLabel       string
	Count          int
}

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("%s %q rule[%d] non-readonly verbs [%s] with label %q (%d overlay(s))",
		d.Kind, d.Resource, d.RuleIndex, strings.Join(d.BadVerbs, ", "), d.AggLabel, d.Count)
}

// WildcardError records an RBAC wildcard finding.
type WildcardError struct {
	File, Resource, Kind string
	RuleIndex            int
	Field                string
	// Annotations carries the ClusterRole/Role's own metadata.annotations,
	// so a gitops-ci.k8s.io/exempt-rbac-wildcards annotation on the
	// resource itself can grant an exemption (see exempt.Accepts) - not
	// just an EXEMPTIONS=(...) selector in test.sh.
	Annotations map[string]string
}

func (e WildcardError) String() string {
	return fmt.Sprintf("%s: %s %q rule[%d] uses wildcard '*' in %s", e.File, e.Kind, e.Resource, e.RuleIndex, e.Field)
}

// DeduplicatedWildcardError is a de-duplicated wildcard error.
type DeduplicatedWildcardError struct {
	Kind, Resource string
	RuleIndex      int
	Field          string
	Count          int
}

func (d DeduplicatedWildcardError) String() string {
	return fmt.Sprintf("%s %q rule[%d] wildcard in %s (%d overlay(s))", d.Kind, d.Resource, d.RuleIndex, d.Field, d.Count)
}

// readOnlyExempt maps apiGroup/resource to allowed verbs.
var readOnlyExempt = map[string]map[string]map[string]bool{
	"metrics.k8s.io":        {"pods": {"create": true}},
	"monitoring.coreos.com": {"prometheuses/api": {"create": true, "update": true}},
}

// ValidateFile validates RBAC readonly violations in a file.
func ValidateFile(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReader(f, path)
}

// ValidateReader validates RBAC readonly violations from a reader.
func ValidateReader(r io.Reader, source string) []ValidationError {
	var errs []ValidationError
	dec := yaml.NewDecoder(r)
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		mapping := doc.Content[0]
		if mapping.Kind != yaml.MappingNode {
			continue
		}
		if quickString(findKey(mapping, "kind")) != "ClusterRole" {
			continue
		}
		meta := findKey(mapping, "metadata")
		labels := extractLabels(meta)
		aggLabel := ""
		for _, l := range AggregateLabels {
			if labels[l] == "true" {
				aggLabel = l
				break
			}
		}
		if aggLabel == "" {
			continue
		}
		resource := quickName(mapping)
		rules := findKey(mapping, "rules")
		if rules == nil || rules.Kind != yaml.SequenceNode {
			continue
		}
		for i, ruleNode := range rules.Content {
			if ruleNode.Kind != yaml.MappingNode {
				continue
			}
			bad := badVerbs(ruleNode, source)
			if len(bad) > 0 {
				errs = append(errs, ValidationError{
					File: source, Kind: "ClusterRole", Resource: resource,
					BadVerbs: bad, RuleIndex: i, AggLabel: aggLabel,
				})
			}
		}
	}
	return errs
}

// ValidateWildcards validates wildcards in ClusterRole/Role docs.
func ValidateWildcards(path string) []WildcardError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateWildcardsReader(f, path)
}

// ValidateWildcardsReader validates wildcards from a reader.
func ValidateWildcardsReader(r io.Reader, source string) []WildcardError {
	var errs []WildcardError
	dec := yaml.NewDecoder(r)
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		mapping := doc.Content[0]
		if mapping.Kind != yaml.MappingNode {
			continue
		}
		kind := quickString(findKey(mapping, "kind"))
		if kind != "ClusterRole" && kind != "Role" {
			continue
		}
		resource := quickName(mapping)
		annotations := extractAnnotations(findKey(mapping, "metadata"))
		rules := findKey(mapping, "rules")
		if rules == nil || rules.Kind != yaml.SequenceNode {
			continue
		}
		for i, ruleNode := range rules.Content {
			if ruleNode.Kind != yaml.MappingNode {
				continue
			}
			for _, field := range []string{"verbs", "resources", "apiGroups"} {
				if hasWildcard(findKey(ruleNode, field)) {
					errs = append(errs, WildcardError{
						File: source, Kind: kind, Resource: resource,
						RuleIndex: i, Field: field, Annotations: annotations,
					})
				}
			}
		}
	}
	return errs
}

// Deduplicate de-duplicates readonly errors.
func Deduplicate(errs []ValidationError) []DeduplicatedError {
	seen := make(map[string]*DeduplicatedError)
	order := make([]string, 0, len(errs))
	for _, e := range errs {
		sortedVerbs := append([]string(nil), e.BadVerbs...)
		sort.Strings(sortedVerbs)
		key := fmt.Sprintf("%s/%s/%d/%s/%s", e.Kind, e.Resource, e.RuleIndex, e.AggLabel, strings.Join(sortedVerbs, ","))
		if d, ok := seen[key]; ok {
			d.Count++
			continue
		}
		seen[key] = &DeduplicatedError{
			Kind: e.Kind, Resource: e.Resource, RuleIndex: e.RuleIndex,
			BadVerbs: e.BadVerbs, AggLabel: e.AggLabel, Count: 1,
		}
		order = append(order, key)
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// DeduplicateWildcards de-duplicates wildcard errors.
func DeduplicateWildcards(errs []WildcardError) []DeduplicatedWildcardError {
	seen := make(map[string]*DeduplicatedWildcardError)
	order := make([]string, 0, len(errs))
	for _, e := range errs {
		key := fmt.Sprintf("%s/%s/%d/%s", e.Kind, e.Resource, e.RuleIndex, e.Field)
		if d, ok := seen[key]; ok {
			d.Count++
			continue
		}
		seen[key] = &DeduplicatedWildcardError{
			Kind: e.Kind, Resource: e.Resource, RuleIndex: e.RuleIndex,
			Field: e.Field, Count: 1,
		}
		order = append(order, key)
	}
	out := make([]DeduplicatedWildcardError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// FormatWildcardComment formats wildcard findings for a PR comment.
func FormatWildcardComment(errors []WildcardError) string {
	if len(errors) == 0 {
		return ""
	}
	ded := DeduplicateWildcards(errors)
	var b strings.Builder
	b.WriteString(WildcardMarker + "\n")
	b.WriteString("### RBAC Wildcard Usage Detected\n\n")
	b.WriteString("| Kind | Name | Rule | Field | Overlays |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, d := range ded {
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %d |\n", d.Kind, d.Resource, d.RuleIndex, d.Field, d.Count)
	}
	b.WriteString("\n<details><summary>Why avoid wildcards?</summary>\n\n")
	b.WriteString("- Wildcards can grant broader access than intended.\n")
	b.WriteString("- Prefer explicit verbs, resources, and apiGroups.\n")
	b.WriteString("</details>\n")
	return b.String()
}

func badVerbs(rule *yaml.Node, _ string) []string {
	if rule == nil {
		return nil
	}
	var bad []string
	verbs := stringSlice(findKey(rule, "verbs"))
	apiGroups := stringSlice(findKey(rule, "apiGroups"))
	resources := stringSlice(findKey(rule, "resources"))
	for _, v := range verbs {
		if v == "*" {
			bad = append(bad, v)
			continue
		}
		if ReadonlyVerbs[v] {
			continue
		}
		if !isExemptVerb(v, apiGroups, resources) {
			bad = append(bad, v)
		}
	}
	return bad
}

// isExemptVerb reports whether verb is allowed, for every listed apiGroup
// and resource, by the readOnlyExempt allowlist. Every apiGroup must have
// an exact entry in readOnlyExempt (a wildcard apiGroup, "*", is never
// exempt - it would otherwise bypass the allowlist's narrow scoping
// entirely), and every resource under that group must explicitly allow
// this verb.
//
// This does an exact map lookup per (apiGroup, resource) pair rather than
// iterating readOnlyExempt's entries directly - iterating the whole map
// and bailing on the first apiGroup that doesn't match the entry currently
// being visited made the result depend on Go's intentionally-randomized
// map iteration order, so the same input could exempt or reject a verb
// differently from one call to the next.
func isExemptVerb(verb string, apiGroups, resources []string) bool {
	if len(apiGroups) == 0 || len(resources) == 0 {
		return false
	}
	for _, g := range apiGroups {
		resMap, ok := readOnlyExempt[g]
		if !ok {
			return false
		}
		for _, r := range resources {
			verbs, ok := resMap[r]
			if !ok || !verbs[verb] {
				return false
			}
		}
	}
	return true
}

func stringSlice(n *yaml.Node) []string {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(n.Content))
	for _, c := range n.Content {
		out = append(out, c.Value)
	}
	return out
}

func hasWildcard(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.SequenceNode {
		return false
	}
	for _, c := range n.Content {
		if c.Value == "*" {
			return true
		}
	}
	return false
}

func extractLabels(meta *yaml.Node) map[string]string {
	return extractStringMap(meta, "labels")
}

// extractAnnotations returns a ClusterRole/Role's metadata.annotations, so
// callers can plumb them onto a Finding for annotation-based exemptions
// (gitops-ci.k8s.io/exempt-<check-id>) - the same mechanism extractLabels
// already supports for the readonly check's aggregate-label detection.
func extractAnnotations(meta *yaml.Node) map[string]string {
	return extractStringMap(meta, "annotations")
}

// extractStringMap reads a flat string-keyed map (labels or annotations)
// from a metadata node.
func extractStringMap(meta *yaml.Node, key string) map[string]string {
	out := make(map[string]string)
	if meta == nil || meta.Kind != yaml.MappingNode {
		return out
	}
	obj := findKey(meta, key)
	if obj == nil || obj.Kind != yaml.MappingNode {
		return out
	}
	for i := 0; i < len(obj.Content); i += 2 {
		out[obj.Content[i].Value] = obj.Content[i+1].Value
	}
	return out
}

func quickName(mapping *yaml.Node) string {
	if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
		if n := quickString(findKey(meta, "name")); n != "" {
			return n
		}
	}
	return "(unnamed)"
}

func quickString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
