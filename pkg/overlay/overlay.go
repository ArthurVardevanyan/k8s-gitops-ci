package overlay

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
	Overlay string
	YAMLFile string
	Err     error
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
func GetOverlaysToTest(app string, changedFiles []string, ignoreTestChanges bool) ([]string, bool, string) {
	if app == "" {
		return nil, false, ""
	}
	all := FindAllOverlays(app)
	if len(all) == 0 {
		return nil, false, ""
	}
	var selected []string
	fullTest := false
	trigger := ""
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
	var results []BuildResult
	exclude := make(map[string]bool)
	for _, e := range opts.AVPExclude {
		exclude[e] = true
	}
	for _, ov := range opts.Overlays {
		if opts.Progress != nil {
			opts.Progress("building %s", ov)
		}
		outFile := filepath.Join(opts.OutputDir, filepath.Base(ov)+".yaml")
		res := buildOverlay(ov, opts.Strategy, exclude, outFile, opts.PreBuildHook)
		results = append(results, res)
	}
	return results
}

func buildOverlay(overlay string, strategy Strategy, exclude map[string]bool, outFile string, pre func(string, string) error) BuildResult {
	if pre != nil {
		if err := pre(overlay, outFile); err != nil {
			return BuildResult{Overlay: overlay, Err: err}
		}
	}
	ctx := context.Background()
	isExcluded := IsExcluded(overlay, exclude)
	switch strategy {
	case StrategyKustomize:
		return runKustomize(ctx, overlay, outFile)
	case StrategyKustomizeAVP:
		if isExcluded {
			return runKustomize(ctx, overlay, outFile)
		}
		return runKustomizeAVP(ctx, overlay, outFile)
	case StrategyHelm:
		return runHelm(ctx, overlay, outFile)
	case StrategyHelmAVP:
		if isExcluded {
			return runHelm(ctx, overlay, outFile)
		}
		return runHelmAVP(ctx, overlay, outFile)
	default:
		return BuildResult{Overlay: overlay, Err: fmt.Errorf("unknown strategy %q", strategy)}
	}
}

func runKustomize(_ context.Context, overlay, outFile string) BuildResult {
	cmd := exec.Command("kustomize", "build", overlay)
	out, err := cmd.Output()
	if err != nil {
		return BuildResult{Overlay: overlay, Err: fmtErr(cmd, err)}
	}
	if err := os.WriteFile(outFile, out, 0o644); err != nil {
		return BuildResult{Overlay: overlay, Err: err}
	}
	return BuildResult{Overlay: overlay, YAMLFile: outFile}
}

func runKustomizeAVP(_ context.Context, overlay, outFile string) BuildResult {
	build := exec.Command("kustomize", "build", overlay)
	buildOut, err := build.Output()
	if err != nil {
		return BuildResult{Overlay: overlay, Err: fmtErr(build, err)}
	}
	avp := exec.Command("argocd-vault-plugin", "generate", "-")
	avp.Stdin = strings.NewReader(string(buildOut))
	out, err := avp.Output()
	if err != nil {
		return BuildResult{Overlay: overlay, Err: fmtErr(avp, err)}
	}
	if err := os.WriteFile(outFile, out, 0o644); err != nil {
		return BuildResult{Overlay: overlay, Err: err}
	}
	return BuildResult{Overlay: overlay, YAMLFile: outFile}
}

func runHelm(_ context.Context, overlay, outFile string) BuildResult {
	_ = overlay
	if _, err := os.Stat(filepath.Join(overlay, "values.yaml")); err != nil {
		return BuildResult{Overlay: overlay, Err: fmt.Errorf("missing values.yaml: %w", err)}
	}
	return BuildResult{Overlay: overlay, Err: fmt.Errorf("helm build not implemented")}
}

func runHelmAVP(_ context.Context, overlay, outFile string) BuildResult {
	res := runHelm(context.Background(), overlay, outFile)
	if res.Err != nil {
		return res
	}
	return BuildResult{Overlay: overlay, Err: fmt.Errorf("helm+avp build not implemented")}
}

func fmtErr(cmd *exec.Cmd, err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
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
