package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
)

// TestRunAll_ScaffoldSkippedWithoutConfig guards the fix for a real
// over-eager-validation bug found while wiring this up: an app with
// overlay changes but no .scafctl config at all (the common case for a
// generic-core consumer that doesn't use scafctl-based scaffolding) must
// never be treated as a scaffold execution failure just because
// HasScaffoldEnabled defaults to true with no test.sh.
func TestRunAll_ScaffoldSkippedWithoutConfig(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	// kustomize-fix is unrelated to what this test exercises, and this
	// minimal fixture isn't in kustomize's real canonical form, which
	// would otherwise always flag a spurious Kustomize Fix finding and
	// fail the HasFailures() assertion below. shellcheck is disabled for
	// the same reason: it now hard-fails when the CLI isn't installed,
	// and this dev/CI image doesn't have it.
	res, err := RunAll(Options{Dirs: []string{d}, DisabledChecks: []string{"kustomize-fix", "shellcheck"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected no failures for an app with no scaffold config at all")
	}
	var scaffoldSection ReportSection
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" {
			scaffoldSection = s
		}
	}
	if scaffoldSection.Status == StatusError {
		t.Errorf("expected the Scaffold Validation section to be clean, got:\n%s", scaffoldSection.Body)
	}
}

// TestRunAll_ScaffoldExecutionFailureBlocks exercises the real (not mocked)
// scafctl binary: it's installed in this dev/CI image but its "scaffold"
// subcommand doesn't exist here, so an app that DOES opt in (has a
// .scafctl config) with a changed overlay must surface that execution
// failure as a blocking error, distinct from the "no config, skip
// entirely" case above. App identity for scaffold config lookups is
// repo-root-relative (matching pkg/configdiff's own convention), so this
// chdirs into the temp repo root rather than using an absolute app path
// like the other end-to-end tests in this package.
func TestRunAll_ScaffoldExecutionFailureBlocks(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{"myapp"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a scaffold execution failure to be surfaced as a logger failure")
	}
	var scaffoldSection ReportSection
	for _, s := range res.Sections {
		if s.Name == "Scaffold Validation" {
			scaffoldSection = s
		}
	}
	if scaffoldSection.Status != StatusError {
		t.Errorf("expected the Scaffold Validation section to report the failure, got:\n%s", scaffoldSection.Body)
	}
	if !strings.Contains(scaffoldSection.Body, "Scaffold Exec") {
		t.Errorf("expected a Scaffold Exec entry, got:\n%s", scaffoldSection.Body)
	}
}

