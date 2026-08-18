package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

func TestTableSpecForCheck(t *testing.T) {
	t.Parallel()
	ts, ok := TableSpecForCheck("namespace")
	if !ok {
		t.Fatal("expected namespace TableSpec to be registered")
	}
	if ts.Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestRenderColumnedTable_Empty(t *testing.T) {
	t.Parallel()
	out := RenderColumnedTable(nil, "namespace")
	if out != "" {
		t.Errorf("expected empty for nil findings")
	}
}

func TestRenderColumnedTable_WithFindings(t *testing.T) {
	t.Parallel()
	findings := []check.Finding{
		{CheckID: "namespace", Kind: "Pod", Name: "my-pod", File: "a.yaml", Message: "missing namespace"},
	}
	out := RenderColumnedTable(findings, "namespace")
	if !strings.Contains(out, "my-pod") || !strings.Contains(out, "missing namespace") {
		t.Errorf("unexpected table output: %s", out)
	}
}

func TestBuildComplianceSubs(t *testing.T) {
	t.Parallel()
	findings := []check.Finding{
		{CheckID: "namespace", Kind: "Pod", Name: "p", File: "a.yaml", Message: "missing namespace"},
		{CheckID: "image-checksum", Kind: "Deployment", Name: "d", File: "b.yaml", Value: "nginx:latest", Message: "not pinned"},
	}
	out := BuildComplianceSubs(findings)
	if !strings.Contains(out, "Namespace Scope") {
		t.Errorf("expected namespace section in output: %s", out)
	}
	if !strings.Contains(out, "Image Digest") {
		t.Errorf("expected image section in output: %s", out)
	}
}

func TestSanitizeCell(t *testing.T) {
	t.Parallel()
	out := sanitizeCell("foo|bar\nbaz")
	// pipe is escaped as \| for markdown; newline is replaced by space
	if strings.Contains(out, "\n") {
		t.Errorf("cell should not contain newline: %q", out)
	}
	if !strings.Contains(out, `\|`) {
		t.Errorf("pipe should be escaped in cell: %q", out)
	}
}

func TestComposeKustomizeBuildSection_NoErrors(t *testing.T) {
	t.Parallel()
	s := ComposeKustomizeBuildSection(3, nil, "", false, nil, nil, true, "", 0, "")
	if s.Status != StatusPassed {
		t.Errorf("expected StatusPassed, got %v", s.Status)
	}
	if !strings.Contains(s.Body, "3 overlay(s)") {
		t.Errorf("unexpected body: %s", s.Body)
	}
}

func TestComposeKustomizeBuildSection_GroupsBuildErrorsByRootCause(t *testing.T) {
	t.Parallel()
	buildErrs := []string{
		"kustomize build app/overlays/a: accumulating components: no such file or directory",
		"kustomize build app/overlays/b: accumulating components: no such file or directory",
	}
	s := ComposeKustomizeBuildSection(2, buildErrs, "", false, nil, nil, true, "", 0, "")
	if s.Status != StatusError {
		t.Error("expected an error section")
	}
	if !strings.Contains(s.Body, "**2 overlay(s)**") {
		t.Errorf("expected the two overlays to be grouped under one shared cause, got:\n%s", s.Body)
	}
}

func TestComposeKustomizeBuildSection_HookTable(t *testing.T) {
	t.Parallel()
	hookTable := "| App | PRE_BUILD |\n| --- | --- |\n| `app` | ✅ defined |"

	passed := ComposeKustomizeBuildSection(1, nil, hookTable, false, nil, nil, true, "", 0, "")
	if passed.Status != StatusPassed {
		t.Errorf("expected a passing hook table not to mark the section as an error, got %v", passed.Status)
	}
	if !strings.Contains(passed.Body, "✅ Hooks") {
		t.Errorf("expected an icon-bearing Hooks sub-dropdown, got:\n%s", passed.Body)
	}
	if !strings.Contains(passed.Body, "PRE_BUILD") {
		t.Errorf("expected the hook table to render, got:\n%s", passed.Body)
	}

	failed := ComposeKustomizeBuildSection(1, nil, hookTable, true, nil, nil, true, "", 0, "")
	if failed.Status != StatusError {
		t.Error("expected a failing hook to mark the section as an error")
	}
	if !strings.Contains(failed.Body, "❌ Hooks") {
		t.Errorf("expected an icon-bearing failed Hooks sub-dropdown, got:\n%s", failed.Body)
	}
}

// TestComposeKustomizeBuildSection_KustomizeFix guards that a real,
// working fix command is attached per affected directory - unlike the
// dead hintByCheck["kustomize fix"] entry in comments.go (never actually
// reachable, since nothing produces a LintFinding with Check=="kustomize
// fix"), this is the fix hint a reviewer actually sees in a real PR
// comment (see cmd/k8s-gitops-ci/main.go's runKustomizeFix, which now
// actually applies the fix given -dir/-all, unlike this read-only check).
func TestComposeKustomizeBuildSection_KustomizeFix(t *testing.T) {
	t.Parallel()
	fixNeeded := []string{
		"okd/okd-configuration/overlays/sandbox/kustomization.yaml",
		"okd/okd-configuration/overlays/prod/kustomization.yaml",
	}
	s := ComposeKustomizeBuildSection(2, nil, "", false, fixNeeded, nil, true, "", 0, "")
	if s.Status != StatusError {
		t.Error("expected a Kustomize Fix finding to mark the section as an error")
	}
	if !strings.Contains(s.Body, "❌ Kustomize Fix") {
		t.Errorf("expected an icon-bearing Kustomize Fix sub-dropdown, got:\n%s", s.Body)
	}
	for _, want := range []string{
		"k8s-gitops-ci kustomize-fix -dir okd/okd-configuration/overlays/sandbox",
		"k8s-gitops-ci kustomize-fix -dir okd/okd-configuration/overlays/prod",
	} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("expected a working fix command %q, got:\n%s", want, s.Body)
		}
	}
}

