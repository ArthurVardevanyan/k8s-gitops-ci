package validator

import (
	"path/filepath"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
)

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
