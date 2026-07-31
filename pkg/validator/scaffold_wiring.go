package validator

import (
	"fmt"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/configdiff"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
)

// scaffoldValidationResult aggregates every app's scaffold.Run outcome for
// this run's "Scaffold Validation" report section.
type scaffoldValidationResult struct {
	// DriftLines/ExecErrors are pre-formatted, one entry per drifted
	// overlay / execution failure - ComposeScaffoldValidationSection
	// expects a single driftSummary string, so runScaffoldValidation joins
	// DriftLines with newlines for that call.
	DriftLines []string
	ExecErrors []string
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
// A drifted overlay or scaffold-tool execution failure is always treated
// as blocking (a simpler, more conservative policy than trying to
// distinguish "pre-existing drift the PR didn't cause" - which would need
// re-running scaffold against the merge-base template/config, a real but
// substantially riskier technique this pass deliberately doesn't attempt).
func runScaffoldValidation(opts Options, apps, changed []string, log *logger.Logger) scaffoldValidationResult {
	changeGroups, _ := opts.Providers.ChangeGroups()
	workers := Workers(opts)
	result := scaffoldValidationResult{}
	tested := make(map[string]bool)
	var mu sync.Mutex

	record := func(app string, summary *scaffold.Summary) {
		mu.Lock()
		defer mu.Unlock()
		tested[app] = true
		for _, ov := range summary.MismatchFiles {
			result.DriftLines = append(result.DriftLines, fmt.Sprintf("%s: overlay `%s` drifted from its scaffold template/config", app, ov))
			log.ErrorInSection("Scaffold", "drift: %s/%s", app, ov)
		}
		for _, e := range summary.Errors {
			result.ExecErrors = append(result.ExecErrors, fmt.Sprintf("%s: %s", app, e))
			log.ErrorInSection("Scaffold", "%s: %s", app, e)
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
			jobs1 = append(jobs1, scaffoldJob{app: app, trigger: "fan-out", overlays: overlays})
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
			jobs2 = append(jobs2, scaffoldJob{app: aff.App, trigger: trigger, overlays: overlays})
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
			jobs3 = append(jobs3, scaffoldJob{app: app, trigger: trigger, overlays: overlays})
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