// TestComposeKustomizeBuildSection_KustomizeFixCheckError guards that a
// CheckFix failure (most commonly kustomize.ErrCLINotFound - see
// pkg/kustomize's package doc comment for why that's a hard failure, not
// a graceful skip) renders as its own StatusError, distinct from "no fix
// needed" - silently reporting a clean bill of health for a check that
// never actually ran would be worse than surfacing the failure.
func TestComposeKustomizeBuildSection_KustomizeFixCheckError(t *testing.T) {
	t.Parallel()
	s := ComposeKustomizeBuildSection(1, nil, "", false, nil, errors.New("kustomize not found in PATH"), true, "", 0, "")
	if s.Status != StatusError {
		t.Error("expected a CheckFix failure to mark the section as an error")
	}
	if !strings.Contains(s.Body, "❌ Kustomize Fix") {
		t.Errorf("expected an icon-bearing Kustomize Fix sub-dropdown, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "kustomize not found in PATH") {
		t.Errorf("expected the check error to be surfaced in the body, got:\n%s", s.Body)
	}
}

// TestComposeKustomizeBuildSection_KustomizeFixDisabled guards that
// --disable-checks kustomize-fix (stepKustomizeFix in phases.go) renders
// a "Disabled." summary rather than a misleading "up to date" - the
// check never actually ran, so it must not claim a clean bill of health.
func TestComposeKustomizeBuildSection_KustomizeFixDisabled(t *testing.T) {
	t.Parallel()
	s := ComposeKustomizeBuildSection(1, nil, "", false, nil, nil, false, "", 0, "")
	if s.Status == StatusError {
		t.Error("expected a disabled check not to mark the section as an error")
	}
	if !strings.Contains(s.Body, "Disabled.") {
		t.Errorf("expected a Disabled. summary, got:\n%s", s.Body)
	}
}

// TestComposeKustomizeBuildSection_GhostPatches guards the actual bug this
// section's composeGhostPatchesChild/composeParentFromChildren rewrite
// fixed: a non-blocking-only ghost patch used to render with no icon at
// all (a bare "Ghost Patch Details" dropdown, distinct from - and
// unindented relative to - a separate icon-bearing "- ⚠️ **Ghost
// Patches**" bullet above it) and used to unconditionally fail the parent
// section regardless of blocking status. Now it's a single icon-bearing
// sub-dropdown whose own status (and the parent's inherited worst-case
// status) reflects blocking vs. warning-only per docs/CI.md's "Ghost Patch
// Detection".
func TestComposeKustomizeBuildSection_GhostPatches(t *testing.T) {
	t.Parallel()
	ghostTable := "| Overlay | Target |\n| --- | --- |\n| `app/overlays/a` | Deployment/foo |"

	// Non-blocking (warning-only) ghost: no blocking rows, so this must
	// roll the parent up to StatusWarning, not StatusError (and not stay
	// StatusPassed either, which would hide it entirely).
	warn := ComposeKustomizeBuildSection(1, nil, "", false, nil, nil, true, ghostTable, 0, "")
	if warn.Status != StatusWarning {
		t.Errorf("expected a non-blocking-only ghost patch to roll the section up to StatusWarning, got %v", warn.Status)
	}
	if !strings.Contains(warn.Body, "⚠️ Ghost Patches") {
		t.Errorf("expected an icon-bearing warning Ghost Patches sub-dropdown, got:\n%s", warn.Body)
	}
	if !strings.Contains(warn.Body, "Deployment/foo") {
		t.Errorf("expected the ghost patch table to render, got:\n%s", warn.Body)
	}

	// Blocking ghost: must fail the section and show ❌.
	blocking := ComposeKustomizeBuildSection(1, nil, "", false, nil, nil, true, ghostTable, 1, "")
	if blocking.Status != StatusError {
		t.Error("expected a blocking ghost patch to mark the section as an error")
	}
	if !strings.Contains(blocking.Body, "❌ Ghost Patches") {
		t.Errorf("expected an icon-bearing failed Ghost Patches sub-dropdown, got:\n%s", blocking.Body)
	}
}

func TestComposeDriftProtectionSection_NoUnprotectedApps(t *testing.T) {
	t.Parallel()
	s := ComposeDriftProtectionSection(nil)
	if s.Status != StatusPassed {
		t.Errorf("expected StatusPassed when there are no unprotected apps, got %v", s.Status)
	}
	if !strings.Contains(s.Body, "drift protection enabled") {
		t.Errorf("expected the all-protected message, got:\n%s", s.Body)
	}
}

func TestComposeDriftProtectionSection_ListsUnprotectedApps(t *testing.T) {
	t.Parallel()
	s := ComposeDriftProtectionSection([]string{"myapp", "otherapp"})
	// Non-blocking - a coverage gap warning, not a drift finding - but
	// still worth a ⚠️, not a plain ✅ that would hide it.
	if s.Status != StatusWarning {
		t.Errorf("expected drift-protection gaps to roll up to StatusWarning (non-blocking), got %v", s.Status)
	}
	if !strings.Contains(s.Body, "`myapp`") || !strings.Contains(s.Body, "`otherapp`") {
		t.Errorf("expected both unprotected apps listed, got:\n%s", s.Body)
	}
}

func TestComposeScaffoldValidationSection_NoErrors(t *testing.T) {
	t.Parallel()
	s := ComposeScaffoldValidationSection("", nil, nil, "", nil)
	if s.Status != StatusPassed {
		t.Errorf("expected StatusPassed, got %v", s.Status)
	}
}

func TestComposeScaffoldValidationSection_WithDrift(t *testing.T) {
	t.Parallel()
	s := ComposeScaffoldValidationSection("some drift", []string{"exec failed"}, []string{"cluster-a"}, "", nil)
	if s.Status != StatusError {
		t.Error("expected error section")
	}
}

// TestComposeScaffoldValidationSection_PreExistingDriftAloneIsNonBlocking
// guards that pre-existing drift (drift that also exists against the
// merge-base template/config, and whose overlay this PR doesn't touch -
// see computeBaselineMismatches in scaffold_wiring.go) never marks the
// section as blocking on its own, matching the same non-blocking policy
// missing clusters already gets - it rolls the parent up to StatusWarning,
// not StatusError.
func TestComposeScaffoldValidationSection_PreExistingDriftAloneIsNonBlocking(t *testing.T) {
	t.Parallel()
	s := ComposeScaffoldValidationSection("", nil, nil, "myapp: overlay `staging` drifted from its scaffold template/config (pre-existing, not introduced by this PR)", nil)
	if s.Status != StatusWarning {
		t.Errorf("expected pre-existing drift alone to roll up to StatusWarning (non-blocking), got %v:\n%s", s.Status, s.Body)
	}
	if !strings.Contains(s.Body, "Pre-Existing Scaffold Drift") {
		t.Errorf("expected a Pre-Existing Scaffold Drift sub-dropdown, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "myapp: overlay `staging`") {
		t.Errorf("expected the pre-existing drift detail, got:\n%s", s.Body)
	}
}

// TestComposeScaffoldValidationSection_MissingClustersAloneIsNonBlocking
// guards a real bug found while wiring real data through this parameter:
// missingClusters records overlays scaffold.Run skipped rather than
// validated (not yet rolled out, or removed by this PR) - see scaffold.
// Run's own doc comment ("skipped ... never Failed") - so it must never,
// on its own (no drift, no exec errors), mark the section as blocking or
// even as an active warning: it rolls the parent up to StatusInfo (a
// purely informational FYI), not StatusWarning/StatusError.
func TestComposeScaffoldValidationSection_MissingClustersAloneIsNonBlocking(t *testing.T) {
	t.Parallel()
	s := ComposeScaffoldValidationSection("", nil, []string{"myapp/staging"}, "", nil)
	if s.Status != StatusInfo {
		t.Errorf("expected missing clusters alone to roll up to StatusInfo (informational, non-blocking), got %v:\n%s", s.Status, s.Body)
	}
	if !strings.Contains(s.Body, "`myapp/staging`") {
		t.Errorf("expected the missing cluster to be listed, got:\n%s", s.Body)
	}
}

// TestComposeScaffoldValidationSection_DisabledOverlaysAloneIsNonBlocking
// guards that a config-disabled overlay this PR modified rolls the section
// up to StatusWarning (an "are you sure you meant to edit a disabled
// overlay?" signal), never StatusError - the overlay is skipped by intent
// (see scaffold.OverlayConfigDisabled) so the pipeline still passes.
func TestComposeScaffoldValidationSection_DisabledOverlaysAloneIsNonBlocking(t *testing.T) {
	t.Parallel()
	s := ComposeScaffoldValidationSection("", nil, nil, "", []string{"myapp/retired1"})
	if s.Status != StatusWarning {
		t.Errorf("expected a modified disabled overlay to roll up to StatusWarning (non-blocking), got %v:\n%s", s.Status, s.Body)
	}
	if !strings.Contains(s.Body, "Disabled Overlays") {
		t.Errorf("expected a Disabled Overlays sub-dropdown, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "`myapp/retired1`") {
		t.Errorf("expected the disabled overlay to be listed, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "disabled") {
		t.Errorf("expected a message explaining scaffolding was skipped, got:\n%s", s.Body)
	}
}
