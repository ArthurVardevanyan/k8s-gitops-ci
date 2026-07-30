package overlay

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Strategy identifies how to build an overlay.
type Strategy string

const (
	StrategyKustomize    Strategy = "kustomize"
	StrategyKustomizeAVP Strategy = "kustomize-avp"
	StrategyHelm         Strategy = "helm"
	StrategyHelmAVP      Strategy = "helm-avp"
)

// SecretAuthHint is an org-injectable AVP auth hint function (mirror of provider).
var SecretAuthHint func(stderr string) string

// BuildOptions configures the overlay build loop.
type BuildOptions struct {
	App          string
	Strategy     Strategy
	Overlays     []string
	OutputDir    string
	AVPExclude   []string
	Parallel     bool
	PreBuildHook func(overlayPath, outputPath string) error
	Progress     ProgressFunc
}

// ProgressFunc emits build progress.
type ProgressFunc func(format string, args ...any)

// BuildResult captures the result of building a single overlay.
type BuildResult struct {
	Overlay  string
	YAMLFile string
	Err      error
}

// FindAllOverlays returns all overlay directories under app/overlays.
func FindAllOverlays(app string) []string {
	dir := filepath.Join(app, "overlays")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// HasKustomization reports whether an overlay uses kustomize.
func HasKustomization(overlay string) bool {
	_, err := os.Stat(filepath.Join(overlay, "kustomization.yaml"))
	return err == nil
}

// IsExcluded reports whether overlay is in the exclusion set.
func IsExcluded(overlay string, exclude map[string]bool) bool {
	base := filepath.Base(overlay)
	return exclude[base] || exclude[overlay]
}

// GetOverlaysToTest maps changed files to the set of overlays to build.
func GetOverlaysToTest(app string, changedFiles []string, ignoreTestChanges bool) (overlays []string, fullTest bool, trigger string) {
	if app == "" {
		return nil, false, ""
	}
	all := FindAllOverlays(app)
	if len(all) == 0 {
		return nil, false, ""
	}
	var selected []string
	fullTest = false
	trigger = ""
	for _, f := range changedFiles {
		if !strings.HasPrefix(f, app+"/") {
			continue
		}
		rel := strings.TrimPrefix(f, app+"/")
		if rel == "" {
			continue
		}
		parts := strings.Split(rel, "/")
		if len(parts) >= 2 && parts[0] == "base" {
			selected = all
			fullTest = true
			trigger = "base"
			break
		}
		if len(parts) >= 3 && parts[0] == "overlays" {
			ov := filepath.Join(app, "overlays", parts[1])
			if !contains(selected, ov) {
				selected = append(selected, ov)
			}
			trigger = "overlay"
		}
		if len(parts) >= 2 && parts[0] == "components" {
			trigger = "component"
		}
		_ = ignoreTestChanges
	}
	if len(selected) == 0 && trigger == "component" {
		selected = all
		fullTest = true
	}
	return selected, fullTest, trigger
}

// RunBuildLoop builds the selected overlays.
func RunBuildLoop(opts BuildOptions) []BuildResult {
	if len(opts.Overlays) == 0 {
		return nil
	}
	results := make([]BuildResult, 0, len(opts.Overlays))
	exclude := make(map[string]bool)
	for _, e := range opts.AVPExclude {
		exclude[e] = true
	}
	for _, ov := range opts.Overlays {
		if opts.Progress != nil {
			opts.Progress("building %s", ov)
		}
		outFile := filepath.Join(opts.OutputDir, filepath.Base(ov)+".yaml")
		res := buildOverlay(opts.App, ov, opts.Strategy, exclude, outFile, opts.PreBuildHook)
		results = append(results, res)
	}
	return results
}

func buildOverlay(app, overlay string, strategy Strategy, exclude map[string]bool, outFile string, pre func(string, string) error) BuildResult {
	if pre != nil {
		if err := pre(overlay, outFile); err != nil {
			return BuildResult{Overlay: overlay, Err: err}
		}
	}
	isExcluded := IsExcluded(overlay, exclude)

	var render func() ([]byte, error)
	switch strategy {
	case StrategyKustomize, StrategyKustomizeAVP:
		render = func() ([]byte, error) { return renderKustomize(overlay) }
	case StrategyHelm, StrategyHelmAVP:
		render = func() ([]byte, error) { return renderHelm(app, overlay) }
	default:
		return BuildResult{Overlay: overlay, Err: fmt.Errorf("unknown strategy %q", strategy)}
	}

	out, err := render()
	if err != nil {
		return BuildResult{Overlay: overlay, Err: err}
	}

	useAVP := (strategy == StrategyKustomizeAVP || strategy == StrategyHelmAVP) && !isExcluded
	if useAVP {
		out, err = runAVP(out)
		if err != nil {
			return BuildResult{Overlay: overlay, Err: err}
		}
	}

	if err := os.WriteFile(outFile, out, 0o600); err != nil {
		return BuildResult{Overlay: overlay, Err: err}
	}
	return BuildResult{Overlay: overlay, YAMLFile: outFile}
}

// renderKustomize builds overlay using the native Kustomize SDK (the same
// engine the `kustomize` CLI itself runs on), avoiding a runtime dependency
// on a `kustomize` binary being installed in the CI image.
func renderKustomize(overlay string) ([]byte, error) {
	fSys := filesys.MakeFsOnDisk()
	opts := krusty.MakeDefaultOptions()
	opts.LoadRestrictions = types.LoadRestrictionsNone
	k := krusty.MakeKustomizer(opts)

	resMap, err := k.Run(fSys, overlay)
	if err != nil {
		return nil, fmt.Errorf("kustomize build %s: %w", overlay, err)
	}

	out, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("kustomize render %s: %w", overlay, err)
	}
	return out, nil
}

