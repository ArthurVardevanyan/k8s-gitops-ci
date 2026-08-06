package kubeconform

import (
	"bytes"
	"errors"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// IsManifestYAML reports whether data looks like a Kubernetes manifest -
// i.e. at least one of its YAML documents has a top-level (root-mapping)
// `kind` or `apiVersion` key. Those two fields are the TypeMeta every
// Kubernetes object must carry, and are exactly what kubeconform keys on,
// so their presence is the only reliable "this is a manifest" signal;
// fields like metadata/data/spec are neither necessary (RBAC/ConfigMap
// objects omit spec/data) nor sufficient (plenty of non-manifest YAML
// carries a top-level data:/metadata:) and are deliberately NOT consulted.
//
// Detection is intentionally root-level only: a nested `kind:` buried in an
// Ansible var or Helm value must not make a non-manifest file look like a
// manifest, so this checks the direct children of each document's root
// mapping node rather than doing an indentation-insensitive line scan.
//
// Fail-safe biases toward validating: a file whose YAML fails to parse, or
// one that decodes to no real document at all (empty / comments-only /
// whitespace-only), returns true so the caller still routes it to
// kubeconform (which reports genuine parse errors and marks truly-empty
// input as Empty) - only a file that parses to one or more documents, none
// of which carries a root kind/apiVersion, is classified as non-manifest.
func IsManifestYAML(data []byte) bool {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	sawDoc := false
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// Malformed YAML: don't classify as non-manifest here - let the
			// kubeconform validator and the separate YAML-syntax check
			// surface the problem instead of silently skipping the file.
			return true
		}
		// A document with no content (a stray `---` separator or a
		// comments-only section) carries no header; skip it without deciding.
		if len(doc.Content) == 0 {
			continue
		}
		sawDoc = true
		if docHasManifestHeader(&doc) {
			return true
		}
	}
	// No real document decoded (empty/comments-only): leave it to the
	// validator's existing empty-file handling rather than list it as a
	// skipped non-manifest.
	return !sawDoc
}

// docHasManifestHeader reports whether a decoded YAML document's root is a
// mapping that has a top-level `kind` or `apiVersion` key.
func docHasManifestHeader(doc *yaml.Node) bool {
	if len(doc.Content) == 0 {
		return false
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false
	}
	return hasRootKey(root, "kind") || hasRootKey(root, "apiVersion")
}

// hasRootKey reports whether a YAML mapping node has a direct child key
// named key. A mapping node's Content is a flat [k1, v1, k2, v2, ...] slice,
// so keys sit at the even indexes.
func hasRootKey(mapping *yaml.Node, key string) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return true
		}
	}
	return false
}

// partitionManifests splits files into those that look like Kubernetes
// manifests (validate) and those that don't (skip), per IsManifestYAML. A
// file that can't be read is kept in manifests so the validator's own read
// path handles it exactly as before, preserving prior behavior.
func partitionManifests(files []string) (manifests, skipped []string) {
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			manifests = append(manifests, f)
			continue
		}
		if IsManifestYAML(data) {
			manifests = append(manifests, f)
		} else {
			skipped = append(skipped, f)
		}
	}
	return manifests, skipped
}
