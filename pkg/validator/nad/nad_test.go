package nad

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFiles_ValidNAD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nad.yaml")
	_ = os.WriteFile(path, []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: nad
spec:
  config: '{"cniVersion":"0.3.1"}'
`), 0o644)
	errs := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateFiles_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nad.yaml")
	_ = os.WriteFile(path, []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: nad
spec:
  config:
`), 0o644)
	errs := ValidateFiles([]string{path})
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "spec.config is empty") {
		t.Errorf("expected empty config error: %v", errs)
	}
}

func TestValidateFiles_NonNADFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "svc.yaml")
	_ = os.WriteFile(path, []byte(`kind: Service
`), 0o644)
	errs := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Errorf("expected none for non-NAD: %v", errs)
	}
}

func TestValidateDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "nad.yaml"), []byte(`kind: NetworkAttachmentDefinition
spec:
  config: ''
`), 0o644)
	errs := ValidateDir(dir)
	if len(errs) != 1 {
		t.Errorf("expected one error: %v", errs)
	}
}


