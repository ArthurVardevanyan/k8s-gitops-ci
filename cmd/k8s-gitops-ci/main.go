package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/config"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/csv"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/ghostpatch"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/kustomize"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/golangci"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/markdownlint"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/prettier"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/shellcheck"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/yamlsyntax"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/pipeline"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/nad"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "pipeline", "ci":
		err = runPipeline(args)
	case "build-yaml":
		err = runBuildYAML(args)
	case "test-all":
		err = runTestAll(args)
	case "scan-all":
		err = runScanAll(args)
	case "markdownlint":
		err = runMarkdownlint(args)
	case "prettier":
		err = runPrettier(args)
	case "shellcheck":
		err = runShellcheck(args)
	case "golangci":
		err = runGolangci(args)
	case "kubeconform":
		err = runKubeconform(args)
	case "kustomize-fix":
		err = runKustomizeFix(args)
	case "check-starting-csv":
		err = runCheckStartingCSV(args)
	case "ghost-patches":
		err = runGhostPatches(args)
	case "update-scaffold-status":
		err = runUpdateScaffoldStatus(args)
	case "sort-configs":
		err = runSortConfigs(args)
	case "yaml-syntax":
		err = runYAMLSyntax(args)
	case "validate-nad":
		err = runValidateNAD(args)
	case "version", "--version", "-v":
		fmt.Println(version.String())
	case "--help", "-h", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── pipeline ──────────────────────────────────────────────────────────────────

