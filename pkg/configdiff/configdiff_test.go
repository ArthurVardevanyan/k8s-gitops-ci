package configdiff

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

func TestDetectAffectedApps_NoConfigFiles(t *testing.T) {
	got := DetectAffectedApps([]string{"app/foo.yaml"}, "", "", nil)
	if len(got) != 0 {
		t.Errorf("expected none: %v", got)
	}
}

func TestEnvironmentPrefixes_DefaultsEmpty(t *testing.T) {
	if len(EnvironmentPrefixes) != 0 {
		t.Error("expected empty map default")
	}
}

func TestDetectTemplateChanges(t *testing.T) {
	old := convention.ScaffoldDir
	convention.ScaffoldDir = ".scafctl"
	defer func() { convention.ScaffoldDir = old }()
	got := DetectTemplateChanges([]string{".scafctl/templates/app/base.yaml"})
	if len(got) != 1 || got[0] != "app" {
		t.Errorf("unexpected template apps: %v", got)
	}
}

func TestDedup(t *testing.T) {
	got := dedup([]string{"a", "b", "a", "c"})
	if len(got) != 3 {
		t.Errorf("unexpected dedup: %v", got)
	}
}

// TestParseTopLevel_NestedSequences verifies the scaffold-config layout where
// environments/changeGroups are sequences of items keyed by name/group and
// nested under overlayDefinitions, while overrides remains a key->value mapping.
func TestParseTopLevel_NestedSequences(t *testing.T) {
	data := []byte(`
overlayDefinitions:
  overrides:
    chg_pd01:
      disabled: true
  environments:
    - name: pre-prod
      features:
        cascade_delete:
          - container-security-operator
    - name: prod
      features: {}
  changeGroups:
    - group: 1
    - group: 3
`)
	overrides, envs, groups := parseTopLevel(data)

	if len(overrides) != 1 || overrides["chg_pd01"] == nil {
		t.Fatalf("expected overrides chg_pd01, got %v", overrides)
	}
	if len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d: %v", len(envs), envs)
	}
	if _, ok := envs["pre-prod"]; !ok {
		t.Errorf("missing pre-prod environment: %v", envs)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 change groups, got %d: %v", len(groups), groups)
	}
	if _, ok := groups["1"]; !ok {
		t.Errorf("missing group 1: %v", groups)
	}
}

// TestParseTopLevel_RootMapping verifies the original generic layout — root-level
// mappings for environments/changeGroups — still parses alongside overrides
// nested under overlayDefinitions, so existing consumers do not regress.
func TestParseTopLevel_RootMapping(t *testing.T) {
	data := []byte(`
overlayDefinitions:
  overrides:
    zzz:
      disabled: true
environments:
  prod:
    features: {}
changeGroups:
  enterprise: {}
`)
	overrides, envs, groups := parseTopLevel(data)

	if len(overrides) != 1 || overrides["zzz"] == nil {
		t.Fatalf("expected overrides zzz, got %v", overrides)
	}
	if len(envs) != 1 || envs["prod"] == nil {
		t.Fatalf("expected environments prod, got %v", envs)
	}
	if len(groups) != 1 || groups["enterprise"] == nil {
		t.Fatalf("expected changeGroups enterprise, got %v", groups)
	}
}

// TestParseTopLevel_ChangedEnvironment is a parse-level check that a change to
// one environment item is reflected in a yamlEqual comparison.
func TestParseTopLevel_ChangedEnvironment(t *testing.T) {
	before := []byte(`
overlayDefinitions:
  environments:
    - name: pre-prod
      features: {}
`)
	after := []byte(`
overlayDefinitions:
  environments:
    - name: pre-prod
      features:
        cascade_delete:
          - container-security-operator
`)
	_, envBefore, _ := parseTopLevel(before)
	_, envAfter, _ := parseTopLevel(after)

	if yamlEqual(envBefore["pre-prod"], envAfter["pre-prod"]) {
		t.Error("expected env difference to be detected")
	}
}

// TestParseTopLevel_MalformedNestedFallsBackToRoot verifies a usable
// document-root section is still detected when the nested one exists but is an
// unsupported kind (e.g. a malformed scalar), rather than silently shadowing it.
func TestParseTopLevel_MalformedNestedFallsBackToRoot(t *testing.T) {
	data := []byte(`
overlayDefinitions:
  environments: not-a-map-or-list
environments:
  prod:
    features: {}
`)
	_, envs, _ := parseTopLevel(data)

	if len(envs) != 1 || envs["prod"] == nil {
		t.Fatalf("expected root environments prod after malformed nested section, got %v", envs)
	}
}

