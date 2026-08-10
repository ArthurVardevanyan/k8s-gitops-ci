package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSource_ExplicitOverride(t *testing.T) {
	cases := []struct {
		name   string
		signal Source
		want   Source
	}{
		{"main override", SourceMain, SourceMain},
		{"pr override", SourcePR, SourcePR},
		{"local override", SourceLocal, SourceLocal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveSource(tc.signal, "", true); got != tc.want {
				t.Errorf("ResolveSource(%q) = %q, want %q", tc.signal, got, tc.want)
			}
		})
	}
}

func TestResolveSource_EventTypePullRequest(t *testing.T) {
	if got := ResolveSource(eventTypePullRequest, "", true); got != SourceMain {
		t.Errorf("pull_request with PR -> main got %q", got)
	}
	if got := ResolveSource(eventTypePullRequest, "", false); got != SourceMain {
		t.Errorf("pull_request without PR -> main got %q", got)
	}
}

func TestResolveSource_EventTypePush(t *testing.T) {
	if got := ResolveSource(eventTypePush, "", true); got != SourceLocal {
		t.Errorf("push (merge queue) -> local got %q", got)
	}
}

func TestResolveSource_EventTypeOnComment(t *testing.T) {
	cases := []struct {
		name    string
		comment string
		prSet   bool
		want    Source
	}{
		{"hook-test on PR", "/hook-test", true, SourcePR},
		{"hook-test with args on PR", "/hook-test pp2000", true, SourcePR},
		{"hook-test without PR", "/hook-test", false, SourcePR},
		{"other comment on PR", "/retest", true, SourceMain},
		{"other comment without PR", "/retest", false, SourceLocal},
		{"empty comment on PR", "", true, SourceMain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveSource(eventTypeOnComment, tc.comment, tc.prSet); got != tc.want {
				t.Errorf("ResolveSource(on-comment, %q, %v) = %q, want %q",
					tc.comment, tc.prSet, got, tc.want)
			}
		})
	}
}

func TestResolveSource_FailsClosed(t *testing.T) {
	if got := ResolveSource("", "", true); got != SourceMain {
		t.Errorf("empty with PR -> main got %q", got)
	}
	if got := ResolveSource("", "", false); got != SourceLocal {
		t.Errorf("empty without PR -> local got %q", got)
	}
	if got := ResolveSource("unknown", "", true); got != SourceMain {
		t.Errorf("unknown with PR -> main got %q", got)
	}
	if got := ResolveSource("unknown", "", false); got != SourceLocal {
		t.Errorf("unknown without PR -> local got %q", got)
	}
}

func TestIsHookTestComment(t *testing.T) {
	cases := []struct {
		name    string
		comment string
		want    bool
	}{
		{"bare", "/hook-test", true},
		{"with args", "/hook-test pp2000", true},
		{"leading spaces", "  /hook-test", true},
		{"not hook-test", "/retest", false},
		{"prefix of other cmd", "/hook-testing", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isHookTestComment(tc.comment); got != tc.want {
				t.Errorf("isHookTestComment(%q) = %v, want %v", tc.comment, got, tc.want)
			}
		})
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
