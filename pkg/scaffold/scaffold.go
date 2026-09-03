// Package scaffold wraps a config-scaffolding CLI (scafctl by default, or
// an org-provided equivalent such as a vendored cldctl) to detect scaffold
// drift: overlays whose committed content no longer matches what the tool
// would generate from the current template/config. Two drift-detection
// strategies are supported (see DriftMode): DiffDirs (generate-to-tmp +
// directory diff, the scafctl contract) and DryRunParse (parse the tool's
// dry-run "would create" output), selectable because different scaffolding
// CLIs expose fundamentally different invocation and output contracts.
package scaffold

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
)

// Binary and ConfigSource are generic core defaults, deliberately mutable
// (not const) so an org layer can retarget this package at its own
// scaffolding CLI/config-source name (e.g. a vendored `cldctl` wrapping
// `scafctl` with an org-specific `--config-source`) via a Configure()-style
// package-var override, without needing a fork of this package.
var (
	Binary       = "scafctl"
	ConfigSource = "repo-config"
)

// DriftDetectionMode selects how Run detects scaffold drift for one app.
type DriftDetectionMode int

const (
	// DiffDirs (the generic default) generates every overlay into a temp
	// directory (Binary scaffold --config <ConfigSource>=<path> --output
	// <dir>) and compares each committed overlay against it. This is the
	// scafctl contract.
	DiffDirs DriftDetectionMode = iota
	// DryRunParse runs the scaffold tool in dry-run mode and parses its
	// "would create" output (see CreatedFileMarkers) to find overlays that
	// would be regenerated - i.e. drifted. This is for tools (e.g. a
	// vendored cldctl) whose dry-run output enumerates the files it would
	// write, rather than supporting the --output-to-dir contract DiffDirs
	// assumes. The per-run command is built by ScaffoldArgs.
	DryRunParse
)

// DriftMode selects Run's drift-detection strategy. Defaults to DiffDirs
// (the scafctl contract); an org layer whose scaffolding CLI behaves
// differently (see DryRunParse) overrides it via a Configure()-style seam,
// alongside ScaffoldArgs and CreatedFileMarkers.
var DriftMode = DiffDirs

// ScaffoldArgs builds the argument slice (after Binary) for a single
// DryRunParse invocation. fullTest selects the "all overlays in one shot"
// form (cluster is ""); otherwise it's a single-cluster run. Required when
// DriftMode is DryRunParse; ignored under DiffDirs. Org-injected.
var ScaffoldArgs func(app, cluster string, fullTest bool) []string

// CreatedFileMarkers are the substrings a DryRunParse tool prints before a
// path it would create; ExtractCreatedFiles treats the text after the
// marker as the created-file path. Defaults to the generic "created "
// prefix; an org layer sets its tool's own marker(s).
var CreatedFileMarkers = []string{"created "}

// HookKeyword names the test.sh hook function this package's build wiring
// looks for (see pkg/hook) - not an org-configurable seam, so it stays const.
const HookKeyword = "run_scafctl_scaffold"

// ExcludedClusters names overlay/cluster names that are always skipped by
// scaffold-drift validation, independent of IsOverlayDisabled/
// IsChangeGroupDisabled - e.g. clusters an org knows are permanently
// exempt from scaffold generation (a decommissioned or special-purpose
// cluster) rather than merely opted out via per-app config. Empty by
// default (the generic core skips nothing); an org layer may populate it
// from a Configure() seam.
var ExcludedClusters = map[string]bool{}

// IsExcludedCluster reports whether cluster is in ExcludedClusters.
func IsExcludedCluster(cluster string) bool {
	return ExcludedClusters[cluster]
}

// runTimeout bounds a single scafctl invocation (the whole app's overlays,
// generated in one shot into a temp dir - see Run) so a hung or slow
// scaffold-tool invocation can't stall the pipeline indefinitely. Each retry
// attempt gets its own fresh runTimeout budget (see retryExec).
const runTimeout = 2 * time.Minute

