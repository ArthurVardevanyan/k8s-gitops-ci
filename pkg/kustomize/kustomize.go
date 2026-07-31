package kustomize

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

// CheckFix reports kustomization files that are not normalized.
func CheckFix(files []string) ([]string, error) {
	var need []string
	for _, f := range files {
		if !strings.HasSuffix(f, "kustomization.yaml") {
			continue
		}
		if strings.Contains(f, convention.ScaffoldTemplatesPrefix()) {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		normal, err := NormalizeYAML(data)
		if err != nil {
			continue
		}
		if string(normal) != string(data) {
			need = append(need, f)
		}
	}
	return need, nil
}

// NeedsKustomizeFix reports whether a single file needs normalization without mutating it.
func NeedsKustomizeFix(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	normal, err := NormalizeYAML(data)
	if err != nil {
		return false, err
	}
	return string(normal) != string(data), nil
}

// Fix normalizes all kustomization.yaml files under dir.
func Fix(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fixed []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if e.Name() != "kustomization.yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if strings.Contains(path, convention.ScaffoldTemplatesPrefix()) {
			continue
		}
		normal, err := NormalizeYAMLPath(path)
		if err != nil {
			continue
		}
		if err := os.WriteFile(path, normal, 0o644); err == nil {
			fixed = append(fixed, path)
		}
	}
	return fixed, nil
}

// NormalizeYAMLPath reads and normalizes a kustomization file.
func NormalizeYAMLPath(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NormalizeYAML(data)
}

// NormalizeYAML canonicalizes YAML field ordering (maps sorted by key).
func NormalizeYAML(data []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return data, nil
	}
	// yaml.Node doesn't retain whether the source had a leading "---"
	// document-start marker (it's semantically a no-op for a single
	// document), so we must detect it from the raw input ourselves in
	// order to preserve it and keep normalization non-destructive.
	leadingSeparator := hasLeadingDocumentMarker(data)
	var docs []yaml.Node
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var root yaml.Node
		if err := dec.Decode(&root); err != nil {
			if errors.Is(err, errorEOF()) {
				break
			}
			return nil, err
		}
		sortNodeRecursively(&root)
		docs = append(docs, root)
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for i, d := range docs {
		if i > 0 || (i == 0 && leadingSeparator) {
			buf.WriteString("---\n")
		}
		if err := enc.Encode(&d); err != nil {
			return nil, err
		}
	}
	_ = enc.Close()
	return []byte(strings.TrimSuffix(buf.String(), "\n") + "\n"), nil
}

// hasLeadingDocumentMarker reports whether the first non-blank line of data
// is a bare YAML document-start marker ("---"), e.g. a kustomization.yaml
// that begins:
//
//	---
//	apiVersion: kustomize.config.k8s.io/v1beta1
//	kind: Kustomization
func hasLeadingDocumentMarker(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return trimmed == "---"
	}
	return false
}

func sortNodeRecursively(n *yaml.Node) {
	if n.Kind == yaml.MappingNode {
		sortMappingNode(n)
	}
	for _, c := range n.Content {
		sortNodeRecursively(c)
	}
}

func sortMappingNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode || len(node.Content) < 4 {
		return
	}
	pairs := make([][2]*yaml.Node, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		pairs = append(pairs, [2]*yaml.Node{node.Content[i], node.Content[i+1]})
	}
	// preserve "apiVersion" and "kind" first if present
	priority := map[string]int{"apiVersion": 0, "kind": 1}
	sort.SliceStable(pairs, func(i, j int) bool {
		pi, oki := priority[pairs[i][0].Value]
		pj, okj := priority[pairs[j][0].Value]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return pairs[i][0].Value < pairs[j][0].Value
	})
	node.Content = node.Content[:0]
	for _, p := range pairs {
		node.Content = append(node.Content, p[0], p[1])
	}
}

func errorEOF() error {
	return io.EOF
}

// FormatFixNeeded renders a human-readable message for kustomization files needing fix.
func FormatFixNeeded(files []string) string {
	if len(files) == 0 {
		return ""
	}
	apps := AppsFromFiles(files)
	var b strings.Builder
	b.WriteString("The following kustomization.yaml files need `kustomize edit fix --vars`:\n")
	for _, a := range apps {
		b.WriteString("  - ")
		b.WriteString(a)
		b.WriteByte('\n')
	}
	return b.String()
}

// AppsFromFiles maps kustomization paths to app names.
func AppsFromFiles(files []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		if len(parts) >= 2 {
			app := parts[len(parts)-2]
			if !seen[app] {
				seen[app] = true
				out = append(out, app)
			}
		}
	}
	return out
}
