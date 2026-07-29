package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

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
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/pipeline"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/provider"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/scaffold"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
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

	fs.StringVar(&opts.URL, "url", opts.URL, "repository URL")
	fs.StringVar(&opts.PR, "pr", opts.PR, "pull request number")
	fs.StringVar(&opts.Revision, "revision", opts.Revision, "git revision")
	fs.StringVar(&opts.TargetBranch, "target-branch", opts.TargetBranch, "target branch")
	fs.StringVar(&opts.HookSource, "hook-source", opts.HookSource, "hook source (main|pr|local)")
	fs.StringVar(&opts.TriggerComment, "trigger-comment", opts.TriggerComment, "trigger comment text")
	fs.BoolVar(&opts.LintOnly, "lint-only", false, "lint only, skip build checks")
	fs.BoolVar(&opts.SkipAVP, "skip-avp", false, "skip argocd-vault-plugin")
	fs.BoolVar(&opts.SkipGolangci, "skip-golangci", false, "skip golangci-lint")
	fs.BoolVar(&opts.NoComment, "no-comment", false, "do not post PR comment")
	fs.BoolVar(&opts.Verbose, "verbose", false, "verbose output")
	fs.IntVar(&opts.Concurrency, "concurrency", 0, "worker concurrency (0=auto)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return pipeline.Run(opts)
}

// ── build-yaml ────────────────────────────────────────────────────────────────

func runBuildYAML(args []string) error {
	fs := flag.NewFlagSet("build-yaml", flag.ExitOnError)
	var app, cluster string
	fs.StringVar(&app, "app", "", "app name")
	fs.StringVar(&cluster, "cluster", "", "cluster name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := validator.Options{Apps: []string{app}, Clusters: []string{cluster}}
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	for _, s := range res.Sections {
		fmt.Printf("=== %s ===\n%s\n", s.Name, s.Body)
	}
	return nil
}

// ── test-all / scan-all ───────────────────────────────────────────────────────

func runTestAll(args []string) error {
	fs := flag.NewFlagSet("test-all", flag.ExitOnError)
	fs.Parse(args)
	opts := validator.Options{}
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	for _, s := range res.Sections {
		fmt.Printf("=== %s ===\n%s\n", s.Name, s.Body)
	}
	if res.Blocking {
		return fmt.Errorf("test-all: validation failed")
	}
	return nil
}

func runScanAll(args []string) error {
	fs := flag.NewFlagSet("scan-all", flag.ExitOnError)
	fs.Parse(args)
	opts := validator.Options{}
	res, err := validator.RunAll(opts)
	if err != nil {
		return err
	}
	for _, s := range res.Sections {
		if s.Error {
			fmt.Printf("[FAIL] %s\n%s\n", s.Name, s.Body)
		}
	}
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

func runShellcheck(args []string) error {
	violations, out, err := shellcheck.Run(args)
	if out != "" {
		fmt.Print(out)
	}
	if len(violations) > 0 {
		return fmt.Errorf("%d shellcheck violation(s)", len(violations))
	}
	if err != nil && !errors.Is(err, shellcheck.ErrCLINotFound) {
		return err
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

func runSortConfigs(args []string) error {
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

// ── help ──────────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Println(`Usage: k8s-gitops-ci <command> [flags]

Pipeline:
  pipeline          Run the full CI pipeline (aliases: ci)
  build-yaml        Build YAML for a specific app/cluster
  test-all          Run all validators against the working tree
  scan-all          Full-repo scan, print failing sections

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

Version:
  version           Show version information

Run 'k8s-gitops-ci <command> --help' for per-command flags.`)
}
