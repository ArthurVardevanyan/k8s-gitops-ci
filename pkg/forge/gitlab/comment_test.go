package gitlab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertComment_Disabled(t *testing.T) {
	if err := UpsertComment(NewDisabledClient(), "marker", "body"); err != nil {
		t.Errorf("disabled client should no-op: %v", err)
	}
}

func TestDeleteComments_Disabled(t *testing.T) {
	if err := DeleteComments(NewDisabledClient(), "marker"); err != nil {
		t.Errorf("disabled client should no-op: %v", err)
	}
}

// installFakeGLab creates an executable "glab" shim that records CLI args and stdin.
// If existingNoteID is non-empty, the shim responds to notes list calls with that note ID.
func installFakeGLab(t *testing.T, existingNoteID string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocations.log")
	script := fmt.Sprintf(`#!/bin/sh
{
  printf 'ARGS:'
  for a in "$@"; do printf ' %%s' "$a"; done
  printf '\n'
  printf 'STDIN:%%s\n' "$(cat)"
  echo '---'
} >> %q

# Check if this is listing notes: glab api --paginate projects/.../merge_requests/.../notes
case "$1 $2" in
  "api --paginate"|"api projects"*)
    for arg in "$@"; do
      case "$arg" in
        */notes)
          if [ -n %q ]; then
            printf '[{"id": %s, "body": "<!-- marker --> existing", "system": false}]\n'
          else
            printf '[]\n'
          fi
          exit 0
          ;;
      esac
    done
    ;;
esac
exit 0
`, logPath, existingNoteID, existingNoteID)

	shPath := filepath.Join(dir, "glab")
	if err := os.WriteFile(shPath, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake glab: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading invocation log: %v", err)
	}
	return string(data)
}

func TestUpsertComment_NewComment_BodyReachesStdin(t *testing.T) {
	logPath := installFakeGLab(t, "")
	c := NewClient("https://gitlab.com/group/repo", "10")

	const body = "## GitLab CI Report\n\nall checks passed"
	if err := UpsertComment(c, "<!-- marker -->", body); err != nil {
		t.Fatalf("UpsertComment error: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "POST") {
		t.Errorf("expected POST to create new note, got log:\n%s", log)
	}
	if !strings.Contains(log, "body=@-") {
		t.Errorf("expected -F body=@- for stdin streaming, got log:\n%s", log)
	}
	if !strings.Contains(log, "STDIN:"+body) {
		t.Errorf("expected body to reach stdin, got log:\n%s", log)
	}
}

func TestUpsertComment_ExistingComment_PUTsByID(t *testing.T) {
	logPath := installFakeGLab(t, "456")
	c := NewClient("https://gitlab.com/group/repo", "10")

	const body = "## Updated Report\n\nnew details"
	if err := UpsertComment(c, "<!-- marker -->", body); err != nil {
		t.Fatalf("UpsertComment error: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "PUT") {
		t.Errorf("expected PUT to update existing note, got log:\n%s", log)
	}
	if !strings.Contains(log, "notes/456") {
		t.Errorf("expected note ID 456 in path, got log:\n%s", log)
	}
	if !strings.Contains(log, "STDIN:"+body) {
		t.Errorf("expected body to reach stdin, got log:\n%s", log)
	}
}

func TestDeleteComments_DeletesMatchingNotes(t *testing.T) {
	logPath := installFakeGLab(t, "789")
	c := NewClient("https://gitlab.com/group/repo", "10")

	if err := DeleteComments(c, "<!-- marker -->"); err != nil {
		t.Fatalf("DeleteComments error: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "DELETE") {
		t.Errorf("expected DELETE for note, got log:\n%s", log)
	}
	if !strings.Contains(log, "notes/789") {
		t.Errorf("expected note ID 789 in DELETE path, got log:\n%s", log)
	}
}
