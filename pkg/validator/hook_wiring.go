package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// resolveHookSource decides which test.sh source to trust for this run.
// When no explicit source is set and there is no PR context (i.e. a local
// test run), it defaults to SourceLocal so uncommitted
// test.sh changes in the working tree are picked up automatically.  In
// pipeline/PR mode (opts.PR non-empty) it falls through to
// hook.ResolveSource's fail-closed SourceMain default, preventing a PR
// from smuggling in a weakened test.sh.
func resolveHookSource(opts Options) hook.Source {
	signal := opts.HookSource
	if signal == "" && opts.PR == "" {
		signal = hook.SourceLocal
	}
	return hook.ResolveSource(signal, opts.TriggerComment, opts.PR != "")
}

// resolveAppHookConfigs resolves each app's test.sh once, up front, so the
// same *hook.Config is reused for exemption-selector merging, every overlay
// build under that app, and its POST_VALIDATE_HOOK. Callers must defer
// cleanupAppHookConfigs(result) - SourceMain resolution writes a temp file
// per app (see hook.Resolve) that must be removed once the run is done.
func resolveAppHookConfigs(apps []string, source hook.Source) map[string]*hook.Config {
	cfgs := make(map[string]*hook.Config, len(apps))
	for _, app := range apps {
		cfg, err := hook.Resolve(app, source)
		if err != nil || cfg == nil {
			continue
		}
		cfgs[app] = cfg
	}
	return cfgs
}

// cleanupAppHookConfigs removes any temp test.sh files resolveAppHookConfigs
// created (SourceMain only - see hook.CleanupConfig).
func cleanupAppHookConfigs(cfgs map[string]*hook.Config) {
	for _, cfg := range cfgs {
		hook.CleanupConfig(cfg)
	}
}