// TestRunAll_ScaffoldReadmeCheckDisabledByDefault guards the
// "scaffold-readme" step's default-off gating: a README scaffold-status
// table with a stale/missing row must NOT be reported unless the step is
// explicitly enabled - see docs/CI.md#scaffold-validation for why.
func TestRunAll_ScaffoldReadmeCheckDisabledByDefault(t *testing.T) {
	chdirTemp(t)
	// A stale row ("removed" has no on-disk overlay) would fail
	// CheckReadmeStatus if it ran.
	table := scaffold.GenerateScaffoldTable([]scaffold.StatusRow{{App: "myapp", Overlay: "removed", Status: "✅ ok"}})
	mustWrite(t, "README.md", "# Readme\n\n"+table)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	// kustomize-fix is unrelated to what this test exercises, and this
	// minimal fixture isn't in kustomize's real canonical form, which
	// would otherwise always flag a spurious Kustomize Fix finding and
	// fail the HasFailures() assertion below. shellcheck is disabled for
	// the same reason: it now hard-fails when the CLI isn't installed,
	// and this dev/CI image doesn't have it.
	res, err := RunAll(Options{Dirs: []string{"myapp"}, DisabledChecks: []string{"kustomize-fix", "shellcheck"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger != nil && res.Logger.HasFailures() {
		t.Error("expected the scaffold-readme check to be skipped by default")
	}
	for _, s := range res.Sections {
		if s.Name == "Static Checks" && s.Status == StatusError {
			t.Errorf("expected a clean Static Checks section by default, got:\n%s", s.Body)
		}
	}
}

// TestRunAll_ScaffoldReadmeCheckEnabledViaEnabledChecks is the positive
// counterpart: once explicitly enabled, the same stale row must surface.
func TestRunAll_ScaffoldReadmeCheckEnabledViaEnabledChecks(t *testing.T) {
	chdirTemp(t)
	table := scaffold.GenerateScaffoldTable([]scaffold.StatusRow{{App: "myapp", Overlay: "removed", Status: "✅ ok"}})
	mustWrite(t, "README.md", "# Readme\n\n"+table)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{"myapp"}, EnabledChecks: []string{"scaffold-readme"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected the scaffold-readme check to report the stale row once enabled")
	}
	found := false
	for _, s := range res.Sections {
		if s.Name == "Static Checks" && s.Status == StatusError && strings.Contains(s.Body, "Scaffold Table") {
			found = true
		}
	}
	if !found {
		t.Error("expected the Static Checks section to report a Scaffold Table error once enabled")
	}
}

// chdirTemp chdirs into a fresh temp dir, restoring the original working
// directory on cleanup - findUnprotectedApps (like scaffold.
// HasScaffoldEnabled/HasScaffoldConfig) resolves app/.scafctl paths
// relative to the process CWD, matching a real pipeline run's repo-root
// CWD.

// TestFindUnprotectedApps_NoTemplateIsNeverUnprotected guards that an app
// with overlay changes but no scaffold template at all (scaffold-drift
// detection isn't even available for it) is never flagged - there being
// nothing to protect against isn't the same as protection being disabled.
func TestFindUnprotectedApps_NoTemplateIsNeverUnprotected(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "test.sh"), "SCAFFOLD=false\n")

	got := findUnprotectedApps([]string{filepath.Join("myapp", "overlays", "prod", "kustomization.yaml")})
	if len(got) != 0 {
		t.Errorf("expected no unprotected apps without a scaffold template, got %v", got)
	}
}

// TestFindUnprotectedApps_DisabledWithTemplate is the positive case: an app
// with a scaffold template AND overlay changes AND SCAFFOLD=false is
// flagged as unprotected.
func TestFindUnprotectedApps_DisabledWithTemplate(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "test.sh"), "SCAFFOLD=false\n")
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "template.yaml"), "kind: Deployment\n")

	got := findUnprotectedApps([]string{filepath.Join("myapp", "overlays", "prod", "kustomization.yaml")})
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("expected [myapp] to be unprotected, got %v", got)
	}
}

// TestFindUnprotectedApps_EnabledIsNeverFlagged guards the negative case:
// an app with a template but scaffold drift protection actually enabled
// (the default - no SCAFFOLD=false) is never flagged.
func TestFindUnprotectedApps_EnabledIsNeverFlagged(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "test.sh"), "echo hi\n")
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "template.yaml"), "kind: Deployment\n")

	got := findUnprotectedApps([]string{filepath.Join("myapp", "overlays", "prod", "kustomization.yaml")})
	if len(got) != 0 {
		t.Errorf("expected no unprotected apps when scaffold protection is enabled, got %v", got)
	}
}

// TestFindUnprotectedApps_TemplateOrConfigChangeAlsoAttributes guards that
// a change under .scafctl/templates/<app>/ or .scafctl/configs/<app>.yaml
// (not just <app>/overlays/...) also attributes to that app, matching
// runScaffoldValidation's own template/config change triggers.
func TestFindUnprotectedApps_TemplateOrConfigChangeAlsoAttributes(t *testing.T) {
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "test.sh"), "SCAFFOLD=false\n")
	mustWrite(t, filepath.Join(".scafctl", "templates", "myapp", "template.yaml"), "kind: Deployment\n")

	got := findUnprotectedApps([]string{filepath.Join(".scafctl", "templates", "myapp", "template.yaml")})
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("expected a template-only change to still attribute to myapp, got %v", got)
	}

	got = findUnprotectedApps([]string{filepath.Join(".scafctl", "configs", "myapp.yaml")})
	if len(got) != 1 || got[0] != "myapp" {
		t.Errorf("expected a config-only change to still attribute to myapp, got %v", got)
	}
}

func TestFlattenSkippedClusters_Empty(t *testing.T) {
	if got := flattenSkippedClusters(nil); got != nil {
		t.Errorf("expected nil for an empty/nil map, got %v", got)
	}
}

