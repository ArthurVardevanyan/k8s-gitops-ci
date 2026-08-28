package psa

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const Marker = "<!-- psa-namespace-labels -->"

// ValidModes lists the required PSA modes.
var ValidModes = []string{"enforce", "warn", "audit"}

// ValidLevels lists valid PSA levels.
var ValidLevels = map[string]bool{"restricted": true, "baseline": true, "privileged": true}

// ValidationError records a missing/invalid PSA label.
type ValidationError struct {
	File, Name    string
	MissingLabels []string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: Namespace %q missing PSA labels: [%s]", e.File, e.Name, strings.Join(e.MissingLabels, ", "))
}

// DeduplicatedError aggregates PSA errors.
type DeduplicatedError struct {
	Name          string
	MissingLabels []string
	Count         int
}

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("Namespace %q missing PSA labels: [%s] (%d overlay(s))", d.Name, strings.Join(d.MissingLabels, ", "), d.Count)
}

// ValidateFile validates Namespace PSA labels in a file.
func ValidateFile(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReader(f, path)
}

// ValidateReader validates Namespace PSA labels from a reader.
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
		if quickString(findKey(mapping, "kind")) != "Namespace" {
			continue
		}
		name := ""
		if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
			name = quickString(findKey(meta, "name"))
		}
		labels := extractLabels(findKey(mapping, "metadata"))
		var missing []string
		for _, mode := range ValidModes {
			levelKey := "pod-security.kubernetes.io/" + mode
			versionKey := levelKey + "-version"
			if v, ok := labels[levelKey]; !ok {
				missing = append(missing, levelKey)
			} else if !ValidLevels[v] {
				missing = append(missing, fmt.Sprintf("%s (invalid value %q)", levelKey, v))
			}
			if v, ok := labels[versionKey]; !ok {
				missing = append(missing, versionKey)
			} else if v != "latest" {
				missing = append(missing, fmt.Sprintf("%s (expected \"latest\", got %q)", versionKey, v))
			}
		}
		if len(missing) > 0 {
			errs = append(errs, ValidationError{File: source, Name: name, MissingLabels: missing})
		}
	}
	return errs
}

// Deduplicate aggregates validation errors preserving order.
func Deduplicate(errs []ValidationError) []DeduplicatedError {
	seen := make(map[string]*DeduplicatedError)
	order := make([]string, 0, len(errs))
	for _, e := range errs {
		key := e.Name
		if d, ok := seen[key]; ok {
			d.Count++
			for _, l := range e.MissingLabels {
				if !contains(d.MissingLabels, l) {
					d.MissingLabels = append(d.MissingLabels, l)
				}
			}
			continue
		}
		labels := append([]string{}, e.MissingLabels...)
		sort.Strings(labels)
		seen[key] = &DeduplicatedError{Name: e.Name, MissingLabels: labels, Count: 1}
		order = append(order, key)
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// FormatComment renders missing PSA labels as a PR comment block.
func FormatComment(errs []ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	ded := Deduplicate(errs)
	var b strings.Builder
	b.WriteString(Marker + "\n")
	b.WriteString("> [!WARNING]\n")
	b.WriteString("### Namespace Pod Security Admission Labels Missing\n\n")
	b.WriteString("| Namespace | Missing Labels | Overlays |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, d := range ded {
		labels := ""
		for i, l := range d.MissingLabels {
			if i > 0 {
				labels += ", "
			}
			labels += "`" + l + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d |\n", d.Name, labels, d.Count)
	}
	b.WriteString("\n<details><summary>Required PSA labels</summary>\n\n")
	b.WriteString("```yaml\nmetadata:\n  labels:\n")
	for _, mode := range ValidModes {
		fmt.Fprintf(&b, "    pod-security.kubernetes.io/%s: restricted\n", mode)
		fmt.Fprintf(&b, "    pod-security.kubernetes.io/%s-version: latest\n", mode)
	}
	b.WriteString("```\n</details>\n")
	b.WriteString("Valid levels: `restricted`, `baseline`, `privileged`")
	return b.String()
}

// FindCommentedNamespaces scans every YAML file under dir/base for
// commented-out PSA labels. It walks the whole base/ directory rather than
// looking only for a file literally named namespace.yaml, since the
// Namespace manifest may live under any filename.
func FindCommentedNamespaces(dir string) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	baseDir := filepath.Join(dir, "base")
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil //nolint:nilerr // filepath.Walk convention: skip entry, keep walking
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // skip unreadable file, keep walking
		}
		for ns, ls := range findCommentedPSALabels(data) {
			if out[ns] == nil {
				out[ns] = ls
			} else {
				for l := range ls {
					out[ns][l] = true
				}
			}
		}
		return nil
	})
	return out
}

// commentedLabelSuffixes lists the label-key suffixes checked alongside
// each PSA mode: "" for the level label (e.g.
// "pod-security.kubernetes.io/enforce") and "-version" for its paired
// version label (e.g. "pod-security.kubernetes.io/enforce-version").
var commentedLabelSuffixes = []string{"", "-version"}

func findCommentedPSALabels(data []byte) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	var curNs string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			inner := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			for _, mode := range ValidModes {
				for _, suffix := range commentedLabelSuffixes {
					key := "pod-security.kubernetes.io/" + mode + suffix
					if strings.HasPrefix(inner, key+":") {
						if curNs != "" {
							if out[curNs] == nil {
								out[curNs] = make(map[string]bool)
							}
							out[curNs][key] = true
						}
					}
				}
			}
		}
		if strings.HasPrefix(trimmed, "name:") {
			curNs = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		}
	}
	return out
}

func extractLabels(metadata *yaml.Node) map[string]string {
	labels := make(map[string]string)
	if metadata == nil || metadata.Kind != yaml.MappingNode {
		return labels
	}
	obj := findKey(metadata, "labels")
	if obj == nil || obj.Kind != yaml.MappingNode {
		return labels
	}
	for i := 0; i < len(obj.Content); i += 2 {
		labels[obj.Content[i].Value] = obj.Content[i+1].Value
	}
	return labels
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

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
