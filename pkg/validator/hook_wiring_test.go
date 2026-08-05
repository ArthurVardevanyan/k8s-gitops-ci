package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

// kustomizeStrategy is the appBuildStrategy value most buildOverlayWithHooks
// tests want - plain kustomize, no AVP exclusions - since strategy
// detection itself is covered by pkg/overlay's own tests.
var kustomizeStrategy = appBuildStrategy{Strategy: overlay.StrategyKustomize}

func TestResolveHookSource_LocalRunDefaultsToLocal(t *testing.T) {
	t.Parallel()
	// No explicit --hook-source and no PR → local run; should read working
	// tree test.sh without requiring --hook-source local.
	if got := resolveHookSource(Options{}); got != hook.SourceLocal {
		t.Errorf("expected local run (no signal, no PR) to default to SourceLocal, got %q", got)
	}
}

func TestResolveHookSource_PRRunDefaultsToMain(t *testing.T) {
	t.Parallel()
	// PR set with no explicit --hook-source → pipeline run; must still
	// fail-closed to SourceMain so the PR's own test.sh is never trusted.
	if got := resolveHookSource(Options{PR: "42"}); got != hook.SourceMain {
		t.Errorf("expected PR run (no signal, PR set) to fail closed to SourceMain, got %q", got)
	}
}

func TestResolveHookSource_ExplicitLocalHonored(t *testing.T) {
	t.Parallel()
	if got := resolveHookSource(Options{HookSource: "local"}); got != hook.SourceLocal {
		t.Errorf("expected explicit local override honored, got %q", got)
	}
}

func TestResolveHookSource_PRWithoutHookTestCommentFallsBackToMain(t *testing.T) {
	t.Parallel()
	opts := Options{HookSource: hook.SourcePR, PR: "42", TriggerComment: "/deploy"}
	if got := resolveHookSource(opts); got != hook.SourceMain {
		t.Errorf("expected a PR signal without the exact /hook-test comment to fail closed to main, got %q", got)
	}
}

func TestResolveAppHookConfigs(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "test.sh"), "SCAFFOLD=false\n")

	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	cfg, ok := cfgs[app]
	if !ok || cfg == nil {
		t.Fatalf("expected a resolved config for %q, got %v", app, cfgs)
	}
	if cfg.Scaffold {
		t.Error("expected Scaffold=false from the app's test.sh")
	}
}

func TestHookExemptSelectorsAndErrors_MergesAcrossApps(t *testing.T) {
	t.Parallel()
	cfgs := map[string]*hook.Config{
		"appA": {ExemptSelectors: []hook.ExemptSelector{{Check: "image-checksum", Value: "registry.example.com/app:latest"}}},
		"appB": {ExemptSelectors: []hook.ExemptSelector{{Check: "cluster-name", Match: "dev"}}},
	}
	selectors, errs := hookExemptSelectorsAndErrors(cfgs)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(selectors) != 2 {
		t.Fatalf("expected 2 merged selectors, got %d: %+v", len(selectors), selectors)
	}
	want := map[string]bool{"image-checksum": false, "cluster-name": false}
	for _, s := range selectors {
		want[s.Check] = true
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("expected a merged selector for check %q", id)
		}
	}
}

func TestHookExemptSelectorsAndErrors_CollectsAndPrefixesErrors(t *testing.T) {
	t.Parallel()
	cfgs := map[string]*hook.Config{
		"apps/myapp": {ExemptErrors: []string{"missing check= in exemption \"foo\""}},
	}
	_, errs := hookExemptSelectorsAndErrors(cfgs)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if !strings.HasPrefix(errs[0], "apps/myapp: test.sh EXEMPTIONS: ") {
		t.Errorf("expected the error to be prefixed with the app, got %q", errs[0])
	}
}

func TestMergeHookOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		current, next, want hookOutcome
	}{
		{hookNotDefined, hookNotDefined, hookNotDefined},
		{hookNotDefined, hookRan, hookRan},
		{hookRan, hookNotDefined, hookRan},
		{hookRan, hookFailed, hookFailed},
		{hookFailed, hookRan, hookFailed}, // a later success never un-fails an app
		{hookFailed, hookNotDefined, hookFailed},
	}
	for _, c := range cases {
		if got := mergeHookOutcome(c.current, c.next); got != c.want {
			t.Errorf("mergeHookOutcome(%v, %v) = %v, want %v", c.current, c.next, got, c.want)
		}
	}
}

func TestAnyHookFailed(t *testing.T) {
	t.Parallel()
	if anyHookFailed(map[string]*appHookResult{}) {
		t.Error("expected no failure for an empty result set")
	}
	if anyHookFailed(map[string]*appHookResult{"app": nil}) {
		t.Error("expected a nil entry to be skipped, not treated as a failure")
	}
	allPassed := map[string]*appHookResult{
		"app": {PreBuild: hookRan, PostBuild: hookRan, PostValidate: hookNotDefined},
	}
	if anyHookFailed(allPassed) {
		t.Error("expected no failure when every hook ran or wasn't defined")
	}
	oneFailed := map[string]*appHookResult{
		"a": {PreBuild: hookRan},
		"b": {PostValidate: hookFailed},
	}
	if !anyHookFailed(oneFailed) {
		t.Error("expected a failure when any app's hook failed")
	}
}

func TestBuildOverlayWithHooks_NoHooksDefined(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "deployment.yaml"), "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	mustWrite(t, filepath.Join(d, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")

	buildErr, pre, post, _ := buildOverlayWithHooks(overlayRef{path: d, cluster: "foo"}, nil, kustomizeStrategy)
	if buildErr != "" {
		t.Errorf("expected a clean build, got error: %q", buildErr)
	}
	if pre != hookNotDefined || post != hookNotDefined {
		t.Errorf("expected no hook outcomes without a cfg, got pre=%v post=%v", pre, post)
	}
}

