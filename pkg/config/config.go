package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

// defaultBinaryName is the executable name used in fix hints when the caller
// does not supply a branded binary name.
const defaultBinaryName = "k8s-gitops-ci"

// Dir returns the scaffold configs directory.
func Dir() string {
	return filepath.Join(convention.ScaffoldDir, "configs")
}

// SortConfigs sorts override keys in every config file in Dir.
func SortConfigs() (int, error) {
	d := Dir()
	entries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("%s directory not found", d)
		}
		return 0, err
	}
	var count int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(d, e.Name())
		if err := sortFile(path); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// CheckSortOrder returns files in Dir whose overrides are not sorted.
func CheckSortOrder() ([]string, error) {
	d := Dir()
	entries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var unsorted []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(d, e.Name())
		ok, err := checkFile(path)
		if err != nil {
			return nil, err
		}
		if !ok {
			unsorted = append(unsorted, path)
		}
	}
	return unsorted, nil
}

// FormatUnsortedError formats the list of unsorted config files. binaryName
// is the invoked executable name used in the runnable fix hint; when empty it
// falls back to the default binary name.
func FormatUnsortedError(files []string, binaryName string) string {
	if len(files) == 0 {
		return ""
	}
	if binaryName == "" {
		binaryName = defaultBinaryName
	}
	var b strings.Builder
	b.WriteString("Config override keys are not sorted in the following files:\n")
	for _, f := range files {
		b.WriteString("  - ")
		b.WriteString(f)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "Run '%s sort-configs' to fix.", binaryName)
	return b.String()
}

func sortFile(path string) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sorted, err := sortBytes(original)
	if err != nil {
		return err
	}
	if string(sorted) == string(original) {
		return nil
	}
	return os.WriteFile(path, sorted, 0o644)
}

func checkFile(path string) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return true, err
	}
	sorted, err := sortBytes(original)
	if err != nil {
		return true, nil //nolint:nilerr // unparseable - treat as sorted, not a check failure
	}
	return string(sorted) == string(original), nil
}

func sortBytes(data []byte) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return data, nil
	}
	doc := root.Content[0]
	if od := findKey(doc, "overlayDefinitions"); od != nil {
		if overrides := findKey(od, "overrides"); overrides != nil {
			sortMappingNode(overrides)
		}
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return []byte(buf.String()), nil
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func sortMappingNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode || len(node.Content) < 4 {
		return
	}
	pairs := make([][2]*yaml.Node, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		pairs = append(pairs, [2]*yaml.Node{node.Content[i], node.Content[i+1]})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i][0].Value < pairs[j][0].Value
	})
	node.Content = node.Content[:0]
	for _, p := range pairs {
		node.Content = append(node.Content, p[0], p[1])
	}
}