func TestFlattenSkippedClusters_SortsAppsAndClusters(t *testing.T) {
	got := flattenSkippedClusters(map[string][]string{
		"zapp": {"staging", "dev"},
		"aapp": {"prod"},
	})
	want := []string{"aapp/prod", "zapp/dev", "zapp/staging"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestFlattenDisabledClusters_Empty(t *testing.T) {
	if got := flattenDisabledClusters(nil); got != nil {
		t.Errorf("expected nil for an empty/nil map, got %v", got)
	}
}

func TestFlattenDisabledClusters_SortsAppsAndClusters(t *testing.T) {
	got := flattenDisabledClusters(map[string][]string{
		"zapp": {"staging", "dev"},
		"aapp": {"prod"},
	})
	want := []string{"aapp/prod", "zapp/dev", "zapp/staging"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestIsOverlayRelatedToChangedFiles(t *testing.T) {
	// A component change only relates to an overlay whose kustomization
	// actually references that (version-partitioned) component directory,
	// so this test needs real on-disk overlays to resolve refs against.
	chdirTemp(t)
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"),
		"resources:\n  - ../../base\n  - ../../components/foo/v0.21.0\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"),
		"resources:\n  - ../../base\n  - ../../components/foo/v0.19.1\n")

	cases := []struct {
		name    string
		cluster string
		changed []string
		want    bool
	}{
		{"overlay itself changed", "prod", []string{"myapp/overlays/prod/kustomization.yaml"}, true},
		{"base changed (flows into every overlay)", "prod", []string{"myapp/base/deployment.yaml"}, true},
		{"referenced component version changed", "prod", []string{"myapp/components/foo/v0.21.0/patch.yaml"}, true},
		{"different component version changed (not referenced)", "prod", []string{"myapp/components/foo/v0.19.1/patch.yaml"}, false},
		{"referenced component version changed for dev overlay", "dev", []string{"myapp/components/foo/v0.19.1/patch.yaml"}, true},
		{"a different overlay changed", "prod", []string{"myapp/overlays/dev/kustomization.yaml"}, false},
		{"an unrelated app changed", "prod", []string{"otherapp/overlays/prod/kustomization.yaml"}, false},
		{"nothing changed", "prod", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isOverlayRelatedToChangedFiles("myapp", c.cluster, c.changed); got != c.want {
				t.Errorf("isOverlayRelatedToChangedFiles(%q, %v) = %v, want %v", c.cluster, c.changed, got, c.want)
			}
		})
	}
}

func TestIsOverlayScaffoldRelated(t *testing.T) {
	// Unlike isOverlayRelatedToChangedFiles, scaffold relatedness is a
	// pure path-prefix check over the files that unambiguously change an
	// overlay's generated output - it needs no on-disk kustomization refs.
	// Only the overlay's own files and the app's scaffold template count. A
	// base/ or components/ edit is never scaffold-related (it isn't a
	// scaffold input at all), and an app scaffold-config edit is also not
	// directly related - it is app-wide and ambiguous, settled precisely by
	// generated-content comparison (computeBaselineDrift), not the path.
	chdirTemp(t)

	cases := []struct {
		name    string
		cluster string
		changed []string
		want    bool
	}{
		{"overlay itself changed", "c1", []string{"myapp/overlays/c1/kustomization.yaml"}, true},
		{"overlay own patch changed", "c1", []string{"myapp/overlays/c1/patch.yaml"}, true},
		{"template changed", "c1", []string{".scafctl/templates/myapp/overlays/kustomization.yaml"}, true},
		{"config yaml changed is NOT directly related", "c1", []string{".scafctl/configs/myapp.yaml"}, false},
		{"config yml changed is NOT directly related", "c1", []string{".scafctl/configs/myapp.yml"}, false},
		{"base changed is NOT scaffold-related", "c1", []string{"myapp/base/deployment.yaml"}, false},
		{"directly referenced component changed is NOT scaffold-related", "c1", []string{"myapp/components/foo/v1/x.yaml"}, false},
		{"transitively referenced version-variant component changed is NOT scaffold-related", "c1", []string{"myapp/components/foo/v1-variant/x.yaml"}, false},
		{"different app template does not relate", "c1", []string{".scafctl/templates/otherapp/overlays/kustomization.yaml"}, false},
		{"a different overlay changed", "c1", []string{"myapp/overlays/c2/kustomization.yaml"}, false},
		{"an unrelated app changed", "c1", []string{"otherapp/overlays/c1/kustomization.yaml"}, false},
		{"nothing changed", "c1", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isOverlayScaffoldRelated("myapp", c.cluster, c.changed); got != c.want {
				t.Errorf("isOverlayScaffoldRelated(%q, %v) = %v, want %v", c.cluster, c.changed, got, c.want)
			}
		})
	}
}

