package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

func writeNamespaceWithCommentedLabels(t *testing.T, appRoot string) {
	t.Helper()
	base := filepath.Join(appRoot, "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	ns := `apiVersion: v1
kind: Namespace
metadata:
  name: foo
  labels:
    # pod-security.kubernetes.io/enforce: restricted
    # pod-security.kubernetes.io/enforce-version: latest
    # pod-security.kubernetes.io/warn: restricted
    # pod-security.kubernetes.io/warn-version: latest
    # pod-security.kubernetes.io/audit: restricted
    # pod-security.kubernetes.io/audit-version: latest
    other: label
`
	if err := os.WriteFile(filepath.Join(base, "namespace.yaml"), []byte(ns), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFilterCommentedPSAFindings_SuppressesFullyCommentedNamespace(t *testing.T) {
	appRoot := t.TempDir()
	writeNamespaceWithCommentedLabels(t, appRoot)
	nsFile := filepath.Join(appRoot, "base", "namespace.yaml")

	missing := "pod-security.kubernetes.io/enforce,pod-security.kubernetes.io/enforce-version," +
		"pod-security.kubernetes.io/warn,pod-security.kubernetes.io/warn-version," +
		"pod-security.kubernetes.io/audit,pod-security.kubernetes.io/audit-version"
	findings := []check.Finding{{
		CheckID: "psa-labels", File: nsFile, Name: "foo",
		Extra: map[string]string{psaMissingLabelsExtraKey: missing},
	}}

	out := filterCommentedPSAFindings(findings)
	if len(out) != 0 {
		t.Errorf("expected the finding to be suppressed (every missing label is commented out), got %+v", out)
	}
}

func TestFilterCommentedPSAFindings_KeepsFindingWhenNotAllLabelsCommented(t *testing.T) {
	appRoot := t.TempDir()
	// Only comment out the enforce label, not the others.
	base := filepath.Join(appRoot, "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	ns := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: foo\n  labels:\n    # pod-security.kubernetes.io/enforce: restricted\n"
	if err := os.WriteFile(filepath.Join(base, "namespace.yaml"), []byte(ns), 0o644); err != nil {
		t.Fatal(err)
	}
	nsFile := filepath.Join(base, "namespace.yaml")

	missing := "pod-security.kubernetes.io/enforce,pod-security.kubernetes.io/warn"
	findings := []check.Finding{{
		CheckID: "psa-labels", File: nsFile, Name: "foo",
		Extra: map[string]string{psaMissingLabelsExtraKey: missing},
	}}

	out := filterCommentedPSAFindings(findings)
	if len(out) != 1 {
		t.Errorf("expected the finding to survive (warn label isn't commented anywhere), got %+v", out)
	}
}

func TestFilterCommentedPSAFindings_NeverSuppressesInvalidValueFindings(t *testing.T) {
	// A label that's *present* with an invalid value must never be
	// suppressed by a comment, even one commenting out that same label
	// key - the MissingLabels entry for an invalid value carries extra
	// text that can't match a bare commented key.
	appRoot := t.TempDir()
	writeNamespaceWithCommentedLabels(t, appRoot)
	nsFile := filepath.Join(appRoot, "base", "namespace.yaml")

	missing := `pod-security.kubernetes.io/enforce (invalid value "foo")`
	findings := []check.Finding{{
		CheckID: "psa-labels", File: nsFile, Name: "foo",
		Extra: map[string]string{psaMissingLabelsExtraKey: missing},
	}}

	out := filterCommentedPSAFindings(findings)
	if len(out) != 1 {
		t.Errorf("expected an invalid-value finding never to be suppressed, got %+v", out)
	}
}

func TestFilterCommentedPSAFindings_IgnoresNonPSAFindings(t *testing.T) {
	findings := []check.Finding{{CheckID: "namespace", File: "x.yaml", Name: "foo"}}
	out := filterCommentedPSAFindings(findings)
	if len(out) != 1 {
		t.Errorf("expected non-psa-labels findings to pass through untouched, got %+v", out)
	}
}

func TestFilterCommentedPSAFindings_NoAppRootLeavesFindingUntouched(t *testing.T) {
	findings := []check.Finding{{
		CheckID: "psa-labels", File: "overlays/prod/namespace.yaml", Name: "foo",
		Extra: map[string]string{psaMissingLabelsExtraKey: "pod-security.kubernetes.io/enforce"},
	}}
	out := filterCommentedPSAFindings(findings)
	if len(out) != 1 {
		t.Errorf("expected a finding with no resolvable app root to pass through, got %+v", out)
	}
}

func TestAppRootFromBaseFile(t *testing.T) {
	root, ok := appRootFromBaseFile(filepath.FromSlash("myapp/base/namespace.yaml"))
	if !ok || root != filepath.FromSlash("myapp") {
		t.Errorf("appRootFromBaseFile = (%q, %v), want (%q, true)", root, ok, "myapp")
	}
	if _, ok := appRootFromBaseFile(filepath.FromSlash("overlays/prod/namespace.yaml")); ok {
		t.Error("expected no app root when there's no 'base' segment")
	}
}

func TestAllLabelsCommented(t *testing.T) {
	commented := map[string]bool{"a": true, "b": true}
	if !allLabelsCommented("a,b", commented) {
		t.Error("expected all-commented to be true when every label is present")
	}
	if allLabelsCommented("a,c", commented) {
		t.Error("expected false when one label isn't commented")
	}
	if allLabelsCommented("", commented) {
		t.Error("expected false for an empty missing-labels list")
	}
	if allLabelsCommented("a", nil) {
		t.Error("expected false for a nil commented map")
	}
}