// scratchWorkspace sets up a temp workspace representing a gitops repo: a
// scaffold config file for app, overlay directories for the given clusters,
// and fetchMainConfig overridden to return the provided main-branch config.
// The test process is chdir'd into the workspace and restored via t.Cleanup.
func scratchWorkspace(t *testing.T, mainConfig []byte, clusters []string) {
	t.Helper()
	dir := t.TempDir()

	for _, c := range clusters {
		if err := os.MkdirAll(filepath.Join(dir, "app", "overlays", c), 0o755); err != nil {
			t.Fatalf("mkdir overlay %s: %v", c, err)
		}
	}

	scaffoldDir := filepath.Join(dir, ".scafctl")
	if err := os.MkdirAll(filepath.Join(scaffoldDir, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir scaffold configs: %v", err)
	}

	oldDir := convention.ScaffoldDir
	convention.ScaffoldDir = ".scafctl"
	oldFetch := fetchMainConfig
	fetchMainConfig = func(string) []byte { return mainConfig }
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}

	t.Cleanup(func() {
		convention.ScaffoldDir = oldDir
		fetchMainConfig = oldFetch
		if err := os.Chdir(oldCwd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

// writeCurrentConfig writes the current (working-tree) config for app.
func writeCurrentConfig(t *testing.T, data []byte) {
	t.Helper()
	path := filepath.Join(convention.ScaffoldDir, "configs", "app.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// setPrefixes sets EnvironmentPrefixes for the test and restores it on cleanup.
func setPrefixes(t *testing.T, prefixes map[string][]string) {
	t.Helper()
	old := EnvironmentPrefixes
	EnvironmentPrefixes = prefixes
	t.Cleanup(func() { EnvironmentPrefixes = old })
}

func TestDetectAffectedApps_EnvironmentChange(t *testing.T) {
	setPrefixes(t, map[string][]string{"pre-prod": {"pp", "pd"}})

	mainConfig := []byte(`
overlayDefinitions:
  environments:
    - name: pre-prod
      features: {}
`)
	scratchWorkspace(t, mainConfig, []string{"pp1010", "pp1011", "pd1660", "unrelated"})

	// current config differs only in the pre-prod environment's features
	writeCurrentConfig(t, []byte(`
overlayDefinitions:
  environments:
    - name: pre-prod
      features:
        cascade_delete:
          - container-security-operator
`))

	got := DetectAffectedApps([]string{".scafctl/configs/app.yaml"}, "", "", nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 affected app, got %d: %v", len(got), got)
	}
	aff := got[0]
	if aff.Trigger != "config-environment" {
		t.Errorf("expected trigger config-environment, got %q", aff.Trigger)
	}
	if aff.FullTest {
		t.Error("expected FullTest false for an environment change")
	}

	want := []string{"pp1010", "pp1011", "pd1660"}
	sort.Strings(want)
	sort.Strings(aff.Clusters)
	if len(aff.Clusters) != len(want) {
		t.Fatalf("expected clusters %v, got %v", want, aff.Clusters)
	}
	for i, c := range want {
		if aff.Clusters[i] != c {
			t.Errorf("cluster[%d] = %q, want %q (all: %v)", i, aff.Clusters[i], c, aff.Clusters)
		}
	}
	for _, c := range aff.Clusters {
		if c == "unrelated" {
			t.Errorf("expected unrelated cluster to be excluded, got %v", aff.Clusters)
		}
	}
}

func TestDetectAffectedApps_ChangeGroupChange(t *testing.T) {
	setPrefixes(t, nil)

	mainConfig := []byte(`
overlayDefinitions:
  changeGroups:
    - group: 1
    - group: 3
`)
	scratchWorkspace(t, mainConfig, []string{"pp1010", "pp1011"})

	// current config adds a change group
	writeCurrentConfig(t, []byte(`
overlayDefinitions:
  changeGroups:
    - group: 1
    - group: 3
    - group: 6
`))

	// live per-cluster change-group map comes from the cluster-metadata provider
	changeGroups := map[string]int{
		"pp1010": 1,
		"pp1011": 3,
	}
	got := DetectAffectedApps([]string{".scafctl/configs/app.yaml"}, "", "", changeGroups)
	if len(got) != 1 {
		t.Fatalf("expected 1 affected app, got %d: %v", len(got), got)
	}
	aff := got[0]
	if aff.Trigger != "config-changeGroup" {
		t.Errorf("expected trigger config-changeGroup, got %q", aff.Trigger)
	}
	if !aff.FullTest {
		t.Error("expected FullTest true for a change-group change")
	}
	want := []string{"pp1010", "pp1011"}
	sort.Strings(want)
	sort.Strings(aff.Clusters)
	if len(aff.Clusters) != len(want) {
		t.Fatalf("expected clusters %v, got %v", want, aff.Clusters)
	}
	for i, c := range want {
		if aff.Clusters[i] != c {
			t.Errorf("cluster[%d] = %q, want %q", i, aff.Clusters[i], c)
		}
	}
}

func TestDetectAffectedApps_OverrideChange(t *testing.T) {
	mainConfig := []byte(`
overlayDefinitions:
  overrides:
    pp1010:
      disabled: true
`)
	scratchWorkspace(t, mainConfig, []string{"pp1010", "pp1011"})

	// current config overrides a different cluster
	writeCurrentConfig(t, []byte(`
overlayDefinitions:
  overrides:
    pp1011:
      disabled: true
`))

	got := DetectAffectedApps([]string{".scafctl/configs/app.yaml"}, "", "", nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 affected app, got %d: %v", len(got), got)
	}
	aff := got[0]
	if aff.Trigger != "config-override" {
		t.Errorf("expected trigger config-override, got %q", aff.Trigger)
	}
	if aff.FullTest {
		t.Error("expected FullTest false for an override change")
	}
	if len(aff.Clusters) != 1 || aff.Clusters[0] != "pp1011" {
		t.Errorf("expected override cluster pp1011, got %v", aff.Clusters)
	}
}

func TestDetectAffectedApps_NoChange(t *testing.T) {
	setPrefixes(t, nil)
	cfg := []byte(`
overlayDefinitions:
  environments:
    - name: pre-prod
      features: {}
  changeGroups:
    - group: 1
`)
	scratchWorkspace(t, cfg, []string{"pp1010"})
	writeCurrentConfig(t, cfg)

	changeGroups := map[string]int{"pp1010": 1}
	got := DetectAffectedApps([]string{".scafctl/configs/app.yaml"}, "", "", changeGroups)
	if len(got) != 0 {
		t.Fatalf("expected no affected app when config unchanged, got %d: %v", len(got), got)
	}
}
