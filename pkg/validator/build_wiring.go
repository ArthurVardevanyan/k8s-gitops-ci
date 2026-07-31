package validator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/ghostpatch"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

// appFromOverlayPath extracts the app root from an overlay path detected by
// detectOverlays (e.g. "apps/myapp/overlays/mycluster" -> "apps/myapp").
func appFromOverlayPath(ovPath string) string {
	slash := filepath.ToSlash(ovPath)
	if idx := strings.Index(slash, "/overlays/"); idx >= 0 {
		return filepath.FromSlash(slash[:idx])
	}
	return ovPath
}

// uniqueApps returns the deduplicated, sorted set of app roots referenced by
// the given overlays.
func uniqueApps(overlays []overlayRef) []string {
	seen := map[string]bool{}
	var apps []string
	for _, ov := range overlays {
		app := appFromOverlayPath(ov.path)
		if !seen[app] {
			seen[app] = true
			apps = append(apps, app)
		}
	}
	sort.Strings(apps)
	return apps
}

// buildOverlayError builds this repo's real overlay-build error format
// ("kustomize build <overlay>: <cause>", matching pkg/overlay/overlay.go's
// own fmt.Errorf("kustomize build %s: %w", ...) - see comments.go's
// groupBuildErrors) by rendering the overlay via overlay.RenderKustomize,
// the same mechanism this repo's kubeconform-over-rendered-overlays path
// already uses (pkg/validator/kubeconform_overlay.go). Returns "" when the
// overlay builds successfully.
func buildOverlayError(overlayPath string) string {
	if _, err := overlay.RenderKustomize(overlayPath); err != nil {
		return fmt.Sprintf("kustomize build %s: %s", overlayPath, err)
	}
	return ""
}

// hookCell renders a single hooks-matrix table cell: "—" when the hook
// isn't defined, "✅ defined" when it is. (This repo doesn't execute hooks
// as part of validation - see the doc comment on buildHookTable - so this
// can't distinguish "ran" from "defined" the way the reference
// implementation's richer hookCell(defined, failed) does.)
func hookCell(defined bool) string {
	if defined {
		return "✅ defined"
	}
	return "—"
}

// buildHookTable renders a "| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |"
// markdown table from each app's test.sh (via hook.FindTestScript/
// ParseTestScript), showing which hooks are defined. Returns "" when no app
// defines any hook, so the caller can render a plain "no hooks defined"
// line instead of an empty table.
//
// This only reports whether a hook is *defined*, not whether it *ran and
// passed* - actually executing arbitrary per-app hook scripts during
// validation is a larger, riskier change (real side effects, needs a
// sandboxed working tree) intentionally left out of this pass; pkg/hook.Runner
// already exists for that and can be wired in a follow-up once that's
// scoped.
func buildHookTable(apps []string) string {
	rows := make([]string, 0, len(apps))
	for _, app := range apps {
		cfg, err := hook.ParseTestScript(hook.FindTestScript(app))
		if err != nil || cfg == nil {
			continue
		}
		if !cfg.HasPreBuild && !cfg.HasPostBuild && !cfg.HasPostValidate {
			continue
		}
		rows = append(rows, fmt.Sprintf("| `%s` | %s | %s | %s |",
			app, hookCell(cfg.HasPreBuild), hookCell(cfg.HasPostBuild), hookCell(cfg.HasPostValidate)))
	}
	if len(rows) == 0 {
		return ""
	}
	header := "| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |\n| --- | --- | --- | --- |"
	return header + "\n" + strings.Join(rows, "\n") + "\n"
}

// buildGhostTable renders a "| Overlay | Target |" markdown table of ghost
// patches (kustomize patches targeting a resource absent from the rendered
// base) detected across the given apps via pkg/ghostpatch.CheckApp. Returns
// "" when no ghost patches are found, so the caller can render a plain
// "none detected" line instead of an empty table.
func buildGhostTable(apps []string) string {
	var rows []string
	for _, app := range apps {
		results, err := ghostpatch.CheckApp(app)
		if err != nil {
			continue
		}
		for _, r := range results {
			for _, g := range r.Ghosts {
				rows = append(rows, fmt.Sprintf("| `%s` | %s |", r.Overlay, g.Target.String()))
			}
		}
	}
	if len(rows) == 0 {
		return ""
	}
	header := "| Overlay | Target |\n| --- | --- |"
	return header + "\n" + strings.Join(rows, "\n") + "\n"
}
