package main

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

func TestBuildCIReport(t *testing.T) {
	cases := []struct {
		name         string
		in           ciReportBody
		wantContains []string
		wantExcludes []string
	}{
		{
			name: "ci pass, replay pass",
			in:   ciReportBody{ciStatus: statusPass, replayStatus: statusPass},
			wantContains: []string{
				ciReportMarker,
				"## CI Pipeline Status",
				"✅ **`task ci` passed**",
				"Live regression replay (HomeLab)",
				"All replayed PRs passed",
				"does not block merge",
			},
		},
		{
			name: "ci fail still reports and blocks",
			in:   ciReportBody{ciStatus: statusFail, replayStatus: statusSkipped},
			wantContains: []string{
				"❌ **`task ci` failed**",
				"This blocks merge",
				"The replay was skipped",
			},
		},
		{
			name: "replay fail is framed as non-blocking review prompt",
			in:   ciReportBody{ciStatus: statusPass, replayStatus: statusFail},
			wantContains: []string{
				"✅ **`task ci` passed**",
				"review the diff",
				"not necessarily a regression",
			},
			wantExcludes: []string{
				// A replay failure must never present as a blocking failure.
				"blocks merge\n",
			},
		},
		{
			name: "embeds the replay report when provided",
			in: ciReportBody{
				ciStatus:     statusPass,
				replayStatus: statusFail,
				replayReport: "| PR | Result |\n|----|--------|\n| #1 | ❌ Fail |",
			},
			wantContains: []string{
				"| #1 | ❌ Fail |",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildCIReport(c.in)
			if !strings.HasPrefix(got, ciReportMarker) {
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

func TestReadReplayReport(t *testing.T) {
	if got := readReplayReport(""); got != "" {
		t.Errorf("empty path should yield empty string, got %q", got)
	}
	if got := readReplayReport(filepath.Join(t.TempDir(), "nope.md")); got != "" {
		t.Errorf("missing file should yield empty string, got %q", got)
	}
	f := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(f, []byte("  hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readReplayReport(f); got != "hello" {
		t.Errorf("readReplayReport trimmed content = %q, want %q", got, "hello")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
