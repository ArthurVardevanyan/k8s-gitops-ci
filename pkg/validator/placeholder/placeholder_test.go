package placeholder

import (
	"os"
	"testing"
)

func TestValidateFile_HasPlaceholders(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/x.yaml"
	_ = os.WriteFile(path, []byte("image: <REGISTRY>/img\n"), 0o644)
	errs := ValidateFile(path)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

func TestValidateFile_KnownNonPlaceholders(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/x.yaml"
	_ = os.WriteFile(path, []byte("content: <HTML>\n"), 0o644)
	errs := ValidateFile(path)
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateFileWithOptions_SkipAVP(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/x.yaml"
	_ = os.WriteFile(path, []byte("secret: <path:secret/data#value>\n"), 0o644)
	errs := ValidateFileWithOptions(path, Options{CheckAVP: false})
	if len(errs) != 0 {
		t.Errorf("expected no errors when AVP skipped: %v", errs)
	}
}

func TestValidateFileWithOptions_AVPPatterns(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/x.yaml"
	_ = os.WriteFile(path, []byte("secret: <path:secret/data#value>\n"), 0o644)
	errs := ValidateFileWithOptions(path, Options{CheckAVP: true})
	if len(errs) != 1 {
		t.Errorf("expected AVP placeholder: %v", errs)
	}
}
