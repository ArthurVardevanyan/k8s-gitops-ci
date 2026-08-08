package cireport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeStatus(t *testing.T) {
	cases := []struct {
		in   string
		want status
	}{
		{"pass", statusPass},
		{"passed", statusPass},
		{"success", statusPass},
		{"0", statusPass},
		{"warn", statusWarn},
		{"warning", statusWarn},
		{"1", statusWarn},
		{"fail", statusFail},
		{"failed", statusFail},
		{"error", statusFail},
		{"", statusSkipped},
		{"skipped", statusSkipped},
		{"weird", statusUnknown},
	}
	for _, c := range cases {
		if got := normalizeStatus(c.in); got != c.want {
			t.Errorf("normalizeStatus(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStatusIcon(t *testing.T) {
	cases := map[status]string{
		statusPass:    "✅",
		statusWarn:    "⚠️",
		statusFail:    "❌",
		statusSkipped: "⚪",
		statusUnknown: "⚠️",
	}
	for s, want := range cases {
		if got := s.icon(); got != want {
			t.Errorf("status(%q).icon() = %q, want %q", s, got, want)
		}
	}
}

func TestBuild(t *testing.T) {
	cases := []struct {
		name         string
		in           Options
		wantContains []string
		wantExcludes []string
	}{
		{
			name: "ci pass, replay pass, custom label",
			in:   Options{CIStatus: "pass", ReplayStatus: "pass", ReplayLabel: "HomeLab"},
			wantContains: []string{
				Marker,
				"## CI Pipeline Status",
				"✅ **`task ci` passed**",
				"Live regression replay (HomeLab)",
				"All replayed PRs passed",
				"does not block merge",
			},
			wantExcludes: []string{
				// The redundant footer was removed.
				"Posted by",
			},
		},
		{
			name: "default label when none given",
			in:   Options{CIStatus: "pass", ReplayStatus: "skipped"},
			wantContains: []string{
				"Live regression replay (live GitOps repo)",
			},
		},
		{
			name: "docs link rendered only when provided",
			in:   Options{CIStatus: "pass", ReplayStatus: "pass", DocsURL: "https://example.test/docs"},
			wantContains: []string{
				"[the docs](https://example.test/docs)",
			},
		},
		{
			name: "ci fail still reports and blocks",
			in:   Options{CIStatus: "fail", ReplayStatus: "skipped"},
			wantContains: []string{
				"❌ **`task ci` failed**",
				"blocks merge",
				"See the pipeline logs for the full output",
				"The replay was skipped",
			},
		},
		{
			name: "ci fail embeds failing-step detail when provided",
			in: Options{
				CIStatus:     "fail",
				CIDetail:     "golangci-lint: 1 issue\nfoo.go:1:1: something",
				ReplayStatus: "skipped",
			},
			wantContains: []string{
				"❌ **`task ci` failed**",
				"The failing step detail is below",
				"Failing step detail",
				"foo.go:1:1: something",
			},
		},
		{
			name: "replay warn is framed as a non-blocking review prompt",
			in:   Options{CIStatus: "pass", ReplayStatus: "warn"},
			wantContains: []string{
				"✅ **`task ci` passed**",
				"⚠️ Expand: Live regression replay",
				"review the diff",
				"not necessarily a regression",
			},
			wantExcludes: []string{
				// A replay result must never present as a blocking failure.
				"blocks merge",
			},
		},
		{
			name: "replay fail (harness error) is not a review prompt",
			in:   Options{CIStatus: "pass", ReplayStatus: "fail"},
			wantContains: []string{
				"✅ **`task ci` passed**",
				"harness/setup error",
			},
			wantExcludes: []string{
				"review the diff",
				"blocks merge",
			},
		},
		{
			name: "embeds the replay report when provided",
			in: Options{
				CIStatus:     "pass",
				ReplayStatus: "warn",
				ReplayReport: "| PR | Result |\n|----|--------|\n| #1 | ❌ Fail |",
			},
			wantContains: []string{
				"| #1 | ❌ Fail |",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Build(c.in)
			if !strings.HasPrefix(got, Marker) {
				t.Errorf("body must start with the marker for UpsertComment to find it; got prefix %q", firstLine(got))
			}
			for _, want := range c.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("report missing %q\n--- body ---\n%s", want, got)
				}
			}
			for _, ex := range c.wantExcludes {
				if strings.Contains(got, ex) {
					t.Errorf("report unexpectedly contains %q\n--- body ---\n%s", ex, got)
				}
			}
		})
	}
}

func TestReadDetailFile(t *testing.T) {
	if got := ReadDetailFile(""); got != "" {
		t.Errorf("empty path should yield empty string, got %q", got)
	}
	if got := ReadDetailFile(filepath.Join(t.TempDir(), "nope.md")); got != "" {
		t.Errorf("missing file should yield empty string, got %q", got)
	}
	f := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(f, []byte("  hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadDetailFile(f); got != "hello" {
		t.Errorf("ReadDetailFile trimmed content = %q, want %q", got, "hello")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
