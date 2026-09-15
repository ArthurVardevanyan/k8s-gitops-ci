package overlay

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// k8sYAMLName is the set of filenames that kustomize recognizes as a
// kustomization root file — see the kyaml package's KustomizationFileNames.
var k8sYAMLName = map[string]bool{
	"kustomization.yaml": true,
	"kustomization.yml":  true,
	"Kustomization":      true,
}

// k8sYAMLFile reports whether name is a kustomization root file.
func k8sYAMLFile(name string) bool {
	return k8sYAMLName[filepath.Base(name)]
}

// generatorBlock represents a single configMapGenerator or secretGenerator
// entry in a kustomization.yaml. Fields are defined directly (not via
// embedding) because yaml.v3 does not unmarshal embedded structs
// correctly inside slice elements; inlining the fields avoids a silent
// data loss.
type generatorBlock struct {
	Name  string   `yaml:"name"`
	Files []string `yaml:"files"`
	Envs  []string `yaml:"envs"`
}

// k8sYAMLGenerator holds the generator entries from a kustomization.yaml.
type k8sYAMLGenerator struct {
	ConfigMapGenerator []generatorBlock `yaml:"configMapGenerator"`
	SecretGenerator    []generatorBlock `yaml:"secretGenerator"`
}

// GeneratorInputFiles returns the set of files referenced by any
// configMapGenerator or secretGenerator in the base, components, and
// overlays under appRoot. Each entry is resolved relative to its
// declaring kustomization.yaml's directory, then returned as a sorted,
// deduplicated set. Files that can't be parsed or are absent are silently
// skipped so a malformed kustomization never breaks the caller.
//
// Only non-manifest inputs (files and env entries) that kustomize
// treats as data payloads are returned — the caller is expected to
// drop matching paths from raw-pass validation so they don't show up
// as "non-manifest YAML" noise.
func GeneratorInputFiles(appRoot string) []string {
	var results []string

	kFiles := walkKustomizationFiles(appRoot)
	for _, kFile := range kFiles {
		entries := readGeneratorEntries(kFile)
		// Resolve entries relative to the kustomization file's parent dir.
		kDir := filepath.Dir(kFile)
		for _, e := range entries {
			for _, f := range append(e.Files, e.Envs...) {
				// kustomize resolves entries relative to the
				// kustomization.yaml directory; strip leading
				// "./" or "\\" to normalise edge-case paths.
				resolved := filepath.Join(kDir, f)
				results = append(results, resolved)
			}
		}
	}
	sort.Strings(results)
	return dedupSorted(results)
}

// walkKustomizationFiles walks base/, components/ (recursively), and
// overlays/ under appRoot and returns every file whose base name matches
// one of the kustomization root-file names (kustomization.yaml/.yml,
// Kustomization). It skips overlay/ dirs because overlay
// kustomization.yaml files are the "app root" boundary and their
// generators are already captured when the caller iterates roots;
// scanning overlays would duplicate base/component entries.
//
// We include overlays/ to handle cases where an overlay declares its
// own generators (which is valid kustomize — an overlay can have
// patches, generators, etc.).
func walkKustomizationFiles(root string) []string {
	var result []string
	// Walk the top-level app root dir looking for kustomization files
	// and directories to descend into.
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		fullPath := filepath.Join(root, e.Name())
		if e.IsDir() {
			switch e.Name() {
			case "base", "overlays", "components":
				result = append(result, walkDir(fullPath)...)
			}
		} else if k8sYAMLFile(e.Name()) {
			result = append(result, fullPath)
		}
	}
	return result
}

// walkDir recursively walks a directory, collecting kustomization root
// files and descending into subdirectories.
func walkDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())
		if e.IsDir() {
			result = append(result, walkDir(fullPath)...)
		} else if k8sYAMLFile(e.Name()) {
			result = append(result, fullPath)
		}
	}
	return result
}

// readGeneratorEntries reads the configMapGenerator and secretGenerator
// blocks from a kustomization.yaml file. Parse errors are silently
// skipped (return nil). The returned slice holds the files/env entries
// collected from all generator blocks.
func readGeneratorEntries(kFile string) []struct {
	Files []string
	Envs  []string
} {
	data, err := os.ReadFile(kFile)
	if err != nil {
		return nil
	}
	var gen k8sYAMLGenerator
	if err := yaml.Unmarshal(data, &gen); err != nil {
		return nil
	}
	var results []struct {
		Files []string
		Envs  []string
	}
	for _, g := range gen.ConfigMapGenerator {
		results = append(results, struct {
			Files []string
			Envs  []string
		}{Files: g.Files, Envs: g.Envs})
	}
	for _, g := range gen.SecretGenerator {
		results = append(results, struct {
			Files []string
			Envs  []string
		}{Files: g.Files, Envs: g.Envs})
	}
	return results
}

// dedupSorted returns the unique elements of a sorted string slice.
func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	out = append(out, in[0])
	for i := 1; i < len(in); i++ {
		if in[i] != in[i-1] {
			out = append(out, in[i])
		}
	}
	return out
}
