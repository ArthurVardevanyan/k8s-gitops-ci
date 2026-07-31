package kustomize

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// kustomizationFilenames lists the file names kustomize itself recognizes
// for a kustomization root, in the order it tries them.
var kustomizationFilenames = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

// ResolveRefs recursively resolves every directory/file referenced by dir's
// kustomization file (resources, components, bases), returning the full,
// flattened transitive reference chain (paths relative to the caller's
// working directory, matching the form the entries themselves are written
// in, joined onto dir). Returns nil if dir has no kustomization file or it
// fails to parse.
//
// This is a pure, read-only parse of the kustomization.yaml graph (via
// gopkg.in/yaml.v3) - it never shells out to kustomize and never calls the
// kustomize Go build API (that's reserved for actually rendering an
// overlay's output, e.g. RenderKustomize); it exists purely so callers can
// answer "does this overlay depend, directly or transitively, on that
// changed directory?" without doing a full build.
func ResolveRefs(dir string) []string {
	return resolveRefs(dir, map[string]bool{})
}

// resolveRefs is the cycle-guarded recursive worker behind ResolveRefs.
// visited tracks absolute directory paths already expanded, so a
// resources/components/bases cycle (or a diamond-shaped dependency graph
// where the same base is referenced from multiple places) can't cause
// unbounded recursion.
func resolveRefs(dir string, visited map[string]bool) []string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}
	if visited[absDir] {
		return nil
	}
	visited[absDir] = true

	var data []byte
	for _, name := range kustomizationFilenames {
		if d, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			data = d
			break
		}
	}
	if data == nil {
		return nil
	}

	var kustomization struct {
		Resources  []string `yaml:"resources"`
		Components []string `yaml:"components"`
		Bases      []string `yaml:"bases"`
	}
	if err := yaml.Unmarshal(data, &kustomization); err != nil {
		return nil
	}

	entries := make([]string, 0, len(kustomization.Resources)+len(kustomization.Components)+len(kustomization.Bases))
	entries = append(entries, kustomization.Resources...)
	entries = append(entries, kustomization.Components...)
	entries = append(entries, kustomization.Bases...)

	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		resolved := filepath.Clean(filepath.Join(dir, entry))
		refs = append(refs, resolved)
		if info, err := os.Stat(resolved); err == nil && info.IsDir() {
			refs = append(refs, resolveRefs(resolved, visited)...)
		}
	}
	return refs
}