// RetryAttempts bounds how many total execution attempts are made per
// scaffold-tool invocation when it fails with a transient error signature
// (see IsTransientError). 1 disables retrying entirely. Generic default of 3;
// an org layer may tune it via a Configure()-style package-var override.
var RetryAttempts = 3

// RetryBackoff is the initial sleep between transient-failure attempts; it
// doubles after each failed attempt (attempts slept: base, 2x, 4x, ...).
var RetryBackoff = 3 * time.Second

// OnRetry, when set, is invoked after each transient failure that triggers a
// further attempt, primarily for observability (logging/reporting). It is a
// deliberate generic seam (nil by default) so callers can surface retry counts
// without the core printing to stdout.
var OnRetry func(attempt, maxAttempts int, err error)

// IsTransientError decides whether a failed scaffold-tool invocation's
// diagnostic text looks like a transient failure worth retrying (network
// EOF/reset/timeout during an in-tool remote fetch) rather than a genuine,
// non-transient error (bad config, real drift). Generic default matches the
// common transient network signatures; an org layer may override it if its
// tool emits a different signature. Implementations must NOT treat context
// deadline-exceeded as transient - a genuinely hung tool should fail fast, not
// extend CI wall-clock.
var IsTransientError = defaultIsTransientError

var transientErrorRe = regexp.MustCompile(`(?i)unexpected eof|connection reset|broken pipe|i/o timeout|no such host|connection refused|handshake timeout|temporary failure|temporarily unavailable`)

// defaultIsTransientError is the generic default for IsTransientError: a
// substring match against the common transient network signatures.
func defaultIsTransientError(text string) bool {
	return transientErrorRe.MatchString(text)
}

// retryExec runs fn (a single scaffold-tool execution attempt that returns its
// diagnostic output on success and an error on failure) up to RetryAttempts
// times, retrying only when the prior attempt failed and its diagnostic text
// matches IsTransientError - so a genuine failure still fails fast on attempt
// 1. OnRetry, if set, is called before each subsequent attempt. The retry
// configuration vars are snapshot into locals up front: this gives each call a
// self-consistent view and guarantees retryExec never itself writes/mutates the
// package vars. Callers must not concurrently reassign the package vars while
// retries are in flight - snapshotting does not make that safe (concurrent
// reads and writes of a variable are still a data race), it only means this
// function won't be the one mutating shared state.
func retryExec(fn func() (string, error)) (string, error) {
	retryAttempts := RetryAttempts
	if retryAttempts < 1 {
		retryAttempts = 1
	}
	retryBackoff := RetryBackoff
	isTransient := IsTransientError
	if isTransient == nil {
		isTransient = defaultIsTransientError
	}
	onRetry := OnRetry

	var (
		output string
		err    error
	)
	for attempt := 1; ; attempt++ {
		output, err = fn()
		// Classify against both the attempt's diagnostic output and the error
		// text: for exec failures the interesting transient signature (a
		// network EOF/reset from an in-tool remote fetch) typically appears in
		// the tool's captured stdout/stderr, while err.Error() alone is often
		// just "exit status N". The output is ANSI-stripped by the caller.
		// A context deadline (timeout) is never retried - even if partial
		// output happens to match a transient signature - so a genuinely hung
		// tool fails fast instead of silently extending wall-clock.
		if err == nil || attempt >= retryAttempts || errors.Is(err, context.DeadlineExceeded) || !isTransient(output+"\n"+err.Error()) {
			return output, err
		}
		if onRetry != nil {
			onRetry(attempt, retryAttempts, err)
		}
		time.Sleep(computeBackoff(retryBackoff, attempt))
	}
}

// maxBackoffShift caps the exponential backoff doubling at 2^20 (≈ a million
// times the base). Any larger shift means the sleep is already impractically
// long, so capping avoids overflowing a time.Duration (a signed int64) when an
// org overrides RetryAttempts to a very large value.
const maxBackoffShift = 20

