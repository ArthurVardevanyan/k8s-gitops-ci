package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/configdiff"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/git"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
)

// scaffoldValidationResult aggregates every app's scaffold.Run outcome for
// this run's "Scaffold Validation" report section.
type scaffoldValidationResult struct {
	// DriftLines/ExecErrors are pre-formatted, one entry per blocking
	// drifted overlay / execution failure - ComposeScaffoldValidationSection
	// expects a single driftSummary string, so runScaffoldValidation joins
	// DriftLines with newlines for that call.
	DriftLines []string
	// PreExistingDriftLines are drifted overlays whose generated content the
	// PR's config/template changes do not alter (see computeBaselineDrift) -
	// non-blocking, surfaced for visibility only. This is the direct/indirect
	// split finalizeCompliance already draws for doc/overlay check findings,
	// applied to scaffold drift: an overlay this PR is already modifying must
	// still fix any drift found there (see isOverlayScaffoldRelated), even if
	// that same drift also exists at the merge-base. Only the overlay's own
	// files and the app's scaffold template are "directly touched"; a
	// base/components edit is never scaffold-related, and a config edit is
	// ambiguous until generated content is compared (see computeBaselineDrift).
	PreExistingDriftLines []string
	ExecErrors            []string
	// SkippedClusters records, per app, every overlay scaffold.Run skipped
	// rather than validated (scaffold.Summary.SkippedClusters - disabled
	// via config/change-group, or no on-disk directory yet: a cluster not
	// yet rolled out, or removed by this PR). Never a failure on its own
	// (see scaffold.Run's own doc comment); flattenSkippedClusters turns
	// this into ComposeScaffoldValidationSection's informational
	// missingClusters list.
	SkippedClusters map[string][]string
	// DisabledClusters records, per app, every overlay scaffold.Run skipped
	// because its scaffold config marks it disabled
	// (scaffold.Summary.DisabledClusters - the
	// `overlayDefinitions.overrides.<cluster>.disabled` flag), rather than
	// for any other reason (no directory yet, etc.). Unlike SkippedClusters
	// this is filtered to overlays the PR itself modified, because a disabled
	// overlay is only worth a warning when this PR is actively changing its
	// files - a disabled overlay untouched by the PR is expected and silent.
	// Never a failure on its own; ComposeScaffoldValidationSection surfaces
	// it as a warning.
	DisabledClusters map[string][]string
}

