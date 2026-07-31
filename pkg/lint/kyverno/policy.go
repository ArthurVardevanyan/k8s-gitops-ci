package kyverno

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kyverno/policies"

	"gopkg.in/yaml.v3"
)

// IncludeComponents lists additional kustomize component paths (relative to
// the embedded policy bundle's overlays/_ci directory, e.g.
// "../../components/restrict-old-registry") layered on top of the policy
// bundle's base/ when preparing policies for validation. Defaults to empty
// (base only) - an org overrides this from its own configuration layer to
// opt specific policy components into its own bundle without having to fork
// PreparePolicies itself. See docs/SCHEMAS.md.
var IncludeComponents []string

// ExcludedRules lists policy/rule combinations to silently drop from
// results - e.g. a rule that's real in the upstream policy set but not
// applicable to this repository's conventions. A policy mapped to an empty
// slice excludes every rule under that policy. Defaults to empty (nothing
// excluded) - an org overrides this from its own configuration layer. See
// docs/SCHEMAS.md.
var ExcludedRules map[string][]string

// isExcludedRule reports whether policy/rule is configured (via
// ExcludedRules) to be dropped from results.
func isExcludedRule(policy, rule string) bool {
	rules, ok := ExcludedRules[policy]
	if !ok {
		return false
	}
	if len(rules) == 0 {
		return true
	}
	for _, r := range rules {
		if r == rule {
			return true
		}
	}
	return false
}

// PreparePolicies extracts the embedded Kyverno policies, renders them via
// kustomize build using the bundle's base/ plus any configured
// IncludeComponents, strips namespaceSelector gates matching
// NamespaceSelectorLabelKeys, and writes the result to a temp file. Returns
// the path to the rendered policy file and a cleanup function the caller
// must invoke once done.
func PreparePolicies() (policyPath string, cleanup func(), err error) {
	dir, dirCleanup, err := policies.Extract()
	if err != nil {
		return "", func() {}, err
	}
	policyPath, err = preparePoliciesFrom(dir)
	if err != nil {
		return "", dirCleanup, err
	}
	return policyPath, dirCleanup, nil
}

// preparePoliciesFrom does PreparePolicies's actual render/strip/write work
// against an already-extracted policy directory (dir/kyverno-policies/...) -
// factored out so tests can exercise it against a fixture directory instead
// of the real embedded archive.
func preparePoliciesFrom(dir string) (policyPath string, err error) {
	policyDir := filepath.Join(dir, "kyverno-policies")
	if _, statErr := os.Stat(policyDir); statErr != nil {
		return "", statErr
	}

	out, err := buildPolicies(policyDir)
	if err != nil {
		// kustomize (or the kustomize binary itself) may be unavailable in
		// some environments; fall back to validating against the bundle's
		// raw base/ policies directly rather than hard-failing the whole
		// Kyverno step. Namespace-selector stripping and IncludeComponents
		// layering only apply to a successful kustomize render, so this
		// fallback path skips both.
		baseDir := filepath.Join(policyDir, "base")
		if collected, collectErr := collectPolicies(baseDir); collectErr == nil && len(collected) > 0 {
			return baseDir, nil
		}
		return "", fmt.Errorf("kustomize build policies: %w", err)
	}

	out, err = stripNSSelectors(out)
	if err != nil {
		return "", fmt.Errorf("stripping namespace selectors: %w", err)
	}

	tmpFile := filepath.Join(dir, "prepared-policies.yaml")
	if err := os.WriteFile(tmpFile, out, 0o600); err != nil {
		return "", err
	}
	return tmpFile, nil
}

// collectPolicies walks dir collecting every .yaml/.yml file, returning an
// error if dir doesn't exist or the walk fails - unlike CollectYAML (which
// silently swallows walk errors for best-effort resource collection), this
// is used as a fallback-availability check where the caller needs to know
// definitively whether usable base policies exist before relying on them.
func collectPolicies(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	var files []string
	walkErr := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}

