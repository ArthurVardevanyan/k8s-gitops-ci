package validator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/changeset"
)

// resolveTargetOverlays expands Options.Apps/Options.Clusters into a
// synthetic "changed files" list covering the targeted app(s)/cluster(s)'
// overlay and base directories, for ad-hoc targeted validation (the
// build-yaml CLI command's --app/--cluster flags, and equally usable from
// pipeline/test-all/scan-all - see docs/CI.md's "Ad-hoc overlay build"
// row) - independent of git history/diffing entirely, unlike every other
// resolveChangeset path.
//
//   - Both Apps and Clusters given: every (app, cluster) combination whose
//     overlays/<cluster> directory actually exists on disk, plus that
//     app's base/ - silently skipping combinations that don't exist on
//     disk rather than erroring, so a typo'd cluster name for one app in
//     a multi-app/-cluster invocation doesn't abort the whole run. An app
//     with none of the given clusters found is skipped entirely (base/
//     alone, with no matching overlay, would be a misleading partial
//     scope).
//   - Apps only (no Clusters): each app's entire directory (base + every
//     overlay), since there's no cluster to narrow to.
//   - Clusters only (no Apps): every app discovered in the repo (via
//     changeset.GetAllFiles + detectAppRoots) that has an
//     overlays/<cluster> directory for one of the given cluster names.
//
// Returns an error only if nothing in Apps/Clusters resolves to any
// directory on disk at all (every entry was a typo, or no app in the repo
// has a matching cluster) - a completely empty, silently-passing run would
// be more surprising than a hard failure here.
func resolveTargetOverlays(opts Options) ([]string, error) {
	apps := opts.Apps
	if len(apps) == 0 {
		discovered, err := discoverAppRoots()
		if err != nil {
			return nil, fmt.Errorf("discovering app roots for --cluster targeting: %w", err)
		}
		apps = discovered
	}

	dirs := make([]string, 0, len(apps))
	for _, app := range apps {
		if _, err := os.Stat(app); err != nil {
			continue // typo'd/nonexistent app - skip, don't abort the run
		}
		if len(opts.Clusters) == 0 {
			dirs = append(dirs, app)
			continue
		}
		var overlayDirs []string
		for _, cluster := range opts.Clusters {
			ov := filepath.Join(app, "overlays", cluster)
			if _, err := os.Stat(ov); err == nil {
				overlayDirs = append(overlayDirs, ov)
			}
		}
		if len(overlayDirs) == 0 {
			continue // none of the requested clusters exist for this app
		}
		dirs = append(dirs, filepath.Join(app, "base"))
		dirs = append(dirs, overlayDirs...)
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("no matching app/cluster overlay found for apps=%v clusters=%v", opts.Apps, opts.Clusters)
	}
	return changeset.GetFilesUnderDirs(dirs)
}

// discoverAppRoots finds every kustomize "app root" directory in the
// repository (see detectAppRoots), for the --cluster-only targeting case
// where no --app was given to narrow the search.
func discoverAppRoots() ([]string, error) {
	allFiles, err := changeset.GetAllFiles()
	if err != nil {
		return nil, err
	}
	return detectAppRoots(allFiles), nil
}
