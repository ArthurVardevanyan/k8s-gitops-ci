package validator

import (
	"bytes"
	"regexp"
	"strings"
)

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
			return strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("kind:"))))
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
			return strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("apiVersion:"))))
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
