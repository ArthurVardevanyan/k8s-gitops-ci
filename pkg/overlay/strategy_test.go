package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectStrategy_KustomizeNoAVP(t *testing.T) {
	dir := makeApp(t)
	if got := DetectStrategy(dir, true); got != StrategyKustomize {
		t.Errorf("DetectStrategy = %q, want %q", got, StrategyKustomize)
	}
}

func TestDetectStrategy_KustomizeWithAVPPlaceholder(t *testing.T) {
	dir := makeApp(t)
	writeAVPMarker(t, filepath.Join(dir, "overlays", "dev", "secret.yaml"), "<path:secret#field>")
	if got := DetectStrategy(dir, true); got != StrategyKustomizeAVP {
		t.Errorf("DetectStrategy = %q, want %q", got, StrategyKustomizeAVP)
	}
}

func TestDetectStrategy_KustomizationWinsOverChartYAML(t *testing.T) {
	// A base/ with both kustomization.yaml and Chart.yaml (the chart
	// consumed via kustomize's helmCharts inflator) must still pick
	// kustomize, not helm.
	dir := makeApp(t)
	if err := os.WriteFile(filepath.Join(dir, "base", "Chart.yaml"), []byte("apiVersion: v2\nname: x\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectStrategy(dir, true); got != StrategyKustomize {
		t.Errorf("DetectStrategy = %q, want %q", got, StrategyKustomize)
	}
}

func TestDetectStrategy_HelmNoAVP(t *testing.T) {
	dir := makeHelmApp(t)
	if got := DetectStrategy(dir, true); got != StrategyHelm {
		t.Errorf("DetectStrategy = %q, want %q", got, StrategyHelm)
	}
}

func TestDetectStrategy_HelmWithAVPAnnotation(t *testing.T) {
	dir := makeHelmApp(t)
	writeAVPMarker(t, filepath.Join(dir, "overlays", "dev", "annotations.yaml"), "avp.kubernetes.io/path: secret/data/foo")
	if got := DetectStrategy(dir, true); got != StrategyHelmAVP {
		t.Errorf("DetectStrategy = %q, want %q", got, StrategyHelmAVP)
	}
}

func TestDetectStrategy_NeitherKustomizeNorHelmDefaultsToKustomize(t *testing.T) {
	dir := t.TempDir()
	if got := DetectStrategy(dir, true); got != StrategyKustomize {
		t.Errorf("DetectStrategy = %q, want %q (the safe default)", got, StrategyKustomize)
	}
}

func TestDetectStrategy_AVPDisabledIgnoresIndicators(t *testing.T) {
	dir := makeApp(t)
	writeAVPMarker(t, filepath.Join(dir, "overlays", "dev", "secret.yaml"), "<path:secret#field>")
	if got := DetectStrategy(dir, false); got != StrategyKustomize {
		t.Errorf("DetectStrategy(avpEnabled=false) = %q, want %q", got, StrategyKustomize)
	}
}

func TestDetectStrategy_AVPDisabledHelmStillDetected(t *testing.T) {
	// Disabling AVP must only skip the AVP variant, not Helm detection
	// itself - a pure-Helm app with no AVP indicators still renders via
	// Helm either way.
	dir := makeHelmApp(t)
	if got := DetectStrategy(dir, false); got != StrategyHelm {
		t.Errorf("DetectStrategy(avpEnabled=false) = %q, want %q", got, StrategyHelm)
	}
}

func TestAppHasAVPIndicators_DirectPluginReference(t *testing.T) {
	dir := makeApp(t)
	writeAVPMarker(t, filepath.Join(dir, "base", "cmp.yaml"), "plugin: argocd-vault-plugin")
	if !AppHasAVPIndicators(dir) {
		t.Error("expected AVP indicator to be detected")
	}
}

func TestAppHasAVPIndicators_None(t *testing.T) {
	dir := makeApp(t)
	if AppHasAVPIndicators(dir) {
		t.Error("expected no AVP indicator")
	}
}

func TestAppHasAVPIndicators_IgnoresNonYAML(t *testing.T) {
	dir := makeApp(t)
	writeAVPMarker(t, filepath.Join(dir, "README.md"), "<path:secret#field>")
	if AppHasAVPIndicators(dir) {
		t.Error("expected non-YAML files to be ignored")
	}
}

func writeAVPMarker(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
