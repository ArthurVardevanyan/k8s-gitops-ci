package crb

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ValidationError records a ClusterRoleBinding ServiceAccount subject missing namespace.
type ValidationError struct {
	File, Kind, Name, Subject, Message string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s %q subject %s: %s", e.File, e.Kind, e.Name, e.Subject, e.Message)
}

// DeduplicatedError aggregates CRB namespace findings.
type DeduplicatedError struct {
	Kind, Name, Subject string
	Count               int
}

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("%s %q subject %q missing namespace (%d overlay(s))", d.Kind, d.Name, d.Subject, d.Count)
}

// ValidateFile validates ClusterRoleBindings in a file.
func ValidateFile(path string) []ValidationError {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return ValidateBytes(data, path)
}

// ValidateBytes validates ClusterRoleBindings from bytes.
func ValidateBytes(data []byte, source string) []ValidationError {
	var errs []ValidationError
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
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
		if quickString(findKey(mapping, "kind")) != "ClusterRoleBinding" {
			continue
		}
		name := quickName(mapping)
		subjects := findKey(mapping, "subjects")
		if subjects == nil || subjects.Kind != yaml.SequenceNode {
			continue
		}
		for _, sub := range subjects.Content {
			if sub.Kind != yaml.MappingNode {
				continue
			}
			if quickString(findKey(sub, "kind")) != "ServiceAccount" {
				continue
			}
			if quickString(findKey(sub, "namespace")) == "" {
				subName := quickString(findKey(sub, "name"))
				errs = append(errs, ValidationError{
					File: source, Kind: "ClusterRoleBinding", Name: name,
					Subject: subName, Message: "ServiceAccount subject missing namespace (will default to 'default')",
				})
			}
		}
	}
	return errs
}

// Deduplicate aggregates CRB errors preserving order.
func Deduplicate(errs []ValidationError) []DeduplicatedError {
	seen := make(map[string]*DeduplicatedError)
	var order []string
	for _, e := range errs {
		key := e.Kind + "/" + e.Name + "/" + e.Subject
		if d, ok := seen[key]; ok {
			d.Count++
			continue
		}
		seen[key] = &DeduplicatedError{Kind: e.Kind, Name: e.Name, Subject: e.Subject, Count: 1}
		order = append(order, key)
	}
	var out []DeduplicatedError
	for _, k := range order {
		out = append(out, *seen[k])
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