func TestIsAppScaffoldConfigChanged(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
		want    bool
	}{
		{"config yaml changed", []string{".scafctl/configs/myapp.yaml"}, true},
		{"config yml changed", []string{".scafctl/configs/myapp.yml"}, true},
		{"other app config", []string{".scafctl/configs/otherapp.yaml"}, false},
		{"overlay changed", []string{"myapp/overlays/c1/kustomization.yaml"}, false},
		{"nothing changed", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isAppScaffoldConfigChanged("myapp", c.changed); got != c.want {
				t.Errorf("isAppScaffoldConfigChanged(%v) = %v, want %v", c.changed, got, c.want)
			}
		})
	}
}

func TestComputeBaselineDrift_EmptyBaseRefSkipsEntirely(t *testing.T) {
	// A local test run against a live working tree always has an empty
	// BaseRef (see gitDiff's own doc comment) - this must be an instant
	// no-op, never attempting a git call or generating anything.
	log := logger.NewLogger(false, "")
	res := computeBaselineDrift(Options{}, "myapp", []string{"c1"}, log)
	if len(res.PreExisting) != 0 || len(res.Diffs) != 0 {
		t.Errorf("expected an empty result, got %+v", res)
	}
}

// runGitForTest runs a git command in the current directory, failing the test
// on error - used to build small real repos so computeBaselineDrift's
// merge-base + worktree machinery can be exercised end to end.
func runGitForTest(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// withFakeGenerate substitutes generateScaffold for the duration of a test.
func withFakeGenerate(t *testing.T, fn func(scaffold.GenerateOptions) (*scaffold.GenerateResult, error)) {
	t.Helper()
	orig := generateScaffold
	generateScaffold = fn
	t.Cleanup(func() { generateScaffold = orig })
}

// gitRepoWithConfigRevision builds a real repo whose merge-base ("old-main")
// has scaffold config v1 and whose HEAD has v2, plus a committed overlay.
func gitRepoWithConfigRevision(t *testing.T) {
	t.Helper()
	chdirTemp(t)
	runGitForTest(t, "init", "-q")
	runGitForTest(t, "config", "user.email", "test@example.com")
	runGitForTest(t, "config", "user.name", "Test")

	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "v1\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "c1", "kustomization.yaml"), "committed\n")
	runGitForTest(t, "add", "-A")
	runGitForTest(t, "commit", "-q", "-m", "base")
	runGitForTest(t, "branch", "old-main")

	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "v2\n")
	runGitForTest(t, "add", "-A")
	runGitForTest(t, "commit", "-q", "-m", "pr change")
}

// TestComputeBaselineDrift_EqualGeneratedContentIsPreExisting is the core
// regression guard: when the PR's config edit does not change an overlay's
// generated output, the drift there is external and must be downgraded rather
// than blocking.
func TestComputeBaselineDrift_EqualGeneratedContentIsPreExisting(t *testing.T) {
	gitRepoWithConfigRevision(t)
	withFakeGenerate(t, func(scaffold.GenerateOptions) (*scaffold.GenerateResult, error) {
		return &scaffold.GenerateResult{Overlays: map[string]map[string]string{
			"c1": {"kustomization.yaml": "identical-hash"},
		}}, nil
	})

	log := logger.NewLogger(false, "")
	res := computeBaselineDrift(Options{BaseRef: "old-main"}, "myapp", []string{"c1"}, log)
	if !res.PreExisting["c1"] {
		t.Errorf("expected c1 to be classified external, got %+v", res)
	}
	if len(res.Diffs) != 0 {
		t.Errorf("expected no diffs, got %+v", res.Diffs)
	}
}

