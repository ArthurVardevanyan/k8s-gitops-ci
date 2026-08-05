package validator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// complianceAttributionCtx carries the per-PR state needed to decide whether a
// rendered finding is blocking (the resource's source file was directly changed
// in this PR) or an indirect warning (pre-existing, surfaced for visibility).
// Built once in runBuildAndPostBuild and threaded into the section composer.
type complianceAttributionCtx struct {
	changedKeys    map[string][]string        // "Kind/Name" → source files that define it
	directOverlays map[string]bool            // "app/cluster" of overlays with directly-modified files
	baseApps       map[string]bool            // apps whose base/component dir was changed
	overlaysByDir  map[string]map[string]bool // changed dir → set of overlay keys it feeds
}

// kindNameKey is the standard resource identity key for a finding.
func kindNameKey(f check.Finding) string { return f.Kind + "/" + f.Name }

// changedResourceKeys parses every non-kustomization changed YAML file (raw
// source, not rendered output) and maps each declared resource (Kind/Name) to
// the files that define it. A PR's change to a resource makes findings on that
// resource blocking; unchanged resources from the same base are warnings only.
func changedResourceKeys(changedFiles []string) map[string][]string {
	keys := make(map[string][]string)
	for _, f := range changedFiles {
		if !strings.HasSuffix(f, ".yaml") && !strings.HasSuffix(f, ".yml") {
			continue
		}
		if base := filepath.Base(f); base == "kustomization.yaml" || base == "kustomization.yml" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var doc struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Name string `yaml:"name"`
				} `yaml:"metadata"`
			}
			err := decoder.Decode(&doc)
			if err != nil {
				break
			}
			if doc.Kind != "" && doc.Metadata.Name != "" {
				key := doc.Kind + "/" + doc.Metadata.Name
				keys[key] = append(keys[key], f)
			}
		}
	}
	return keys
}

// directlyChangedOverlays returns the set of overlay keys (app/cluster) whose
// overlay directory contains a directly-changed file - i.e. the PR modified a
// resource IN this overlay's own dir, not just a shared base/component.
func directlyChangedOverlays(changedFiles []string) map[string]bool {
	direct := make(map[string]bool)
	for _, f := range changedFiles {
		parts := strings.Split(filepath.ToSlash(f), "/")
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "overlays" && i+1 < len(parts) && i > 0 {
				app := parts[0]
				cluster := parts[i+1]
				direct[app+"/"+cluster] = true
				break
			}
		}
	}
	return direct
}

// appsWithBaseChanges returns apps whose changed files are outside overlays/
// (base/, components/, or top-level app files). Base/component changes flow
// into every overlay and can make base-derived findings indirect warnings.
func appsWithBaseChanges(changedFiles []string) map[string]bool {
	apps := make(map[string]bool)
	for _, f := range changedFiles {
		parts := strings.SplitN(filepath.ToSlash(f), "/", 3)
		if len(parts) < 2 {
			continue
		}
		app := parts[0]
		if !strings.Contains(f, "/overlays/") {
			apps[app] = true
		}
	}
	return apps
}

// isFileInOverlay reports whether filePath lives inside the given overlay
// (app/overlays/<cluster>). Used by isResourceAffected to confirm that a
// finding's source file from changedResourceKeys actually feeds this overlay.
func isFileInOverlay(filePath, app, cluster string) bool {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "overlays" && i > 0 && i+1 < len(parts) {
			if parts[0] == app || (i >= 2 && parts[i-2] == "templates" && parts[i-1] == app) {
				return parts[i+1] == cluster
			}
		}
	}
	return false
}

// isResourceAffected returns true when resourceKey's source files include one
// that feeds this overlay's build chain. For a directly-changed overlay (the PR
// touched files under overlays/<cluster>), it requires the defining source file
// is actually inside that overlay (not just a shared base). For base/component
// changes, it checks overlaysByDir scoping (which kustomization ref chain
// resolution, via overlay.RefsChangedDir, provides).
func isResourceAffected(resourceKey string, ctx *complianceAttributionCtx, overlayPath string) bool {
	app := appFromOverlayPath(overlayPath)
	cluster := filepath.Base(overlayPath)
	key := app + "/" + cluster

	// Direct overlay change: the resource must be defined by a file under this
	// specific overlay dir to be blocking.
	if ctx.directOverlays[key] {
		sourceFiles := ctx.changedKeys[resourceKey]
		for _, sf := range sourceFiles {
			if isFileInOverlay(sf, app, cluster) {
				return true
			}
		}
		return false
	}

	// Base/component change: check if a changed dir feeds this overlay.
	sourceFiles := ctx.changedKeys[resourceKey]
	for _, sf := range sourceFiles {
		dir := filepath.Dir(sf)
		if overlays, ok := ctx.overlaysByDir[dir]; ok && overlays[key] {
			return true
		}
	}
	return false
}

// buildAttributionCtx constructs the complianceAttributionCtx from the PR's
// changed files and the overlays detected for them. Reuses the existing
// overlay.RefsChangedDir and overlay.FilterOverlaysByRefs for kustomization
// ref-chain scoping. Called once in runBuildAndPostBuild.
func buildAttributionCtx(changedFiles, apps []string) *complianceAttributionCtx {
	baseApps := appsWithBaseChanges(changedFiles)
	ctx := &complianceAttributionCtx{
		changedKeys:    changedResourceKeys(changedFiles),
		directOverlays: directlyChangedOverlays(changedFiles),
		baseApps:       baseApps,
		overlaysByDir:  overlayDirsByChangedPaths(changedFiles, baseApps, apps),
	}
	return ctx
}

// overlayDirsByChangedPaths maps each changed directory (outside overlays/)
// to the set of overlay keys that reference it via kustomize ref chains.
// Only populated when there are base/component changes (the overlay-scoping
// signal for pre-existing findings). Uses overlay.RefsChangedDir for the
// actual kustomization ref-chain traversal, shared with the overlay-build
// discovery path.
func overlayDirsByChangedPaths(changedFiles []string, baseApps map[string]bool, apps []string) map[string]map[string]bool {
	if len(baseApps) == 0 {
		return nil
	}

	targetDirs := changedComponentDirsOf(changedFiles, baseApps)
	if len(targetDirs) == 0 {
		return nil
	}

	result := make(map[string]map[string]bool)
	for _, app := range apps {
		if !baseApps[app] {
			continue
		}
		overlaysDir := filepath.Join(app, "overlays")
		entries, err := os.ReadDir(overlaysDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			cluster := entry.Name()
			overlayDir := filepath.Join(overlaysDir, cluster)
			overlayKey := app + "/" + cluster

			if overlay.RefsChangedDir(overlayDir, targetDirs) {
				if result[overlayDir] == nil {
					result[overlayDir] = make(map[string]bool)
				}
				result[overlayDir][overlayKey] = true
			}
		}
	}
	return result
}

// changedComponentDirsOf returns the unique directory paths (non-overlays/)
// that were changed across the given base/apps. Used by overlayDirsByChangedPaths
// to scope which component/base dirs to check against each overlay's ref chain.
func changedComponentDirsOf(changedFiles []string, baseApps map[string]bool) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, f := range changedFiles {
		parts := strings.SplitN(filepath.ToSlash(f), "/", 3)
		if len(parts) < 2 {
			continue
		}
		app := parts[0]
		if !baseApps[app] {
			continue
		}
		if strings.Contains(f, "/overlays/") {
			continue
		}
		d := filepath.Dir(f)
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}
