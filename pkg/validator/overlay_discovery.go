package validator

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

// ExtraNonAppDirs names repository path prefixes that must never be treated
// as a kustomize "app root", even if one happens to contain a base/,
// components/, or overlays/ subdirectory (e.g. a vendored example,
// test-fixture, or internal-tooling directory checked into the repo whose
// layout coincidentally matches an app's shape). Each key is matched on
// path-segment boundaries: a key equals the file's first segment, equals
// the whole (slash-normalized) file path, or is a directory prefix of it
// (key + "/"). This lets a single-segment key like "vendor" exclude a
// top-level dir while a multi-segment key like ".scafctl/templates"
// excludes a nested subtree. Empty by default - the generic core has
// nothing to guard against; an org layer may populate it from a
// Configure()-style seam.
var ExtraNonAppDirs = map[string]bool{}

// isExtraNonAppPath reports whether f falls under any ExtraNonAppDirs prefix
// (segment-boundary aware, slash-normalized) or is a scaffold template file.
// Scaffold templates (<ScaffoldDir>/templates/, e.g. .scafctl/templates/)
// are Go-templated source that renders *into* real overlays - they are not
// themselves buildable kustomize overlays (they frequently lack a
// kustomization.yaml at the rendered-overlay path and contain unresolved
// {{ ... }} actions), so they must be excluded from app-root/overlay-build
// discovery just as they already are from every manifest/YAML validation
// path (see convention.IsScaffoldTemplate).
func isExtraNonAppPath(f string) bool {
	if convention.IsScaffoldTemplate(f) {
		return true
	}
	slash := filepath.ToSlash(f)
	for prefix := range ExtraNonAppDirs {
		p := filepath.ToSlash(prefix)
		if slash == p || strings.HasPrefix(slash, p+"/") {
			return true
		}
	}
	return false
}

// detectAppRoots scans files for kustomize "app root" directories - i.e.
// directories that directly contain a base/, components/, or overlays/
// subdirectory - and returns the deduplicated subset that actually have at
// least one buildable overlay under <root>/overlays. Rendering these
// overlays (rather than schema-validating their raw, pre-build source
// fragments) avoids false positives for resources that are only completed
// via a Kustomize patch/component (e.g. a base WasmPlugin missing its
// `spec.url`, later supplied by a patch component) or that are never
// resources at all (e.g. files consumed only as secretGenerator/
// configMapGenerator data). A file matched by ExtraNonAppDirs (or a
// scaffold template) is skipped entirely, regardless of its shape.
//
// This is the shared app-root/overlay discovery used across the pipeline -
// by the Overlay Build step (detectOverlaysForChanges in build_wiring.go),
// the kubeconform raw-pass overlay exclusion and its change-scoped overlay set
// (kubeconform_overlay.go), non-app checks (nonapp_wiring.go), and
// --cluster/--app targeting (target_wiring.go) - not just kubeconform.
func detectAppRoots(files []string) []string {
	seen := map[string]bool{}
	roots := make([]string, 0, len(files))
	for _, f := range files {
		if isExtraNonAppPath(f) {
			continue
		}
		parts := strings.Split(filepath.ToSlash(f), "/")
		for i, p := range parts {
			if i == 0 {
				continue
			}
			if p == "base" || p == "components" || p == "overlays" {
				// strings.Join (not filepath.Join, which drops leading empty
				// path components) so an absolute input path's leading "/"
				// is preserved - parts[0] is "" for an absolute unix path,
				// and filepath.Join silently discards leading empty
				// elements, turning "/tmp/foo/app1" into the relative
				// "tmp/foo/app1" and breaking every filesystem check
				// (overlay.FindAllOverlays, os.Stat, ...) done against root
				// afterwards when the caller passes absolute paths (e.g.
				// Options.Dirs-based local runs, as opposed to git-diff's
				// always-repo-relative paths).
				root := filepath.FromSlash(strings.Join(parts[:i], "/"))
				if !seen[root] {
					seen[root] = true
					roots = append(roots, root)
				}
				break
			}
		}
	}
	sort.Strings(roots)

	withOverlays := make([]string, 0, len(roots))
	for _, root := range roots {
		if len(overlay.FindAllOverlays(root)) > 0 {
			withOverlays = append(withOverlays, root)
		}
	}
	return withOverlays
}
