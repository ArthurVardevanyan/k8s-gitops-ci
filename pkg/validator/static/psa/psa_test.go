package psa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReader_CompleteLabels(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: good
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/audit-version: latest
`
	errs := ValidateReader(strings.NewReader(data), "ns.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateReader_MissingAllLabels(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: bad
`
	errs := ValidateReader(strings.NewReader(data), "ns.yaml")
	if len(errs) != 1 || len(errs[0].MissingLabels) != 6 {
		t.Fatalf("expected 6 missing labels: %v", errs)
	}
}

func TestValidateReader_InvalidLevel(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: ns
  labels:
    pod-security.kubernetes.io/enforce: bad
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/audit-version: latest
`
	errs := ValidateReader(strings.NewReader(data), "ns.yaml")
	if len(errs) != 1 || !strings.Contains(errs[0].MissingLabels[0], "invalid value") {
		t.Errorf("expected invalid level error: %v", errs)
	}
}

func TestFormatComment(t *testing.T) {
	err := ValidationError{File: "ns.yaml", Name: "ns", MissingLabels: []string{"pod-security.kubernetes.io/enforce"}}
	s := FormatComment([]ValidationError{err})
	if !strings.Contains(s, Marker) || !strings.Contains(s, "Namespace") {
		t.Errorf("unexpected comment: %q", s)
	}
}

func TestFormatComment_Empty(t *testing.T) {
	if s := FormatComment(nil); s != "" {
		t.Errorf("expected empty: %q", s)
	}
}

// ── findCommentedPSALabels ────────────────────────────────────────────────

func TestFindCommentedPSALabels_AllSix(t *testing.T) {
	data := `apiVersion: v1
kind: Namespace
metadata:
  name: my-ns
  labels:
    # pod-security.kubernetes.io/enforce: restricted
    # pod-security.kubernetes.io/enforce-version: latest
    # pod-security.kubernetes.io/warn: restricted
    # pod-security.kubernetes.io/warn-version: latest
    # pod-security.kubernetes.io/audit: restricted
    # pod-security.kubernetes.io/audit-version: latest
`
	got := findCommentedPSALabels([]byte(data))
	labels, ok := got["my-ns"]
	if !ok || len(labels) != 6 {
		t.Fatalf("expected 6 commented labels for my-ns, got %v", got)
	}
}

func TestFindCommentedPSALabels_VersionSuffixDetected(t *testing.T) {
	// Regression: the level label (e.g. ".../enforce") and its paired
	// "-version" label (e.g. ".../enforce-version") are different keys.
	// Previously only the bare level-label prefix was checked, so a
	// commented-out *-version label alone was never detected.
	data := `metadata:
  name: my-ns
  labels:
    # pod-security.kubernetes.io/enforce-version: latest
    # pod-security.kubernetes.io/warn-version: latest
    # pod-security.kubernetes.io/audit-version: latest
`
	got := findCommentedPSALabels([]byte(data))
	labels, ok := got["my-ns"]
	if !ok || len(labels) != 3 {
		t.Fatalf("expected 3 commented -version labels for my-ns, got %v", got)
	}
	for _, mode := range ValidModes {
		key := "pod-security.kubernetes.io/" + mode + "-version"
		if !labels[key] {
			t.Errorf("expected %s detected in %v", key, labels)
		}
	}
}

func TestFindCommentedPSALabels_LevelAndVersionBothDetected(t *testing.T) {
	data := `metadata:
  name: my-ns
  labels:
    # pod-security.kubernetes.io/enforce: restricted
    # pod-security.kubernetes.io/enforce-version: latest
`
	got := findCommentedPSALabels([]byte(data))
	labels, ok := got["my-ns"]
	if !ok || len(labels) != 2 {
		t.Fatalf("expected both the level and version labels detected, got %v", got)
	}
	if !labels["pod-security.kubernetes.io/enforce"] || !labels["pod-security.kubernetes.io/enforce-version"] {
		t.Errorf("expected both distinct keys present: %v", labels)
	}
}

func TestFindCommentedPSALabels_ActiveLabelsNotCounted(t *testing.T) {
	data := `metadata:
  name: my-ns
  labels:
    pod-security.kubernetes.io/enforce: restricted
`
	got := findCommentedPSALabels([]byte(data))
	if len(got) != 0 {
		t.Errorf("expected no commented labels for an active (uncommented) label, got %v", got)
	}
}

// ── FindCommentedNamespaces ───────────────────────────────────────────────

func TestFindCommentedNamespaces_WalksAllFilesUnderBase(t *testing.T) {
	// Regression: previously, if base/namespace.yaml existed at all, the
	// function returned immediately after checking only that one file -
	// any other file under base/ was never scanned, even one containing
	// the actual commented-out labels.
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePSAFixture(t, filepath.Join(baseDir, "namespace.yaml"),
		"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: my-ns\n")
	writePSAFixture(t, filepath.Join(baseDir, "other.yaml"),
		"metadata:\n  name: my-ns\n  labels:\n    # pod-security.kubernetes.io/enforce: restricted\n")

	got := FindCommentedNamespaces(dir)
	labels, ok := got["my-ns"]
	if !ok || len(labels) != 1 {
		t.Fatalf("expected 1 commented label found in other.yaml, got %v", got)
	}
}

func TestFindCommentedNamespaces_NamespaceManifestNotNamedNamespaceYaml(t *testing.T) {
	// The Namespace manifest itself may live under any filename - the
	// function must not require it to be literally named namespace.yaml.
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePSAFixture(t, filepath.Join(baseDir, "ns.yaml"),
		"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: my-ns\n  labels:\n    # pod-security.kubernetes.io/audit: restricted\n")

	got := FindCommentedNamespaces(dir)
	if labels, ok := got["my-ns"]; !ok || len(labels) != 1 {
		t.Fatalf("expected 1 commented label found in ns.yaml, got %v", got)
	}
}

func writePSAFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
