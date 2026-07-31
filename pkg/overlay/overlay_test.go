package overlay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindAllOverlays_NoDir(t *testing.T) {
	if got := FindAllOverlays("/tmp/not-an-app-lkj"); got != nil {
		t.Errorf("expected nil: %v", got)
	}
}

func TestIsExcluded(t *testing.T) {
	if !IsExcluded("app/overlays/excluded", map[string]bool{"excluded": true}) {
		t.Error("expected excluded")
	}
}

func TestGetOverlaysToTest_NoChanges(t *testing.T) {
	dir := makeApp(t)
	ov, full, _ := GetOverlaysToTest(dir, []string{}, false)
	if len(ov) != 0 || full {
		t.Errorf("expected no overlays: %v full=%v", ov, full)
	}
}

func TestGetOverlaysToTest_BaseChange(t *testing.T) {
	dir := makeApp(t)
	ov, full, _ := GetOverlaysToTest(dir, []string{dir + "/base/kustomization.yaml"}, false)
	if !full || len(ov) != 2 {
		t.Errorf("expected full test: %v full=%v", ov, full)
	}
}

func TestGetOverlaysToTest_SpecificOverlay(t *testing.T) {
	dir := makeApp(t)
	ov, full, _ := GetOverlaysToTest(dir, []string{dir + "/overlays/dev/kustomization.yaml"}, false)
	if full || len(ov) != 1 {
		t.Errorf("expected one overlay: %v full=%v", ov, full)
	}
}

func TestRunBuildLoop_Empty(t *testing.T) {
	res := RunBuildLoop(BuildOptions{})
	if len(res) != 0 {
		t.Errorf("expected empty: %v", res)
	}
}

func TestRunBuildLoop_KustomizeBuild(t *testing.T) {
	dir := makeApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	out := t.TempDir()
	res := RunBuildLoop(BuildOptions{App: dir, Overlays: []string{ov}, Strategy: StrategyKustomize, OutputDir: out})
	if len(res) != 1 {
		t.Fatalf("expected one result: %v", res)
	}
	if res[0].Err != nil {
		t.Fatalf("unexpected error: %v", res[0].Err)
	}
	got, err := os.ReadFile(res[0].YAMLFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(got), "kind: ConfigMap") {
		t.Errorf("expected rendered ConfigMap, got: %s", got)
	}
	if !strings.Contains(string(got), "dev-cm") {
		t.Errorf("expected dev-overlay name prefix, got: %s", got)
	}
}

func TestRunBuildLoop_KustomizeAVPExcluded(t *testing.T) {
	// When the overlay is excluded from AVP, the build must succeed without
	// invoking (or requiring) the argocd-vault-plugin binary.
	dir := makeApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	out := t.TempDir()
	res := RunBuildLoop(BuildOptions{
		App:        dir,
		Overlays:   []string{ov},
		Strategy:   StrategyKustomizeAVP,
		OutputDir:  out,
		AVPExclude: []string{"dev"},
	})
	if len(res) != 1 {
		t.Fatalf("expected one result: %v", res)
	}
	if res[0].Err != nil {
		t.Fatalf("unexpected error: %v", res[0].Err)
	}
}

func TestRunBuildLoop_HelmBuild(t *testing.T) {
	dir := makeHelmApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	out := t.TempDir()
	res := RunBuildLoop(BuildOptions{App: dir, Overlays: []string{ov}, Strategy: StrategyHelm, OutputDir: out})
	if len(res) != 1 {
		t.Fatalf("expected one result: %v", res)
	}
	if res[0].Err != nil {
		t.Fatalf("unexpected error: %v", res[0].Err)
	}
	got, err := os.ReadFile(res[0].YAMLFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "kind: Deployment") {
		t.Errorf("expected rendered Deployment, got: %s", s)
	}
	if !strings.Contains(s, "replicas: 3") {
		t.Errorf("expected overridden replicaCount=3, got: %s", s)
	}
	if strings.Contains(s, "NOTES.txt") {
		t.Errorf("NOTES.txt should be filtered out of output, got: %s", s)
	}
}

func TestRunBuildLoop_HelmMissingValues(t *testing.T) {
	dir := makeHelmApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	if err := os.Remove(filepath.Join(ov, "values.yaml")); err != nil {
		t.Fatalf("removing values.yaml: %v", err)
	}
	out := t.TempDir()
	res := RunBuildLoop(BuildOptions{App: dir, Overlays: []string{ov}, Strategy: StrategyHelm, OutputDir: out})
	if len(res) != 1 {
		t.Fatalf("expected one result: %v", res)
	}
	if res[0].Err == nil || !strings.Contains(res[0].Err.Error(), "missing values.yaml") {
		t.Errorf("expected missing values.yaml error, got: %v", res[0].Err)
	}
}