// TestComputeBaselineDrift_DifferingContentBlocks is the other half: when the
// PR's config edit does change an overlay's generated output, the drift is the
// PR's responsibility and must stay blocking, with the differing files
// reported.
func TestComputeBaselineDrift_DifferingContentBlocks(t *testing.T) {
	gitRepoWithConfigRevision(t)
	withFakeGenerate(t, func(opts scaffold.GenerateOptions) (*scaffold.GenerateResult, error) {
		cfg, _ := os.ReadFile(filepath.Join(opts.WorkDir, ".scafctl", "configs", "myapp.yaml"))
		val := "v1"
		if strings.Contains(string(cfg), "v2") {
			val = "v2"
		}
		return &scaffold.GenerateResult{Overlays: map[string]map[string]string{
			"c1": {"kustomization.yaml": val},
		}}, nil
	})

	log := logger.NewLogger(false, "")
	res := computeBaselineDrift(Options{BaseRef: "old-main"}, "myapp", []string{"c1"}, log)
	if res.PreExisting["c1"] {
		t.Errorf("expected c1 NOT to be classified external, got %+v", res)
	}
	if got := res.Diffs["c1"]; len(got) != 1 || got[0] != "kustomization.yaml" {
		t.Errorf("expected the differing file to be reported, got %v", got)
	}
}

// TestComputeBaselineDrift_NeitherSideGeneratedIsInconclusive guards the
// conservative edge: equal content is only meaningful when at least one side
// actually generated the overlay.
func TestComputeBaselineDrift_NeitherSideGeneratedIsInconclusive(t *testing.T) {
	gitRepoWithConfigRevision(t)
	withFakeGenerate(t, func(scaffold.GenerateOptions) (*scaffold.GenerateResult, error) {
		return &scaffold.GenerateResult{Overlays: map[string]map[string]string{}}, nil
	})

	log := logger.NewLogger(false, "")
	res := computeBaselineDrift(Options{BaseRef: "old-main"}, "myapp", []string{"c1"}, log)
	if res.PreExisting["c1"] {
		t.Errorf("expected an inconclusive overlay to stay blocking, got %+v", res)
	}
}

// TestComputeBaselineDrift_DoesNotMutateWorkingTree guards the safety property
// that replaced the old in-place baseline swap: the comparison must never read
// or write the caller's own working tree, only throwaway worktrees.
func TestComputeBaselineDrift_DoesNotMutateWorkingTree(t *testing.T) {
	gitRepoWithConfigRevision(t)
	withFakeGenerate(t, func(scaffold.GenerateOptions) (*scaffold.GenerateResult, error) {
		return &scaffold.GenerateResult{Overlays: map[string]map[string]string{}}, nil
	})

	configPath := filepath.Join(".scafctl", "configs", "myapp.yaml")
	overlayPath := filepath.Join("myapp", "overlays", "c1", "kustomization.yaml")
	log := logger.NewLogger(false, "")
	_ = computeBaselineDrift(Options{BaseRef: "old-main"}, "myapp", []string{"c1"}, log)

	if got, _ := os.ReadFile(configPath); string(got) != "v2\n" {
		t.Errorf("config = %q, want unchanged %q", got, "v2\n")
	}
	if got, _ := os.ReadFile(overlayPath); string(got) != "committed\n" {
		t.Errorf("overlay = %q, want unchanged %q", got, "committed\n")
	}
}

// TestComputeBaselineDrift_OutsideGitRepoDegrades ensures a non-git working
// tree (or any worktree failure) degrades to "nothing external", never panics
// or fails the run.
func TestComputeBaselineDrift_OutsideGitRepoDegrades(t *testing.T) {
	chdirTemp(t)
	log := logger.NewLogger(false, "")
	res := computeBaselineDrift(Options{BaseRef: "old-main"}, "myapp", []string{"c1"}, log)
	if len(res.PreExisting) != 0 {
		t.Errorf("expected no external overlays outside a git repo, got %+v", res)
	}
}

func TestRunScaffoldApps_EmptyJobsIsNoOp(t *testing.T) {
	called := false
	runScaffoldApps(nil, nil, nil, 4, func(string, *scaffold.Summary) { called = true })
	if called {
		t.Error("expected record to never be called with no jobs")
	}
}
