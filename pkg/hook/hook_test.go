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

func TestParseTestScript_PreBuildHookCmd(t *testing.T) {
	cfg := parseString("PRE_BUILD_HOOK=run_my_script")
	if !cfg.HasPreBuild {
		t.Error("expected HasPreBuild=true")
	}
	if cfg.PreBuildCmd != "run_my_script" {
		t.Errorf("PreBuildCmd = %q, want %q", cfg.PreBuildCmd, "run_my_script")
	}
}

func TestParseTestScript_PostBuildHookCmd_ExportPrefixed(t *testing.T) {
	cfg := parseString("export POST_BUILD_HOOK=validate_cluster_version")
	if !cfg.HasPostBuild {
		t.Error("expected HasPostBuild=true")
	}
	if cfg.PostBuildCmd != "validate_cluster_version" {
		t.Errorf("PostBuildCmd = %q, want %q", cfg.PostBuildCmd, "validate_cluster_version")
	}
}

func TestParseTestScript_PostValidateHookCmd_Quoted(t *testing.T) {
	cfg := parseString(`POST_VALIDATE_HOOK="check_duplicates"`)
	if !cfg.HasPostValidate {
		t.Error("expected HasPostValidate=true")
	}
	if cfg.PostValidateCmd != "check_duplicates" {
		t.Errorf("PostValidateCmd = %q, want %q", cfg.PostValidateCmd, "check_duplicates")
	}
}

func TestParseTestScript_NoHooks(t *testing.T) {
	cfg := parseString("SCAFFOLD=true")
	if cfg.HasPreBuild || cfg.HasPostBuild || cfg.HasPostValidate {
		t.Errorf("expected no hooks defined: %+v", cfg)
	}
	if cfg.PreBuildCmd != "" || cfg.PostBuildCmd != "" || cfg.PostValidateCmd != "" {
		t.Errorf("expected empty hook commands: %+v", cfg)
	}
}

func TestParseTestScript_EmptyHookValueIsNotDefined(t *testing.T) {
	// A directive present but assigned an empty value doesn't count as
	// "defined" - there's nothing to invoke.
	cfg := parseString("PRE_BUILD_HOOK=")
	if cfg.HasPreBuild {
		t.Error("expected HasPreBuild=false for an empty value")
	}
}