// computeBackoff returns the sleep between retry attempts: base doubled
// attempt-1 times. A non-positive base yields zero sleep (no backoff). Growth
// is clamped at maxBackoffShift and the result is clamped to the max
// time.Duration on overflow, so overridden settings can't produce a negative
// or undefined sleep.
func computeBackoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 || attempt <= 0 {
		return 0
	}
	shift := attempt - 1
	if shift > maxBackoffShift {
		shift = maxBackoffShift
	}
	factor := int64(1) << uint(shift)
	d := base * time.Duration(factor)
	if base != 0 && d/time.Duration(factor) != base {
		return time.Duration(math.MaxInt64)
	}
	return d
}

// RunOptions configures one app's scaffold-drift run.
type RunOptions struct {
	App     string
	Trigger string // caller-supplied label (e.g. "overlay", "fan-out", "config-changeGroup") for logging/reporting; not interpreted here.
	// Overlays lists the cluster/overlay names (matching <App>/overlays/<name>)
	// to check for drift. An overlay disabled via IsOverlayDisabled or
	// IsChangeGroupDisabled, or with no corresponding on-disk directory (a
	// cluster not yet rolled out, or removed by this PR), is skipped rather
	// than treated as a failure - see Summary.Skipped/SkippedClusters.
	Overlays []string
	// ChangedFiles is this run's full changeset, consulted only for
	// diagnostic purposes today (kept for callers that want to correlate
	// drift with what the PR actually touched).
	ChangedFiles []string
	// ChangeGroups is the org's cluster-name -> change-group mapping (see
	// provider.Providers.ChangeGroups) - a cluster mapped to group 0 is
	// treated as opted out of scaffold validation (IsChangeGroupDisabled).
	// nil/empty disables this filter entirely (the generic default).
	ChangeGroups map[string]int
	// FullTest requests the "all overlays in one shot" form under the
	// DryRunParse drift mode (see ScaffoldArgs' fullTest param) - set by the
	// caller for template/change-group fan-out triggers where every overlay
	// of the app is re-checked at once. Ignored under DiffDirs.
	FullTest bool
}

// Summary aggregates one app's Run outcome across every overlay checked.
type Summary struct {
	App                            string
	Total, Passed, Skipped, Failed int
	MismatchFiles                  []string // overlay-relative paths that differ from freshly-scaffolded content
	Errors                         []string // execution failures (scafctl missing/failed, timeout, ...), distinct from content drift
	SkippedClusters                []string // overlays skipped: disabled (scaffoldDisabled/change-group/excluded) or no on-disk directory yet
	DisabledClusters               []string // overlays skipped because their scaffold config marks them disabled (`overlayDefinitions.overrides.<cluster>.disabled`); a subset of SkippedClusters, kept distinct so callers can warn on them specifically
}

// runScafctl is the actual scafctl invocation, factored into a package var
// so tests can substitute a fake without needing the real binary installed
// (matching the "org-injectable override" pattern used elsewhere in this
// repo, e.g. overlay.SecretAuthHint) - scafctl is an external, org-provided
// tool this package has no control over the availability of.
var runScafctl = execScafctl

