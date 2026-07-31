package validator

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

// ExtraNonAppDirs names top-level repository directories that must never be
// treated as a kustomize "app root", even if one happens to contain a
// base/, components/, or overlays/ subdirectory (e.g. a vendored example,
// test-fixture, or internal-tooling directory checked into the repo whose
// layout coincidentally matches an app's shape). Empty by default - the
// generic core has nothing to guard against; an org layer may populate it
// from a Configure()-style seam.
var ExtraNonAppDirs = map[string]bool{}

// detectAppRoots scans files for kustomize "app root" directories - i.e.
// directories that directly contain a base/, components/, or overlays/
// subdirectory - and returns the deduplicated subset that actually have at
// least one buildable overlay under <root>/overlays. Rendering these
// overlays (rather than schema-validating their raw, pre-build source
// fragments) avoids false positives for resources that are only completed
// via a Kustomize patch/component (e.g. a base WasmPlugin missing its
// `spec.url`, later supplied by a patch component) or that are never
// resources at all (e.g. files consumed only as secretGenerator/
// configMapGenerator data). A file whose top-level directory is listed in
// ExtraNonAppDirs is skipped entirely, regardless of its shape.
func detectAppRoots(files []string) []string {
	seen := map[string]bool{}
	roots := make([]string, 0, len(files))
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		if len(parts) > 0 && ExtraNonAppDirs[parts[0]] {
			continue
		}
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

// renderAppOverlays renders every overlay of appRoot and validates the
// combined rendered output with kubeconform. It returns the merged
// validation result and whether every overlay of appRoot built
// successfully. If any overlay fails to build, ok is false and the caller
// should fall back to raw, per-file validation for appRoot so that nothing
// silently goes unchecked.
func renderAppOverlays(appRoot string, opts kubeconform.Options) (res *kubeconform.Result, ok bool) {
	overlays := overlay.FindAllOverlays(appRoot)
	if len(overlays) == 0 {
		return nil, false
	}
	combined := &kubeconform.Result{}
	for _, ov := range overlays {
		out, err := overlay.RenderKustomize(ov)
		if err != nil {
			return nil, false
		}
		name := filepath.Join(ov, "_kustomize-build.yaml")
		r, err := kubeconform.ValidateBytes(name, out, opts)
		if err != nil {
			return nil, false
		}
		combined.Merge(r)
	}
	return combined, true
}

// validateWithRenderedOverlays runs kubeconform against files, but for any
// file that lives under an app root whose overlays all build successfully,
// validates the rendered (kustomize build) manifests for that app instead
// of the raw source file. Files outside any buildable app root (or under an
// app root where at least one overlay fails to build) are still validated
// raw, so coverage is never silently dropped.
func validateWithRenderedOverlays(files []string, opts kubeconform.Options) (*kubeconform.Result, error) {
	appRoots := detectAppRoots(files)

	combined := &kubeconform.Result{}
	coveredRoots := make([]string, 0, len(appRoots))
	for _, root := range appRoots {
		r, ok := renderAppOverlays(root, opts)
		if !ok {
			continue
		}
		combined.Merge(r)
		coveredRoots = append(coveredRoots, root)
	}

	rawFiles := excludeUnderRoots(files, coveredRoots)
	rawRes, err := kubeconform.ValidateFiles(rawFiles, opts)
	if err != nil {
		return nil, err
	}
	combined.Merge(rawRes)
	return combined, nil
}

// excludeUnderRoots drops files that live under any of roots.
func excludeUnderRoots(files, roots []string) []string {
	if len(roots) == 0 {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		if !isUnderAnyRoot(f, roots) {
			out = append(out, f)
		}
	}
	return out
}

func isUnderAnyRoot(f string, roots []string) bool {
	fSlash := filepath.ToSlash(f)
	for _, r := range roots {
		rSlash := filepath.ToSlash(r)
		if fSlash == rSlash || strings.HasPrefix(fSlash, rSlash+"/") {
			return true
		}
	}
	return false
}
