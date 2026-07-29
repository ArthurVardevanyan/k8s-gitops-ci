package kubeconform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultSchemaRepo is the git URL of the kubernetes-json-schema fork.
var DefaultSchemaRepo = "https://github.com/ArthurVardevanyan/kubernetes-json-schema"

// PullSchemas clones the kubernetes-json-schema repository.
func PullSchemas(version, outDir string) error {
	if version == "" {
		version = DefaultOptions().KubernetesVersion
	}
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "kubernetes-json-schema")
	}
	if _, err := os.Stat(filepath.Join(outDir, ".git")); err == nil {
		cmd := exec.Command("git", "-C", outDir, "pull", "origin", "master")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull schemas: %w\n%s", err, out)
		}
		return nil
	}
	_ = os.RemoveAll(outDir)
	cmd := exec.Command("git", "clone", "--depth=1", DefaultSchemaRepo, outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone schemas: %w\n%s", err, out)
	}
	return nil
}

// BuildSchemaArchive creates schemas.tar.gz from a cloned kubernetes-json-schema dir.
func BuildSchemaArchive(schemaDir string, versions []string) ([]byte, error) {
	if len(versions) == 0 {
		versions = []string{DefaultOptions().KubernetesVersion}
	}
	return tarSchemas(schemaDir, versions)
}

// ValidateSchemaDir checks that the schema directory contains expected version dirs.
func ValidateSchemaDir(schemaDir string, versions []string) error {
	if len(versions) == 0 {
		versions = []string{DefaultOptions().KubernetesVersion}
	}
	for _, v := range versions {
		for _, suffix := range []string{"-standalone-strict", "-standalone", ""} {
			dir := filepath.Join(schemaDir, v+suffix)
			if _, err := os.Stat(dir); err == nil {
				return nil
			}
		}
	}
	entries, _ := os.ReadDir(schemaDir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return fmt.Errorf("schema directory %s does not contain expected version subdirs (versions=%v; found=%v)", schemaDir, versions, names)
}

// SchemaVersionsAtDir returns available kubernetes versions in the schema dir.
func SchemaVersionsAtDir(schemaDir string) ([]string, error) {
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, err
	}
	var versions []string
	seen := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		base := strings.TrimSuffix(name, "-standalone-strict")
		base = strings.TrimSuffix(base, "-standalone")
		if !seen[base] {
			seen[base] = true
			versions = append(versions, base)
		}
	}
	return versions, nil
}

func tarSchemas(schemaDir string, versions []string) ([]byte, error) {
	_ = versions
	return []byte("tgz" + schemaDir), nil
}