// hookExemptSelectorsAndErrors bridges each app's hook-layer
// hook.ExemptSelector entries (parsed from that app's test.sh EXEMPTIONS=(...))
// into the core exempt.Selector shape the check engine evaluates, and
// collects every app's ExemptErrors (malformed EXEMPTIONS tokens) prefixed
// with the offending app so a syntax error is fail-closed (exempts nothing)
// AND surfaces as a blocking error, per docs/HOOKS.md's documented intent.
func hookExemptSelectorsAndErrors(cfgs map[string]*hook.Config) (selectors []exempt.Selector, errs []string) {
	apps := make([]string, 0, len(cfgs))
	for app := range cfgs {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	for _, app := range apps {
		cfg := cfgs[app]
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
		for _, e := range cfg.ExemptErrors {
			errs = append(errs, fmt.Sprintf("%s: test.sh EXEMPTIONS: %s", app, e))
		}
	}
	return selectors, errs
}

// hookMisdeclaredErrors collects every app's hook.MisdeclaredHooks entries
// (a reserved hook name defined as a bash function without its directive
// assignment - a hook that would silently never run) prefixed with the
// offending app, so a dead validation gate fails closed instead of shipping.
func hookMisdeclaredErrors(cfgs map[string]*hook.Config) []string {
	apps := make([]string, 0, len(cfgs))
	for app := range cfgs {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	var errs []string
	for _, app := range apps {
		cfg := cfgs[app]
		for _, m := range cfg.MisdeclaredHooks {
			errs = append(errs, fmt.Sprintf("%s: test.sh: %s", app, m))
		}
	}
	return errs
}

// hookOutcome is the per-hook result recorded for an app's build-report row.
type hookOutcome int

const (
	hookNotDefined hookOutcome = iota
	hookRan
	hookFailed
)

// appHookResult aggregates one app's hook outcomes across every overlay it
// builds - PRE_BUILD_HOOK/POST_BUILD_HOOK run once per overlay (see
// hook.RunPreBuildHook/RunPostBuildHook's doc comments), so a single app
// with multiple overlays can see a mix of pass/fail; mergeHookOutcome keeps
// the worst (a single failure marks the row ❌ even if other overlays
// passed) so a partial failure is never silently hidden.
type appHookResult struct {
	PreBuild, PostBuild, PostValidate hookOutcome
}

// mergeHookOutcome folds next into current, keeping hookFailed sticky (a
// later success never downgrades an already-recorded failure) and
// otherwise taking the more-informative of the two (hookRan over
// hookNotDefined).
func mergeHookOutcome(current, next hookOutcome) hookOutcome {
	if current == hookFailed || next == hookFailed {
		return hookFailed
	}
	if current == hookRan || next == hookRan {
		return hookRan
	}
	return hookNotDefined
}

// anyHookFailed reports whether any app in results recorded a hookFailed
// outcome on any of its three hooks - used by runBuildAndPostBuild to give
// the Kustomize Build report's "Hooks" line its own pass/fail icon
// (compose_sections.go's ComposeKustomizeBuildSection), independent of the
// per-cell ✅/❌ already shown inside the hook table itself (hookCell in
// build_wiring.go). A hook failure is always also folded into buildErrs
// (so "Overlay Build" already reflects it too, see
// buildOverlayWithHooks/runAppPostValidateHooks) - this just lets the
// "Hooks" summary bullet stop silently omitting its own icon in that case.
func anyHookFailed(results map[string]*appHookResult) bool {
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.PreBuild == hookFailed || r.PostBuild == hookFailed || r.PostValidate == hookFailed {
			return true
		}
	}
	return false
}

// hookBuildRoot is the per-run root under which per-app hook build
// directories are materialized only when actually needed (an app with a
// POST_BUILD_HOOK or POST_VALIDATE_HOOK - see needsBuildDir). Kept as a
// package-level var (not a const) purely so tests can point it at a temp
// directory without touching the real OS temp dir.
var hookBuildRoot = filepath.Join(os.TempDir(), "k8s-gitops-ci-builds")

// needsBuildDir reports whether cfg's hooks require the rendered overlay
// YAML to actually be written to disk (POST_BUILD_HOOK's $1 YAML_FILE arg,
// or POST_VALIDATE_HOOK's build-directory arg) rather than staying purely
// in-memory - the common no-hook-defined case skips this I/O entirely.
func needsBuildDir(cfg *hook.Config) bool {
	return cfg != nil && (cfg.HasPostBuild || cfg.HasPostValidate)
}

// appBuildDir returns the per-app scratch directory PRE/POST_BUILD_HOOK's
// rendered-YAML output and POST_VALIDATE_HOOK's build-directory argument
// use, sanitized so nested app paths ("apps/myapp") don't create nested
// directories or collide across apps.
func appBuildDir(app string) string {
	safe := strings.ReplaceAll(filepath.ToSlash(app), "/", "_")
	return filepath.Join(hookBuildRoot, safe)
}

// buildOverlayWithHooks builds a single overlay - via strategy.Strategy
// (plain kustomize by default; kustomize/helm optionally piped through AVP
// secret resolution when the app's content and Options warrant it, see
// resolveAppBuildStrategies) - running the app's PRE_BUILD_HOOK before and
// POST_BUILD_HOOK after (when defined). It returns a non-empty buildErr
// (matching buildOverlayError's "kustomize build <overlay>: <cause>" format
// for build failures, so comments.go's groupBuildErrors still recognizes
// it) on any failure - a failing PRE_BUILD_HOOK skips the build entirely; a
// failing POST_BUILD_HOOK is reported even though the build itself
// succeeded. pre/post report which hooks were attempted and how they went,
// for the caller to fold into the app's aggregated appHookResult under its
// own lock. rendered is the overlay's fully-built YAML on success (nil on
// any failure) - the caller reuses it for Kyverno policy validation
// (runKyvernoValidation) instead of rendering the same overlay a second
// time.
//
// log is used purely for --verbose diagnostic timing (render vs. pre/post
// hook duration for this one overlay) - see the doc comment on the
// "overlay %s:" Debug line below. Every real call site already has one in
// scope; a nil log is tolerated (the deferred Debug emission below is a
// no-op in that case) purely so tests/future callers don't have to
// construct a throwaway logger just to satisfy this diagnostic-only param.
func buildOverlayWithHooks(ov overlayRef, cfg *hook.Config, strategy appBuildStrategy, log *logger.Logger) (buildErr string, pre, post hookOutcome, rendered []byte) {
	app := appFromOverlayPath(ov.path)
	outFile := filepath.Join(appBuildDir(app), filepath.Base(ov.path)+".yaml")

	var preDur, renderDur, postDur time.Duration
	defer func() {
		if log == nil {
			return
		}
		// --verbose-only breakdown of where this overlay's build time
		// went, so a slow Build YAML phase (see runBuildAndPostBuild's
		// per-overlay tc.RecordStep, which times this whole function as
		// one opaque unit) can be localized to render vs. a specific
		// hook without re-running with ad hoc instrumentation. render
		// includes the argocd-vault-plugin subprocess for AVP strategies
		// (overlay.RenderWithStrategy pipes through runAVP internally) -
		// not split out further to avoid threading a logger dependency
		// into the generic pkg/overlay package for a single Debug line.
		log.Debug("overlay %s: render=%s pre-hook=%s post-hook=%s", ov.path, renderDur.Round(time.Millisecond), preDur.Round(time.Millisecond), postDur.Round(time.Millisecond))
	}()

	if cfg != nil && cfg.HasPreBuild {
		preStart := time.Now()
		err := hook.RunPreBuildHook(cfg, ov.path, outFile)
		preDur = time.Since(preStart)
		if err != nil {
			return fmt.Sprintf("kustomize build %s: pre-build hook: %s", ov.path, err), hookFailed, hookNotDefined, nil
		}
		pre = hookRan
	}

	renderStart := time.Now()
	out, err := overlay.RenderWithStrategy(app, ov.path, strategy.Strategy, strategy.Exclude)
	renderDur = time.Since(renderStart)
	if err != nil {
		// overlay.RenderWithStrategy's kustomize renderer (see
		// overlay.renderKustomize) already emits errors that embed the overlay
		// path in comments.go's groupBuildErrors-recognizable form ("kustomize
		// build <overlay>: <cause>" for a build failure, "kustomize render
		// <overlay>: <cause>" when the resolved manifest map fails to marshal
		// to YAML), so pass those through verbatim instead of re-prefixing.
		// Re-wrapping produced a duplicated "kustomize build <overlay>:
		// kustomize build <overlay>: ..." (or "kustomize render ...") message
		// that wasted comment/console budget and obscured the real cause. Other
		// render paths - Helm strategies (overlay.renderHelm's "helm load
		// chart ..."/"helm template ..." errors), the AVP pass (overlay.runAVP),
		// and "unknown strategy" - emit errors without any kustomize prefix, so
		// wrap those here to keep every overlay-build failure attributable to
		// its overlay by groupBuildErrors.
		return renderBuildError(ov.path, err), pre, post, nil
	}

	if cfg != nil && cfg.HasPostBuild {
		if err := os.MkdirAll(filepath.Dir(outFile), 0o750); err != nil {
			return fmt.Sprintf("kustomize build %s: post-build hook: writing rendered YAML: %s", ov.path, err), pre, hookFailed, nil
		}
		if err := os.WriteFile(outFile, out, 0o600); err != nil {
			return fmt.Sprintf("kustomize build %s: post-build hook: writing rendered YAML: %s", ov.path, err), pre, hookFailed, nil
		}
		postStart := time.Now()
		err := hook.RunPostBuildHook(cfg, outFile, ov.path)
		postDur = time.Since(postStart)
		if err != nil {
			return fmt.Sprintf("kustomize build %s: post-build hook: %s", ov.path, err), pre, hookFailed, nil
		}
		post = hookRan
	} else if cfg != nil && cfg.HasPostValidate {
		// No POST_BUILD_HOOK, but POST_VALIDATE_HOOK still needs this
		// overlay's rendered YAML present in the app's build directory.
		if err := os.MkdirAll(filepath.Dir(outFile), 0o750); err == nil {
			_ = os.WriteFile(outFile, out, 0o600)
		}
	}

	return "", pre, post, out
}

// renderBuildError normalizes an overlay.RenderWithStrategy failure into the
// canonical "kustomize build <overlay>: <cause>" build-error format that
// comments.go's groupBuildErrors parses to attribute the failure to its
// overlay.
//
// renderKustomize already returns errors carrying that attribution - either
// with the "kustomize build <overlay>: " prefix (krusty build failure) or the
// "kustomize render <overlay>: " prefix (resMap.AsYaml failure) - so those are
// returned verbatim; re-prefixing them would produce a duplicated
// "kustomize build <overlay>: kustomize build <overlay>: ..." (or
// "kustomize render ...") message. Every other render path (Helm strategies,
// the AVP pass via runAVP, and unknown strategies) emits an error with no
// kustomize prefix, so those are wrapped here to keep the failure attributed
// to its overlay.
func renderBuildError(overlayPath string, err error) string {
	renderErr := err.Error()
	if strings.HasPrefix(renderErr, "kustomize build ") || strings.HasPrefix(renderErr, "kustomize render ") {
		return renderErr
	}
	return fmt.Sprintf("kustomize build %s: %s", overlayPath, renderErr)
}

// runAppPostValidateHooks runs each app's POST_VALIDATE_HOOK (once, after
// every one of its overlays has been built - see
// hook.RunPostValidateHook's doc comment) and returns any failures in
// buildOverlayError's "kustomize build <app>: <cause>" format. It also
// removes each app's scratch build directory (see appBuildDir) once its
// POST_VALIDATE_HOOK has had a chance to read it, regardless of outcome.
//
// log is used purely for --verbose diagnostic timing of each app's
// POST_VALIDATE_HOOK - this hook runs once per app (not once per overlay
// like PRE/POST_BUILD, see buildOverlayWithHooks), and was previously
// entirely untimed even in the end-of-run timing table, making a slow
// POST_VALIDATE_HOOK (e.g. one that shells out to a cloud API per
// referenced image) invisible. A nil log is tolerated (the Debug line
// below is simply skipped) for the same reason as buildOverlayWithHooks.
func runAppPostValidateHooks(apps []string, cfgs map[string]*hook.Config, results map[string]*appHookResult, log *logger.Logger) (errs []string) {
	for _, app := range apps {
		cfg := cfgs[app]
		dir := appBuildDir(app)
		if cfg != nil && cfg.HasPostValidate {
			start := time.Now()
			err := hook.RunPostValidateHook(cfg, dir, app)
			if log != nil {
				log.Debug("post-validate hook %s: %s", app, time.Since(start).Round(time.Millisecond))
			}
			if err != nil {
				errs = append(errs, fmt.Sprintf("kustomize build %s: post-validate hook: %s", app, err))
				if r := results[app]; r != nil {
					r.PostValidate = hookFailed
				}
			} else if r := results[app]; r != nil {
				r.PostValidate = hookRan
			}
		}
		if needsBuildDir(cfg) {
			_ = os.RemoveAll(dir)
		}
	}
	return errs
}
