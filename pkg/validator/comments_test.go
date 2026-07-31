package validator

import (
	"strings"
	"testing"
)

func TestGroupBuildErrors(t *testing.T) {
	errors := []string{
		"kustomize build kyverno/overlays/pd1040: accumulating components: no such file or directory",
		"kustomize build kyverno/overlays/pd1011: accumulating components: no such file or directory",
		"kustomize build kyverno/overlays/dv1157: accumulating components: no such file or directory",
		"kustomize build falco/overlays/pd11: resource not found",
		"kubeconform violations",
	}

	groups, other := groupBuildErrors(errors)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	if groups[0].Cause != "accumulating components: no such file or directory" {
		t.Errorf("group[0] cause = %q", groups[0].Cause)
	}
	if len(groups[0].Overlays) != 3 {
		t.Errorf("group[0] overlays = %d, want 3", len(groups[0].Overlays))
	}

	if groups[1].Cause != "resource not found" {
		t.Errorf("group[1] cause = %q", groups[1].Cause)
	}
	if len(groups[1].Overlays) != 1 {
		t.Errorf("group[1] overlays = %d, want 1", len(groups[1].Overlays))
	}

	if len(other) != 1 || other[0] != "kubeconform violations" {
		t.Errorf("other = %v, want [kubeconform violations]", other)
	}
}

func TestGroupBuildErrors_NoBuildErrors(t *testing.T) {
	errors := []string{"lint error", "schema validation failed"}
	groups, other := groupBuildErrors(errors)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	if len(other) != 2 {
		t.Errorf("expected 2 other errors, got %d", len(other))
	}
}

func TestGroupBuildErrors_MalformedPrefixFallsBackToOther(t *testing.T) {
	// Starts with the "kustomize build " prefix but has no ": " separator
	// after it (e.g. a truncated/malformed message) - must not panic and
	// must be preserved verbatim in "other" rather than silently dropped.
	errors := []string{"kustomize build incomplete-message-with-no-cause"}
	groups, other := groupBuildErrors(errors)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	if len(other) != 1 || other[0] != errors[0] {
		t.Errorf("other = %v, want %v", other, errors)
	}
}

func TestFormatBuildErrors(t *testing.T) {
	groups := []buildErrorGroup{
		{
			Cause:    "accumulating components: no such file or directory",
			Overlays: []string{"kyverno/overlays/pd1011", "kyverno/overlays/pd1040", "kyverno/overlays/pd1073"},
		},
	}
	var sb strings.Builder
	formatBuildErrors(&sb, groups)
	got := sb.String()

	if !strings.Contains(got, "**3 overlay(s)**") {
		t.Errorf("expected overlay count, got:\n%s", got)
	}
	if !strings.Contains(got, "accumulating components: no such file or directory") {
		t.Errorf("expected cause in output, got:\n%s", got)
	}
	if !strings.Contains(got, "`pd1011`") {
		t.Errorf("expected overlay name, got:\n%s", got)
	}
}

func TestFormatBuildErrors_ManyOverlaysAreTruncated(t *testing.T) {
	overlays := make([]string, 20)
	for i := range overlays {
		overlays[i] = "app/overlays/cluster" + strings.Repeat("0", i)
	}
	groups := []buildErrorGroup{{Cause: "missing dir", Overlays: overlays}}
	var sb strings.Builder
	formatBuildErrors(&sb, groups)
	got := sb.String()

	if !strings.Contains(got, "+15 more") {
		t.Errorf("expected truncation indicator, got:\n%s", got)
	}
}

func TestFormatBuildErrors_LongCauseIsTruncated(t *testing.T) {
	groups := []buildErrorGroup{{Cause: strings.Repeat("x", 300), Overlays: []string{"app/overlays/a"}}}
	var sb strings.Builder
	formatBuildErrors(&sb, groups)
	got := sb.String()

	if !strings.Contains(got, strings.Repeat("x", 200)+"...") {
		t.Errorf("expected the cause to be truncated at 200 chars, got:\n%s", got)
	}
	if strings.Contains(got, strings.Repeat("x", 201)) {
		t.Errorf("expected the cause not to exceed 200 chars before the ellipsis, got:\n%s", got)
	}
}