func TestBuildOverlayWithHooks_PreBuildFailureSkipsBuild(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	ov := filepath.Join(app, "overlays", "prod")
	mustWrite(t, filepath.Join(ov, "deployment.yaml"), "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	mustWrite(t, filepath.Join(ov, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "#!/bin/sh\nPRE_BUILD_HOOK=fail_pre\nfail_pre() {\n\techo boom >&2\n\texit 1\n}\n")

	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	buildErr, pre, post, _ := buildOverlayWithHooks(overlayRef{path: ov, cluster: "prod"}, cfgs[app], kustomizeStrategy)
	if buildErr == "" || !strings.Contains(buildErr, "pre-build hook") {
		t.Errorf("expected a pre-build hook error, got %q", buildErr)
	}
	if pre != hookFailed {
		t.Errorf("expected pre=hookFailed, got %v", pre)
	}
	if post != hookNotDefined {
		t.Errorf("expected post=hookNotDefined (build was skipped), got %v", post)
	}
}

// Deliberately not t.Parallel(): this test writes the package-level
// hookBuildRoot var (see hook_wiring.go), which every other test in this
// package - parallel or not - reads via appBuildDir. Running concurrently
// with another hookBuildRoot-writer would race.
func TestBuildOverlayWithHooks_PostBuildHookRunsWithRenderedYAML(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	ov := filepath.Join(app, "overlays", "prod")
	mustWrite(t, filepath.Join(ov, "deployment.yaml"), "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	mustWrite(t, filepath.Join(ov, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "test.sh"), `#!/bin/sh
POST_BUILD_HOOK=check_yaml
check_yaml() {
	if [ ! -s "$1" ]; then
		echo "rendered YAML file $1 is missing or empty" >&2
		exit 1
	fi
	case "$1" in
		/*) ;;
		*) echo "expected an absolute YAML_FILE path, got $1" >&2; exit 1 ;;
	esac
}
`)

	hookBuildRoot = filepath.Join(t.TempDir(), "builds")
	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	buildErr, pre, post, _ := buildOverlayWithHooks(overlayRef{path: ov, cluster: "prod"}, cfgs[app], kustomizeStrategy)
	if buildErr != "" {
		t.Fatalf("expected a clean build, got error: %q", buildErr)
	}
	if pre != hookNotDefined {
		t.Errorf("expected pre=hookNotDefined (no PRE_BUILD_HOOK), got %v", pre)
	}
	if post != hookRan {
		t.Errorf("expected post=hookRan, got %v", post)
	}
}

// Deliberately not t.Parallel(): writes the package-level hookBuildRoot var
// (see the matching comment on TestBuildOverlayWithHooks_PostBuildHookRunsWithRenderedYAML above).
func TestBuildOverlayWithHooks_PostBuildHookFailureIsReported(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	ov := filepath.Join(app, "overlays", "prod")
	mustWrite(t, filepath.Join(ov, "deployment.yaml"), "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	mustWrite(t, filepath.Join(ov, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "#!/bin/sh\nPOST_BUILD_HOOK=fail_post\nfail_post() {\n\techo bad >&2\n\texit 1\n}\n")

	hookBuildRoot = filepath.Join(t.TempDir(), "builds")
	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	buildErr, _, post, _ := buildOverlayWithHooks(overlayRef{path: ov, cluster: "prod"}, cfgs[app], kustomizeStrategy)
	if buildErr == "" || !strings.Contains(buildErr, "post-build hook") {
		t.Errorf("expected a post-build hook error, got %q", buildErr)
	}
	if post != hookFailed {
		t.Errorf("expected post=hookFailed, got %v", post)
	}
}

func TestBuildOverlayWithHooks_ReturnsRenderedYAMLOnSuccess(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "deployment.yaml"), "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	mustWrite(t, filepath.Join(d, "kustomization.yaml"), "resources:\n  - deployment.yaml\n")

	buildErr, _, _, rendered := buildOverlayWithHooks(overlayRef{path: d, cluster: "foo"}, nil, kustomizeStrategy)
	if buildErr != "" {
		t.Fatalf("expected a clean build, got error: %q", buildErr)
	}
	if !strings.Contains(string(rendered), "kind: Deployment") {
		t.Errorf("expected the rendered YAML to be returned, got: %s", rendered)
	}
}

func TestBuildOverlayWithHooks_NoRenderedYAMLOnBuildFailure(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	buildErr, _, _, rendered := buildOverlayWithHooks(overlayRef{path: filepath.Join(d, "does-not-exist"), cluster: "foo"}, nil, kustomizeStrategy)
	if buildErr == "" {
		t.Fatal("expected a build error for a missing overlay")
	}
	if rendered != nil {
		t.Errorf("expected no rendered YAML on build failure, got: %s", rendered)
	}
}

// Deliberately not t.Parallel(): writes the package-level hookBuildRoot var
// (see the matching comment on TestBuildOverlayWithHooks_PostBuildHookRunsWithRenderedYAML above).
func TestRunAppPostValidateHooks(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "test.sh"), `#!/bin/sh
POST_VALIDATE_HOOK=check_build_dir
check_build_dir() {
	[ -d "$1" ] || { echo "build dir $1 missing" >&2; exit 1; }
}
`)
	hookBuildRoot = filepath.Join(t.TempDir(), "builds")
	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	if err := os.MkdirAll(appBuildDir(app), 0o750); err != nil {
		t.Fatal(err)
	}
	results := map[string]*appHookResult{app: {}}

	errs := runAppPostValidateHooks([]string{app}, cfgs, results)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if results[app].PostValidate != hookRan {
		t.Errorf("expected PostValidate=hookRan, got %v", results[app].PostValidate)
	}
	if _, err := os.Stat(appBuildDir(app)); !os.IsNotExist(err) {
		t.Error("expected the app build dir to be cleaned up after POST_VALIDATE_HOOK ran")
	}
}

// Deliberately not t.Parallel(): writes the package-level hookBuildRoot var
// (see the matching comment on TestBuildOverlayWithHooks_PostBuildHookRunsWithRenderedYAML above).
func TestRunAppPostValidateHooks_Failure(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "test.sh"), "#!/bin/sh\nPOST_VALIDATE_HOOK=fail_pv\nfail_pv() {\n\techo bad >&2\n\texit 1\n}\n")
	hookBuildRoot = filepath.Join(t.TempDir(), "builds")
	cfgs := resolveAppHookConfigs([]string{app}, hook.SourceLocal)
	results := map[string]*appHookResult{app: {}}

	errs := runAppPostValidateHooks([]string{app}, cfgs, results)
	if len(errs) != 1 || !strings.Contains(errs[0], "post-validate hook") {
		t.Fatalf("expected 1 post-validate hook error, got %v", errs)
	}
	if results[app].PostValidate != hookFailed {
		t.Errorf("expected PostValidate=hookFailed, got %v", results[app].PostValidate)
	}
}

func TestNeedsBuildDir(t *testing.T) {
	t.Parallel()
	if needsBuildDir(nil) {
		t.Error("expected nil config to not need a build dir")
	}
	if needsBuildDir(&hook.Config{}) {
		t.Error("expected a config with no hooks to not need a build dir")
	}
	if !needsBuildDir(&hook.Config{HasPostBuild: true}) {
		t.Error("expected HasPostBuild to need a build dir")
	}
	if !needsBuildDir(&hook.Config{HasPostValidate: true}) {
		t.Error("expected HasPostValidate to need a build dir")
	}
}

func TestAppBuildDir_SanitizesNestedAppPaths(t *testing.T) {
	t.Parallel()
	got := appBuildDir(filepath.Join("apps", "myapp"))
	if strings.Contains(got, string(filepath.Separator)+"apps"+string(filepath.Separator)+"myapp") {
		t.Errorf("expected nested app path segments to be flattened, got %q", got)
	}
	if !strings.HasPrefix(got, hookBuildRoot) {
		t.Errorf("expected the build dir to live under hookBuildRoot, got %q", got)
	}
}

// ── end-to-end: EXEMPTIONS wiring ───────────────────────────────────────────

// TestRunAll_HookExemptionsSuppressImageChecksumFinding is the end-to-end
// regression guard for docs/HOOKS.md's previously-documented limitation
// ("a real EXEMPTIONS=(...) entry in a test.sh today has zero effect on
// validation"): an app's test.sh EXEMPTIONS entry must now actually
// suppress the matching finding.
func TestRunAll_HookExemptionsSuppressImageChecksumFinding(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"),
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\nspec:\n  template:\n    spec:\n      containers:\n      - name: c\n        image: registry.example.com/app:latest\n")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources:\n  - ../../base\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "EXEMPTIONS=(check=image-checksum,value=registry.example.com/app:latest)\n")

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	for _, f := range res.Check.Findings {
		if f.CheckID == "image-checksum" {
			t.Errorf("expected the image-checksum finding to be exempted via test.sh EXEMPTIONS, but it's still a finding: %+v", f)
		}
	}
	found := false
	for _, e := range res.Check.Exempted {
		if e.CheckID == "image-checksum" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an image-checksum entry in Check.Exempted, got %+v", res.Check.Exempted)
	}
}

// TestRunAll_WithoutHookExemptionsImageChecksumFindingBlocks is the control
// for the test above: the same unpinned image, with no EXEMPTIONS entry,
// must still block.
func TestRunAll_WithoutHookExemptionsImageChecksumFindingBlocks(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "deployment.yaml"),
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\nspec:\n  template:\n    spec:\n      containers:\n      - name: c\n        image: registry.example.com/app:latest\n")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - deployment.yaml\n")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources:\n  - ../../base\n")

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	found := false
	for _, f := range res.Check.Findings {
		if f.CheckID == "image-checksum" {
			found = true
		}
	}
	if !found {
		t.Error("expected an image-checksum finding without a matching EXEMPTIONS entry")
	}
}

// TestRunAll_MalformedHookExemptionBlocks guards the fail-closed design of
// a malformed EXEMPTIONS token: it must not silently exempt anything, and
// must itself surface as a blocking failure so the author notices and fixes
// the selector rather than unknowingly under-exempting.
func TestRunAll_MalformedHookExemptionBlocks(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "EXEMPTIONS=(kind=Application,name=x,path=source.path)\n") // missing check=

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a malformed EXEMPTIONS entry to be surfaced as a logger failure")
	}
}

// ── end-to-end: hook execution wiring ────────────────────────────────────────

func TestRunAll_FailingPostBuildHookBlocks(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "#!/bin/sh\nPOST_BUILD_HOOK=fail_post\nfail_post() {\n\techo boom >&2\n\texit 1\n}\n")
	hookBuildRoot = filepath.Join(t.TempDir(), "builds")

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a failing POST_BUILD_HOOK to be surfaced as a logger failure")
	}
	var kb ReportSection
	for _, s := range res.Sections {
		if s.Name == "Kustomize Build" {
			kb = s
		}
	}
	if kb.Status != StatusError || !strings.Contains(kb.Body, "post-build hook") {
		t.Errorf("expected the Kustomize Build section to report the post-build hook failure, got:\n%s", kb.Body)
	}
}

// TestRunAll_KustomizeFixFindingBlocks guards the real bug reported
// against this: composeKustomizeFixChild rendered a Kustomize Fix finding
// as a StatusError ("❌") sub-dropdown, but runBuildAndPostBuild never
// called log.ErrorInSection for it (unlike every sibling check in this
// same section - Overlay Build errors, blocking Ghost Patches), so
// res.Logger.HasFailures() (and thus "pipeline"'s exit code) stayed false
// even with a real, visibly-❌ finding in the report - see Result.Failed.
// kustomize.CheckFix shells out to the real kustomize CLI (see
// pkg/kustomize's package doc comment), so this needs the real binaries
// installed - matching the exec.LookPath+t.Skip pattern already used
// elsewhere in this repo for CLI-wrapping tests.
func TestRunAll_KustomizeFixFindingBlocks(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	if _, err := exec.LookPath("prettier"); err != nil {
		t.Skip("prettier not installed")
	}
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	// A deprecated `vars:` block: kustomize edit fix --vars converts it
	// to `replacements:`, so kustomize.CheckFix flags this file.
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - resource.yaml
vars:
  - name: FOO
    objref:
      kind: ConfigMap
      name: my-configmap
      apiVersion: v1
    fieldref:
      fieldpath: data.foo
`)
	mustWrite(t, filepath.Join(app, "overlays", "prod", "resource.yaml"),
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\n")
	hookBuildRoot = filepath.Join(t.TempDir(), "builds")

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if res.Logger == nil || !res.Logger.HasFailures() {
		t.Error("expected a Kustomize Fix finding to be surfaced as a logger failure")
	}
	if !res.Failed() {
		t.Error("expected a Kustomize Fix finding to make Result.Failed() report true")
	}
	var kb ReportSection
	for _, s := range res.Sections {
		if s.Name == "Kustomize Build" {
			kb = s
		}
	}
	if kb.Status != StatusError || !strings.Contains(kb.Body, "kustomize edit fix") {
		t.Errorf("expected the Kustomize Build section to report the fix finding, got:\n%s", kb.Body)
	}
}

func TestRunAll_SuccessfulHooksReportRanInBuildSection(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "test.sh"), `#!/bin/sh
PRE_BUILD_HOOK=ok_pre
POST_BUILD_HOOK=ok_post
ok_pre() { :; }
ok_post() { :; }
`)
	hookBuildRoot = filepath.Join(t.TempDir(), "builds")

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	var kb ReportSection
	for _, s := range res.Sections {
		if s.Name == "Kustomize Build" {
			kb = s
		}
	}
	if strings.Count(kb.Body, "✅ ran") != 2 {
		t.Errorf("expected both PRE_BUILD and POST_BUILD to report '✅ ran', got:\n%s", kb.Body)
	}
}
