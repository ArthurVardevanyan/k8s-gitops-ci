package validator

import (
	"bytes"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

// normalizeScalar cleans a raw YAML scalar taken from a `key: value` header
// line so it can be compared exactly.
//
// These headers are read with minimal parsing for speed, but the raw text is
// not the value: `kind: "Pod"` yields `"Pod"` with quotes, which matches no
// registered kind and silently skips every check for that document. Since
// runtime checks are non-exemptable, a skip caused by quoting style is
// invisible rather than merely wrong.
//
// The comment must be stripped before unquoting and must itself be
// quote-aware, because both orders of the two features occur: `kind: "Pod"
// # why` has a comment after a quoted scalar, while `kind: "Pod # not a
// comment"` has a '#' inside one.
func normalizeScalar(s string) string {
	s = strings.TrimSpace(s)

	// Drop a trailing comment. In YAML a '#' only opens one at the start of
	// the scalar or after whitespace, and never inside quotes.
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			s = s[:i]
			i = len(s)
		}
	}
	s = strings.TrimSpace(s)

	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// quickKind extracts the document's root-level kind.
func quickKind(data []byte) string {
	return rootScalar(data, "kind")
}

// quickAPIVersion extracts the document's root-level apiVersion.
func quickAPIVersion(data []byte) string {
	return rootScalar(data, "apiVersion")
}

// quickNamespace extracts the document's metadata.namespace. It is used to
// populate check.Finding.Namespace on resource-compliance findings so that
// attribution keys (resourceKeyFor) stay namespace-aware. Without it, two
// resources sharing a Kind and Name across different namespaces (e.g. a
// workload sourced from a shared component and referenced by many overlays)
// collapse onto one "Kind/Name" key, which misattributes unrelated findings
// and can turn a pre-existing issue in an untouched namespace into a blocking
// one when the PR only changed the other namespace's copy.
func quickNamespace(data []byte) string {
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return ""
	}
	meta, _ := root["metadata"].(map[string]interface{})
	ns, _ := meta["namespace"].(string)
	return ns
}

// rootScalar returns the value of a root-level string field.
//
// This drives dispatch, so an answer that is merely close is worse than no
// answer: naming a kind no check claims silently skips every rule for the
// document, and because runtime findings are non-exemptable there is nothing
// downstream to notice the absence.
//
// A line scan cannot do this correctly. Trimming each line before testing it
// makes a nested key indistinguishable from a root one, so a RoleBinding
// whose roleRef.kind appears above its own kind dispatches as a ClusterRole,
// and every RoleBinding rule is skipped. A scan also leaves escapes encoded,
// so kind: "P\u006fd" matches nothing. The parser has none of these problems,
// and the document is parsed again by every check that runs against it, so
// one more parse is not what makes dispatch expensive.
func rootScalar(data []byte, key string) string {
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err == nil {
		s, _ := root[key].(string)
		return s
	}
	// The parser rejected the document. It is not a manifest any check can
	// act on, but the caller still wants a best-effort label for it, and a
	// root-level key is the only one worth guessing at.
	return scanRootScalar(data, key)
}

// scanRootScalar looks for an unindented "key:" line, used only for input the
// YAML parser rejected.
func scanRootScalar(data []byte, key string) string {
	prefix := []byte(key + ":")
	for _, line := range bytes.Split(data, []byte("\n")) {
		// Only column zero: an indented match is a nested field, which is
		// the thing that made the previous implementation wrong.
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		if bytes.HasPrefix(line, prefix) {
			return normalizeScalar(string(bytes.TrimPrefix(line, prefix)))
		}
	}
	return ""
}

var kyvernoGroupRe = regexp.MustCompile(`(?i)^kyverno\.io/(v\d+)$`)

// isKyvernoPolicyDoc returns true if the document is a Kyverno policy.
func isKyvernoPolicyDoc(data []byte) bool {
	av := quickAPIVersion(data)
	if !kyvernoGroupRe.MatchString(av) {
		return false
	}
	k := quickKind(data)
	return strings.EqualFold(k, "ClusterPolicy") || strings.EqualFold(k, "Policy")
}
