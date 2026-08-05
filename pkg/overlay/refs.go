package overlay

import (
	"path/filepath"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/kustomize"
)

// HasOverlays reports whether app has an overlays/ directory containing at
// least one overlay - the same on-disk signal FindAllOverlays uses to
// discover overlays, exposed as a boolean predicate for app-root detection
// (see validator.detectAppRoots, which walks a changed file's ancestor
// directories looking for the nearest one with an overlays/ dir).
func HasOverlays(app string) bool {
	return len(FindAllOverlays(app)) > 0
}

// dirPair pairs a changed directory's original (as-given) and absolute
// forms, so refsMatchChangedDirs can compare against a kustomization ref
// chain (itself resolved to absolute paths) without repeatedly re-resolving
// the same changed dir for every overlay/ref pair.
type dirPair struct {
	rel, abs string
}

// FilterOverlaysByRefs narrows overlays down to only those whose
// kustomization reference chain (resources/components/bases, resolved
// transitively via kustomize.ResolveRefs) includes at least one of the
// directories changed under app (outside overlays/). This is what lets a
// change to a shared base/component file resolve to just the overlay(s)
// that actually consume it, instead of naively treating every overlay as
// affected - or, in the current (pre-fix) code, none at all, since a bare
// base-file change contains no "overlays/" path segment for the naive
// detector to key off of.
//
// Safety valve: if scoping would produce an empty set (e.g. the only
// changes are to files that don't appear in any kustomization ref chain,
// such as an app-root-level test.sh), the original, unfiltered overlays
// slice is returned - this function only ever narrows the build scope, it
// never silently drops it to zero.
func FilterOverlaysByRefs(app string, overlays, changedFiles []string) []string {
	changedDirs := appChangedDirs(app, changedFiles)
	if len(changedDirs) == 0 {
		return overlays
	}

	pairs := make([]dirPair, 0, len(changedDirs))
	for _, d := range changedDirs {
		if abs, err := filepath.Abs(d); err == nil {
			pairs = append(pairs, dirPair{rel: d, abs: abs})
		}
	}
	if len(pairs) == 0 {
		return overlays
	}

	var filtered []string
	for _, ov := range overlays {
		refs := kustomize.ResolveRefs(ov)
		if refsMatchChangedDirs(refs, pairs) {
			filtered = append(filtered, ov)
		}
	}

	if len(filtered) == 0 {
		return overlays
	}
	return filtered
}

// RefsChangedDir reports whether the overlay rooted at overlayDir has
// a kustomization reference chain (resources/components/bases, resolved
// transitively via kustomize.ResolveRefs) that includes at least one of
// changedDirs. This is the single-overlay form of the scoping
// FilterOverlaysByRefs performs across a set: it lets a caller answer "does
// this one overlay actually consume that changed base/component directory?"
// - notably distinguishing version-partitioned component directories (e.g.
// components/foo/v0.21.0 vs components/foo/v0.19.1), so a change to one
// version only relates to the overlays that reference that version.
//
// changedDirs entries are directories (as given, working-dir-relative);
// non-resolvable entries are skipped. Returns false when changedDirs is
// empty or none resolve.
func RefsChangedDir(overlayDir string, changedDirs []string) bool {
	if len(changedDirs) == 0 {
		return false
	}
	pairs := make([]dirPair, 0, len(changedDirs))
	for _, d := range changedDirs {
		if abs, err := filepath.Abs(d); err == nil {
			pairs = append(pairs, dirPair{rel: d, abs: abs})
		}
	}
	if len(pairs) == 0 {
		return false
	}
	return refsMatchChangedDirs(kustomize.ResolveRefs(overlayDir), pairs)
}

// appChangedDirs extracts the set of directories (not overlays/, not the
// app root itself) that changedFiles touched within app. These are the
// candidate "changed base/component directories" FilterOverlaysByRefs scopes
// each overlay's reference chain against. Files directly at the app root
// (e.g. "myapp/test.sh") are excluded since they can never appear inside a
// kustomization resources/components/bases entry.
func appChangedDirs(app string, changedFiles []string) []string {
	seen := map[string]bool{}
	var dirs []string
	prefix := app + "/"
	for _, f := range changedFiles {
		f = filepath.ToSlash(f)
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		if strings.Contains(f, "/overlays/") {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(f))
		if dir == app {
			continue
		}
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// refsMatchChangedDirs reports whether any resolved kustomization ref
// matches one of the changed directories, using bidirectional prefix
// matching: a ref that is an ancestor of a changed dir, or a descendant of
// one, counts as a match. This handles both "the overlay refs a whole
// components/foo directory and only a subpath under it changed" and "the
// overlay refs a specific subpath and a shared parent directory changed".
func refsMatchChangedDirs(refs []string, pairs []dirPair) bool {
	for _, ref := range refs {
		absRef, err := filepath.Abs(ref)
		if err != nil {
			continue
		}
		for _, dp := range pairs {
			if absRef == dp.abs ||
				strings.HasPrefix(dp.abs, absRef+string(filepath.Separator)) ||
				strings.HasPrefix(absRef, dp.abs+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}
