package kubeconform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSchemaDir(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "1.29.0-standalone-strict"), 0o755)
	if err := ValidateSchemaDir(d, []string{"1.29.0"}); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestSchemaVersionsAtDir(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "1.29.0-standalone-strict"), 0o755)
	_ = os.MkdirAll(filepath.Join(d, "1.30.0"), 0o755)
	vs, err := SchemaVersionsAtDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Errorf("expected 2 versions, got %v", vs)
	}
}
