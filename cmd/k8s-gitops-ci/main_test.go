package main

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close() //nolint:errcheck
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

func TestStringSliceFlag_AccumulatesInOrder(t *testing.T) {
	var apps []string
	f := newStringSliceFlag(&apps)
	if err := f.Set("a"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := f.Set("b"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(apps) != 2 || apps[0] != "a" || apps[1] != "b" {
		t.Fatalf("expected [a b], got %v", apps)
	}
	if got := f.String(); got != "a,b" {
		t.Errorf("String() = %q, want %q", got, "a,b")
	}
}

func TestStringSliceFlag_EmptyString(t *testing.T) {
	var clusters []string
	f := newStringSliceFlag(&clusters)
	if got := f.String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
}

// TestBindValidatorFlags_ParsesAndApplies guards test's flag
// parity with "pipeline": every scoping/check-enablement flag pipeline
// exposes must parse here and land on the corresponding validator.Options
// field via applyTo.
func TestBindValidatorFlags_ParsesAndApplies(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	vf := bindValidatorFlags(fs)
	args := []string{
		"--url=https://example.com/org/repo",
		"--pr=42",
		"--target-branch=origin/main",
		"--hook-source=pr",
		"--dirs=kubernetes/,tekton/",
		"--disable-checks=sync-options,golangci",
		"--enable-checks=kyverno",
		"--concurrency=4",
		"--assume-openshift",
		"--verbose",
		"--lint-only",
		"--quiet",
		"--all",
		"--app=foo",
		"--app=bar",
		"--cluster=prod",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var opts validator.Options
	vf.applyTo(&opts)

	switch {
	case opts.RepoURL != "https://example.com/org/repo":
		t.Errorf("RepoURL = %q", opts.RepoURL)
	case opts.PR != "42":
		t.Errorf("PR = %q", opts.PR)
	case opts.BaseRef != "origin/main":
		t.Errorf("BaseRef = %q", opts.BaseRef)
	case opts.HookSource != "pr":
		t.Errorf("HookSource = %q", opts.HookSource)
	case len(opts.Dirs) != 2 || opts.Dirs[0] != "kubernetes/" || opts.Dirs[1] != "tekton/":
		t.Errorf("Dirs = %v", opts.Dirs)
	case len(opts.DisabledChecks) != 2 || opts.DisabledChecks[0] != "sync-options" || opts.DisabledChecks[1] != "golangci":
		t.Errorf("DisabledChecks = %v", opts.DisabledChecks)
	case len(opts.EnabledChecks) != 1 || opts.EnabledChecks[0] != "kyverno":
		t.Errorf("EnabledChecks = %v", opts.EnabledChecks)
	case opts.Concurrency != 4:
		t.Errorf("Concurrency = %d", opts.Concurrency)
	case !opts.AssumeOpenShift:
		t.Errorf("AssumeOpenShift = %v, want true", opts.AssumeOpenShift)
	case !opts.LintOnly:
		t.Errorf("LintOnly = %v, want true", opts.LintOnly)
	case !opts.Verbose:
		t.Errorf("Verbose = %v, want true", opts.Verbose)
	case !opts.Quiet:
		t.Errorf("Quiet = %v, want true", opts.Quiet)
	case !opts.FullScan:
		t.Errorf("FullScan = %v, want true", opts.FullScan)
	case len(opts.Apps) != 2 || opts.Apps[0] != "foo" || opts.Apps[1] != "bar":
		t.Errorf("Apps = %v", opts.Apps)
	case len(opts.Clusters) != 1 || opts.Clusters[0] != "prod":
		t.Errorf("Clusters = %v", opts.Clusters)
	}
}

// TestBindValidatorFlags_DefaultsLeaveOptionsZeroValue ensures no flag being
// passed doesn't grow spurious non-nil-but-empty slices/strings that would
// change behavior (e.g. an empty-but-non-nil Dirs would still be
// fine since FilterByPrefixes treats empty as "no restriction", but this
// guards the split helpers stay nil on empty input).
func TestBindValidatorFlags_DefaultsLeaveOptionsZeroValue(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	vf := bindValidatorFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var opts validator.Options
	vf.applyTo(&opts)

	if opts.RepoURL != "" || opts.PR != "" || opts.BaseRef != "" || opts.HookSource != "" {
		t.Errorf("expected empty strings, got RepoURL=%q PR=%q BaseRef=%q HookSource=%q", opts.RepoURL, opts.PR, opts.BaseRef, opts.HookSource)
	}
	if opts.Dirs != nil || opts.DisabledChecks != nil || opts.EnabledChecks != nil {
		t.Errorf("expected nil slices, got Dirs=%v DisabledChecks=%v EnabledChecks=%v", opts.Dirs, opts.DisabledChecks, opts.EnabledChecks)
	}
	if opts.Concurrency != 0 || opts.AssumeOpenShift || opts.LintOnly || opts.Verbose || opts.Quiet || opts.FullScan {
		t.Errorf("expected zero values, got Concurrency=%d AssumeOpenShift=%v LintOnly=%v Verbose=%v Quiet=%v FullScan=%v", opts.Concurrency, opts.AssumeOpenShift, opts.LintOnly, opts.Verbose, opts.Quiet, opts.FullScan)
	}
	if len(opts.Apps) != 0 || len(opts.Clusters) != 0 {
		t.Errorf("expected empty Apps/Clusters, got %v / %v", opts.Apps, opts.Clusters)
	}
}

// TestParseTestOptions_PositionalDirsAndFlagDirsCoexist guards that
// positional [dirs...] take precedence over the --dirs flag now that both
// map to the single opts.Dirs field.
func TestParseTestOptions_PositionalDirsAndFlagDirsCoexist(t *testing.T) {
	opts, err := parseTestOptions([]string{"--dirs=kubernetes/,tekton/", "appA", "appB"})
	if err != nil {
		t.Fatalf("parseTestOptions: %v", err)
	}
	if len(opts.Dirs) != 2 || opts.Dirs[0] != "appA" || opts.Dirs[1] != "appB" {
		t.Errorf("Dirs = %v, want [appA appB] (positional precedence)", opts.Dirs)
	}
}

// TestParseTestOptions_NoPositionalDirsLeavesDirsNil ensures test
// without positional args doesn't grow a spurious non-nil-but-empty Dirs,
// which would otherwise change resolveChangeset's branch (Dirs vs.
// git-diff) per pkg/validator/validator.go.
func TestParseTestOptions_NoPositionalDirsLeavesDirsNil(t *testing.T) {
	opts, err := parseTestOptions([]string{"--verbose"})
	if err != nil {
		t.Fatalf("parseTestOptions: %v", err)
	}
	if len(opts.Dirs) != 0 {
		t.Errorf("Dirs = %v, want empty", opts.Dirs)
	}
	if !opts.Verbose {
		t.Errorf("Verbose = false, want true")
	}
}

// TestParseTestOptions_QuietAndAllFlags ensures --quiet and --all parse
// correctly into opts.Quiet and opts.FullScan.
func TestParseTestOptions_QuietAndAllFlags(t *testing.T) {
	opts, err := parseTestOptions([]string{"--quiet", "--all"})
	if err != nil {
		t.Fatalf("parseTestOptions: %v", err)
	}
	if !opts.Quiet {
		t.Errorf("Quiet = %v, want true", opts.Quiet)
	}
	if !opts.FullScan {
		t.Errorf("FullScan = %v, want true", opts.FullScan)
	}
}

func TestSplitCommaList(t *testing.T) {
	cases := map[string][]string{
		"":                nil,
		"   ":             nil,
		"a":               {"a"},
		"a,b":             {"a", "b"},
		"a, b ,, c":       {"a", "b", "c"},
		"kubernetes/,ci/": {"kubernetes/", "ci/"},
	}
	for in, want := range cases {
		got := splitCommaList(in)
		if len(got) != len(want) {
			t.Fatalf("splitCommaList(%q) = %v, want %v", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("splitCommaList(%q) = %v, want %v", in, got, want)
			}
		}
	}
}

// ── kustomize-fix ─────────────────────────────────────────────────────────────

// TestRunKustomizeFix_RequiresDirOrAll guards against silently doing
// nothing (or picking an arbitrary default) when invoked with neither
// -dir nor -all - the exact "the help is kind of useless" complaint this
// command's rewrite addresses: previously it took bare positional file
// args and only ever checked, never actually fixed, anything.
func TestRunKustomizeFix_RequiresDirOrAll(t *testing.T) {
	if err := runKustomizeFix(nil); err == nil {
		t.Error("expected an error when neither -dir nor -all is given")
	}
}

// TestRunKustomizeFix_DirAndAllAreMutuallyExclusive guards against
// silently preferring one flag over the other when both are given.
func TestRunKustomizeFix_DirAndAllAreMutuallyExclusive(t *testing.T) {
	if err := runKustomizeFix([]string{"-dir", ".", "-all"}); err == nil {
		t.Error("expected an error when both -dir and -all are given")
	}
}

// TestRunKustomizeFix_DirActuallyWritesFixedFiles guards the core fix:
// unlike the old positional-args-only, read-only "check" behavior,
// "kustomize-fix -dir <path>" must actually rewrite a non-normalized
// kustomization.yaml under that path (recursively) - via the real
// `kustomize edit fix --vars` (see pkg/kustomize's package doc comment
// for why this shells out rather than reimplementing kustomize's own
// logic), converting a deprecated `vars:` block to `replacements:` too.
func TestRunKustomizeFix_DirActuallyWritesFixedFiles(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	if _, err := exec.LookPath("prettier"); err != nil {
		t.Skip("prettier not installed")
	}
	root := t.TempDir()
	nested := filepath.Join(root, "overlays", "sandbox", "kustomization.yaml")
	unfixed := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - resource.yaml
vars:
  - name: FOO
    objref:
      kind: ConfigMap
      name: my-configmap
      apiVersion: v1
    fieldref:
      fieldpath: data.foo
`
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte(unfixed), 0o644); err != nil {
		t.Fatal(err)
	}

	var out string
	var err error
	out = captureStdout(t, func() { err = runKustomizeFix([]string{"-dir", root}) })
	if err != nil {
		t.Fatalf("runKustomizeFix: %v", err)
	}
	if !strings.Contains(out, "Fixed 1 kustomization.yaml file") {
		t.Errorf("expected a 'Fixed 1 ...' summary, got: %s", out)
	}

	after, err := os.ReadFile(nested)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) == unfixed {
		t.Error("expected the file to actually be rewritten on disk")
	}
	if !strings.Contains(string(after), "replacements:") || strings.Contains(string(after), "vars:") {
		t.Errorf("expected vars: to actually be converted to replacements: (--vars), got: %s", after)
	}
}

// TestRunKustomizeFix_NoFilesNeedFixing guards the clean-tree message.
func TestRunKustomizeFix_NoFilesNeedFixing(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not installed")
	}
	root := t.TempDir()
	out := captureStdout(t, func() {
		if err := runKustomizeFix([]string{"-dir", root}); err != nil {
			t.Errorf("runKustomizeFix: %v", err)
		}
	})
	if !strings.Contains(out, "All kustomization.yaml files are up to date.") {
		t.Errorf("expected the up-to-date message, got: %s", out)
	}
}
