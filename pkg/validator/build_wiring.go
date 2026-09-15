package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

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

// detectAllOverlays finds every overlay in the repository, regardless of
// changeset. Walks all app roots (directories containing base/, components/,
// or overlays/ subdirectories), then collects all overlays under each app
// via FindAllOverlays (the simple overlay enumeration function). This is the
// overlay discovery source for FullScan mode.
func detectAllOverlays() []overlayRef {
	allFiles, err := getAllRepoFiles()
	if err != nil {
		// If the walk fails, fall back to empty (the caller will
		// handle the error appropriately).
		return nil
	}
	apps := detectAppRoots(allFiles)
	seen := map[string]bool{}
	var refs []overlayRef
	for _, app := range apps {
		for _, ov := range overlay.FindAllOverlays(app) {
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
// rendered base) detected across renderedOverlays via
// pkg/ghostpatch.ClassifyRendered, and separately returns the blocking
// subset (a ghost patch on an overlay whose own kustomization.yaml this PR
// changed) so the caller can fold it into the overall pass/fail decision. A
// ghost on an overlay this PR did not touch - pre-existing drift - or
// introduced by a brand-new overlay, is surfaced for visibility only
// (non-blocking).
//
// renderedOverlays is expected to be the Build YAML phase's own per-overlay
// render output (see runBuildAndPostBuild's renderedOverlays var) - i.e.
// only the overlays this run actually built, not every overlay on disk
// under each app. This is deliberate: ghost-patch detection previously
// walked every overlay directory under each app (see git history's
// ClassifyApp) and re-rendered each one via kustomize, which for an app
// with hundreds of overlays and a PR touching only a handful dominated the
// Build YAML phase's wall time for no benefit - a ghost patch on an
// overlay this PR never touched or built isn't this run's concern (see
// ClassifyOverlay's blocking rule, which already only fires for a changed
// overlay's own kustomization.yaml). An overlay that failed to build (and
// so has no entry in renderedOverlays) is simply skipped here too - its
// build failure already surfaces loudly via buildErrs/Overlay Build, and
// there is no rendered YAML to check ghost targets against anyway.
//
// Returns table == "" when no ghost patches are found at all, so the
// caller can render a plain "none detected" line instead of an empty
// table.
func buildGhostTable(renderedOverlays []renderedOverlay, changed, addedFiles []string) (table string, blockingCount int) {
	overlays := make([]ghostpatch.RenderedOverlay, 0, len(renderedOverlays))
	for _, ro := range renderedOverlays {
		overlays = append(overlays, ghostpatch.RenderedOverlay{Path: ro.overlay, YAML: ro.data})
	}
	results, err := ghostpatch.ClassifyRendered(overlays, changed, addedFiles)
	if err != nil {
		return "", 0
	}
	var rows []string
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
	if len(rows) == 0 {
		return "", 0
	}
	header := "| Overlay | Target |\n| --- | --- |"
	return header + "\n" + strings.Join(rows, "\n") + "\n", blockingCount
}

// resourceIdentity identifies a single YAML document by its kind and
// resource name. It is used for matching changed files against the
// rendered output of a kustomize overlay: a changed file is considered
// "covered" by a rendered overlay only when every document in the file
// has an identity match (kind+name) in at least one of the overlay's
// successfully-rendered output documents. Namespace is not compared
// because overlay namespace transforms would break a naive match.
type resourceIdentity struct {
	Kind string
	Name string
}

// resourceIdentitySet is a helper type for fast membership testing.
type resourceIdentitySet map[resourceIdentity]bool

// documentIdentity extracts the kind+name identity from a single YAML
// document, returning the identity and true when the document has a
// recognized apiVersion/kind pair; (zero, false) otherwise.
func documentIdentity(doc []byte) (resourceIdentity, bool) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return resourceIdentity{}, false
	}
	kind, _ := root["kind"].(string)
	if kind == "" {
		return resourceIdentity{}, false
	}
	meta, _ := root["metadata"].(map[string]interface{})
	name, _ := meta["name"].(string)
	if name == "" {
		return resourceIdentity{}, false
	}
	return resourceIdentity{Kind: kind, Name: name}, true
}

// splitDocuments splits data into YAML documents (same logic as
// phases.go:splitDocuments to stay in sync).
func parseDocuments(data []byte) (resourceIdentitySet, bool) {
	ids := make(resourceIdentitySet)
	for _, doc := range splitDocuments(data) {
		ident, ok := documentIdentity(doc)
		if ok {
			ids[ident] = true
		}
	}
	return ids, len(ids) > 0
}

// resourceIdentityFromFile extracts the resource identities (kind+name)
// from a YAML file on disk. It reads the file, splits it into documents,
// and returns the set of identities found. Returns false when the file
// is empty or contains no recognized Kubernetes documents.
func resourceIdentityFromFile(path string) (resourceIdentitySet, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return parseDocuments(data)
}

// filesCoveredByRenderedContent returns the set of files (cleaned) whose
// documents are actually present in the successfully rendered output of
// at least one overlay their changed paths are related to. A file is
// covered only if ALL of its YAML documents (matched on kind+name) appear
// in the rendered output of at least one relevant overlay (an overlay
// whose kustomization chain includes the file via a component/base
// reference).
//
// This is stricter than the path-based proxy used by coverByScopedOverlays,
// which exclude a file from the raw pass just because its path is related
// to an overlay - if the overlay's render does not actually contain the
// file's resources (e.g., a brand-new component whose resources are absent
// from the render), the file falls back to the raw pass so nothing is
// silently skipped.
func filesCoveredByRenderedContent(renderedOverlays []renderedOverlay, files []string) map[string]bool {
	if len(renderedOverlays) == 0 {
		return nil
	}
	covered := make(map[string]bool)
	// For each changed file: parse its identities, find relevant rendered
	// overlays, check if all its identities appear in any relevant render.
	// Use a goroutine pool to avoid unbounded fan-out.
	type work struct {
		filePath   string
		cleanPath  string
		identities resourceIdentitySet
	}
	jobs := make(chan work, len(files))
	type jobResult struct {
		cleanPath string
		covered   bool
	}
	results := make(chan jobResult, len(files))
	var wg sync.WaitGroup

	// Memoize parseDocuments per overlay: many changed files can relate to
	// the same overlay, and re-parsing an identical rendered manifest per
	// file is redundant work. parseDocuments is pure w.r.t. ro.data, so a
	// shared read-mostly cache is safe; guard with RWMutex since the worker
	// pool below reads/writes it concurrently.
	type parsedRender struct {
		ids resourceIdentitySet
		ok  bool
	}
	var renderCacheMu sync.RWMutex
	renderCache := make(map[string]parsedRender, len(renderedOverlays))
	renderedIDsFor := func(ro renderedOverlay) (resourceIdentitySet, bool) {
		renderCacheMu.RLock()
		if v, ok := renderCache[ro.overlay]; ok {
			renderCacheMu.RUnlock()
			return v.ids, v.ok
		}
		renderCacheMu.RUnlock()
		ids, ok := parseDocuments(ro.data)
		renderCacheMu.Lock()
		renderCache[ro.overlay] = parsedRender{ids: ids, ok: ok}
		renderCacheMu.Unlock()
		return ids, ok
	}

	for i := 0; i < 16 && i <= len(files); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Find relevant rendered overlays (those that the file is
				// related to via the overlay's kustomization chain).
				// We only check overlays where this file's identities
				// could plausibly appear (same app+cluster prefix).
				for _, ro := range renderedOverlays {
					app := appFromOverlayPath(ro.overlay)
					cluster := filepath.Base(ro.overlay)
					if isOverlayRelatedToChangedFiles(app, cluster, []string{job.filePath}) {
						renderedIDs, ok := renderedIDsFor(ro)
						if ok && identitiesMatch(job.identities, renderedIDs) {
							results <- jobResult{job.cleanPath, true}
							break
						}
					}
				}
			}
		}()
	}
	for _, f := range files {
		ids, ok := resourceIdentityFromFile(f)
		if !ok {
			continue
		}
		clean := filepath.Clean(f)
		jobs <- work{filePath: f, cleanPath: clean, identities: ids}
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()
	for r := range results {
		if r.covered {
			covered[r.cleanPath] = true
		}
	}
	return covered
}

// identitiesMatch returns true when every identity in want is present in
// have (a superset check). This is used to verify that all documents from
// a changed file are represented in an overlay's rendered output.
func identitiesMatch(want, have resourceIdentitySet) bool {
	for id := range want {
		if !have[id] {
			return false
		}
	}
	return true
}