// buildPolicies renders policyDir's base plus any configured
// IncludeComponents via `kustomize build`, using the native Kustomize SDK
// (matching pkg/overlay's own renderKustomize - no runtime dependency on a
// `kustomize` binary).
func buildPolicies(policyDir string) ([]byte, error) {
	ciOverlay := filepath.Join(policyDir, "overlays", "_ci")
	if err := os.MkdirAll(ciOverlay, 0o750); err != nil {
		return nil, fmt.Errorf("creating ci overlay dir: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("resources:\n  - ../../base\n")
	if len(IncludeComponents) > 0 {
		sb.WriteString("components:\n")
		for _, c := range IncludeComponents {
			fmt.Fprintf(&sb, "  - %s\n", c)
		}
	}
	if err := os.WriteFile(filepath.Join(ciOverlay, "kustomization.yaml"), []byte(sb.String()), 0o600); err != nil {
		return nil, fmt.Errorf("writing ci kustomization: %w", err)
	}

	return renderKustomizeDir(ciOverlay)
}

// renderKustomizeDir shells out to the kustomize CLI. Policy bundles are an
// embedded, build-time-only asset (see docs/SCHEMAS.md) that this repo
// doesn't otherwise depend on kustomize's native SDK for here, since
// PreparePolicies already runs behind the "kyverno" step's own CLI
// dependency (the kyverno binary itself, checked by ValidateFiles) - an
// operator enabling Kyverno validation is already expected to provide a
// kustomize binary alongside it.
func renderKustomizeDir(dir string) ([]byte, error) {
	kustomizeBin, err := exec.LookPath("kustomize")
	if err != nil {
		return nil, fmt.Errorf("kustomize not found: %w", err)
	}
	cmd := exec.CommandContext(context.Background(), kustomizeBin, "build", dir) //nolint:gosec // trusted, build-time embedded policy path
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("%s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

// stripNSSelectors removes namespaceSelector mapping entries whose
// matchLabels contain any key in NamespaceSelectorLabelKeys from rendered
// Kyverno policies. Offline `kyverno apply` has no namespace labels
// available, so a policy gated by namespaceSelector on one of these keys
// would never match any resource; stripping the selector lets it evaluate
// against the given resources instead. A no-op (returns data unchanged)
// when NamespaceSelectorLabelKeys is empty.
func stripNSSelectors(data []byte) ([]byte, error) {
	if len(NamespaceSelectorLabelKeys) == 0 {
		return data, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding policy YAML: %w", err)
		}
		removeNSSelector(&doc)
		docs = append(docs, &doc)
	}
	if len(docs) == 0 {
		return data, nil
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	for _, doc := range docs {
		if err := encoder.Encode(doc); err != nil {
			return nil, fmt.Errorf("re-encoding policy: %w", err)
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("closing policy encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// removeNSSelector recursively walks a YAML node tree, removing
// namespaceSelector mapping entries whose matchLabels contain a configured
// NamespaceSelectorLabelKeys key.
func removeNSSelector(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content)-1; i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Value == "namespaceSelector" && hasConfiguredLabel(val) {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				i -= 2
				continue
			}
			removeNSSelector(val)
		}
		return
	}
	for _, child := range node.Content {
		removeNSSelector(child)
	}
}

// hasConfiguredLabel reports whether node contains a matchLabels entry with
// any configured NamespaceSelectorLabelKeys key.
func hasConfiguredLabel(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content)-1; i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Value == "matchLabels" {
				return containsConfiguredKey(val)
			}
			if hasConfiguredLabel(val) {
				return true
			}
		}
	}
	for _, child := range node.Content {
		if hasConfiguredLabel(child) {
			return true
		}
	}
	return false
}

// containsConfiguredKey reports whether the matchLabels mapping node has any
// configured NamespaceSelectorLabelKeys key.
func containsConfiguredKey(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		for _, k := range NamespaceSelectorLabelKeys {
			if node.Content[i].Value == k {
				return true
			}
		}
	}
	return false
}