func TestFixHints(t *testing.T) {
	tests := []struct {
		name     string
		findings []LintFinding
		want     []string
	}{
		{
			name:     "no actionable findings",
			findings: []LintFinding{{Check: "shellcheck", Files: []string{"foo.sh"}}, {Check: "golangci"}},
			want:     nil,
		},
		{
			name:     "config sort",
			findings: []LintFinding{{Check: "config-sort", Files: []string{"config.yaml"}}},
			want:     []string{"k8s-gitops-ci sort-configs"},
		},
		{
			name:     "kustomize fix with files",
			findings: []LintFinding{{Check: "kustomize fix", Files: []string{"app1/base/kustomization.yaml", "app2/overlays/cluster/kustomization.yaml"}}},
			want:     []string{"k8s-gitops-ci kustomize-fix app1/base/kustomization.yaml app2/overlays/cluster/kustomization.yaml"},
		},
		{
			name:     "prettier with files",
			findings: []LintFinding{{Check: "prettier", Files: []string{"deploy.yaml", "values.yaml"}}},
			want:     []string{"prettier --write deploy.yaml values.yaml"},
		},
		{
			name:     "markdownlint with files",
			findings: []LintFinding{{Check: "markdownlint", Files: []string{"docs/CI.md", "docs/DEVELOPMENT.md"}}},
			want:     []string{"markdownlint docs/CI.md docs/DEVELOPMENT.md"},
		},
		{
			name:     "markdownlint without files",
			findings: []LintFinding{{Check: "markdownlint"}},
			want:     []string{"markdownlint <file>"},
		},
		{
			name:     "scaffold table",
			findings: []LintFinding{{Check: "scaffold table"}},
			want:     []string{"k8s-gitops-ci update-scaffold-status"},
		},
		{
			name: "multiple actionable",
			findings: []LintFinding{
				{Check: "config-sort", Files: []string{"config.yaml"}},
				{Check: "prettier", Files: []string{"a.yaml"}},
				{Check: "shellcheck", Files: []string{"b.sh"}},
			},
			want: []string{"k8s-gitops-ci sort-configs", "prettier --write a.yaml"},
		},
		{
			name: "deduplicates repeated check",
			findings: []LintFinding{
				{Check: "config-sort", Files: []string{"config.yaml"}},
				{Check: "config-sort", Files: []string{"other.yaml"}},
			},
			want: []string{"k8s-gitops-ci sort-configs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixHints(tt.findings)
			if len(got) != len(tt.want) {
				t.Fatalf("fixHints() = %v, want %v", got, tt.want)
			}
			for i, h := range got {
				if h != tt.want[i] {
					t.Errorf("fixHints()[%d] = %q, want %q", i, h, tt.want[i])
				}
			}
		})
	}
}

// TestFixHints_EveryHintedCommandIsARegisteredSubcommand guards against
// hinting a command this binary doesn't actually support: every non-
// third-party command name below must appear as a case in
// cmd/k8s-gitops-ci/main.go's subcommand dispatch.
func TestFixHints_EveryHintedCommandIsARegisteredSubcommand(t *testing.T) {
	registered := map[string]bool{
		"sort-configs":           true,
		"kustomize-fix":          true,
		"update-scaffold-status": true,
		"markdownlint":           true,
		"prettier":               true,
		"shellcheck":             true,
		"golangci":               true,
		"kubeconform":            true,
		"check-starting-csv":     true,
		"ghost-patches":          true,
		"yaml-syntax":            true,
		"build-yaml":             true,
		"test-all":               true,
		"scan-all":               true,
		"pipeline":               true,
		"ci":                     true,
		"version":                true,
	}
	for check, hint := range hintByCheck {
		fields := strings.Fields(hint.command)
		if len(fields) < 2 {
			t.Fatalf("check %q: unexpected command shape %q", check, hint.command)
		}
		if fields[0] != "k8s-gitops-ci" {
			// Third-party tool (e.g. "prettier", "markdownlint") - not this
			// binary's own subcommand list, nothing to verify here.
			continue
		}
		sub := fields[1]
		if !registered[sub] {
			t.Errorf("check %q hints at %q, but %q is not a registered subcommand in cmd/k8s-gitops-ci/main.go", check, hint.command, sub)
		}
	}
}

func TestTruncateDetails(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello world",
			maxLen: 100,
			want:   "hello world",
		},
		{
			name:   "truncates at last newline",
			input:  "line1\nline2\nline3\nline4",
			maxLen: 15,
			want:   "line1\nline2\n... (truncated)",
		},
		{
			name:   "exact length unchanged",
			input:  "exact",
			maxLen: 5,
			want:   "exact",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateDetails(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateDetails() = %q, want %q", got, tt.want)
			}
		})
	}
}
