package gitlab

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/forge"
)

func TestGlabStdin_InheritsParentEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes a POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := "#!/bin/sh\necho \"MARKER=$GITOPS_CI_TEST_MARKER\"\necho \"HOST=$GITLAB_HOST\"\necho \"TOKEN=$GITLAB_TOKEN\"\n"
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GITOPS_CI_TEST_MARKER", "inherited")
	t.Setenv("CI_JOB_TOKEN", "job-secret")

	c := &Client{host: "gitlab.example.com", repo: "org/repo", mr: "1"}
	out, err := c.glab("mr", "view")
	if err != nil {
		t.Fatalf("glab() error: %v", err)
	}
	want := "MARKER=inherited\nHOST=gitlab.example.com\nTOKEN=job-secret"
	if out != want {
		t.Errorf("glab() output = %q, want %q", out, want)
	}
}

func TestExtractProject(t *testing.T) {
	cases := []struct {
		in          string
		wantHost    string
		wantProject string
	}{
		{
			in:          "https://gitlab.com/group/subgroup/project.git",
			wantHost:    "gitlab.com",
			wantProject: "group/subgroup/project",
		},
		{
			in:          "git@gitlab.example.com:group/project.git",
			wantHost:    "gitlab.example.com",
			wantProject: "group/project",
		},
		{
			in:          "http://self-hosted:8080/nested/deep/group/repo.git",
			wantHost:    "self-hosted",
			wantProject: "nested/deep/group/repo",
		},
		{
			in:          "gitlab.com/org/repo",
			wantHost:    "gitlab.com",
			wantProject: "org/repo",
		},
		{
			in:          "bare/repo/path",
			wantHost:    "",
			wantProject: "bare/repo/path",
		},
		{
			in:          "",
			wantHost:    "",
			wantProject: "",
		},
	}
	for _, tc := range cases {
		gotHost, gotProject := ExtractProject(tc.in)
		if gotHost != tc.wantHost || gotProject != tc.wantProject {
			t.Errorf("ExtractProject(%q) = (%q, %q), want (%q, %q)",
				tc.in, gotHost, gotProject, tc.wantHost, tc.wantProject)
		}
	}
}

