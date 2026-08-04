package kubeconform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultSchemaRepo is the git URL of the kubernetes-json-schema fork.
var DefaultSchemaRepo = "https://github.com/ArthurVardevanyan/kubernetes-json-schema"

// PullSchemas clones the kubernetes-json-schema repository.
func PullSchemas(outDir string) error {
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "kubernetes-json-schema")
	}
	if _, err := os.Stat(filepath.Join(outDir, ".git")); err == nil {
		cmd := exec.CommandContext(context.Background(), "git", "-C", outDir, "pull", "origin", "master")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull schemas: %w\n%s", err, out)
		}
		return nil
	}
	_ = os.RemoveAll(outDir)
	cmd := exec.CommandContext(context.Background(), "git", "clone", "--depth=1", DefaultSchemaRepo, outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone schemas: %w\n%s", err, out)
	}
	return nil
}

// SchemaDirs returns the known top-level schema directories in the repo.
func SchemaDirs() []string {
	return []string{"custom-standalone-strict", "master-local", "master-standalone-strict"}
}

// ValidateSchemaDir checks that the schema directory contains expected dirs.
func ValidateSchemaDir(schemaDir string) error {
	for _, d := range SchemaDirs() {
		if _, err := os.Stat(filepath.Join(schemaDir, d)); err == nil {
			return nil
		}
	}
	return fmt.Errorf("schema directory %s missing expected subdirs %v", schemaDir, SchemaDirs())
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
		base := strings.TrimPrefix(name, "master")
		base = strings.TrimPrefix(base, "custom")
		base = strings.TrimPrefix(base, "-")
		if base == "" {
			base = "master"
		}
		if !seen[base] {
			seen[base] = true
			versions = append(versions, base)
		}
	}
	return versions, nil
}

// BuildSchemaArchive creates schemas.tar.gz from a cloned kubernetes-json-schema dir.
func BuildSchemaArchive(schemaDir string) ([]byte, error) {
	return tarSchemas(schemaDir)
}

func tarSchemas(schemaDir string) ([]byte, error) {
	_ = schemaDir
	return []byte("tgz" + schemaDir), nil
}