// execScafctl runs `scafctl scaffold --config repo-config=<configPath>
// --output <outputDir>`, generating every overlay's scaffolded content
// under outputDir/<overlay>/... in one shot.
func execScafctl(ctx context.Context, configPath, outputDir string) error {
	cmd := exec.CommandContext(ctx, Binary, "scaffold", "--config", ConfigSource+"="+configPath, "--output", outputDir) //nolint:gosec // Binary/ConfigSource are operator-controlled package-level overrides, not user input
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

// HasScaffoldEnabled reports whether app's test.sh opts into scaffold-drift
// validation (SCAFFOLD= - see docs/HOOKS.md; defaults to enabled when
// test.sh is absent or doesn't mention it). This is independent of whether
// app actually has a scafctl config at all - see HasScaffoldConfig, the
// gate callers should also check before calling Run.
func HasScaffoldEnabled(app string) bool {
	cfg, err := hook.ParseTestScript(filepath.Join(app, "test.sh"))
	if err != nil {
		return false
	}
	return cfg.Scaffold
}

// HasScaffoldConfig reports whether app has opted into scafctl-based
// scaffolding at all (a .scafctl/configs/<app>.{yaml,yml} file exists).
// Callers should check this before calling Run - an app with no scaffold
// config was never scaffolded in the first place, so Run erroring on a
// missing config for it would be a false positive, not real drift.
func HasScaffoldConfig(app string) bool {
	return configFilePath(app) != ""
}

// scaffoldConfigFields is the subset of an app's scafctl config
// (.scafctl/configs/<app>.yaml) this package reads directly, alongside
// whatever scafctl's own schema defines - a config file is both scafctl's
// real input and, via these additional keys, this CI tool's own opt-out /
// disable convention. Unknown keys are ignored by scafctl (config tools
// universally tolerate this), so adding scaffoldDisabled and the overlay-
// definitions override fields here doesn't require any scafctl-side change.
type scaffoldConfigFields struct {
	// ScaffoldDisabled lists overlay/cluster names to skip scaffold-drift
	// validation for, even though their overlay directory exists - e.g. a
	// cluster mid-migration whose overlay is intentionally hand-maintained
	// for now. See IsOverlayDisabled.
	ScaffoldDisabled []string `yaml:"scaffoldDisabled"`
	// OverlayDefinitions mirrors the config's per-overlay override section,
	// whose per-cluster `disabled: true` (when set) opts an overlay out of
	// scaffold-drift validation even though its directory exists - see
	// IsOverlayConfigDisabled and OverlayConfigDisabled. The nested shape is
	// read with generic yaml tags only; the exact section/field paths are
	// the hardcoded scaffold-config schema this repo tracks over time (see
	// the config-layout seam discussion in the tracker issue) and are
	// re-consulted by the OverlayConfigDisabled default.
	OverlayDefinitions struct {
		Overrides map[string]struct {
			Disabled bool `yaml:"disabled"`
		} `yaml:"overrides"`
	} `yaml:"overlayDefinitions"`
}

// configFilePath returns the on-disk path to app's scafctl config, or ""
// if none of the recognized extensions exist.
func configFilePath(app string) string {
	for _, ext := range []string{".yaml", ".yml"} {
		p := filepath.Join(convention.ScaffoldDir, "configs", app+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// readScaffoldConfigFields parses app's scafctl config for the fields this
// package reads. A missing config, or one with no scaffoldDisabled key,
// returns a zero-value (nothing disabled) rather than an error - config-
// driven disabling is opt-in.
func readScaffoldConfigFields(app string) scaffoldConfigFields {
	path := configFilePath(app)
	if path == "" {
		return scaffoldConfigFields{}
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from convention.ScaffoldDir, a repo-relative constant, not user input
	if err != nil {
		return scaffoldConfigFields{}
	}
	var fields scaffoldConfigFields
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return scaffoldConfigFields{}
	}
	return fields
}

// IsOverlayDisabled reports whether app's scafctl config opts cluster out
// of scaffold-drift validation via a top-level scaffoldDisabled list (see
// scaffoldConfigFields). A missing config or key means nothing is disabled.
func IsOverlayDisabled(app, cluster string) bool {
	for _, c := range readScaffoldConfigFields(app).ScaffoldDisabled {
		if c == cluster {
			return true
		}
	}
	return false
}

// OverlayConfigDisabled reports whether app's scaffold config marks cluster
// as disabled for overlay generation (e.g. a `disabled: true` flag under
// the per-overlay override entry), so scaffold-drift validation skips it
// even though its on-disk overlay directory exists. It is an injected
// package-var seam (defaulting to defaultOverlayConfigDisabled) so an org
// whose scaffold tool structures its config differently can override the
// lookup - the generic default reads the widely-used
// `overlayDefinitions.overrides.<cluster>.disabled` shape, which is the
// same schema-layout documented in the tracker issue for the config-layout
// seam. See IsOverlayConfigDisabled for the raw read.
var OverlayConfigDisabled = defaultOverlayConfigDisabled

// defaultOverlayConfigDisabled is the generic default for
// OverlayConfigDisabled: it reads the `overlayDefinitions.overrides.<cluster>.disabled`
// flag from app's scaffold config. This layout is the de-facto default the
// rest of the repo already assumes (see pkg/config and pkg/configdiff); a
// config absent of the overlay-definitions section yields false for every
// cluster.
func defaultOverlayConfigDisabled(app, cluster string) bool {
	return readScaffoldConfigFields(app).
		OverlayDefinitions.
		Overrides[cluster].
		Disabled
}

// IsOverlayConfigDisabled reports whether app's scaffold config marks
// cluster disabled via the `overlayDefinitions.overrides.<cluster>.disabled`
// flag (see defaultOverlayConfigDisabled). It is the non-overridable raw
// read of that specific shape; callers wanting the seam (org-overridable,
// documented) should use OverlayConfigDisabled instead.
func IsOverlayConfigDisabled(app, cluster string) bool {
	return defaultOverlayConfigDisabled(app, cluster)
}

// IsChangeGroupDisabled reports whether cluster is mapped to change-group 0
// in changeGroups - this repo's convention (shared with pkg/configdiff) for
// "this cluster opted out of change-group-triggered scaffold fan-out".
// A cluster absent from changeGroups (or a nil/empty map, the default with
// no ClusterMetadata provider wired) is never considered disabled by this
// check alone.
func IsChangeGroupDisabled(cluster string, changeGroups map[string]int) bool {
	group, ok := changeGroups[cluster]
	return ok && group == 0
}

// overlayDir returns app's on-disk overlay directory for cluster.
func overlayDir(app, cluster string) string {
	return filepath.Join(app, "overlays", cluster)
}

// overlayExists reports whether app has an on-disk overlay directory for
// cluster - false for a cluster not yet rolled out (referenced by config
// but not yet merged) or removed by this PR (deleted by the diff being
// validated); either way there's nothing to compare scaffolded content
// against, so Run skips it rather than failing.
func overlayExists(app, cluster string) bool {
	info, err := os.Stat(overlayDir(app, cluster))
	return err == nil && info.IsDir()
}

// Run generates opts.App's overlays via scafctl (once, into a temp
// directory - the existing, real invocation contract) and compares each of
// opts.Overlays against the freshly-generated content, bounded-parallel
// (up to runtime.NumCPU()*2 at once) since the comparison itself (a
// recursive directory diff) is independent per overlay. An overlay is
// skipped - counted in Summary.Skipped/SkippedClusters, never Failed -
// when it's disabled (OverlayConfigDisabled/IsOverlayDisabled/
// IsChangeGroupDisabled/IsExcludedCluster) or has no on-disk directory
// (overlayExists); an overlay skipped purely because its config marks it
// disabled is additionally recorded in Summary.DisabledClusters so callers
// can distinguish "opted out on purpose" from "not rolled out yet". Everything
// else is either a content
// mismatch (Summary.MismatchFiles) or an execution failure
// (Summary.Errors, e.g. a missing scafctl binary or a run that itself
// failed/timed out).
func Run(opts RunOptions) *Summary {
	summary := &Summary{App: opts.App}

	configPath := configFilePath(opts.App)
	if configPath == "" {
		summary.Errors = append(summary.Errors, fmt.Sprintf("scaffold config not found for %s", opts.App))
		summary.Failed = len(opts.Overlays)
		summary.Total = len(opts.Overlays)
		return summary
	}

	var toRun []string
	for _, cluster := range opts.Overlays {
		switch {
		// A missing directory (not yet rolled out, or removed by this PR)
		// takes precedence over a config-disabled flag: such an overlay is a
		// plain skip, never a DisabledClusters warning - warning about a
		// deleted overlay would be misleading.
		case !overlayExists(opts.App, cluster):
			summary.Skipped++
			summary.SkippedClusters = append(summary.SkippedClusters, cluster)
		case OverlayConfigDisabled(opts.App, cluster):
			summary.Skipped++
			summary.SkippedClusters = append(summary.SkippedClusters, cluster)
			summary.DisabledClusters = append(summary.DisabledClusters, cluster)
		case IsOverlayDisabled(opts.App, cluster), IsChangeGroupDisabled(cluster, opts.ChangeGroups), IsExcludedCluster(cluster):
			summary.Skipped++
			summary.SkippedClusters = append(summary.SkippedClusters, cluster)
		default:
			toRun = append(toRun, cluster)
		}
	}
	summary.Total = len(opts.Overlays)
	if len(toRun) == 0 {
		return summary
	}

	switch DriftMode {
	case DryRunParse:
		runDryRunParse(opts, toRun, summary)
	default:
		runDiffDirs(opts, configPath, toRun, summary)
	}
	return summary
}

// runDiffDirs is the generic (scafctl) drift-detection body: generate every
// overlay in toRun into a temp directory in one shot, then compare each
// committed overlay against the freshly-generated content, bounded-parallel.
func runDiffDirs(opts RunOptions, configPath string, toRun []string, summary *Summary) {
	tmp, err := os.MkdirTemp("", "scaffold-*")
	if err != nil {
		summary.Errors = append(summary.Errors, err.Error())
		summary.Failed += len(toRun)
		return
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	out, err := retryExec(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		if runErr := runScafctl(ctx, configPath, tmp); runErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				// A hung tool is a timeout, not a transient blip - never retry.
				return stripANSI(runErr.Error()), fmt.Errorf("%w: %s", context.DeadlineExceeded, stripANSI(runErr.Error()))
			}
			return stripANSI(runErr.Error()), runErr
		}
		return "", nil
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// A hung tool is a timeout (intentionally not retried) - report it
			// clearly so the user understands why it stopped.
			summary.Errors = append(summary.Errors, fmt.Sprintf("scaffold timed out for %s (%s)", opts.App, runTimeout))
		} else {
			summary.Errors = append(summary.Errors, fmt.Sprintf("scaffold command failed: %s", out))
		}
		summary.Failed += len(toRun)
		return
	}

	workers := runtime.NumCPU() * 2
	if workers > len(toRun) {
		workers = len(toRun)
	}
	jobs := make(chan string, len(toRun))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cluster := range jobs {
				diff, diffErr := diffDirs(filepath.Join(tmp, cluster), overlayDir(opts.App, cluster))
				mu.Lock()
				switch {
				case diffErr != nil:
					summary.Failed++
					summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %s", cluster, diffErr))
				case diff != "":
					summary.Failed++
					summary.MismatchFiles = append(summary.MismatchFiles, cluster)
				default:
					summary.Passed++
				}
				mu.Unlock()
			}
		}()
	}
	for _, cluster := range toRun {
		jobs <- cluster
	}
	close(jobs)
	wg.Wait()
}

