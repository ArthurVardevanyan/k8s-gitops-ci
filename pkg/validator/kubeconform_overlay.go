package validator

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
)

// kubeconformSchemaOpts returns kubeconform options configured against the
// pre-extracted schema directory when one is available (validator.Options
// mirrors pkg/pipeline's Setup-phase prefetch), otherwise falling back to a
// lazy per-call schema extraction exactly as the standalone paths do. The
// returned options never carry a cleanup that outlives the caller; the lazy
// path's cleanup is the caller's responsibility.
func kubeconformSchemaOpts(opts Options) (kcOpts kubeconform.Options, cleanup func()) {
	kcOpts = kubeconform.DefaultOptions()
	cleanup = func() {}
	if opts.SchemaDir != "" {
		kcOpts.SchemaDir = opts.SchemaDir
		return kcOpts, cleanup
	}
	schemaDir, c, err := kubeconform.ExtractSchemas()
	if err != nil {
		return kcOpts, cleanup
	}
	// ExtractSchemas is an overridable seam (orgs swap it for their own schema
	// source), so its cleanup must not be assumed non-nil - a nil override
	// would panic at a caller's `defer cleanup()`. Substituting a no-op keeps
	// the seam safe.
	if c == nil {
		c = func() {}
	}
	kcOpts.SchemaDir = schemaDir
	return kcOpts, c
}

// validateRenderedOverlays runs kubeconform against the fully-rendered overlay
// output produced by the Build YAML phase (buildOverlayWithHooks ->
// overlay.RenderWithStrategy). Those bytes are already strategy-aware,
// AVP-resolved, and Helm-inclusive - exactly the manifests that will deploy -
// so this validates what actually ships rather than raw, pre-build source.
// Validation runs in parallel across overlays (bounded by workers). Any overlay
// already failed to build and simply won't appear in rendered; it was already
// reported as a build error, so there is nothing to validate here.
func validateRenderedOverlays(rendered []renderedOverlay, kcOpts kubeconform.Options, workers int) *kubeconform.Result {
	combined := &kubeconform.Result{}
	if len(rendered) == 0 {
		return combined
	}
	if workers > len(rendered) {
		workers = len(rendered)
	}
	if workers < 1 {
		workers = 1
	}

	var mu sync.Mutex
	jobs := make(chan renderedOverlay, len(rendered))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ro := range jobs {
				r, err := kubeconform.ValidateBytes(filepath.Join(ro.overlay, "_kustomize-build.yaml"), ro.data, kcOpts)
				mu.Lock()
				if err != nil {
					// A ValidateBytes error is a genuine setup/construction
					// failure (e.g. invalid schema locations) for this overlay,
					// not "no findings". Surfacing it as a result error makes
					// the rendered kubeconform section fail loudly instead of
					// silently dropping the overlay and appearing to pass.
					combined.Errors++
					combined.Details = append(combined.Details, kubeconform.FileResult{
						Filename: filepath.Join(ro.overlay, "_kustomize-build.yaml"),
						Status:   "error",
						Errors:   []string{err.Error()},
					})
				} else {
					combined.Merge(r)
				}
				mu.Unlock()
			}
		}()
	}
	for _, ro := range rendered {
		jobs <- ro
	}
	close(jobs)
	wg.Wait()
	return combined
}

// coverByScopedOverlays returns the set of changed files that participate in
// the build chain of at least one scoped overlay, computed pre-build from the
// overlay paths alone (no rendered bytes needed). Such files are validated from
// the rendered output (the post-build validateRenderedOverlays pass) rather than
// raw source, so a changed manifest inside an overlay is not schema-checked
// twice (raw source with unresolved AVP placeholders + authoritative rendered).
func coverByScopedOverlays(scoped []overlayRef, files []string) map[string]bool {
	if len(scoped) == 0 || len(files) == 0 {
		return nil
	}
	covered := make(map[string]bool, len(files))
	for _, f := range files {
		clean := filepath.Clean(f)
		for _, ov := range scoped {
			app := appFromOverlayPath(ov.path)
			if isOverlayRelatedToChangedFiles(app, ov.cluster, []string{f}) {
				covered[clean] = true
				break
			}
		}
	}
	return covered
}

// filesNotCovered returns the subset of files not in the covered set.
func filesNotCovered(files []string, covered map[string]bool) []string {
	if len(covered) == 0 {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		if !covered[filepath.Clean(f)] {
			out = append(out, f)
		}
	}
	return out
}

// isUnderAnyRoot reports whether path is equal to or nested under one of
// roots.
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
