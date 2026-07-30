package namespace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError records a namespace scope violation.
type ValidationError struct {
	File, Kind, Name, Message string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s/%s: %s", e.File, e.Kind, e.Name, e.Message)
}

// DeduplicatedError is a de-duplicated namespace violation.
type DeduplicatedError struct {
	Kind, Name, Message string
	Count               int
}

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("%s %q %s (%d overlay(s))", d.Kind, d.Name, d.Message, d.Count)
}

// ValidateFile validates a file and returns namespace-scope errors.
func ValidateFile(path string) []ValidationError {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return ValidateBytes(data, path)
}

// ValidateBytes validates YAML bytes for namespace-scope issues.
func ValidateBytes(data []byte, source string) []ValidationError {
	var errs []ValidationError
	dec := yaml.NewDecoder(bytesReader(data))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if isEOF(err) {
				break
			}
			errs = append(errs, ValidationError{File: source, Message: fmt.Sprintf("YAML decode error: %v", err)})
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
		if kind == "" || stringsHasSuffix(kind, "List") {
			continue
		}
		apiVersion := quickString(findKey(mapping, "apiVersion"))
		group := extractGroup(apiVersion)
		scopeKey := group + "/" + kind
		scope, known := lookupScope(scopeKey)
		if !known {
			name := quickName(mapping)
			errs = append(errs, ValidationError{File: source, Kind: kind, Name: name, Message: unknownResourceMsg(scopeKey)})
			continue
		}
		if scope {
			continue // cluster-scoped
		}
		if ns := findKey(mapping, "metadata"); ns != nil && ns.Kind == yaml.MappingNode {
			if quickString(findKey(ns, "namespace")) != "" {
				continue
			}
		}
		name := quickName(mapping)
		errs = append(errs, ValidationError{File: source, Kind: kind, Name: name, Message: "namespace-scoped resource missing metadata.namespace"})
	}
	return errs
}

// Deduplicate de-duplicates validation errors preserving first order.
func Deduplicate(errs []ValidationError) []DeduplicatedError {
	seen := make(map[string]int)
	order := make([]string, 0, len(errs))
	for _, e := range errs {
		key := e.Kind + "/" + e.Name + "/" + e.Message
		if _, ok := seen[key]; !ok {
			order = append(order, key)
		}
		seen[key]++
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		parts := split3(k)
		out = append(out, DeduplicatedError{Kind: parts[0], Name: parts[1], Message: parts[2], Count: seen[k]})
	}
	return out
}

func lookupScope(key string) (clusterScoped, known bool) {
	if ExtraResourceScope != nil {
		if v, ok := ExtraResourceScope[key]; ok {
			return v, true
		}
	}
	if v, ok := resourceScope[key]; ok {
		return v, true
	}
	return false, false
}

func extractGroup(apiVersion string) string {
	if apiVersion == "v1" {
		return ""
	}
	if idx := stringIndex(apiVersion, "/"); idx != -1 {
		return apiVersion[:idx]
	}
	return ""
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

func unknownResourceMsg(key string) string {
	return fmt.Sprintf("unknown resource %q not in scope map; to fix: run 'task update:scoped-resources' against a cluster with this CRD installed, or manually add %q to pkg/validator/namespace/resource_scope.go (true=cluster-scoped, false=namespace-scoped)", key, key)
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF)
}

func stringsHasSuffix(s, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}

func stringIndex(s, substr string) int {
	return strings.Index(s, substr)
}

func split3(k string) [3]string {
	parts := strings.SplitN(k, "/", 3)
	if len(parts) != 3 {
		return [3]string{k, "", ""}
	}
	return [3]string{parts[0], parts[1], parts[2]}
}