func TestEncodeProject(t *testing.T) {
	cases := map[string]string{
		"org/repo":               "org%2Frepo",
		"group/subgroup/project": "group%2Fsubgroup%2Fproject",
		"single":                 "single",
	}
	for in, want := range cases {
		if got := EncodeProject(in); got != want {
			t.Errorf("EncodeProject(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClient_Accessors(t *testing.T) {
	c := NewClient("https://gitlab.example.com/group/subgroup/repo.git", "42")
	if c.Host() != "gitlab.example.com" {
		t.Errorf("Host() = %q, want gitlab.example.com", c.Host())
	}
	if c.Repo() != "group/subgroup/repo" {
		t.Errorf("Repo() = %q, want group/subgroup/repo", c.Repo())
	}
	if c.MR() != "42" {
		t.Errorf("MR() = %q, want 42", c.MR())
	}
	if c.RepoSpec() != "gitlab.example.com/group/subgroup/repo" {
		t.Errorf("RepoSpec() = %q, want gitlab.example.com/group/subgroup/repo", c.RepoSpec())
	}

	cGitlabDotCom := NewClient("https://gitlab.com/group/repo.git", "1")
	if cGitlabDotCom.RepoSpec() != "group/repo" {
		t.Errorf("RepoSpec() = %q, want group/repo", cGitlabDotCom.RepoSpec())
	}
}

func TestClient_IsAvailable(t *testing.T) {
	cases := []struct {
		name string
		c    *Client
		want bool
	}{
		{name: "valid", c: NewClient("https://gitlab.com/org/repo", "12"), want: true},
		{name: "disabled", c: NewDisabledClient(), want: false},
		{name: "empty-mr", c: NewClient("https://gitlab.com/org/repo", ""), want: false},
		{name: "non-numeric-mr", c: NewClient("https://gitlab.com/org/repo", "abc"), want: false},
		{name: "empty-url", c: NewClient("", "12"), want: false},
		{name: "placeholder-mr", c: NewClient("https://gitlab.com/org/repo", "{{ params.pr }}"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.IsAvailable(); got != tc.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateMRTitle_Patterns(t *testing.T) {
	ok := []string{
		"feat: add feature",
		"fix(ci): fix gitlab check",
		"chore: bump deps",
		"revert: revert bad commit",
		"docs(readme)!: update instructions",
	}
	bad := []string{
		"not conventional",
		"",
		"WIP",
	}
	for _, s := range ok {
		if err := ValidateMRTitleString(s); err != nil {
			t.Errorf("%q should be valid: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := ValidateMRTitleString(s); err == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestMRTitleSuggestion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes a POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := "#!/bin/sh\necho \"{\\\"title\\\":\\\"$FAKE_GLAB_TITLE\\\"}\"\n"
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	c := NewClient("https://gitlab.com/org/repo", "1")

	suggest := func(title string) string {
		if !strings.Contains(title, "TICKET") {
			return "consider referencing a ticket"
		}
		return ""
	}

	t.Run("nil hook disables suggestions", func(t *testing.T) {
		orig := TitleSuggestion
		TitleSuggestion = nil
		t.Cleanup(func() { TitleSuggestion = orig })
		t.Setenv("FAKE_GLAB_TITLE", "feat: add thing")
		if got := MRTitleSuggestion(c); got != "" {
			t.Errorf("expected no suggestion when hook is nil, got %q", got)
		}
	})

	t.Run("passing title with suggestion", func(t *testing.T) {
		orig := TitleSuggestion
		TitleSuggestion = suggest
		t.Cleanup(func() { TitleSuggestion = orig })
		t.Setenv("FAKE_GLAB_TITLE", "feat: add thing")
		if got := MRTitleSuggestion(c); got != "consider referencing a ticket" {
			t.Errorf("MRTitleSuggestion = %q, want suggestion", got)
		}
	})

	t.Run("failing required prefix suppresses suggestion", func(t *testing.T) {
		orig := TitleSuggestion
		TitleSuggestion = suggest
		t.Cleanup(func() { TitleSuggestion = orig })
		t.Setenv("FAKE_GLAB_TITLE", "not conventional")
		if got := MRTitleSuggestion(c); got != "" {
			t.Errorf("expected no suggestion when prefix fails, got %q", got)
		}
	})

	t.Run("unavailable client", func(t *testing.T) {
		orig := TitleSuggestion
		TitleSuggestion = suggest
		t.Cleanup(func() { TitleSuggestion = orig })
		if got := MRTitleSuggestion(NewDisabledClient()); got != "" {
			t.Errorf("expected no suggestion for unavailable client, got %q", got)
		}
	})
}

func TestValidateMRChecklistString(t *testing.T) {
	body := `- [x] a
- [ ] b
- [x] c
- [ ] d
`
	spec := forge.ChecklistSpec{
		Items: []forge.ChecklistItem{
			{ID: "a", LabelPattern: "a"},
			{ID: "b", LabelPattern: "b"},
			{ID: "c", LabelPattern: "c"},
			{ID: "d", LabelPattern: "d"},
		},
		Required:  []string{"d"},
		SelectOne: []forge.SelectOneGroup{{Name: "group", Options: []string{"a", "c"}}},
	}
	if err := ValidateMRChecklistString(body, spec); err == nil {
		t.Error("expected error for unchecked required and multiple select-one")
	}
}

func TestGetUnsignedCommits_DisabledClient(t *testing.T) {
	commits, err := GetUnsignedCommits(NewDisabledClient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected no commits, got %v", commits)
	}
}

func TestFormatCommitDesc(t *testing.T) {
	cmt := mrCommit{
		ID:      "1234567890abcdef",
		ShortID: "1234567",
		Title:   "feat: my commit",
	}
	if got := formatCommitDesc(cmt); got != "1234567 feat: my commit" {
		t.Errorf("formatCommitDesc = %q, want '1234567 feat: my commit'", got)
	}

	cmtNoShort := mrCommit{
		ID:    "abcdef1234567890",
		Title: "fix: another",
	}
	if got := formatCommitDesc(cmtNoShort); got != "abcdef1 fix: another" {
		t.Errorf("formatCommitDesc = %q, want 'abcdef1 fix: another'", got)
	}
}

func TestSanitizeURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			in:   "https://gitlab-ci-token:glpat-secret@gitlab.example.com/repo.git",
			want: "https://***@gitlab.example.com/repo.git",
		},
		{
			in:   "http://user:pass@host/path",
			want: "http://***@host/path",
		},
		{
			in:   "https://gitlab.com/org/repo.git",
			want: "https://gitlab.com/org/repo.git",
		},
	}
	for _, tc := range cases {
		if got := sanitizeURL(tc.in); got != tc.want {
			t.Errorf("sanitizeURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGlForgeResolveRevision(t *testing.T) {
	f := NewForge()
	cases := []struct {
		name string
		raw  string
		pr   string
		want string
	}{
		{name: "explicit-raw-wins", raw: "v1.2.3", pr: "42", want: "v1.2.3"},
		{name: "pr-resolves-to-mr-head-ref", raw: "", pr: "42", want: "refs/merge-requests/42/head"},
		{name: "no-raw-no-pr-defaults-to-head", raw: "", pr: "", want: "HEAD"},
		{name: "placeholder-pr-defaults-to-head", raw: "", pr: "{{ params.pr }}", want: "HEAD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.ResolveRevision(tc.raw, tc.pr); got != tc.want {
				t.Errorf("ResolveRevision(%q, %q) = %q, want %q", tc.raw, tc.pr, got, tc.want)
			}
		})
	}
}

func TestGlForgeMatches(t *testing.T) {
	f := NewForge()

	t.Run("explicit-gitlab", func(t *testing.T) {
		if got := f.Matches("", "gitlab"); got != forge.AffinityExplicit {
			t.Errorf("Matches('', 'gitlab') = %v, want AffinityExplicit", got)
		}
	})

	t.Run("explicit-other", func(t *testing.T) {
		if got := f.Matches("https://gitlab.com/org/repo", "github"); got != forge.AffinityNone {
			t.Errorf("Matches(..., 'github') = %v, want AffinityNone", got)
		}
	})

	t.Run("url-affinity-gitlab-com", func(t *testing.T) {
		if got := f.Matches("https://gitlab.com/org/repo", ""); got != forge.AffinityURL {
			t.Errorf("Matches('https://gitlab.com/org/repo', '') = %v, want AffinityURL", got)
		}
	})

	t.Run("url-affinity-custom-gitlab-host", func(t *testing.T) {
		t.Setenv("GITLAB_HOST", "gitlab.myorg.internal")
		if got := f.Matches("https://gitlab.myorg.internal/org/repo", ""); got != forge.AffinityURL {
			t.Errorf("Matches with GITLAB_HOST = %v, want AffinityURL", got)
		}
	})

	t.Run("env-affinity-gitlab-ci", func(t *testing.T) {
		t.Setenv("GITLAB_CI", "true")
		if got := f.Matches("https://unknown.com/org/repo", ""); got != forge.AffinityEnv {
			t.Errorf("Matches in GITLAB_CI = %v, want AffinityEnv", got)
		}
	})

	t.Run("no-match", func(t *testing.T) {
		if got := f.Matches("https://github.com/org/repo", ""); got != forge.AffinityNone {
			t.Errorf("Matches for github = %v, want AffinityNone", got)
		}
	})
}

func TestGlForgeFillFromEnv(t *testing.T) {
	f := NewForge()
	t.Setenv("CI_PROJECT_URL", "https://gitlab.com/group/repo")
	t.Setenv("CI_MERGE_REQUEST_IID", "99")
	t.Setenv("CI_COMMIT_SHA", "0123456789abcdef")
	t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")

	url, pr, rev, branch := f.FillFromEnv()
	if url != "https://gitlab.com/group/repo" {
		t.Errorf("url = %q", url)
	}
	if pr != "99" {
		t.Errorf("pr = %q", pr)
	}
	if rev != "0123456789abcdef" {
		t.Errorf("rev = %q", rev)
	}
	if branch != "main" {
		t.Errorf("branch = %q", branch)
	}
}

func TestCheckAPIError(t *testing.T) {
	msgErr := []byte(`{"message": "401 Unauthorized"}`)
	if err := checkAPIError(msgErr); err == nil || !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Errorf("checkAPIError(msgErr) = %v, want 401 Unauthorized error", err)
	}

	genericErr := []byte(`{"error": "invalid_token"}`)
	if err := checkAPIError(genericErr); err == nil || !strings.Contains(err.Error(), "invalid_token") {
		t.Errorf("checkAPIError(genericErr) = %v, want invalid_token error", err)
	}

	cleanResp := []byte(`[{"id": 1}]`)
	if err := checkAPIError(cleanResp); err != nil {
		t.Errorf("checkAPIError(cleanResp) unexpected error: %v", err)
	}
}

func TestGetUnsignedCommits_MockGlab(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes a POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := `#!/bin/sh
if echo "$*" | grep -q "commits.*signature"; then
  if echo "$*" | grep -q "sha1"; then
    echo '{"verification_status":"verified"}'
  else
    echo '{"message":"404 Not Found"}' >&2
    exit 1
  fi
elif echo "$*" | grep -q "commits"; then
  echo '[{"id":"sha1111111111111111111111111111111111111","short_id":"sha1111","title":"commit 1"},{"id":"sha2222222222222222222222222222222222222","short_id":"sha2222","title":"commit 2"}]'
else
  echo "[]"
fi
`
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	c := NewClient("https://gitlab.com/org/repo", "1")
	unsigned, err := GetUnsignedCommits(c)
	if err != nil {
		t.Fatalf("GetUnsignedCommits error: %v", err)
	}
	if len(unsigned) != 1 || !strings.Contains(unsigned[0], "sha2222 commit 2") {
		t.Errorf("GetUnsignedCommits = %v, want ['sha2222 commit 2']", unsigned)
	}
}

func TestFetchFiles_MockGlab(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes a POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := `#!/bin/sh
if echo "$*" | grep -q "diffs"; then
  echo '[{"new_path":"added.txt","new_file":true},{"old_path":"removed.txt","deleted_file":true},{"old_path":"old.txt","new_path":"new.txt","renamed_file":true},{"new_path":"mod.txt"}]'
else
  echo "[]"
fi
`
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	f := NewForge()
	files, err := f.FetchFiles("https://gitlab.com/org/repo", "1")
	if err != nil {
		t.Fatalf("FetchFiles error: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("FetchFiles returned %d files, want 4", len(files))
	}
	if files[0].Status != "added" || files[0].Filename != "added.txt" {
		t.Errorf("files[0] = %+v, want added.txt added", files[0])
	}
	if files[1].Status != "removed" || files[1].Filename != "removed.txt" {
		t.Errorf("files[1] = %+v, want removed.txt removed", files[1])
	}
	if files[2].Status != "renamed" || files[2].Filename != "new.txt" {
		t.Errorf("files[2] = %+v, want new.txt renamed", files[2])
	}
	if files[3].Status != "modified" || files[3].Filename != "mod.txt" {
		t.Errorf("files[3] = %+v, want mod.txt modified", files[3])
	}
}
