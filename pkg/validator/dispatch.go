package validator

import (
	"bytes"
	"regexp"
	"strings"
)

// normalizeScalar cleans a raw YAML scalar taken from a `key: value` header
// line so it can be compared exactly.
//
// These headers are read with minimal parsing for speed, but the raw text is
// not the value: `kind: "Pod"` yields `"Pod"` with quotes, which matches no
// registered kind and silently skips every check for that document. Since
// runtime checks are non-exemptable, a skip caused by quoting style is
// invisible rather than merely wrong. Trailing comments (`kind: Pod # why`)
// have the same effect.
func normalizeScalar(s string) string {
	s = strings.TrimSpace(s)
	// A quoted scalar may legitimately contain '#', so unquote first and
	// return - there is no trailing comment inside the quotes.
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	// An unquoted scalar ends at a ' #' comment separator.
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// quickKind extracts kind from a YAML document header with minimal parsing.
func quickKind(data []byte) string {
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if bytes.HasPrefix(line, []byte("-")) || bytes.HasPrefix(line, []byte("---")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("kind:")) {
			return normalizeScalar(string(bytes.TrimPrefix(line, []byte("kind:"))))
		}
	}
	return ""
}

// quickAPIVersion extracts apiVersion from a YAML document header.
func quickAPIVersion(data []byte) string {
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if bytes.HasPrefix(line, []byte("-")) || bytes.HasPrefix(line, []byte("---")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("apiVersion:")) {
			return normalizeScalar(string(bytes.TrimPrefix(line, []byte("apiVersion:"))))
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
