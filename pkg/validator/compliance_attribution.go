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

// appRootOf derives the app root (the directory whose overlays/<cluster> tree an
// overlay lives under) from a changed file path, matching appFromOverlayPath's
// prefix-before-"/overlays/" convention. This deliberately does NOT assume the
// app root is the first path segment: repos frequently nest apps a directory or
// more deep (e.g. "kubernetes/<app>/overlays/<cluster>"), so hardcoding
// parts[0] mis-attributes every finding for those layouts.
//
// For an overlay file (…/overlays/<cluster>/…) it returns the prefix before
// "/overlays/". For a base/component/top-level app file it returns the prefix
// before the first "/base/" or "/components/" segment; failing that (a
// top-level app file with no base/component dir) it returns the file's parent
// directory. cluster is the segment immediately after "overlays" for an overlay
// file, or "" otherwise.
func appRootOf(filePath string) (app, cluster string) {
	slash := filepath.ToSlash(filePath)
	if idx := strings.Index(slash, "/overlays/"); idx >= 0 {
		app = slash[:idx]
		rest := strings.TrimPrefix(slash[idx:], "/overlays/")
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			cluster = rest[:i]
		} else {
			cluster = rest
		}
		return filepath.FromSlash(app), cluster
	}
	for _, marker := range []string{"/base/", "/components/"} {
		if idx := strings.Index(slash, marker); idx >= 0 {
			return filepath.FromSlash(slash[:idx]), ""
		}
	}
	return filepath.FromSlash(filepath.Dir(slash)), ""
}

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
		if !strings.Contains(filepath.ToSlash(f), "/overlays/") {
			continue
		}
		app, cluster := appRootOf(f)
		if app == "" || cluster == "" {
			continue
		}
		direct[filepath.ToSlash(app)+"/"+cluster] = true
	}
	return direct
}

// appsWithBaseChanges returns apps whose changed files are outside overlays/
// (base/, components/, or top-level app files). Base/component changes flow
// into every overlay and can make base-derived findings indirect warnings.
func appsWithBaseChanges(changedFiles []string) map[string]bool {
	apps := make(map[string]bool)
	for _, f := range changedFiles {
		if strings.Contains(filepath.ToSlash(f), "/overlays/") {
			continue
		}
		app, _ := appRootOf(f)
		if app != "" {
			apps[filepath.ToSlash(app)] = true
		}
	}
	return apps
}

// isFileInOverlay reports whether filePath lives inside the given overlay
// (app/overlays/<cluster>). Used by isResourceAffected to confirm that a
// finding's source file from changedResourceKeys actually feeds this overlay.
func isFileInOverlay(filePath, app, cluster string) bool {
	fileApp, fileCluster := appRootOf(filePath)
	if fileCluster == "" || fileCluster != cluster {
		return false
	}
	fileApp = filepath.ToSlash(fileApp)
	app = filepath.ToSlash(app)
	if fileApp == app {
		return true
	}
	// Scaffold-template overlays live under templates/<app>/overlays/<cluster>:
	// the resource's file app root ends with the overlay's app root
	// (…/templates/<app> vs <app>). Match on that suffix boundary so a
	// templated overlay's own resources are still attributed to it.
	return strings.HasSuffix(fileApp, "/templates/"+app)
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
	key := filepath.ToSlash(app) + "/" + cluster

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
		if !baseApps[filepath.ToSlash(app)] {
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
			overlayKey := filepath.ToSlash(app) + "/" + cluster

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
		if strings.Contains(filepath.ToSlash(f), "/overlays/") {
			continue
		}
		app, _ := appRootOf(f)
		if app == "" || !baseApps[filepath.ToSlash(app)] {
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
