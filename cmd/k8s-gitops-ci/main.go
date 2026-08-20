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
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/hook"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/kustomize"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/golangci"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/markdownlint"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/prettier"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/shellcheck"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/yamlsyntax"
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
	case "test":
		err = runTest(args)
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
	case "ci-report":
		err = runCIReport(args)
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

	var dirs, disableChecks, enableChecks, hookSource string
	fs.StringVar(&opts.URL, "url", opts.URL, "repository URL (e.g. https://github.com/org/repo — NOT a PR URL; pass the PR number via --pr)")
	fs.StringVar(&opts.PR, "pr", opts.PR, "pull request number")
	fs.StringVar(&opts.Revision, "revision", opts.Revision, "git revision")
	fs.StringVar(&opts.TargetBranch, "target-branch", opts.TargetBranch, "target branch")
	fs.StringVar(&hookSource, "hook-source", string(opts.HookSource), "hook source (main|pr|local)")
	fs.StringVar(&opts.TriggerComment, "trigger-comment", opts.TriggerComment, "trigger comment text")
	fs.StringVar(&dirs, "dirs", "", "comma-separated path prefixes to validate in full, replacing the diff/PR-derived changeset entirely (e.g. kubernetes/,tekton/,.tekton/,okd/)")
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
	opts.Dirs = splitCommaList(dirs)
	opts.HookSource = hook.Source(hookSource)
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
	pipeline.PrintAllSectionsConsole(res.Logger, res.Sections)
	pipeline.PrintRunFooter(res, start)
	return nil
}

// ── test ──────────────────────────────────────────────────────────────────────

// validatorFlagSet holds the flags shared by "test": the same
// changeset-scoping and check-enablement flags "pipeline" exposes, so a
// failing pipeline run can be reproduced with "test" using an equivalent
// flag set.
type validatorFlagSet struct {
	url, pr, targetBranch, hookSource  string
	dirs, disableChecks, enableChecks  string
	concurrency                        int
	assumeOpenshift, verbose, lintOnly bool
	quiet, all                         bool
	apps, clusters                     []string
}

// bindValidatorFlags registers the shared flags on fs and returns the
// backing struct to read after fs.Parse.
func bindValidatorFlags(fs *flag.FlagSet) *validatorFlagSet {
	v := &validatorFlagSet{}
	fs.StringVar(&v.url, "url", "", "repository URL (e.g. https://github.com/org/repo — NOT a PR URL; pass the PR number via --pr)")
	fs.StringVar(&v.pr, "pr", "", "pull request number")
	fs.StringVar(&v.targetBranch, "target-branch", "", "target branch")
	fs.StringVar(&v.hookSource, "hook-source", "", "hook source (main|pr|local)")
	fs.StringVar(&v.dirs, "dirs", "", "comma-separated path prefixes to validate in full, replacing the diff/PR-derived changeset entirely (e.g. kubernetes/,tekton/,.tekton/,okd/)")
	fs.StringVar(&v.disableChecks, "disable-checks", "", "comma-separated IDs to disable entirely (e.g. sync-options, golangci, avp); only affects checks/steps that default to enabled")
	fs.StringVar(&v.enableChecks, "enable-checks", "", "comma-separated IDs to explicitly enable; only affects checks/steps that default to disabled (e.g. kyverno)")
	fs.IntVar(&v.concurrency, "concurrency", 0, "worker concurrency (0=auto)")
	fs.BoolVar(&v.assumeOpenshift, "assume-openshift", false, "treat OpenShift/OKD-default-but-portable API groups (OLM, Prometheus Operator, Gateway API, SR-IOV/Multus/OVN-Kubernetes CNI, Metal3) as exempt from the sync-options check, in addition to the always-exempt OpenShift-exclusive groups (route.openshift.io, config.openshift.io, ...); only enable if ALL target clusters are OpenShift/OKD")
	fs.BoolVar(&v.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&v.lintOnly, "lint-only", false, "lint only, skip build checks")
	fs.BoolVar(&v.quiet, "quiet", false, "quiet: only print failed/warned sections, exit 0")
	fs.BoolVar(&v.all, "all", false, "full repository scan: lint all files on disk and build all overlays (takes priority over --dirs)")
	fs.Var(newStringSliceFlag(&v.apps), "app", "app name to scope validation to (repeatable: --app a --app b)")
	fs.Var(newStringSliceFlag(&v.clusters), "cluster", "cluster name to scope validation to (repeatable: --cluster a --cluster b)")
	return v
}

// applyTo copies the parsed flags onto opts, including Dirs from --dirs.
// For test, which also supports positional [dirs...], parseTestOptions
// overwrites opts.Dirs with the positional args when present, since both are
// the same full-tree-walk changeset source and positional args take
// precedence over the flag.
func (v *validatorFlagSet) applyTo(opts *validator.Options) {
	opts.RepoURL = v.url
	opts.PR = v.pr
	opts.BaseRef = v.targetBranch
	opts.HookSource = hook.Source(v.hookSource)
	opts.Dirs = splitCommaList(v.dirs)
	opts.DisabledChecks = splitCommaList(v.disableChecks)
	opts.EnabledChecks = splitCommaList(v.enableChecks)
	opts.Concurrency = v.concurrency
	opts.AssumeOpenShift = v.assumeOpenshift
	opts.LintOnly = v.lintOnly
	opts.Verbose = v.verbose
	opts.FullScan = v.all
	opts.Quiet = v.quiet
	opts.Apps = v.apps
	opts.Clusters = v.clusters
}