// runScaffoldValidation drives pkg/scaffold.Run across every app this run
// needs to check, mirroring the three ways a change can require scaffold
// re-validation - each phase skips any app already tested by an earlier
// one, and apps within a phase run bounded-parallel (see runScaffoldApps):
//
//  1. Template changes (configdiff.DetectTemplateChanges) - a shared
//     template changed, so every overlay of every app using it needs
//     re-checking (a "full test": every on-disk overlay).
//  2. Config changes (configdiff.DetectAffectedApps) - either specific
//     clusters (an override changed) or, when the diff touched something
//     that fans out cluster-independently (e.g. a changeGroup), a full
//     test of that app too.
//  3. Apps with their own overlay files changed, not already covered by
//     (1) or (2) - only the overlays the PR actually touched
//     (scaffold.ChangedOverlayNames), via the same trigger classification
//     overlay.GetOverlaysToTest already uses for the build phase.
//
// A drifted overlay this PR directly touches - its own files, or its app's
// scaffold template (isOverlayScaffoldRelated) - or a scaffold-tool execution
// failure, is always treated as blocking. A base/ or components/ edit is
// never scaffold-related: it changes the render/build output but not what
// scaffold would generate (see isOverlayScaffoldRelated, and the contrast
// with isOverlayRelatedToChangedFiles used by the build/kubeconform phases).
//
// A drifted overlay this PR does NOT directly touch is ambiguous when the app
// scaffold config changed: a config edit may or may not alter what scaffold
// generates for any given overlay. computeBaselineDrift settles that question
// precisely by generating the app at both the merge-base and HEAD - in two
// throwaway git worktrees, so the caller's working tree is never mutated - and
// comparing the generated content per overlay. Generated content equal at both
// revisions means the PR's config change does not affect the overlay, so the
// drift is caused by something external (e.g. cluster-metadata API data
// changing independently) and is downgraded to a non-blocking
// PreExistingDriftLines entry. Generated content differ means the PR's own
// change altered the overlay's expected output, so it stays blocking - the
// author regenerated the app but missed this overlay. opts.BaseRef must be set
// (an actual CI/PR run, never a local test run against a live working tree,
// which always has an empty BaseRef - see gitDiff's own doc comment); when it
// isn't, or the tool/worktree is unavailable, every ambiguous mismatch stays
// blocking, the conservative pre-comparison policy.
func runScaffoldValidation(opts Options, apps, changed []string, log *logger.Logger) scaffoldValidationResult {
	changeGroups, _ := opts.Providers.ChangeGroups()
	workers := Workers(opts)
	result := scaffoldValidationResult{}
	tested := make(map[string]bool)
	var mu sync.Mutex

	record := func(app string, summary *scaffold.Summary) {
		// computeBaselineDrift (two full scaffold generations, into two git
		// worktrees) is expensive, so it's only ever invoked when actually
		// needed: at least one mismatch this PR doesn't directly touch, and
		// only once per app (memoized here, outside the shared-result mutex
		// below so it doesn't serialize other apps' bookkeeping).
		var ambiguous []string
		for _, ov := range summary.MismatchFiles {
			if !isOverlayScaffoldRelated(app, ov, changed) {
				ambiguous = append(ambiguous, ov)
			}
		}
		var baseline baselineDriftResult
		if len(ambiguous) > 0 {
			baseline = computeBaselineDrift(opts, app, ambiguous, log)
		}
		configChanged := isAppScaffoldConfigChanged(app, changed)

		mu.Lock()
		defer mu.Unlock()
		tested[app] = true
		for _, ov := range summary.MismatchFiles {
			line := fmt.Sprintf("%s: overlay `%s` drifted from its scaffold template/config", app, ov)
			if paths := summary.MismatchPaths[ov]; len(paths) > 0 {
				line += fmt.Sprintf(" [%s]", summarizeFiles(paths))
			}
			switch {
			case isOverlayScaffoldRelated(app, ov, changed):
				result.DriftLines = append(result.DriftLines, line)
				log.ErrorInSection("Scaffold", "drift: %s/%s", app, ov)
			case baseline.PreExisting[ov]:
				result.PreExistingDriftLines = append(result.PreExistingDriftLines, line+" (pre-existing: this PR's scaffold config/template change does not alter this overlay's generated output)")
				log.Warn("scaffold: pre-existing drift (non-blocking): %s/%s", app, ov)
			default:
				blockLine := line
				if configChanged {
					if files := baseline.Diffs[ov]; len(files) > 0 {
						blockLine += fmt.Sprintf(" (this PR's scaffold config change alters its generated output: %s)", summarizeFiles(files))
					}
				}
				result.DriftLines = append(result.DriftLines, blockLine)
				log.ErrorInSection("Scaffold", "drift: %s/%s", app, ov)
			}
		}
		for _, e := range summary.Errors {
			result.ExecErrors = append(result.ExecErrors, fmt.Sprintf("%s: %s", app, e))
			log.ErrorInSection("Scaffold", "%s: %s", app, e)
		}
		if len(summary.SkippedClusters) > 0 {
			if result.SkippedClusters == nil {
				result.SkippedClusters = map[string][]string{}
			}
			result.SkippedClusters[app] = append(result.SkippedClusters[app], summary.SkippedClusters...)
		}
		// A config-disabled overlay is only worth a warning when this PR
		// actually modified it - it's expected (and silent) otherwise.
		for _, ov := range summary.DisabledClusters {
			if !isOverlayScaffoldRelated(app, ov, changed) {
				continue
			}
			if result.DisabledClusters == nil {
				result.DisabledClusters = map[string][]string{}
			}
			result.DisabledClusters[app] = append(result.DisabledClusters[app], ov)
			log.Warn("scaffold: overlay %s/%s is disabled in config; scaffolding skipped (modified in this PR)", app, ov)
		}
	}
	isTested := func(app string) bool {
		mu.Lock()
		defer mu.Unlock()
		return tested[app]
	}

	// 1. Template changes: full test of every app using the changed template.
	var jobs1 []scaffoldJob
	for _, app := range configdiff.DetectTemplateChanges(changed) {
		if !scaffold.HasScaffoldEnabled(app) || !scaffold.HasScaffoldConfig(app) {
			continue
		}
		if overlays := scaffold.FindOverlays(app); len(overlays) > 0 {
			jobs1 = append(jobs1, scaffoldJob{app: app, trigger: "fan-out", overlays: overlays, fullTest: true})
		}
	}
	runScaffoldApps(jobs1, changed, changeGroups, workers, record)

	// 2. Config changes: cluster-specific, or a full test when the change
	// fans out cluster-independently (e.g. a changeGroup reassignment).
	var jobs2 []scaffoldJob
	for _, aff := range configdiff.DetectAffectedApps(changed, opts.RepoURL, opts.PR, changeGroups) {
		if isTested(aff.App) || !scaffold.HasScaffoldEnabled(aff.App) || !scaffold.HasScaffoldConfig(aff.App) {
			continue
		}
		overlays := aff.Clusters
		trigger := aff.Trigger
		if aff.FullTest {
			overlays = scaffold.FindOverlays(aff.App)
		}
		if len(overlays) > 0 {
			jobs2 = append(jobs2, scaffoldJob{app: aff.App, trigger: trigger, overlays: overlays, fullTest: aff.FullTest})
		}
	}
	runScaffoldApps(jobs2, changed, changeGroups, workers, record)

	// 3. Apps with their own overlay changes, not already covered above.
	var jobs3 []scaffoldJob
	for _, app := range apps {
		if isTested(app) || !scaffold.HasScaffoldEnabled(app) || !scaffold.HasScaffoldConfig(app) {
			continue
		}
		_, isFullTest, trigger := overlay.GetOverlaysToTest(app, changed, false)
		if trigger == "" {
			continue
		}
		overlays := scaffold.ChangedOverlayNames(app, changed)
		if isFullTest {
			overlays = scaffold.FindOverlays(app)
		}
		if len(overlays) > 0 {
			jobs3 = append(jobs3, scaffoldJob{app: app, trigger: trigger, overlays: overlays, fullTest: isFullTest})
		}
	}
	runScaffoldApps(jobs3, changed, changeGroups, workers, record)

	return result
}

