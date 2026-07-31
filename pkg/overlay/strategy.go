package overlay

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DetectStrategy inspects app's on-disk layout and content to pick which
// Strategy RenderWithStrategy should use to build its overlays:
//
//   - A base/kustomization.yaml → StrategyKustomize (or StrategyKustomizeAVP
//     if avpEnabled and any AVP indicator is found anywhere under app - see
//     AppHasAVPIndicators). A Chart.yaml alongside the kustomization.yaml is
//     still built via kustomize (consumed through kustomize's helmCharts
//     inflator, not this package's own Helm renderer).
//   - No kustomization.yaml but a base/Chart.yaml → StrategyHelm (or
//     StrategyHelmAVP, same AVP-indicator check).
//   - Neither → StrategyKustomize (the default; RenderWithStrategy's own
//     kustomize error surfaces the real problem, e.g. a missing base/).
//
// avpEnabled=false skips the AVP-indicator scan entirely and always returns
// the non-AVP variant - for an operator running without the
// argocd-vault-plugin binary/a configured secret backend, so an AVP
// indicator in an app's content never forces a build path that would fail
// for reasons unrelated to the change being validated.
func DetectStrategy(app string, avpEnabled bool) Strategy {
	baseDir := filepath.Join(app, "base")
	if HasKustomization(baseDir) {
		if avpEnabled && AppHasAVPIndicators(app) {
			return StrategyKustomizeAVP
		}
		return StrategyKustomize
	}
	if _, err := os.Stat(filepath.Join(baseDir, "Chart.yaml")); err == nil {
		if avpEnabled && AppHasAVPIndicators(app) {
			return StrategyHelmAVP
		}
		return StrategyHelm
	}
	return StrategyKustomize
}

// avpIndicators are substrings that, found anywhere in an app's YAML,
// signal it relies on argocd-vault-plugin secret resolution at sync time:
// a direct reference to the plugin name, one of AVP's own placeholder
// syntaxes (<path:...>, <vault:...>, <aws:...>, <gcp:...>), or its
// Kustomize/Helm plugin annotation.
var avpIndicators = []string{
	"argocd-vault-plugin",
	"<path:",
	"<vault:",
	"<aws:",
	"<gcp:",
	"avp.kubernetes.io",
}

// hasAVPIndicators reports whether content contains any AVP indicator - see
// avpIndicators.
func hasAVPIndicators(content string) bool {
	for _, ind := range avpIndicators {
		if strings.Contains(content, ind) {
			return true
		}
	}
	return false
}

// AppHasAVPIndicators walks every YAML file under app (base/, overlays/,
// components/, ...) and reports whether any contains an AVP indicator (see
// hasAVPIndicators). AVP substitution runs against each rendered overlay,
// so a placeholder anywhere in an overlay's reference chain - not just its
// own overlay directory - means the app needs an AVP strategy; scanning the
// whole app tree once per app (rather than per overlay) reflects that.
func AppHasAVPIndicators(app string) bool {
	appFS := os.DirFS(app)
	found := false
	// WalkDir errors are non-fatal: an unreadable entry simply can't match.
	_ = fs.WalkDir(appFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries, keep walking
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			return nil
		}
		data, readErr := fs.ReadFile(appFS, path)
		if readErr == nil && hasAVPIndicators(string(data)) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
