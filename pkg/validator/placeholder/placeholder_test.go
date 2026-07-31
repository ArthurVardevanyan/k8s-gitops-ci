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
	// must be reported separately. This applies to the angle-bracket/AVP
	// patterns only - sentinels intentionally behave differently (see
	// TestValidateReaderWithOptions_SentinelDuplicateOnOneLine_OneFinding
	// below).
	r := strings.NewReader("path: <REGISTRY>/<REGISTRY>/image\n")
	errs := ValidateReaderWithOptions(r, "x.yaml", Options{})
	if len(errs) != 2 {
		t.Fatalf("expected 2 findings for a duplicated token on one line, got %d: %v", len(errs), errs)
	}
}

func TestValidateReaderWithOptions_SentinelDuplicateOnOneLine_OneFinding(t *testing.T) {
	// Behavior change: sentinel matching now reports only the first match
	// per sentinel per line (matching the reference/"downstream"
	// behavior), unlike the angle-bracket/AVP patterns above which report
	// every occurrence.
	r := strings.NewReader("password: CHANGEME CHANGEME\n")
	errs := ValidateReaderWithOptions(r, "x.yaml", Options{})
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding for a duplicated sentinel on one line, got %d: %v", len(errs), errs)
	}
}

func TestValidateReaderWithOptions_SentinelMatchIsUppercased(t *testing.T) {
	// Behavior change: the reported Match is always uppercased, even when
	// the source line uses lowercase or mixed case.
	r := strings.NewReader("password: changeme\n")
	errs := ValidateReaderWithOptions(r, "x.yaml", Options{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(errs), errs)
	}
	if errs[0].Match != "CHANGEME" {
		t.Errorf("expected the reported Match to be uppercased, got: %q", errs[0].Match)
	}
}

func TestKnownNonPlaceholders_Map(t *testing.T) {
	// Structural test: every knownNonPlaceholders key must be well-formed
	// per this repo's actual map shape - bare uppercase identifiers (no
	// angle brackets), since placeholderRe's capture group already strips
	// the brackets before the map lookup.
	for k := range knownNonPlaceholders {
		if k == "" {
			t.Error("knownNonPlaceholders contains an empty key")
		}
		if strings.ContainsAny(k, "<>") {
			t.Errorf("knownNonPlaceholders key %q must not contain angle brackets (the capture group already strips them)", k)
		}
		if strings.ToUpper(k) != k {
			t.Errorf("knownNonPlaceholders key %q must be uppercase (matches placeholderRe's [A-Z][A-Z0-9_-]* pattern)", k)
		}
	}
}

// --- testdata-fixture-driven tests --------------------------------------

func TestValidateFile_Clean(t *testing.T) {
	errs := ValidateFile("testdata/clean.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings, got: %v", errs)
	}
}

func TestValidateFile_HasPlaceholders_Fixture(t *testing.T) {
	errs := ValidateFile("testdata/has-placeholders.yaml")
	if len(errs) != 3 {
		t.Fatalf("expected 3 findings (<NAMESPACE>, CHANGEME, AVP path), got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_CommentOnly(t *testing.T) {
	errs := ValidateFile("testdata/comment-only.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for a whole-line-comment placeholder, got: %v", errs)
	}
}

func TestValidateFile_AVPPlaceholdersFixture(t *testing.T) {
	errs := ValidateFile("testdata/avp-placeholders.yaml")
	if len(errs) != 4 {
		t.Fatalf("expected all 4 AVP prefixes (path/vault/aws/gcp) to be flagged, got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_Sentinels(t *testing.T) {
	// Regression for the sentinel-matching behavior change: each sentinel
	// yields exactly one, uppercased finding per line, including the
	// duplicated-sentinel-on-one-line case (not 2).
	errs := ValidateFile("testdata/sentinels.yaml")
	if len(errs) != 5 {
		t.Fatalf("expected 5 findings (one per sentinel-bearing line, including exactly 1 for the duplicated-on-one-line case), got %d: %v", len(errs), errs)
	}
	for _, e := range errs {
		if e.Match != strings.ToUpper(e.Match) {
			t.Errorf("expected an uppercased Match, got: %q", e.Match)
		}
	}
}

func TestValidateFile_SentinelBoundary(t *testing.T) {
	// Words that contain a sentinel string as a substring must not be
	// flagged (word-boundary guard).
	errs := ValidateFile("testdata/sentinel-boundary.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for substring-only sentinel matches, got: %v", errs)
	}
}

func TestValidateFile_ArgoCDApplicationSet(t *testing.T) {
	errs := ValidateFile("testdata/argocd-applicationset.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for ArgoCD ApplicationSet Go-template syntax, got: %v", errs)
	}
}

func TestValidateFile_EnvVarReferences(t *testing.T) {
	errs := ValidateFile("testdata/env-var-references.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for ${VAR}-style shell env references, got: %v", errs)
	}
}

func TestValidateFile_NFDGoTemplate(t *testing.T) {
	errs := ValidateFile("testdata/nfd-go-template.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for {{ .Name }}-style Go templates, got: %v", errs)
	}
}

func TestValidateFile_MixedRealAndFalse(t *testing.T) {
	// A real angle-bracket placeholder plus Go-template false positives in
	// one document: only the real placeholder is flagged (both the
	// angle-bracket pattern and the CHANGE_ME sentinel independently match
	// the same literal text, so 2 findings on the same line is expected).
	errs := ValidateFile("testdata/mixed-real-and-false.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 findings (angle-bracket + sentinel match on the same real placeholder), got %d: %v", len(errs), errs)
	}
	for _, e := range errs {
		if e.Line != 5 {
			t.Errorf("expected both findings on line 5, got line %d: %v", e.Line, e)
		}
	}
}

func TestValidateFile_InvalidFixturesYieldNoFindings(t *testing.T) {
	// False-positive-guard cases: each of these must yield exactly zero
	// findings even though they superficially resemble a placeholder.
	for _, f := range []string{
		"testdata/invalid/helm-template.yaml",
		"testdata/invalid/known-non-placeholders.yaml",
		"testdata/invalid/block-scalar-shell.yaml",
	} {
		t.Run(f, func(t *testing.T) {
			errs := ValidateFile(f)
			if len(errs) != 0 {
				t.Errorf("expected no findings for %s, got: %v", f, errs)
			}
		})
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	errs := ValidateFile("testdata/does-not-exist.yaml")
	if errs != nil {
		t.Errorf("expected nil for a nonexistent file, got: %v", errs)
	}
}
