package validator

import (
	"path/filepath"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// resolveNonAppHookConfigs reads a test.sh for each unique directory of
// changed files that does NOT fall under any detected kustomize app root,
// walking upward from that directory toward the repository root and
// stopping at the nearest ancestor that actually declares a test.sh
// (closest-match-wins, the same cascading pattern .gitignore/.editorconfig
// use — not a merge across ancestors). This allows a single test.sh placed
// at a shared parent directory (e.g. okd/test.sh) to cover both files
// directly in that directory and files in its non-app subdirectories (e.g.
// okd/node-config/*.yaml), without requiring a test.sh in every leaf
// directory. Directories like okd/node-config/ — which have no base/,
// overlays/, or components/ structure and are therefore never detected as
// kustomize apps — still benefit from this: their EXEMPTIONS=(...) apply to
// standalone lint steps such as kubeconform (check=kubeconform) that run
// before the Build YAML phase's normal app-hook resolution.
//
// The returned map is keyed by the directory whose test.sh was actually
// used (which may be an ancestor of, not equal to, some changed files'
// directories). Callers must defer cleanupNonAppHookConfigs(result) since
// SourceMain resolution writes a temp file per directory (matching the
// same hook.Resolve contract that resolveAppHookConfigs uses for app-level
// test.sh files).
func resolveNonAppHookConfigs(changed []string, source hook.Source) map[string]*hook.Config {
	appRoots := detectAppRoots(changed)
	seenStartDir := make(map[string]bool)
	cfgs := make(map[string]*hook.Config)
	for _, f := range changed {
		dir := filepath.Dir(f)
		if seenStartDir[dir] {
			continue
		}
		seenStartDir[dir] = true
		// Skip directories that already fall under a kustomize app root —
		// their test.sh is handled by resolveAppHookConfigs during the
		// Build YAML phase, which has the full overlay context. An
		// ancestor of a non-app-root file's directory can never itself be
		// "under" an app root (nesting only decreases while walking
		// upward), so this check need only run once, on the starting
		// directory, before the walk begins.
		if isUnderAnyRoot(f, appRoots) {
			continue
		}
		for cur := dir; ; {
			if _, already := cfgs[cur]; already {
				break
			}
			if hook.Exists(cur, source) {
				if cfg, err := hook.Resolve(cur, source); err == nil && cfg != nil {
					cfgs[cur] = cfg
				}
				break
			}
			if cur == "." {
				break
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	return cfgs
}

// cleanupNonAppHookConfigs removes temp test.sh files that
// resolveNonAppHookConfigs created for SourceMain resolution.
func cleanupNonAppHookConfigs(cfgs map[string]*hook.Config) {
	for _, cfg := range cfgs {
		hook.CleanupConfig(cfg)
	}
}

// nonAppExemptSelectors extracts the merged exempt.Selector slice from a
// map of non-app hook configs, bridging hook.ExemptSelector (the hook
// layer's parsed representation) into exempt.Selector (the core engine's
// evaluation type). The Dir field — a root-anchored directory-prefix match
// added as a recognized EXEMPTIONS=() key alongside file/kind/name/etc. —
// is forwarded here so check=kubeconform,file=... or
// check=kubeconform,dir=... selectors evaluate correctly in the kubeconform
// lint step.
func nonAppExemptSelectors(cfgs map[string]*hook.Config) []exempt.Selector {
	var selectors []exempt.Selector
	for _, cfg := range cfgs {
		for _, sel := range cfg.ExemptSelectors {
			selectors = append(selectors, exempt.Selector{
				Check:     sel.Check,
				File:      sel.File,
				Kind:      sel.Kind,
				Name:      sel.Name,
				Namespace: sel.Namespace,
				Match:     sel.Match,
				Value:     sel.Value,
				Path:      sel.Path,
			})
		}
	}
	return selectors
}
