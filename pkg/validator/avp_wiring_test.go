package validator

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

func TestResolveAppBuildStrategies_NoAVPIndicators(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources: []\n")

	strategies := resolveAppBuildStrategies([]string{app}, true, nil)
	got, ok := strategies[app]
	if !ok {
		t.Fatalf("expected an entry for %q", app)
	}
	if got.Strategy != overlay.StrategyKustomize {
		t.Errorf("Strategy = %q, want %q", got.Strategy, overlay.StrategyKustomize)
	}
	if got.Exclude != nil {
		t.Errorf("expected nil Exclude with no hook.Config, got %v", got.Exclude)
	}
}

func TestResolveAppBuildStrategies_AVPIndicatorSelectsAVPStrategy(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "base", "secret.yaml"), "password: <path:secret#field>\n")

	strategies := resolveAppBuildStrategies([]string{app}, true, nil)
	if got := strategies[app].Strategy; got != overlay.StrategyKustomizeAVP {
		t.Errorf("Strategy = %q, want %q", got, overlay.StrategyKustomizeAVP)
	}
}

func TestResolveAppBuildStrategies_AVPDisabledForcesPlainKustomize(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join(app, "base", "secret.yaml"), "password: <path:secret#field>\n")

	strategies := resolveAppBuildStrategies([]string{app}, false, nil)
	if got := strategies[app].Strategy; got != overlay.StrategyKustomize {
		t.Errorf("Strategy = %q, want %q (avp disabled)", got, overlay.StrategyKustomize)
	}
}

func TestResolveAppBuildStrategies_ExcludeFromHookConfig(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	cfgs := map[string]*hook.Config{app: {AVPExclude: []string{"dev", "staging"}}}

	strategies := resolveAppBuildStrategies([]string{app}, true, cfgs)
	exclude := strategies[app].Exclude
	if !exclude["dev"] || !exclude["staging"] {
		t.Errorf("expected both excluded overlays present, got %v", exclude)
	}
}

// TestRunAll_AVPStrategyAutoDetectedAndExcludeRespected is the end-to-end
// regression guard for docs/CI.md's previously-documented "Not yet wired"
// AVP callout: an app containing an AVP placeholder must render via the
// AVP strategy, and its test.sh AVP_EXCLUDE= entry must still let the
// excluded overlay build cleanly without needing the argocd-vault-plugin
// binary at all (proving the AVP step was actually skipped for it, not
// just that it happened to succeed).
func TestRunAll_AVPStrategyAutoDetectedAndExcludeRespected(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - secret.yaml\n")
	mustWrite(t, filepath.Join(app, "base", "secret.yaml"),
		"apiVersion: v1\nkind: Secret\nmetadata:\n  name: foo\n  namespace: bar\nstringData:\n  password: <path:secret#field>\n")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources:\n  - ../../base\n")
	mustWrite(t, filepath.Join(app, "test.sh"), "AVP_EXCLUDE=\"dev\"\n")

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
	if kb.Status == StatusError {
		t.Errorf("expected the AVP-excluded overlay to build cleanly without the argocd-vault-plugin binary, got:\n%s", kb.Body)
	}
}

// TestRunAll_AVPIndicatorWithoutExcludeInvokesAVP is the negative control
// for the two tests above: with neither an AVP_EXCLUDE= entry nor
// --disable-checks avp, the same AVP-indicator app must actually attempt
// AVP substitution - proving the previous two tests' clean builds came from
// the exclude/disable mechanism actually working, not from AVP never being
// invoked in the first place. argocd-vault-plugin is expected to be
// installed (this repo's own dev/CI image includes it - see
// docs/DEVELOPMENT.md) but unconfigured, so it fails fast rather than
// hanging or silently no-op'ing.
func TestRunAll_AVPIndicatorWithoutExcludeInvokesAVP(t *testing.T) {
	if _, err := exec.LookPath("argocd-vault-plugin"); err != nil {
		t.Skip("argocd-vault-plugin not installed; can't distinguish 'not invoked' from 'invoked and passed'")
	}
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - secret.yaml\n")
	mustWrite(t, filepath.Join(app, "base", "secret.yaml"),
		"apiVersion: v1\nkind: Secret\nmetadata:\n  name: foo\n  namespace: bar\nstringData:\n  password: <path:secret#field>\n")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources:\n  - ../../base\n")

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
	if kb.Status != StatusError {
		t.Error("expected AVP substitution to actually be attempted (and fail, unconfigured) for an AVP-indicator app with no exclude/disable")
	}
}

// TestRunAll_AVPDisabledViaDisabledChecks guards --disable-checks avp: an
// app with a real AVP indicator, and no AVP_EXCLUDE, must still build
// cleanly (proving AVP substitution never ran) once "avp" is disabled.
func TestRunAll_AVPDisabledViaDisabledChecks(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "base", "kustomization.yaml"), "resources:\n  - secret.yaml\n")
	mustWrite(t, filepath.Join(app, "base", "secret.yaml"),
		"apiVersion: v1\nkind: Secret\nmetadata:\n  name: foo\n  namespace: bar\nstringData:\n  password: <path:secret#field>\n")
	mustWrite(t, filepath.Join(app, "overlays", "dev", "kustomization.yaml"), "resources:\n  - ../../base\n")

	res, err := RunAll(Options{Dirs: []string{d}, HookSource: "local", DisabledChecks: []string{"avp"}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	var kb ReportSection
	for _, s := range res.Sections {
		if s.Name == "Kustomize Build" {
			kb = s
		}
	}
	if kb.Status == StatusError {
		t.Errorf("expected the build to succeed without invoking argocd-vault-plugin once avp is disabled, got:\n%s", kb.Body)
	}
}