// scaffoldJob is one app's scaffold.Run input, queued for
// runScaffoldApps's bounded-parallel worker pool.
type scaffoldJob struct {
	app      string
	trigger  string
	overlays []string
	fullTest bool
}

// runScaffoldApps runs scaffold.Run for each job bounded-parallel (up to
// runtime.NumCPU()*2 apps at once, matching Workers' default - each
// scaffold.Run call is itself already bounded-parallel across that one
// app's overlays), invoking record for every result.
func runScaffoldApps(jobs []scaffoldJob, changed []string, changeGroups map[string]int, workers int, record func(app string, summary *scaffold.Summary)) {
	if len(jobs) == 0 {
		return
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	if workers < 1 {
		workers = 1
	}
	ch := make(chan scaffoldJob, len(jobs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				summary := scaffold.Run(scaffold.RunOptions{
					App:          j.app,
					Trigger:      j.trigger,
					Overlays:     j.overlays,
					ChangedFiles: changed,
					ChangeGroups: changeGroups,
					FullTest:     j.fullTest,
				})
				record(j.app, summary)
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
}

// flattenSkippedClusters turns a scaffoldValidationResult.SkippedClusters
// map into ComposeScaffoldValidationSection's flat, deterministically
// ordered "app/cluster" missingClusters list.
func flattenSkippedClusters(skipped map[string][]string) []string {
	if len(skipped) == 0 {
		return nil
	}
	apps := make([]string, 0, len(skipped))
	for app := range skipped {
		apps = append(apps, app)
	}
	sort.Strings(apps)

	var out []string
	for _, app := range apps {
		clusters := append([]string(nil), skipped[app]...)
		sort.Strings(clusters)
		for _, c := range clusters {
			out = append(out, fmt.Sprintf("%s/%s", app, c))
		}
	}
	return out
}

// flattenDisabledClusters turns a scaffoldValidationResult.DisabledClusters
// map into ComposeScaffoldValidationSection's flat, deterministically
// ordered "app/cluster" disabledOverlays list.
func flattenDisabledClusters(disabled map[string][]string) []string {
	if len(disabled) == 0 {
		return nil
	}
	apps := make([]string, 0, len(disabled))
	for app := range disabled {
		apps = append(apps, app)
	}
	sort.Strings(apps)

	var out []string
	for _, app := range apps {
		clusters := append([]string(nil), disabled[app]...)
		sort.Strings(clusters)
		for _, c := range clusters {
			out = append(out, fmt.Sprintf("%s/%s", app, c))
		}
	}
	return out
}

// isOverlayRelatedToChangedFiles reports whether app's cluster overlay -
// or a base/component the overlay actually inherits from - was itself
// touched by this PR's own changed files. A mismatch scaffold.Run finds is
// only ever eligible for the non-blocking pre-existing-drift downgrade (see
// runScaffoldValidation) when this returns false: if the PR is already
// modifying files in the affected overlay (or a base/component the overlay
// inherits from), it must also fix any drift found there, baseline or not.
//
// The overlay's own directory and the app's base/ are treated as coarse
// signals (base/ flows into effectively every overlay), but changes under
// components/ are scoped precisely: a component change only relates to this
// overlay when the overlay's kustomization reference chain actually includes
// that specific component directory. Because components are
// version-partitioned (e.g. components/foo/v0.21.0 vs components/foo/v0.19.1)
// and each overlay pins one version, this stops a change to one version from
// blaming overlays pinned to a different, unaffected version - letting their
// genuinely pre-existing drift fall through to the non-blocking downgrade.
func isOverlayRelatedToChangedFiles(app, cluster string, changedFiles []string) bool {
	overlayPrefix := filepath.ToSlash(filepath.Join(app, "overlays", cluster)) + "/"
	basePrefix := filepath.ToSlash(filepath.Join(app, "base")) + "/"
	componentsPrefix := filepath.ToSlash(filepath.Join(app, "components")) + "/"

	var changedComponentDirs []string
	seen := map[string]bool{}
	for _, cf := range changedFiles {
		cf = filepath.ToSlash(cf)
		if strings.HasPrefix(cf, overlayPrefix) || strings.HasPrefix(cf, basePrefix) {
			return true
		}
		if strings.HasPrefix(cf, componentsPrefix) {
			dir := filepath.ToSlash(filepath.Dir(cf))
			if !seen[dir] {
				seen[dir] = true
				changedComponentDirs = append(changedComponentDirs, dir)
			}
		}
	}
	if len(changedComponentDirs) == 0 {
		return false
	}
	overlayDir := filepath.Join(app, "overlays", cluster)
	return overlay.RefsChangedDir(overlayDir, changedComponentDirs)
}

// isOverlayScaffoldRelated reports whether a changed file makes app's cluster
// overlay this PR's direct responsibility to fix (blocking), rather than
// eligible for the non-blocking pre-existing-drift downgrade.
//
// Only the overlay's own files (<app>/overlays/<cluster>/...) and the app's
// scaffold template (<ScaffoldDir>/templates/<app>/...) are direct: a change
// to either unambiguously changes what scaffold generates for this overlay.
// Nothing under <app>/base/ or <app>/components/ is an input to scaffold, no
// matter how an overlay's kustomization chain references it - the deliberate
// contrast with isOverlayRelatedToChangedFiles, which is a render/build
// heuristic used by the build and kubeconform phases.
//
// The app's scaffold config (<ScaffoldDir>/configs/<app>.{yaml,yml}) is
// deliberately NOT treated as directly related here even though it is a
// genuine scaffold input: a config edit is app-wide and may or may not change
// any particular overlay's output (a per-cluster override, or a config key
// that overlay doesn't exercise, changes nothing for it). That question is
// settled precisely by comparing generated content at the merge-base and HEAD
// (see computeBaselineDrift), not guessed from the path.
func isOverlayScaffoldRelated(app, cluster string, changedFiles []string) bool {
	overlayPrefix := filepath.ToSlash(filepath.Join(app, "overlays", cluster)) + "/"
	templatePrefix := filepath.ToSlash(filepath.Join(convention.ScaffoldTemplatesPrefix(), app)) + "/"

	for _, cf := range changedFiles {
		cf = filepath.ToSlash(cf)
		if strings.HasPrefix(cf, overlayPrefix) {
			return true
		}
		if strings.HasPrefix(cf, templatePrefix) {
			return true
		}
	}
	return false
}

// isAppScaffoldConfigChanged reports whether this PR changed app's scaffold
// config (either recognized extension). A config edit is the trigger for the
// generated-content comparison in computeBaselineDrift; it does not by itself
// make any overlay directly related (see isOverlayScaffoldRelated).
func isAppScaffoldConfigChanged(app string, changedFiles []string) bool {
	configBase := filepath.ToSlash(filepath.Join(convention.ScaffoldConfigsPrefix(), app))
	for _, cf := range changedFiles {
		cf = filepath.ToSlash(cf)
		if cf == configBase+".yaml" || cf == configBase+".yml" {
			return true
		}
	}
	return false
}

// baselineDriftResult is the outcome of computeBaselineDrift for one app's
// ambiguous mismatches.
type baselineDriftResult struct {
	// PreExisting is the set of ambiguous overlays whose generated content is
	// byte-identical at the merge-base and at HEAD - the PR's config/template
	// changes do not alter what scaffold produces for them, so their drift is
	// external and not this PR's to fix.
	PreExisting map[string]bool
	// Diffs maps an overlay that is NOT external to the overlay-relative
	// generated files that differ between the two revisions, so the blocking
	// report can name what the PR's change altered. Populated only when both
	// revisions generated the overlay (an overlay generated on one side only
	// isn't a meaningful file list).
	Diffs map[string][]string
}

// generateScaffold is scaffold.Generate, indirected so tests can substitute
// deterministic generated content without a real scaffold tool.
var generateScaffold = scaffold.Generate

// worktreeMu serializes git worktree creation across apps. The slow part
// (generation) still runs concurrently; only the git plumbing that mutates
// the shared repository's .git/worktrees state is serialized.
var worktreeMu sync.Mutex

// addWorktree is git.AddWorktree, serialized via worktreeMu.
func addWorktree(ctx context.Context, ref string) (dir string, cleanup func(), err error) {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()
	return git.AddWorktree(ctx, ref)
}

// computeBaselineDrift decides which of an app's ambiguous drifted overlays
// are attributable to an external data source rather than this PR, by
// generating the app at both the merge-base and HEAD and comparing each
// overlay's generated content.
//
// It runs each revision's generation in its own throwaway git worktree
// (git.AddWorktree), so the caller's working tree is never read or written by
// the comparison - replacing an earlier baseline technique that mutated the
// app's on-disk config/templates in place. Content is compared after the tool
// runs: an overlay the tool rewrites holds generated content, and one it skips
// because "nothing changed" already equals its generated content, so the
// post-run overlay tree is always the generated tree.
//
// Best-effort and conservative: an empty opts.BaseRef (a local run against a
// live working tree), an unconfigured generator (GenerateArgs unset under
// DryRunParse), or any git/scaffold failure returns an empty result, so every
// ambiguous mismatch stays blocking - the pre-comparison policy. It never
// fails the run.
//
// Safe to call concurrently for different apps; a given app is only
// scaffold-tested once per runScaffoldValidation call (see its isTested
// gating), so this is never called twice concurrently for the same app.
func computeBaselineDrift(opts Options, app string, ambiguous []string, log *logger.Logger) baselineDriftResult {
	res := baselineDriftResult{
		PreExisting: map[string]bool{},
		Diffs:       map[string][]string{},
	}
	if len(ambiguous) == 0 || opts.BaseRef == "" {
		return res
	}
	if scaffold.DriftMode == scaffold.DryRunParse && scaffold.GenerateArgs == nil {
		log.Debug("scaffold baseline: GenerateArgs unset; treating all mismatches as blocking")
		return res
	}

	ctx := context.Background()
	mergeBase, err := git.MergeBase(ctx, opts.BaseRef)
	if err != nil || mergeBase == "" {
		log.Debug("scaffold baseline: could not determine merge-base against %q: %v", opts.BaseRef, err)
		return res
	}

	baseDir, cleanupBase, err := addWorktree(ctx, mergeBase)
	if err != nil {
		log.Debug("scaffold baseline: base worktree: %v", err)
		return res
	}
	defer cleanupBase()
	headDir, cleanupHead, err := addWorktree(ctx, "HEAD")
	if err != nil {
		log.Debug("scaffold baseline: head worktree: %v", err)
		return res
	}
	defer cleanupHead()

	baseGen, err := generateScaffold(scaffold.GenerateOptions{App: app, WorkDir: baseDir})
	if err != nil {
		log.Debug("scaffold baseline: generating %s at %s: %v", app, mergeBase, err)
		return res
	}
	headGen, err := generateScaffold(scaffold.GenerateOptions{App: app, WorkDir: headDir})
	if err != nil {
		log.Debug("scaffold baseline: generating %s at HEAD: %v", app, err)
		return res
	}

	for _, ov := range ambiguous {
		_, baseHas := baseGen.Overlays[ov]
		_, headHas := headGen.Overlays[ov]
		differ, files := scaffold.DiffOverlay(baseGen, headGen, ov)
		if !differ {
			// Equal content only proves external-ness when at least one side
			// actually generated the overlay; if neither did, the comparison
			// is inconclusive, so stay conservative (blocking).
			if baseHas || headHas {
				res.PreExisting[ov] = true
			}
			continue
		}
		if baseHas && headHas {
			res.Diffs[ov] = files
		}
	}
	return res
}

// summarizeFiles renders a file list for a report line, capping it so a large
// drift/diff can't bloat the comment.
func summarizeFiles(files []string) string {
	const maxDiffFiles = 8
	if len(files) <= maxDiffFiles {
		return strings.Join(files, ", ")
	}
	return strings.Join(files[:maxDiffFiles], ", ") + fmt.Sprintf(", ... (+%d more)", len(files)-maxDiffFiles)
}

// findUnprotectedApps identifies apps with modified overlays/scaffold
// templates/scaffold configs that have a scaffold template (i.e. scaffold
// drift detection is available for them at all) but haven't opted into it
// via test.sh - see scaffold.HasScaffoldEnabled/docs/HOOKS.md's SCAFFOLD
// directive. These apps' overlays are never actually re-validated against
// their template by runScaffoldValidation above (HasScaffoldEnabled gates
// every one of its three trigger phases), so a drifted overlay there would
// go completely unnoticed; this surfaces that gap as its own warning
// instead of silently saying nothing.
func findUnprotectedApps(changed []string) []string {
	affected := map[string]bool{}
	for _, f := range changed {
		f = filepath.ToSlash(f)
		parts := strings.SplitN(f, "/", 2)
		if len(parts) < 2 {
			continue
		}
		app := parts[0]
		// Matches both the standard <app>/overlays/<name> layout and the
		// nested <app>/<group>/overlays/<name> layout, so overlay changes
		// are attributed to the top-level app regardless of layout - a
		// nested app without its own scaffold template is filtered out
		// below by the templateDir stat, so attribution staying broad here
		// doesn't cause false positives.
		if strings.Contains(f, "/overlays/") {
			affected[app] = true
		}
		if rest, ok := strings.CutPrefix(f, convention.ScaffoldTemplatesPrefix()); ok {
			if a := strings.SplitN(rest, "/", 2)[0]; a != "" {
				affected[a] = true
			}
		}
		if strings.HasPrefix(f, convention.ScaffoldConfigsPrefix()) {
			base := filepath.Base(f)
			a := strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
			if a != "" {
				affected[a] = true
			}
		}
	}

	apps := make([]string, 0, len(affected))
	for app := range affected {
		apps = append(apps, app)
	}
	sort.Strings(apps)

	var unprotected []string
	for _, app := range apps {
		templateDir := filepath.Join(convention.ScaffoldDir, "templates", app)
		if _, err := os.Stat(templateDir); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(app, "test.sh")); err != nil {
			continue
		}
		if !scaffold.HasScaffoldEnabled(app) {
			unprotected = append(unprotected, app)
		}
	}
	return unprotected
}
