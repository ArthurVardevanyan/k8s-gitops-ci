package gitlab

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtractProject(t *testing.T) {
	cases := []struct {
		url         string
		wantHost    string
		wantProject string
	}{
		{"", "", ""},
		{"https://gitlab.com/org/repo.git", "gitlab.com", "org/repo"},
		{"https://gitlab.example.com/org/group/subgroup/repo", "gitlab.example.com", "org/group/subgroup/repo"},
		{"git@gitlab.com:org/repo.git", "gitlab.com", "org/repo"},
		{"git@gitlab.example.com:group/subgroup/repo.git", "gitlab.example.com", "group/subgroup/repo"},
		{"gitlab.example.com/org/repo", "gitlab.example.com", "org/repo"},
		{"org/repo", "", "org/repo"},
		{"group/subgroup/repo", "", "group/subgroup/repo"},
	}

	for _, tc := range cases {
		gotHost, gotProject := ExtractProject(tc.url)
		if gotHost != tc.wantHost || gotProject != tc.wantProject {
			t.Errorf("ExtractProject(%q) = (%q, %q), want (%q, %q)",
				tc.url, gotHost, gotProject, tc.wantHost, tc.wantProject)
		}
	}
}

func TestEncodeProject(t *testing.T) {
	if got := EncodeProject("group/subgroup/repo"); got != "group%2Fsubgroup%2Frepo" {
		t.Errorf("EncodeProject() = %q, want group%%2Fsubgroup%%2Frepo", got)
	}
}

func TestClient_Basic(t *testing.T) {
	c := NewClient("https://gitlab.example.com/group/subgroup/repo", "42")
	if !c.IsAvailable() {
		t.Error("expected client to be available")
	}
	if c.Repo() != "group/subgroup/repo" {
		t.Errorf("Repo() = %q, want group/subgroup/repo", c.Repo())
	}
	if c.Host() != "gitlab.example.com" {
		t.Errorf("Host() = %q, want gitlab.example.com", c.Host())
	}
	if c.RepoSpec() != "gitlab.example.com/group/subgroup/repo" {
		t.Errorf("RepoSpec() = %q, want gitlab.example.com/group/subgroup/repo", c.RepoSpec())
	}

	disabled := NewDisabledClient()
	if disabled.IsAvailable() {
		t.Error("disabled client should report unavailable")
	}

	noMR := NewClient("https://gitlab.example.com/group/repo", "")
	if noMR.IsAvailable() {
		t.Error("client without MR should report unavailable")
	}
}

func TestClient_GlabStdin_InheritsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := "#!/bin/sh\necho \"MARKER=$GITOPS_CI_TEST_MARKER\"\necho \"HOST=$GITLAB_HOST\"\n"
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GITOPS_CI_TEST_MARKER", "inherited")

	c := NewClient("https://gitlab.example.com/org/repo", "1")
	out, err := c.glab("version")
	if err != nil {
		t.Fatalf("glab() error: %v", err)
	}
	want := "MARKER=inherited\nHOST=gitlab.example.com"
	if out != want {
		t.Errorf("glab() output = %q, want %q", out, want)
	}
}

func TestValidateMRTitle_And_Checklist(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := `#!/bin/sh
cat <<EOF
{"title":"feat(ci): add gitlab support","description":"- [x] All tests pass"}
EOF
`
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	c := NewClient("https://gitlab.com/org/repo", "1")
	if err := ValidateMRTitle(c); err != nil {
		t.Errorf("ValidateMRTitle() error = %v", err)
	}
	if err := ValidateMRChecklist(c); err != nil {
		t.Errorf("ValidateMRChecklist() error = %v", err)
	}
}

func TestGetUnsignedCommits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script assumes POSIX shell")
	}
	dir := t.TempDir()
	fakeGlab := filepath.Join(dir, "glab")
	script := `#!/bin/sh
if echo "$@" | grep -q "commits$"; then
  cat <<EOF
[
  {"id":"abc1234567890","short_id":"abc1234","title":"signed commit"},
  {"id":"def9876543210","short_id":"def9876","title":"unsigned commit"}
]
EOF
elif echo "$@" | grep -q "abc1234567890/signature"; then
  cat <<EOF
{"verification_status":"verified"}
EOF
elif echo "$@" | grep -q "def9876543210/signature"; then
  cat <<EOF
{"verification_status":"unverified"}
EOF
else
  exit 1
fi
`
	if err := os.WriteFile(fakeGlab, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	c := NewClient("https://gitlab.com/org/repo", "1")
	unsigned, err := GetUnsignedCommits(c)
	if err != nil {
		t.Fatalf("GetUnsignedCommits() error = %v", err)
	}
	if len(unsigned) != 1 || !strings.HasPrefix(unsigned[0], "def9876") {
		t.Errorf("GetUnsignedCommits() = %v, want [def9876 unsigned commit]", unsigned)
	}
}