// runDryRunParse is the dry-run/parse drift-detection body: it invokes the
// scaffold tool in dry-run mode (args from ScaffoldArgs) and treats every
// file the tool reports it would create (ExtractCreatedFiles, per
// CreatedFileMarkers) as evidence its overlay drifted from the committed
// content. It classifies each such overlay as a mismatch (the overlay
// exists on disk, or was deleted by this PR) or a skip (a cluster not yet
// rolled out). opts.FullTest selects the single "all overlays at once"
// invocation (the tool enumerates drift across every cluster) vs. a
// per-cluster invocation for each overlay in toRun (bounded-parallel).
func runDryRunParse(opts RunOptions, toRun []string, summary *Summary) {
	if ScaffoldArgs == nil {
		summary.Errors = append(summary.Errors, "scaffold DryRunParse mode requires ScaffoldArgs to be set")
		summary.Failed += len(toRun)
		return
	}

	if opts.FullTest {
		runDryRunFull(opts, toRun, summary)
		return
	}

	workers := runtime.NumCPU() * 2
	if workers > len(toRun) {
		workers = len(toRun)
	}
	jobs := make(chan string, len(toRun))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cluster := range jobs {
				status, files, errMsg := dryRunOneCluster(opts.App, cluster, opts.ChangedFiles)
				mu.Lock()
				switch status {
				case "passed":
					summary.Passed++
				case "skipped":
					summary.Skipped++
					summary.SkippedClusters = append(summary.SkippedClusters, cluster)
				case "mismatch":
					summary.Failed++
					summary.MismatchFiles = append(summary.MismatchFiles, files...)
				default: // "error" - a genuine tool/execution failure
					summary.Failed++
					if errMsg != "" {
						summary.Errors = append(summary.Errors, errMsg)
					}
				}
				mu.Unlock()
			}
		}()
	}
	for _, cluster := range toRun {
		jobs <- cluster
	}
	close(jobs)
	wg.Wait()
}

