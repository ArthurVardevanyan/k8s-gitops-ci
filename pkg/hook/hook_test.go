package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSource_FailsClosed(t *testing.T) {
	if got := ResolveSource("", "", false); got != SourceMain {
		t.Errorf("empty -> main got %q", got)
	}
	if got := ResolveSource("unknown", "", true); got != SourceMain {
		t.Errorf("unknown -> main got %q", got)
	}
	if got := ResolveSource(SourcePR, "/hook-test", true); got != SourcePR {
		t.Errorf("pr + comment -> pr got %q", got)
	}
	if got := ResolveSource(SourcePR, "/hook-test", false); got != SourceMain {
		t.Errorf("pr without prSet -> main got %q", got)
	}
}

func TestParseTestScript_NotExist(t *testing.T) {
	cfg, err := ParseTestScript("/tmp/does-not-exist-test-sh.lkj")
	if err != nil || !cfg.Scaffold {
		t.Fatalf("expected default config: %+v err %v", cfg, err)
	}
}

func TestParseTestScript_Scaffold(t *testing.T) {
	cfg := parseString("SCAFFOLD=false")
	if cfg.Scaffold {
		t.Error("expected scaffold false")
	}
}

func TestParseTestScript_AVPExclude(t *testing.T) {
	cfg := parseString(`AVP_EXCLUDE="cluster1 cluster2"`)
	if len(cfg.AVPExclude) != 2 {
		t.Errorf("expected 2 excludes: %v", cfg.AVPExclude)
	}
}

func TestParseTestScript_Exemptions(t *testing.T) {
	cfg := parseString(`EXEMPTIONS=(check=image-checksum,file=foo.yaml)`)
	if len(cfg.ExemptSelectors) != 1 || cfg.ExemptSelectors[0].Check != "image-checksum" {
		t.Errorf("expected one selector: %+v", cfg.ExemptSelectors)
	}
}

func TestParseTestScript_InvalidExemption(t *testing.T) {
	cfg := parseString(`EXEMPTIONS="badtoken"`)
	if len(cfg.ExemptErrors) == 0 {
		t.Error("expected exempt errors")
	}
}

func TestParseTestScript_MissingCheck(t *testing.T) {
	cfg := parseString(`EXEMPTIONS="file=foo.yaml"`)
	if !strings.Contains(strings.Join(cfg.ExemptErrors, " "), "missing check") {
		t.Errorf("expected missing check error: %v", cfg.ExemptErrors)
	}
}

func parseString(s string) *Config {
	cfg := DefaultConfig()
	cfg.parse(s)
	return cfg
}

func TestHasScaffoldEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")
	_ = os.WriteFile(path, []byte("HOOK=run_scafctl_scaffold\n"), 0o644)
	if !HasScaffoldEnabled(path) {
		t.Error("expected scaffold enabled")
	}
}