func runPipeline(args []string) error {
	fs := flag.NewFlagSet("pipeline", flag.ExitOnError)
	opts := pipeline.EnvOptions()
	opts.Providers = provider.Providers{}

	var dirs, disableChecks, enableChecks string
	fs.StringVar(&opts.URL, "url", opts.URL, "repository URL (e.g. https://github.com/org/repo — NOT a PR URL; pass the PR number via --pr)")
	fs.StringVar(&opts.PR, "pr", opts.PR, "pull request number")
	fs.StringVar(&opts.Revision, "revision", opts.Revision, "git revision")
	fs.StringVar(&opts.TargetBranch, "target-branch", opts.TargetBranch, "target branch")
	fs.StringVar(&opts.HookSource, "hook-source", opts.HookSource, "hook source (main|pr|local)")
	fs.StringVar(&opts.TriggerComment, "trigger-comment", opts.TriggerComment, "trigger comment text")
	fs.StringVar(&dirs, "dirs", "", "comma-separated path prefixes to restrict the changeset to (e.g. kubernetes/,tekton/,.tekton/,okd/)")
	fs.BoolVar(&opts.LintOnly, "lint-only", false, "lint only, skip build checks")
	fs.BoolVar(&opts.PostComment, "comment", false, "post PR comment (default: off)")
	fs.BoolVar(&opts.Verbose, "verbose", false, "verbose output")
	fs.BoolVar(&opts.AssumeOpenShift, "assume-openshift", false, "treat OpenShift/OKD-default-but-portable API groups (OLM, Prometheus Operator, Gateway API, SR-IOV/Multus/OVN-Kubernetes CNI, Metal3) as exempt from the sync-options check, in addition to the always-exempt OpenShift-exclusive groups (route.openshift.io, config.openshift.io, ...); only enable if ALL target clusters are OpenShift/OKD")
	fs.StringVar(&disableChecks, "disable-checks", "", "comma-separated IDs to disable entirely (e.g. sync-options, golangci, avp); only affects checks/steps that default to enabled")
	fs.StringVar(&enableChecks, "enable-checks", "", "comma-separated IDs to explicitly enable; only affects checks/steps that default to disabled (e.g. kyverno)")
	fs.IntVar(&opts.Concurrency, "concurrency", 0, "worker concurrency (0=auto)")
	fs.Var(newStringSliceFlag(&opts.Apps), "app", "app name to scope validation to (repeatable: --app a --app b)")
	fs.Var(newStringSliceFlag(&opts.Clusters), "cluster", "cluster name to scope validation to (repeatable: --cluster a --cluster b)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts.IncludePrefixes = splitCommaList(dirs)
	opts.DisabledChecks = splitCommaList(disableChecks)
	opts.EnabledChecks = splitCommaList(enableChecks)
	return pipeline.Run(opts)
}

// splitCommaList splits a comma-separated flag value, trimming whitespace and
// dropping empty entries.
func splitCommaList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// stringSliceFlag implements flag.Value, collecting each occurrence of a
// repeatable flag (e.g. --app foo --app bar) into the backing slice, in the
// order given.
type stringSliceFlag struct {
	values *[]string
}

func newStringSliceFlag(values *[]string) *stringSliceFlag {
	return &stringSliceFlag{values: values}
}

func (s *stringSliceFlag) String() string {
	if s.values == nil {
		return ""
	}
	return strings.Join(*s.values, ",")
}

func (s *stringSliceFlag) Set(v string) error {
	*s.values = append(*s.values, v)
	return nil
}

// ── build-yaml ────────────────────────────────────────────────────────────────

func runBuildYAML(args []string) error {
	fs := flag.NewFlagSet("build-yaml", flag.ExitOnError)
	var app, cluster string
	var verbose bool
	fs.StringVar(&app, "app", "", "app name")
	fs.StringVar(&cluster, "cluster", "", "cluster name")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	start := time.Now()
	fmt.Println(version.String())
	opts := validator.Options{Apps: []string{app}, Clusters: []string{cluster}, Verbose: verbose}
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	printAllSectionsConsole(res.Logger, res.Sections)
	printRunFooter(res, start)
	return nil
}

// printRunFooter prints the run's TimingCollector.Summary() table (the
// "Step/Duration/Mode" timing breakdown - see pkg/validator/timing.go,
// fully implemented but previously never invoked outside tests) followed by
// Logger.Summary(), for test-all/build-yaml/scan-all - the same footer
// pipeline.Run already prints (there via vr.Logger directly), giving all
// four entry points parity instead of only "pipeline" showing timing/
// summary detail. start is the time.Time captured before RunAll was
// called, used as the timing table's wall-clock total.
func printRunFooter(res *validator.Result, start time.Time) {
	if res == nil || res.Logger == nil {
		return
	}
	if res.Timing != nil {
		if summary := res.Timing.Summary(time.Since(start)); summary != "" {
			res.Logger.Raw(summary)
		}
	}
	fmt.Println(res.Logger.Summary(len(res.Sections), res.FailedSectionCount()))
}

// printAllSectionsConsole prints every section's result to the console: a
// compact "✅ Name: passed" line for passing sections (full detail was
// already streamed live by log during RunAll - see phases.go - so repeating
// it here would just duplicate that output in a different style), and the
// full console-sanitized (see pipeline.SanitizeSectionBodyForConsole) Body
// under a log.SubHeader box for failing ones, matching the "====\n Title\n
// ===="/"----\n Title\n----" banner family log already uses for phases -
// this is the build-yaml/test-all rendering. Split out from its callers so
// the console-vs-PR-markdown handling is unit-testable without invoking
// validator.RunAll (which shells out to git).
func printAllSectionsConsole(log *logger.Logger, sections []validator.Section) {
	for _, s := range sections {
		if s.Error {
			printFailedSectionConsole(log, s)
			continue
		}
		printPassedSectionConsole(log, s.Name)
	}
}

// printFailedSectionsConsole prints only the errored sections' full detail
// (see printAllSectionsConsole) - the scan-all rendering, which (unlike
// test-all/build-yaml) omits passing sections entirely rather than even a
// one-line summary. See printAllSectionsConsole for why this is split out
// from its caller.
func printFailedSectionsConsole(log *logger.Logger, sections []validator.Section) {
	for _, s := range sections {
		if s.Error {
			printFailedSectionConsole(log, s)
		}
	}
}

// printPassedSectionConsole prints a single-line "✅ Name: passed" summary
// for a section that produced no blocking findings - the per-check detail
// was already streamed live via log during RunAll, so this is intentionally
// terse rather than repeating that detail a second time.
func printPassedSectionConsole(log *logger.Logger, name string) {
	line := "✅ " + name + ": passed"
	if log != nil {
		log.Raw(line)
		return
	}
	fmt.Println(line)
}

// printFailedSectionConsole prints a section's console-sanitized Body under
// a log.SubHeader(s.Name) box, so every console entry point (test-all,
// scan-all, build-yaml, and pipeline --verbose's printFailedSectionDetail)
// shares one consistent header style instead of each inventing its own.
func printFailedSectionConsole(log *logger.Logger, s validator.Section) {
	body := pipeline.SanitizeSectionBodyForConsole(s.Body)
	if log != nil {
		log.Raw("")
		log.SubHeader(s.Name)
		log.Raw(body)
		return
	}
	fmt.Printf("\n--- %s ---\n%s\n", s.Name, body)
}

// ── test-all / scan-all ───────────────────────────────────────────────────────

// validatorFlagSet holds the flags shared by test-all and scan-all: the same
// changeset-scoping and check-enablement flags "pipeline" exposes, so a
// failing pipeline run can be reproduced with test-all/scan-all (and vice
// versa) using an equivalent flag set, instead of test-all/scan-all only
// exposing --verbose.
type validatorFlagSet struct {
	url, pr, targetBranch, hookSource string
	dirs, disableChecks, enableChecks string
	concurrency                       int
	assumeOpenshift, verbose          bool
	apps, clusters                    []string
}

// bindValidatorFlags registers the shared flags on fs and returns the
// backing struct to read after fs.Parse.
func bindValidatorFlags(fs *flag.FlagSet) *validatorFlagSet {
	v := &validatorFlagSet{}
	fs.StringVar(&v.url, "url", "", "repository URL (e.g. https://github.com/org/repo — NOT a PR URL; pass the PR number via --pr)")
	fs.StringVar(&v.pr, "pr", "", "pull request number")
	fs.StringVar(&v.targetBranch, "target-branch", "", "target branch")
	fs.StringVar(&v.hookSource, "hook-source", "", "hook source (main|pr|local)")
	fs.StringVar(&v.dirs, "dirs", "", "comma-separated path prefixes to restrict the changeset to (e.g. kubernetes/,tekton/,.tekton/,okd/)")
	fs.StringVar(&v.disableChecks, "disable-checks", "", "comma-separated IDs to disable entirely (e.g. sync-options, golangci, avp); only affects checks/steps that default to enabled")
	fs.StringVar(&v.enableChecks, "enable-checks", "", "comma-separated IDs to explicitly enable; only affects checks/steps that default to disabled (e.g. kyverno)")
	fs.IntVar(&v.concurrency, "concurrency", 0, "worker concurrency (0=auto)")
	fs.BoolVar(&v.assumeOpenshift, "assume-openshift", false, "treat OpenShift/OKD-default-but-portable API groups (OLM, Prometheus Operator, Gateway API, SR-IOV/Multus/OVN-Kubernetes CNI, Metal3) as exempt from the sync-options check, in addition to the always-exempt OpenShift-exclusive groups (route.openshift.io, config.openshift.io, ...); only enable if ALL target clusters are OpenShift/OKD")
	fs.BoolVar(&v.verbose, "verbose", false, "verbose output")
	fs.Var(newStringSliceFlag(&v.apps), "app", "app name to scope validation to (repeatable: --app a --app b)")
	fs.Var(newStringSliceFlag(&v.clusters), "cluster", "cluster name to scope validation to (repeatable: --cluster a --cluster b)")
	return v
}

// applyTo copies the parsed flags onto opts. Dirs (the positional,
// full-tree-walk changeset source) is deliberately left untouched here -
// callers that support it (test-all) set opts.Dirs separately from
// fs.Args(), since it's a distinct changeset source from --dirs
// (IncludePrefixes, which filters a git-diff/PR changeset rather than
// replacing it).
func (v *validatorFlagSet) applyTo(opts *validator.Options) {
	opts.RepoURL = v.url
	opts.PR = v.pr
	opts.BaseRef = v.targetBranch
	opts.HookSource = v.hookSource
	opts.IncludePrefixes = splitCommaList(v.dirs)
	opts.DisabledChecks = splitCommaList(v.disableChecks)
	opts.EnabledChecks = splitCommaList(v.enableChecks)
	opts.Concurrency = v.concurrency
	opts.AssumeOpenShift = v.assumeOpenshift
	opts.Verbose = v.verbose
	opts.Apps = v.apps
	opts.Clusters = v.clusters
}

// parseTestAllOptions parses test-all's flags (a superset of scan-all's:
// same scoping/check-enablement flags as "pipeline", plus positional
// [dirs...]) into a validator.Options, without running anything - split out
// from runTestAll so the flag-to-Options wiring is unit-testable without
// invoking validator.RunAll (which shells out to git).
func parseTestAllOptions(args []string) (validator.Options, error) {
	fs := flag.NewFlagSet("test-all", flag.ExitOnError)
	vf := bindValidatorFlags(fs)
	if err := fs.Parse(args); err != nil {
		return validator.Options{}, err
	}
	var opts validator.Options
	vf.applyTo(&opts)
	// Positional [dirs...] args are a full-tree walk under those paths
	// (bypassing git diff entirely) - kept for backward compatibility
	// alongside the new --dirs flag, which instead filters a git-diff/PR
	// changeset (see resolveChangeset in pkg/validator/validator.go).
	opts.Dirs = fs.Args()
	return opts, nil
}

func runTestAll(args []string) error {
	opts, err := parseTestAllOptions(args)
	if err != nil {
		return err
	}
	start := time.Now()
	fmt.Println(version.String())
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	printAllSectionsConsole(res.Logger, res.Sections)
	printRunFooter(res, start)
	if res.Blocking {
		return fmt.Errorf("test-all: validation failed")
	}
	return nil
}

// parseScanAllOptions parses scan-all's flags (the same scoping/check-
// enablement flags as "pipeline", minus positional dirs) into a
// validator.Options, without running anything - see parseTestAllOptions.
func parseScanAllOptions(args []string) (validator.Options, error) {
	fs := flag.NewFlagSet("scan-all", flag.ExitOnError)
	vf := bindValidatorFlags(fs)
	if err := fs.Parse(args); err != nil {
		return validator.Options{}, err
	}
	var opts validator.Options
	vf.applyTo(&opts)
	return opts, nil
}

func runScanAll(args []string) error {
	opts, err := parseScanAllOptions(args)
	if err != nil {
		return err
	}
	start := time.Now()
	fmt.Println(version.String())
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	printFailedSectionsConsole(res.Logger, res.Sections)
	printRunFooter(res, start)
	return nil
}

// ── linters ───────────────────────────────────────────────────────────────────

func runMarkdownlint(args []string) error {
	out, err := markdownlint.Run(args)
	if out != "" {
		fmt.Print(out)
	}
	if err != nil && !errors.Is(err, markdownlint.ErrCLINotFound) {
		return err
	}
	return nil
}

func runPrettier(args []string) error {
	out, err := prettier.Run(args, nil)
	if out != "" {
		fmt.Print(out)
	}
	if err != nil && !errors.Is(err, prettier.ErrCLINotFound) {
		return err
	}
	return nil
}

// runShellcheck runs all three shellcheck extraction modes over args: raw
// shell script files, bash steps embedded in Tekton Task manifests, and
// bash embedded in workload container commands / ConfigMap .sh keys -
// consistent with how the other standalone lint subcommands are
// structured (one CLI entry point covering everything the Linting phase
// wires in).
func runShellcheck(args []string) error {
	total := 0

	violations, out, err := shellcheck.Run(args)
	if out != "" {
		fmt.Print(out)
	}
	total += len(violations)
	if err != nil && !errors.Is(err, shellcheck.ErrCLINotFound) {
		return err
	}

	tektonResults, tErr := shellcheck.RunTekton(args)
	for _, r := range tektonResults {
		if r.Output != "" {
			fmt.Print(r.Output)
		}
		total += len(r.Violations)
	}
	if tErr != nil && !errors.Is(tErr, shellcheck.ErrCLINotFound) {
		return tErr
	}

	embeddedResults, eErr := shellcheck.RunEmbedded(args)
	for _, r := range embeddedResults {
		if r.Output != "" {
			fmt.Print(r.Output)
		}
		total += len(r.Violations)
	}
	if eErr != nil && !errors.Is(eErr, shellcheck.ErrCLINotFound) {
		return eErr
	}

	if total > 0 {
		return fmt.Errorf("%d shellcheck violation(s)", total)
	}
	return nil
}

func runGolangci(args []string) error {
	out, err := golangci.Run(args)
	if out != "" {
		fmt.Print(out)
	}
	if err != nil && !errors.Is(err, golangci.ErrCLINotFound) {
		return err
	}
	return nil
}

func runKubeconform(args []string) error {
	fs := flag.NewFlagSet("kubeconform", flag.ExitOnError)
	opts := kubeconform.DefaultOptions()
	var version string
	fs.StringVar(&version, "kubernetes-version", opts.KubernetesVersion, "Kubernetes version")
	fs.BoolVar(&opts.Strict, "strict", opts.Strict, "strict mode")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts.KubernetesVersion = version
	files := fs.Args()
	if len(files) == 0 {
		return fmt.Errorf("kubeconform: no files specified")
	}
	if schemaDir, cleanup, err := kubeconform.ExtractSchemas(); err == nil {
		opts.SchemaDir = schemaDir
		defer cleanup()
	}
	res, err := kubeconform.ValidateFiles(files, opts)
	if err != nil {
		return err
	}
	fmt.Print(res.Summary())
	if res.Invalid > 0 || res.Errors > 0 {
		return fmt.Errorf("kubeconform: validation failed")
	}
	return nil
}

// ── kustomize-fix ─────────────────────────────────────────────────────────────

func runKustomizeFix(args []string) error {
	files := args
	if len(files) == 0 {
		return fmt.Errorf("kustomize-fix: no files specified")
	}
	apps := kustomize.AppsFromFiles(files)
	needsFix, err := kustomize.CheckFix(apps)
	if err != nil {
		return err
	}
	if len(needsFix) == 0 {
		fmt.Println("All kustomization.yaml files are up to date.")
		return nil
	}
	fmt.Println(kustomize.FormatFixNeeded(needsFix))
	return fmt.Errorf("kustomize-fix: %d file(s) need fixing", len(needsFix))
}

// ── check-starting-csv ────────────────────────────────────────────────────────

func runCheckStartingCSV(args []string) error {
	mismatches, err := csv.CheckStartingCSVFolderMatch(args)
	if err != nil {
		return err
	}
	if len(mismatches) == 0 {
		fmt.Println("startingCSV: all versions match.")
		return nil
	}
	fmt.Print(csv.FormatMismatches(mismatches))
	return fmt.Errorf("startingCSV: %d mismatch(es)", len(mismatches))
}

// ── ghost-patches ─────────────────────────────────────────────────────────────

func runGhostPatches(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("ghost-patches: overlay path required")
	}
	result, err := ghostpatch.CheckOverlay(args[0], "")
	if err != nil {
		return err
	}
	if len(result) == 0 {
		fmt.Println("No ghost patches found.")
		return nil
	}
	for _, r := range result {
		fmt.Println(r)
	}
	return fmt.Errorf("ghost-patches: %d finding(s)", len(result))
}