// runDryRunFull runs a single "all overlays" dry-run for the whole app and
// maps each would-create file back to its overlay, classifying it as a
// mismatch (overlay exists / deleted by this PR) or a skip (new cluster not
// yet rolled out). A tool execution failure fails every toRun overlay.
func runDryRunFull(opts RunOptions, toRun []string, summary *Summary) {
	args := ScaffoldArgs(opts.App, "", true)
	out, err := retryExec(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, Binary, args...) //nolint:gosec // Binary/ScaffoldArgs are operator-controlled package-level overrides, not user input
		cmd.Env = os.Environ()
		b, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil && ctx.Err() == context.DeadlineExceeded {
			// A hung tool is a timeout, not a transient blip - never retry.
			cmdErr = fmt.Errorf("%w: %s", context.DeadlineExceeded, stripANSI(string(b)))
		}
		return stripANSI(string(b)), cmdErr
	})
	output := out
	if err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("scaffold command failed for %s: %s", opts.App, output))
		summary.Failed += len(toRun)
		return
	}

	created := ExtractCreatedFiles(output)
	if len(created) == 0 {
		summary.Passed += len(toRun)
		return
	}

	var mismatches []string
	seenMismatch := map[string]bool{}
	skipped := map[string]bool{}
	for _, f := range created {
		cluster := ExtractOverlayDir(f)
		switch {
		case cluster == "":
			// A would-create file outside any overlays/<cluster>/ path has
			// no cluster name to normalize to; keep the raw path so the
			// finding isn't silently dropped.
			if !seenMismatch[f] {
				seenMismatch[f] = true
				mismatches = append(mismatches, f)
			}
		case overlayExists(opts.App, cluster), IsInChangedFiles(cluster, opts.ChangedFiles):
			// Normalize to the overlay/cluster name (matching DiffDirs mode
			// and MismatchFiles' documented contract), deduped per cluster.
			if !seenMismatch[cluster] {
				seenMismatch[cluster] = true
				mismatches = append(mismatches, cluster)
			}
		default:
			if !skipped[cluster] {
				skipped[cluster] = true
				summary.Skipped++
				summary.SkippedClusters = append(summary.SkippedClusters, cluster)
			}
		}
	}

	if len(mismatches) > 0 {
		summary.Failed += len(mismatches)
		summary.MismatchFiles = append(summary.MismatchFiles, mismatches...)
	} else {
		summary.Passed += len(toRun)
	}
}

