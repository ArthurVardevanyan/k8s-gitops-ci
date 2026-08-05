package validator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	// PreExistingDriftLines are drifted overlays that also drift against
	// the merge-base template/config (see computeBaselineMismatches) and
	// whose overlay/app this PR didn't touch - non-blocking, surfaced for
	// visibility only. This is the direct/indirect split finalizeCompliance
	// already draws for doc/overlay check findings, applied to scaffold
	// drift: an overlay this PR is already modifying must still fix any
	// drift found there (see isOverlayRelatedToChangedFiles), even if that
	// same drift also exists at the merge-base.
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
// A drifted overlay whose app/overlay this PR's own changes touch, or a
// scaffold-tool execution failure, is always treated as blocking. A
// drifted overlay this PR does NOT touch is checked against the
// merge-base template/config (computeBaselineMismatches - opts.BaseRef
// must be set, i.e. an actual CI/PR run, never a local test-all run
// against a live working tree, which always has an empty BaseRef - see
// gitDiff's own doc comment) and downgraded to a non-blocking,
// PreExistingDriftLines entry when it drifts there too: this is
// deliberately a real but substantially riskier technique than a flat
// "any drift blocks" policy (it mutates the app's on-disk template/config
// files in place for the duration of the re-run - see
// computeBaselineMismatches), reserved for exactly the case it exists to
// fix (drift caused by something external to this PR, e.g. cluster-
// metadata API data changing independently) rather than applied broadly.
func runScaffoldValidation(opts Options, apps, changed []string, log *logger.Logger) scaffoldValidationResult {
	changeGroups, _ := opts.Providers.ChangeGroups()
	workers := Workers(opts)
	result := scaffoldValidationResult{}
	tested := make(map[string]bool)
	var mu sync.Mutex

	record := func(app string, summary *scaffold.Summary) {
		// computeBaselineMismatches (a full scaffold re-run against the
		// merge-base template/config) is expensive and mutates on-disk
		// files, so it's only ever invoked when actually needed: at least
		// one mismatch this PR's own changes don't already explain, and
		// only once per app (memoized here, outside the shared-result
		// mutex below so it doesn't serialize other apps' bookkeeping).
		var baseline map[string]bool
		for _, ov := range summary.MismatchFiles {
			if !isOverlayRelatedToChangedFiles(app, ov, changed) {
				baseline = computeBaselineMismatches(opts, app, log)
				break
			}
		}

		mu.Lock()
		defer mu.Unlock()
		tested[app] = true
		for _, ov := range summary.MismatchFiles {
			line := fmt.Sprintf("%s: overlay `%s` drifted from its scaffold template/config", app, ov)
			switch {
			case isOverlayRelatedToChangedFiles(app, ov, changed):
				result.DriftLines = append(result.DriftLines, line)
				log.ErrorInSection("Scaffold", "drift: %s/%s", app, ov)
			case baseline[ov]:
				result.PreExistingDriftLines = append(result.PreExistingDriftLines, line+" (pre-existing, not introduced by this PR)")
				log.Warn("scaffold: pre-existing drift (non-blocking): %s/%s", app, ov)
			default:
				result.DriftLines = append(result.DriftLines, line)
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

// computeBaselineMismatches re-runs scaffold for app against the
// merge-base version of its template/config, to determine which of this
// run's mismatched overlays already drifted before this PR - e.g. drift
// caused by an external data source (cluster metadata, a shared secret
// value, ...) changing independently of anything the PR itself touched,
// rather than by the PR's own template/config edits. Returns the set of
// mismatched overlay names that also mismatch at the baseline.
//
// Best-effort and conservative: opts.BaseRef being empty (a local test-all
// run against a live working tree - see gitDiff's own doc comment) skips
// baseline diffing entirely, and any git failure (no repo, no merge-base,
// a git-show failure) returns an empty set - "couldn't compute a
// baseline" degrades to "treat every mismatch as new/blocking", the same
// policy this repo used before baseline diffing existed at all. It never
// fails the run.
//
// This mutates app's on-disk template/config files in place for the
// duration of the re-run (scaffold.Run takes no "compare against a
// different template" option) - every backed-up file is restored via a
// single deferred call immediately after backing it up, so an unexpected
// error/panic mid-run can never leave the working tree permanently
// altered. Safe to call concurrently for different apps (each app's
// template/config lives under its own, non-overlapping .scafctl paths);
// a given app is only ever scaffold-tested once per runScaffoldValidation
// call (see its own isTested gating), so this is never called twice
// concurrently for the same app.
func computeBaselineMismatches(opts Options, app string, log *logger.Logger) map[string]bool {
	baseline := map[string]bool{}
	if opts.BaseRef == "" {
		return baseline
	}

	ctx := context.Background()
	mergeBase, err := git.MergeBase(ctx, opts.BaseRef)
	if err != nil || mergeBase == "" {
		log.Debug("scaffold baseline: could not determine merge-base against %q: %v", opts.BaseRef, err)
		return baseline
	}

	configFile := filepath.Join(convention.ScaffoldDir, "configs", app+".yaml")
	templateDir := filepath.Join(convention.ScaffoldDir, "templates", app)

	type fileBackup struct {
		path    string
		content []byte
		existed bool
	}
	var backups []fileBackup
	backupFile := func(path string) {
		content, rerr := os.ReadFile(path) //nolint:gosec // path is derived from convention.ScaffoldDir, a repo-relative constant, not user input
		backups = append(backups, fileBackup{path: path, content: content, existed: rerr == nil})
	}

	backupFile(configFile)
	_ = filepath.WalkDir(templateDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil //nolint:nilerr // filepath.WalkDir convention: skip entry, keep walking
		}
		backupFile(path)
		return nil
	})

	// Restored via defer, not just at the end of the happy path, so a
	// panic (or a future early-return added here later) can never leave
	// the merge-base content sitting in the working tree.
	defer func() {
		for _, b := range backups {
			if b.existed {
				_ = os.WriteFile(b.path, b.content, 0o600)
			} else {
				_ = os.Remove(b.path)
			}
		}
	}()

	if baseContent, showErr := git.ShowRefPath(ctx, mergeBase, configFile); showErr == nil {
		_ = os.WriteFile(configFile, baseContent, 0o600)
	}
	if treeOut, lsErr := exec.CommandContext(ctx, "git", "ls-tree", "-r", "--name-only", mergeBase, templateDir+"/").Output(); lsErr == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(treeOut)), "\n") {
			if f == "" {
				continue
			}
			if content, showErr := git.ShowRefPath(ctx, mergeBase, f); showErr == nil {
				_ = os.WriteFile(f, content, 0o600) //nolint:gosec // f comes from `git ls-tree` under templateDir, a repo-relative constant, not user input
			}
		}
	}

	log.Debug("scaffold baseline: re-running scaffold for %s against %s", app, mergeBase)
	summary := scaffold.Run(scaffold.RunOptions{
		App:      app,
		Trigger:  "baseline",
		Overlays: scaffold.FindOverlays(app),
		FullTest: true,
	})
	for _, ov := range summary.MismatchFiles {
		baseline[ov] = true
	}
	return baseline
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