func TestRenderWithStrategy_Kustomize(t *testing.T) {
	dir := makeApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	out, err := RenderWithStrategy(dir, ov, StrategyKustomize, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "dev-cm") {
		t.Errorf("expected rendered dev overlay, got: %s", out)
	}
}

func TestRenderWithStrategy_Helm(t *testing.T) {
	dir := makeHelmApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	out, err := RenderWithStrategy(dir, ov, StrategyHelm, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "replicas: 3") {
		t.Errorf("expected the helm-rendered overlay, got: %s", out)
	}
}

func TestRenderWithStrategy_KustomizeAVPExcludedSkipsAVP(t *testing.T) {
	// With the overlay excluded, this must not shell out to
	// argocd-vault-plugin at all - a clean render is proof the AVP step
	// was skipped, since the binary isn't available in the test environment.
	dir := makeApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	out, err := RenderWithStrategy(dir, ov, StrategyKustomizeAVP, ExcludeSet([]string{"dev"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "dev-cm") {
		t.Errorf("expected rendered dev overlay, got: %s", out)
	}
}

func TestRenderWithStrategy_UnknownStrategy(t *testing.T) {
	dir := makeApp(t)
	ov := filepath.Join(dir, "overlays", "dev")
	if _, err := RenderWithStrategy(dir, ov, Strategy("bogus"), nil); err == nil {
		t.Fatal("expected an error for an unknown strategy")
	}
}

func TestExcludeSet(t *testing.T) {
	set := ExcludeSet([]string{"dev", "staging"})
	if !set["dev"] || !set["staging"] {
		t.Errorf("expected both names present: %v", set)
	}
	if set["prod"] {
		t.Errorf("expected prod absent: %v", set)
	}
}

func TestAssembleManifests(t *testing.T) {
	rendered := map[string]string{
		"c/templates/notes.txt":       "irrelevant", // not literally NOTES.txt, kept for path-base check below
		"c/templates/NOTES.txt":       "some notes",
		"c/templates/empty.yaml":      "   \n",
		"c/templates/b-resource.yaml": "kind: B\n",
		"c/templates/a-resource.yaml": "kind: A\n",
	}
	out := string(assembleManifests(rendered))
	if strings.Contains(out, "some notes") {
		t.Errorf("NOTES.txt should be filtered: %s", out)
	}
	if strings.Contains(out, "empty.yaml") {
		t.Errorf("empty renders should be filtered: %s", out)
	}
	// Deterministic, sorted order: a-resource before b-resource.
	if strings.Index(out, "kind: A") > strings.Index(out, "kind: B") {
		t.Errorf("expected sorted output, got: %s", out)
	}
}

func makeApp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, ov := range []string{"dev", "prod"} {
		p := filepath.Join(dir, "overlays", ov)
		_ = os.MkdirAll(p, 0o755)
		_ = os.WriteFile(filepath.Join(p, "kustomization.yaml"), []byte(
			"resources:\n- ../../base\nconfigMapGenerator:\n- name: "+ov+"-cm\n  literals:\n  - foo=bar\ngeneratorOptions:\n  disableNameSuffixHash: true\n",
		), 0o644)
	}
	_ = os.MkdirAll(filepath.Join(dir, "base"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "base", "kustomization.yaml"), []byte("resources: []\n"), 0o644)
	return dir
}

// makeHelmApp builds a minimal Helm chart under <app>/base plus an
// overlays/dev values.yaml override, for exercising the native Helm
// rendering path (loader + chartutil + engine).
func makeHelmApp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	base := filepath.Join(dir, "base")
	tmpl := filepath.Join(base, "templates")
	_ = os.MkdirAll(tmpl, 0o755)

	_ = os.WriteFile(filepath.Join(base, "Chart.yaml"), []byte(
		"apiVersion: v2\nname: myapp\nversion: 0.1.0\n",
	), 0o644)
	_ = os.WriteFile(filepath.Join(base, "values.yaml"), []byte(
		"replicaCount: 1\nimage: myapp:latest\n",
	), 0o644)
	_ = os.WriteFile(filepath.Join(tmpl, "deployment.yaml"), []byte(
		"apiVersion: apps/v1\n"+
			"kind: Deployment\n"+
			"metadata:\n"+
			"  name: {{ .Release.Name }}\n"+
			"spec:\n"+
			"  replicas: {{ .Values.replicaCount }}\n"+
			"  template:\n"+
			"    spec:\n"+
			"      containers:\n"+
			"      - name: myapp\n"+
			"        image: {{ .Values.image }}\n",
	), 0o644)
	_ = os.WriteFile(filepath.Join(tmpl, "NOTES.txt"), []byte("Thanks for installing {{ .Chart.Name }}.\n"), 0o644)

	ov := filepath.Join(dir, "overlays", "dev")
	_ = os.MkdirAll(ov, 0o755)
	_ = os.WriteFile(filepath.Join(ov, "values.yaml"), []byte("replicaCount: 3\n"), 0o644)

	return dir
}