// renderHelm renders overlay/values.yaml against app/base using the native
// Helm chart loader + rendering engine (no `pkg/action`, so no dependency on
// Helm's release-storage backend or a live cluster connection). This mirrors
// what `helm template` produces.
func renderHelm(app, overlay string) ([]byte, error) {
	valuesFile := filepath.Join(overlay, "values.yaml")
	if _, err := os.Stat(valuesFile); err != nil {
		return nil, fmt.Errorf("missing values.yaml: %w", err)
	}

	baseDir := filepath.Join(app, "base")
	chrt, err := loader.Load(baseDir)
	if err != nil {
		return nil, fmt.Errorf("helm load chart %s: %w", baseDir, err)
	}

	overrideVals, err := chartutil.ReadValuesFile(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("parsing values %s: %w", valuesFile, err)
	}

	releaseOpts := chartutil.ReleaseOptions{
		Name:      filepath.Base(overlay),
		Namespace: "default",
		IsInstall: true,
	}
	renderVals, err := chartutil.ToRenderValues(chrt, overrideVals, releaseOpts, nil)
	if err != nil {
		return nil, fmt.Errorf("helm values %s: %w", overlay, err)
	}

	rendered, err := engine.Render(chrt, renderVals)
	if err != nil {
		return nil, fmt.Errorf("helm template %s: %w", overlay, err)
	}

	return assembleManifests(rendered), nil
}

// assembleManifests reassembles engine.Render's per-file output into a single
// YAML stream, mirroring `helm template`'s behavior: NOTES.txt and empty
// renders are dropped, remaining documents are emitted in a stable, sorted
// order with a "# Source:" header per document.
func assembleManifests(rendered map[string]string) []byte {
	keys := make([]string, 0, len(rendered))
	for name := range rendered {
		if path.Base(name) == "NOTES.txt" {
			continue
		}
		if strings.TrimSpace(rendered[name]) == "" {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, name := range keys {
		buf.WriteString("---\n# Source: ")
		buf.WriteString(name)
		buf.WriteString("\n")
		buf.WriteString(strings.TrimSpace(rendered[name]))
		buf.WriteString("\n")
	}
	return buf.Bytes()
}

// runAVP pipes rendered YAML through `argocd-vault-plugin generate -`,
// resolving AVP placeholders (<path:...>, <vault:...>, etc.) against the
// configured secret backend. This mirrors what the argocd-vault-plugin
// kustomize/helm CMPs do at deploy time. AVP is kept as a subprocess call
// (rather than an imported library) since embedding it would pull in every
// supported secret-backend SDK (AWS, GCP, Vault, Azure) unconditionally.
func runAVP(in []byte) ([]byte, error) {
	cmd := exec.Command("argocd-vault-plugin", "generate", "-")
	cmd.Stdin = bytes.NewReader(in)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmtErr(cmd, err)
	}
	return out, nil
}

func fmtErr(cmd *exec.Cmd, err error) error {
	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		stderr := string(exitErr.Stderr)
		hint := ""
		if SecretAuthHint != nil {
			hint = SecretAuthHint(stderr)
		}
		msg := fmt.Sprintf("%s failed: %s", cmd.Path, stderr)
		if hint != "" {
			msg += "; " + hint
		}
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("%s: %w", cmd.Path, err)
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