// ── update-scaffold-status ────────────────────────────────────────────────────

func runUpdateScaffoldStatus(_ []string) error {
	return scaffold.UpdateReadmeStatus()
}

// ── sort-configs ──────────────────────────────────────────────────────────────

func runSortConfigs(_ []string) error {
	n, err := config.SortConfigs()
	if err != nil {
		return err
	}
	fmt.Printf("Sorted %d config file(s).\n", n)
	return nil
}

// ── yaml-syntax ───────────────────────────────────────────────────────────────

func runYAMLSyntax(args []string) error {
	violations, err := yamlsyntax.CheckFiles(args)
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		fmt.Println("YAML syntax OK.")
		return nil
	}
	for _, v := range violations {
		fmt.Printf("%s: %s\n", v.File, v.Message)
	}
	return fmt.Errorf("yaml-syntax: %d error(s)", len(violations))
}

// ── validate-nad ──────────────────────────────────────────────────────────────

// runValidateNAD validates NetworkAttachmentDefinition files directly (bypassing
// the full pipeline) - either every YAML file under --dir or the explicit file
// paths given as positional args. --assume-openshift applies the additional
// OVN-Kubernetes-aware semantic tier (see pkg/validator/nad's package doc
// comment); the always-on structural tier runs regardless.
func runValidateNAD(args []string) error {
	fs := flag.NewFlagSet("validate-nad", flag.ExitOnError)
	dir := fs.String("dir", "", "directory to validate")
	assumeOpenshift := fs.Bool("assume-openshift", false, "apply OVN-aware validation (assumes the target CNI is OVN-Kubernetes)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *dir == "" && fs.NArg() == 0 {
		return fmt.Errorf("validate-nad: usage: validate-nad [--assume-openshift] --dir <path> or <file.yaml> [<file.yaml>...]")
	}

	var errs []nad.ValidationError
	if *dir != "" {
		errs = nad.ValidateDir(*dir, *assumeOpenshift)
	} else {
		errs = nad.ValidateFiles(fs.Args(), *assumeOpenshift)
	}

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "invalid NetworkAttachmentDefinition %s: %s\n", e.File, e.Message)
	}
	if len(errs) > 0 {
		return fmt.Errorf("validate-nad: %d error(s)", len(errs))
	}
	fmt.Println("All NetworkAttachmentDefinition files are valid.")
	return nil
}

