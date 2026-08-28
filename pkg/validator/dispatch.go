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