// dryRunOneCluster runs a single-cluster dry-run and classifies the result.
// Returns ("passed"|"skipped"|"mismatch"|"error", mismatchFiles, errMsg).
// "mismatch" is drift (the overlay would be (re)created) - reported via
// MismatchFiles and classified downstream (blocking vs. pre-existing);
// "error" is a genuine tool/execution failure, always blocking.
func dryRunOneCluster(app, cluster string, changedFiles []string) (status string, mismatchFiles []string, errMsg string) {
	args := ScaffoldArgs(app, cluster, false)
	out, err := retryExec(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, Binary, args...) //nolint:gosec // Binary/ScaffoldArgs are operator-controlled package-level overrides, not user input
		cmd.Env = os.Environ()
		b, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil && ctx.Err() == context.DeadlineExceeded {
			// A hung tool is a timeout, not a transient blip - never retry.
			cmdErr = fmt.Errorf("%w: %s", context.DeadlineExceeded, stripANSI(string(b)))
		}
		return stripANSI(string(b)), cmdErr
	})
	output := out

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "error", nil, fmt.Sprintf("scaffold timed out for %s (%s)", cluster, runTimeout)
		}
		// If the overlay was deleted by this PR, a scaffold failure for it
		// is the expected outcome of that removal, not real drift.
		if !overlayExists(app, cluster) {
			return "passed", nil, ""
		}
		return "error", nil, fmt.Sprintf("scaffold command failed for %s: %s", cluster, output)
	}

	created := ExtractCreatedFiles(output)
	if len(created) == 0 {
		return "passed", nil, ""
	}

	// A new cluster not yet rolled out (no on-disk overlay, not touched by
	// this PR) is a skip, not drift.
	if !overlayExists(app, cluster) && !IsInChangedFiles(cluster, changedFiles) {
		return "skipped", nil, ""
	}
	// Drift: the overlay would be (re)created. Report it as a mismatch (by
	// overlay/cluster name, matching MismatchFiles' contract) rather than an
	// execution error, so it flows through the drift classification (which
	// can downgrade untouched, pre-existing drift to non-blocking) instead
	// of unconditionally blocking as an exec failure.
	return "mismatch", []string{cluster}, ""
}

