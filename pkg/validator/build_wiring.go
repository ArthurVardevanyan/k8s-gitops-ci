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

// detectOverlaysForChanges maps a PR's changed files to the overlays that
// actually need building/checking. Unlike the old naive "any path segment
// literally named overlays/" heuristic, this is app-aware: it finds each
// touched app root (detectAppRoots, defined in overlay_discovery.go and
// already shared with the kubeconform-over-rendered-overlays path), asks
// overlay.GetOverlaysToTest which overlays that app's changes imply
// (cluster-specific vs. a base/component change that could affect every
// overlay), and - for base/component changes spanning more than one
// overlay - narrows that down further via overlay.FilterOverlaysByRefs,
// which actually parses each overlay's kustomization reference chain to see
// whether it depends on the changed directory at all. This is what allows a
// change to a shared base file (with no "overlays/" segment anywhere in its
// path) to still resolve to the correct overlay(s), instead of silently
// producing zero overlays.
func detectOverlaysForChanges(changed []string) []overlayRef {
	apps := detectAppRoots(changed)
	seen := map[string]bool{}
	var refs []overlayRef
	for _, app := range apps {
		overlays, _, trigger := overlay.GetOverlaysToTest(app, changed, false)
		if (trigger == "base" || trigger == "component") && len(overlays) > 1 {
			overlays = overlay.FilterOverlaysByRefs(app, overlays, changed)
		}
		for _, ov := range overlays {
			ov = filepath.ToSlash(ov)
			if seen[ov] {
				continue
			}
			seen[ov] = true
			refs = append(refs, overlayRef{path: ov, cluster: filepath.Base(ov)})
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].path < refs[j].path })
	return refs
}

// appFromOverlayPath extracts the app root from an overlay path detected by
// detectOverlaysForChanges (e.g. "apps/myapp/overlays/mycluster" -> "apps/myapp").
func appFromOverlayPath(ovPath string) string {
	slash := filepath.ToSlash(ovPath)
	if idx := strings.Index(slash, "/overlays/"); idx >= 0 {
		return filepath.FromSlash(slash[:idx])
	}
	return ovPath
}

// filesCoveredByRenderedOverlays returns the subset of files (cleaned) that
// participate in the build chain of at least one successfully-rendered
// overlay - i.e. files whose render-sensitive verdict is decided by the
// rendered pass (runDocChecksRendered) and should therefore be skipped by
// the raw pass's render-sensitive tier. A file is covered when it lives
// under an overlay's own overlays/<cluster> dir, its app's base/, or a
// component that overlay's kustomization chain references (the same app-aware
// relatedness the scaffold-drift scoping uses). Files not covered here (e.g.
// a brand-new component not yet wired into any kustomization.yaml) still get
// their render-sensitive checks via the raw fallback, so nothing is skipped.
func filesCoveredByRenderedOverlays(outputs []renderedOverlay, files []string) map[string]bool {
	if len(outputs) == 0 || len(files) == 0 {
		return nil
	}
	covered := make(map[string]bool, len(files))
	for _, f := range files {
		clean := filepath.Clean(f)
		for _, o := range outputs {
			app := appFromOverlayPath(o.overlay)
			cluster := filepath.Base(o.overlay)
			if isOverlayRelatedToChangedFiles(app, cluster, []string{f}) {
				covered[clean] = true
				break
			}
		}
	}
	return covered
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
// isn't defined for the app, "✅ ran" when it ran and passed, "❌ failed"
// when it ran and at least one invocation failed (see mergeHookOutcome -
// a partial failure across an app's several overlays still reports ❌).
func hookCell(outcome hookOutcome) string {
	switch outcome {
	case hookFailed:
		return "❌ failed"
	case hookRan:
		return "✅ ran"
	default:
		return "—"
	}
}

// buildHookTable renders a "| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |"
// markdown table showing, per app, whether each hook is defined and - since
// hooks are actually executed as part of the build (see
// buildOverlayWithHooks/runAppPostValidateHooks in hook_wiring.go) - whether
// it ran successfully. results holds the outcomes accumulated during that
// run (keyed by app, see runBuildAndPostBuild); an app missing from results
// (or with no hooks defined at all in cfgs) falls back to "—" for every
// column. Returns "" when no app defines any hook, so the caller can render
// a plain "no hooks defined" line instead of an empty table.
func buildHookTable(apps []string, cfgs map[string]*hook.Config, results map[string]*appHookResult) string {
	rows := make([]string, 0, len(apps))
	for _, app := range apps {
		cfg := cfgs[app]
		if cfg == nil || (!cfg.HasPreBuild && !cfg.HasPostBuild && !cfg.HasPostValidate) {
			continue
		}
		r := results[app]
		if r == nil {
			r = &appHookResult{}
		}
		rows = append(rows, fmt.Sprintf("| `%s` | %s | %s | %s |",
			app, hookCell(r.PreBuild), hookCell(r.PostBuild), hookCell(r.PostValidate)))
	}
	if len(rows) == 0 {
		return ""
	}
	header := "| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |\n| --- | --- | --- | --- |"
	return header + "\n" + strings.Join(rows, "\n") + "\n"
}

// buildGhostTable renders a "| Overlay | Target | |" markdown table of
// ghost patches (kustomize patches targeting a resource absent from the
// rendered base) detected across the given apps via
// pkg/ghostpatch.ClassifyApp, and separately returns the blocking subset
// (a ghost patch on a kustomization.yaml that isn't itself newly-added in
// this PR and whose patches section did change) so the caller can fold it
// into the overall pass/fail decision - a ghost patch predating this PR,
// or introduced by a brand-new overlay, is surfaced for visibility only.
// Returns table == "" when no ghost patches are found at all, so the
// caller can render a plain "none detected" line instead of an empty
// table.
func buildGhostTable(apps, addedFiles []string) (table string, blockingCount int) {
	var rows []string
	for _, app := range apps {
		results, err := ghostpatch.ClassifyApp(app, addedFiles)
		if err != nil {
			continue
		}
		for _, r := range results {
			for _, g := range r.Ghosts {
				marker := ""
				if g.Blocking {
					marker = " 🚫"
					blockingCount++
				}
				rows = append(rows, fmt.Sprintf("| `%s` | %s |%s", r.Overlay, g.Target.String(), marker))
			}
		}
	}
	if len(rows) == 0 {
		return "", 0
	}
	header := "| Overlay | Target |\n| --- | --- |"
	return header + "\n" + strings.Join(rows, "\n") + "\n", blockingCount
}
