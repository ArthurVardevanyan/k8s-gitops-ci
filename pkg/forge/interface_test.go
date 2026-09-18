package forge

import "testing"

func TestValidPR(t *testing.T) {
	cases := []struct {
		name string
		pr   string
		want bool
	}{
		{name: "empty", pr: "", want: false},
		{name: "numeric", pr: "123", want: true},
		{name: "single-digit", pr: "1", want: true},
		{name: "template-placeholder", pr: "{{ params.pr }}", want: false},
		{name: "bare-template", pr: "{{pr}}", want: false},
		// Not a placeholder; an unresolvable ref fails loudly at clone
		// time rather than silently checking out the wrong branch.
		{name: "non-numeric", pr: "abc", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidPR(tc.pr); got != tc.want {
				t.Errorf("ValidPR(%q) = %v, want %v", tc.pr, got, tc.want)
			}
		})
	}
}

func TestResolvePRRef(t *testing.T) {
	const ghFmt = "refs/pull/%s/head"
	const glFmt = "merge-requests/%s/head"
	cases := []struct {
		name   string
		raw    string
		pr     string
		refFmt string
		want   string
	}{
		{name: "explicit-raw-wins-over-pr", raw: "v1.2.3", pr: "42", refFmt: ghFmt, want: "v1.2.3"},
		{name: "explicit-raw-wins-when-pr-empty", raw: "deadbeef", pr: "", refFmt: ghFmt, want: "deadbeef"},
		{name: "pr-resolves-to-github-head-ref", raw: "", pr: "42", refFmt: ghFmt, want: "refs/pull/42/head"},
		{name: "pr-resolves-to-gitlab-mr-ref", raw: "", pr: "7", refFmt: glFmt, want: "merge-requests/7/head"},
		{name: "no-raw-no-pr-defaults-to-head", raw: "", pr: "", refFmt: ghFmt, want: "HEAD"},
		{name: "placeholder-pr-defaults-to-head", raw: "", pr: "{{ params.pr }}", refFmt: ghFmt, want: "HEAD"},
		{name: "empty-format-defaults-to-head", raw: "", pr: "42", refFmt: "", want: "HEAD"},
		{name: "explicit-raw-wins-over-placeholder-pr", raw: "v1.2.3", pr: "{{ params.pr }}", refFmt: ghFmt, want: "v1.2.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvePRRef(tc.raw, tc.pr, tc.refFmt); got != tc.want {
				t.Errorf("ResolvePRRef(%q, %q, %q) = %q, want %q", tc.raw, tc.pr, tc.refFmt, got, tc.want)
			}
		})
	}
}

// TestNullForgeResolveRevision guards the pre-forge behavior for URLs no
// registered forge claims: an explicit raw wins, a valid PR number
// resolves to the GitHub-style PR head ref, anything else is HEAD. A
// remote that doesn't serve that ref makes the clone fail loudly rather
// than silently validating the default branch.
func TestNullForgeResolveRevision(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		pr   string
		want string
	}{
		{name: "explicit-raw-wins", raw: "v1.2.3", pr: "42", want: "v1.2.3"},
		{name: "pr-resolves-to-github-style-ref", raw: "", pr: "42", want: "refs/pull/42/head"},
		{name: "no-raw-no-pr-defaults-to-head", raw: "", pr: "", want: "HEAD"},
		{name: "placeholder-pr-defaults-to-head", raw: "", pr: "{{ params.pr }}", want: "HEAD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nilForge.ResolveRevision(tc.raw, tc.pr); got != tc.want {
				t.Errorf("nullForge.ResolveRevision(%q, %q) = %q, want %q", tc.raw, tc.pr, got, tc.want)
			}
		})
	}
}