func diffDirs(generated, committed string) (string, error) {
	if _, err := os.Stat(generated); err != nil {
		return "", err
	}
	if _, err := os.Stat(committed); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(context.Background(), "diff", "-rq", generated, committed) //nolint:gosec // both paths are derived from this package's own temp dir + convention-based overlay layout, not user input
	out, _ := cmd.Output()
	return stripANSI(string(out)), nil
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// ExtractCreatedFiles parses a scaffold tool's dry-run output for the paths
// it would create, one per line: a line is a match when it contains any of
// CreatedFileMarkers, and the created-file path is the (trimmed) text after
// that marker. The default marker ("created ") matches the generic scafctl
// text-report form; org layers with a differently-worded dry-run output
// (e.g. cldctl's "Created File:") set CreatedFileMarkers accordingly. Used
// by Run's DryRunParse mode.
func ExtractCreatedFiles(output string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		for _, marker := range CreatedFileMarkers {
			if idx := strings.Index(line, marker); idx >= 0 {
				if p := strings.TrimSpace(line[idx+len(marker):]); p != "" {
					out = append(out, p)
				}
				break
			}
		}
	}
	return out
}

// ExtractOverlayDir returns the overlay/cluster name from a scaffold-
// generated file path (".../overlays/<cluster>/...").
func ExtractOverlayDir(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i, p := range parts {
		if p == "overlays" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// IsInChangedFiles reports whether overlayDir appears in changedFiles.
func IsInChangedFiles(overlayDir string, changedFiles []string) bool {
	for _, f := range changedFiles {
		if strings.Contains(f, "/overlays/"+overlayDir+"/") {
			return true
		}
	}
	return false
}

// ChangedOverlayNames extracts the set of overlay/cluster names touched
// under app/overlays/ in files, deduplicated and in first-seen order.
func ChangedOverlayNames(app string, files []string) []string {
	prefix := app + "/overlays/"
	seen := make(map[string]bool)
	var out []string
	for _, f := range files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := f[len(prefix):]
		if idx := strings.Index(rest, "/"); idx != -1 {
			rest = rest[:idx]
		}
		if rest != "" && !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out
}

// FindOverlays lists every overlay/cluster name under app/overlays/.
func FindOverlays(app string) []string {
	dir := filepath.Join(app, "overlays")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}
