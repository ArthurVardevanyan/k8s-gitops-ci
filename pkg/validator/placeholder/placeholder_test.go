package placeholder

import (
	"os"
	"strings"
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

func TestValidateReaderWithOptions_AcceptsPlainIOReader(t *testing.T) {
	// Regression: the signature previously required *os.File specifically,
	// forcing callers with in-memory content to write a temp file just to
	// call this function. A plain strings.Reader must work directly.
	r := strings.NewReader("image: <REGISTRY>/img\n")
	errs := ValidateReaderWithOptions(r, "in-memory.yaml", Options{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error from a plain io.Reader: %v", errs)
	}
}

func TestValidateReaderWithOptions_ContextPopulated(t *testing.T) {
	// Regression: ValidationError.Context was declared but never set.
	r := strings.NewReader("  image: <REGISTRY>/img  \n")
	errs := ValidateReaderWithOptions(r, "x.yaml", Options{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
	if errs[0].Context != "image: <REGISTRY>/img" {
		t.Errorf("Context = %q, want trimmed line content", errs[0].Context)
	}
}

func TestValidateReaderWithOptions_SentinelCaseInsensitive(t *testing.T) {
	// Regression: sentinel matching used to be case-sensitive, so a
	// lowercase or mixed-case sentinel value slipped through un-flagged.
	cases := []string{
		"password: changeme\n",
		"todo: FixMe later\n",
		"value: Placeholder\n",
	}
	for _, line := range cases {
		errs := ValidateReaderWithOptions(strings.NewReader(line), "x.yaml", Options{})
		if len(errs) != 1 {
			t.Errorf("line %q: expected 1 finding for a non-canonical-case sentinel, got %d: %v", line, len(errs), errs)
		}
	}
}

func TestValidateReaderWithOptions_DuplicateTokenOnOneLine_TwoFindings(t *testing.T) {
	// Regression: findPlaceholders used to dedupe matches per line, so a
	// line with the same placeholder token twice only produced one
	// finding. Each occurrence is a separate unresolved placeholder and
	// must be reported separately.
	r := strings.NewReader("path: <REGISTRY>/<REGISTRY>/image\n")
	errs := ValidateReaderWithOptions(r, "x.yaml", Options{})
	if len(errs) != 2 {
		t.Fatalf("expected 2 findings for a duplicated token on one line, got %d: %v", len(errs), errs)
	}
}