// ── help ──────────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Println(`Usage: k8s-gitops-ci <command> [flags]

Pipeline:
  pipeline          Run the full CI pipeline (aliases: ci)
  build-yaml        Build YAML for a specific app/cluster
  test-all          Run all validators; accepts positional [dirs...] (full-tree
                    walk) or the same --url/--pr/--dirs/--disable-checks/
                    --enable-checks/etc. flags as "pipeline" (default: working-
                    tree git diff)
  scan-all          Like test-all, but only prints failing sections; defaults to
                    an uncommitted working-tree diff (git diff + git diff
                    --cached) - NOT a full-repo scan unless --dirs/--url/--pr
                    is given (use "test-all ." for that)

Linters:
  markdownlint      Run markdownlint on changed files
  prettier          Run prettier --check on changed files
  shellcheck        Run shellcheck on shell scripts
  golangci          Run golangci-lint on Go files
  kubeconform       Run kubeconform schema validation
  yaml-syntax       Check YAML syntax

Static Checks:
  kustomize-fix     Detect kustomization.yaml files needing edit fix
  check-starting-csv Validate startingCSV folder version matches
  ghost-patches     Detect kustomize patches that match no resource
  sort-configs      Sort repo config files
  update-scaffold-status Update scaffold README status table
  validate-nad      Validate NetworkAttachmentDefinition files (structural + optional OVN-aware)

Version:
  version           Show version information

Run 'k8s-gitops-ci <command> --help' for per-command flags.`)
}
