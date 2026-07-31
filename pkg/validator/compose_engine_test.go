package validator

import (
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

func TestTableSpecForCheck(t *testing.T) {
	ts, ok := TableSpecForCheck("namespace")
	if !ok {
		t.Fatal("expected namespace TableSpec to be registered")
	}
	if ts.Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestRenderColumnedTable_Empty(t *testing.T) {
	out := RenderColumnedTable(nil, "namespace")
	if out != "" {
		t.Errorf("expected empty for nil findings")
	}
}

func TestRenderColumnedTable_WithFindings(t *testing.T) {
	findings := []check.Finding{
		{CheckID: "namespace", Kind: "Pod", Name: "my-pod", File: "a.yaml", Message: "missing namespace"},
	}
	out := RenderColumnedTable(findings, "namespace")
	if !strings.Contains(out, "my-pod") || !strings.Contains(out, "missing namespace") {
		t.Errorf("unexpected table output: %s", out)
	}
}

func TestBuildComplianceSubs(t *testing.T) {
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
	s := ComposeKustomizeBuildSection(3, nil, "", nil, "")
	if s.Error {
		t.Error("expected no error")
	}
	if !strings.Contains(s.Body, "3 overlay(s)") {
		t.Errorf("unexpected body: %s", s.Body)
	}
}

func TestComposeKustomizeBuildSection_GroupsBuildErrorsByRootCause(t *testing.T) {
	buildErrs := []string{
		"kustomize build app/overlays/a: accumulating components: no such file or directory",
		"kustomize build app/overlays/b: accumulating components: no such file or directory",
	}
	s := ComposeKustomizeBuildSection(2, buildErrs, "", nil, "")
	if !s.Error {
		t.Error("expected an error section")
	}
	if !strings.Contains(s.Body, "**2 overlay(s)**") {
		t.Errorf("expected the two overlays to be grouped under one shared cause, got:\n%s", s.Body)
	}
}

func TestComposeKustomizeBuildSection_HookAndGhostTables(t *testing.T) {
	s := ComposeKustomizeBuildSection(1, nil, "| App | PRE_BUILD |\n| --- | --- |\n| `app` | ✅ defined |", nil, "| Overlay | Target |\n| --- | --- |\n| `app/overlays/a` | Deployment/foo |")
	if !s.Error {
		t.Error("expected ghost patches to mark the section as an error")
	}
	if !strings.Contains(s.Body, "PRE_BUILD") {
		t.Errorf("expected the hook table to render, got:\n%s", s.Body)
	}
	if !strings.Contains(s.Body, "Deployment/foo") {
		t.Errorf("expected the ghost patch table to render, got:\n%s", s.Body)
	}
}

func TestComposeDriftProtectionSection_NoUnprotectedApps(t *testing.T) {
	s := ComposeDriftProtectionSection(nil)
	if s.Error {
		t.Error("expected no error when there are no unprotected apps")
	}
	if !strings.Contains(s.Body, "drift protection enabled") {
		t.Errorf("expected the all-protected message, got:\n%s", s.Body)
	}
}

func TestComposeDriftProtectionSection_ListsUnprotectedApps(t *testing.T) {
	s := ComposeDriftProtectionSection([]string{"myapp", "otherapp"})
	// Non-blocking - a coverage gap warning, not a drift finding.
	if s.Error {
		t.Error("expected drift-protection gaps to be non-blocking")
	}
	if !strings.Contains(s.Body, "`myapp`") || !strings.Contains(s.Body, "`otherapp`") {
		t.Errorf("expected both unprotected apps listed, got:\n%s", s.Body)
	}
}

func TestComposeScaffoldValidationSection_NoErrors(t *testing.T) {
	s := ComposeScaffoldValidationSection("", nil, nil)
	if s.Error {
		t.Error("expected no error")
	}
}

func TestComposeScaffoldValidationSection_WithDrift(t *testing.T) {
	s := ComposeScaffoldValidationSection("some drift", []string{"exec failed"}, []string{"cluster-a"})
	if !s.Error {
		t.Error("expected error section")
	}
}