// parseTestOptions parses test's flags (same scoping/check-enablement
// flags as "pipeline", plus positional [dirs...]) into a validator.Options,
// without running anything - split out from runTest so the flag-to-Options
// wiring is unit-testable without invoking validator.RunAll (which shells
// out to git).
func parseTestOptions(args []string) (validator.Options, error) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	vf := bindValidatorFlags(fs)
	if err := fs.Parse(args); err != nil {
		return validator.Options{}, err
	}
	var opts validator.Options
	vf.applyTo(&opts)
	// Positional [dirs...] args are a full-tree walk under those paths
	// (bypassing git diff entirely) - alongside the --dirs flag (also folded
	// into Dirs by applyTo). Positional args take precedence when present.
	if len(fs.Args()) > 0 {
		opts.Dirs = fs.Args()
	}
	return opts, nil
}

func runTest(args []string) error {
	opts, err := parseTestOptions(args)
	if err != nil {
		return err
	}
	// pipeline.RunTest owns the full run: RunAll, version banner, console
	// rendering (all sections, or failure-only under --quiet), the timing/
	// summary footer, and the exit rule (--quiet always succeeds; otherwise
	// a failed run returns an error) — exported so any consumer CLI wrapping
	// this core gets identical "test" behavior. See pkg/pipeline/console_format.go.
	return pipeline.RunTest(opts)
}

// ── linters ───────────────────────────────────────────────────────────────────

func runMarkdownlint(args []string) error {
	out, err := markdownlint.Run(markdownlint.FilterMarkdown(args))
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

// runKustomizeFix actually applies kustomize.Fix (writes every non-
// normalized kustomization.yaml under -dir/-all back to disk), unlike the
// read-only Kustomize Fix check the "pipeline"/"test"
// commands already run as part of the Kustomize Build section - this is
// the fix hintByCheck's "kustomize fix" entry (comments.go) actually
// points a reviewer at. -dir and -all are mutually exclusive; passing
// neither (or both) is a usage error rather than silently doing nothing or
// picking one arbitrarily.
func runKustomizeFix(args []string) error {
	fs := flag.NewFlagSet("kustomize-fix", flag.ExitOnError)
	dir := fs.String("dir", "", "Directory to fix, recursively (mutually exclusive with -all)")
	all := fs.Bool("all", false, "Fix every kustomization.yaml under the current directory, recursively")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var target string
	switch {
	case *all && *dir != "":
		return fmt.Errorf("kustomize-fix: -dir and -all are mutually exclusive")
	case *all:
		target = "."
	case *dir != "":
		target = *dir
	default:
		fs.Usage()
		return fmt.Errorf("kustomize-fix: -dir <path> or -all is required")
	}

	fixed, err := kustomize.Fix(target)
	if err != nil {
		return fmt.Errorf("kustomize-fix: %w", err)
	}
	if len(fixed) == 0 {
		fmt.Println("All kustomization.yaml files are up to date.")
		return nil
	}
	fmt.Printf("Fixed %d kustomization.yaml file(s):\n", len(fixed))
	for _, f := range fixed {
		fmt.Println("  " + f)
	}
	return nil
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
// paths given as positional args. Validation dispatches on the CNI type declared
// in each NAD's spec.config (OVN NADs get OVN-aware checks automatically); see
// pkg/validator/nad's package doc comment.
func runValidateNAD(args []string) error {
	fs := flag.NewFlagSet("validate-nad", flag.ExitOnError)
	dir := fs.String("dir", "", "directory to validate")
	// Deprecated/no-op: NAD validation now dispatches on the CNI type declared
	// in each NAD's spec.config, so this flag no longer affects it. Kept for
	// back-compat with callers/scripts that still pass it.
	assumeOpenshift := fs.Bool("assume-openshift", false, "deprecated/ignored: NAD validation now auto-detects OVN NADs by spec.config type")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = *assumeOpenshift // accepted for back-compat; no longer consumed here

	if *dir == "" && fs.NArg() == 0 {
		return fmt.Errorf("validate-nad: usage: validate-nad --dir <path> or <file.yaml> [<file.yaml>...]")
	}

	var errs, warns []nad.ValidationError
	if *dir != "" {
		errs, warns = nad.ValidateDir(*dir)
	} else {
		errs, warns = nad.ValidateFiles(fs.Args())
	}

	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "warning: %s: %s\n", w.File, w.Message)
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
  test              Run all validators; accepts positional [dirs...] (full-tree
                    walk), --all (full-repo scan), --quiet (failure-only output),
                    or the same --url/--pr/--dirs/--disable-checks/
                    --enable-checks/--lint-only/etc. flags as "pipeline"
                    (default: working-tree git diff)

Linters:
  markdownlint      Run markdownlint on changed files
  prettier          Run prettier --check on changed files
  shellcheck        Run shellcheck on shell scripts
  golangci          Run golangci-lint on Go files
  kubeconform       Run kubeconform schema validation
  yaml-syntax       Check YAML syntax

Static Checks:
  kustomize-fix     Normalize (write) kustomization.yaml field ordering;
                    -dir <path> (recursive) or -all (whole working tree) -
                    the "pipeline"/"test" commands already
                    report which files need this via the Kustomize Build
                    section, without writing anything themselves
  check-starting-csv Validate startingCSV folder version matches
  ghost-patches     Detect kustomize patches that match no resource
  sort-configs      Sort repo config files
  update-scaffold-status Update scaffold README status table
  validate-nad      Validate NetworkAttachmentDefinition files (auto-dispatches on CNI type)

CI Meta:
  ci-report         Post/update a self-CI status comment on this repo's own PR
                    (overall task-ci verdict + a non-blocking live-replay section)

Version:
  version           Show version information

Run 'k8s-gitops-ci <command> --help' for per-command flags.`)
}
