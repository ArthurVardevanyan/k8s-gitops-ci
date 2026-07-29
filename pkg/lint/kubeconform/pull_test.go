package kubeconform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSchemaDir(t *testing.T) {
	d := t.TempDir()
	for _, name := range SchemaDirs() {
		_ = os.MkdirAll(filepath.Join(d, name), 0o755)
	}
	if err := ValidateSchemaDir(d); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestSchemaVersionsAtDir(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "master-standalone-strict"), 0o755)
	_ = os.MkdirAll(filepath.Join(d, "master-local"), 0o755)
	vs, err := SchemaVersionsAtDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) == 0 {
		t.Errorf("expected at least one version")
	}
}
