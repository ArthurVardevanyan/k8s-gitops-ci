package gitlab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertComment_Disabled(t *testing.T) {
	if err := UpsertComment(NewDisabledClient(), "m", "body"); err != nil {
		t.Errorf("disabled client should no-op: %v", err)
	}
}

func TestDeleteComments_Disabled(t *testing.T) {
	if err := DeleteComments(NewDisabledClient(), "m"); err != nil {
		t.Errorf("disabled client should no-op: %v", err)
	}
}

func installFakeGlab(t *testing.T, noteID, marker string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocations.log")

	listJSON := "[]\n"
	if noteID != "" {
		listJSON = fmt.Sprintf(`[{"id":%s,"body":"header %s footer","system":false}]`, noteID, marker)
	}

	script := fmt.Sprintf(`#!/bin/sh
{
  printf 'ARGS:'
  for a in "$@"; do printf ' %%s' "$a"; done
  printf '\n'
  printf 'STDIN:%%s\n' "$(cat)"
  echo '---'
} >> %q

case "$*" in
  *"notes"*)
    if ! echo "$*" | grep -q -- "-X "; then
      cat <<'EOF'
%s
EOF
    fi
    ;;
esac
exit 0
`, logPath, listJSON)

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

func TestUpsertComment_NewComment_PostsViaAPI(t *testing.T) {
	logPath := installFakeGlab(t, "", "<!-- marker -->")
	c := NewClient("https://gitlab.example.com/example-org/example-repo", "42")

	const body = "## Report\n\nsome findings here"
	if err := UpsertComment(c, "<!-- marker -->", body); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "api projects/example-org%2Fexample-repo/merge_requests/42/notes -X POST -F body=@-") {
		t.Errorf("expected POST API call with body=@-, got log:\n%s", log)
	}
	if !strings.Contains(log, "STDIN:"+body) {
		t.Errorf("expected body to reach stdin, got log:\n%s", log)
	}
}

func TestUpsertComment_ExistingComment_UpdatesByID(t *testing.T) {
	logPath := installFakeGlab(t, "888", "<!-- marker -->")
	c := NewClient("https://gitlab.example.com/example-org/example-repo", "42")

	const body = "## Updated Report\n\nnew findings"
	if err := UpsertComment(c, "<!-- marker -->", body); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "api projects/example-org%2Fexample-repo/merge_requests/42/notes/888 -X PUT -F body=@-") {
		t.Errorf("expected PUT API call with note ID and body=@-, got log:\n%s", log)
	}
	if !strings.Contains(log, "STDIN:"+body) {
		t.Errorf("expected body to reach stdin, got log:\n%s", log)
	}
}

func TestDeleteComments_DeletesMatchingNotes(t *testing.T) {
	logPath := installFakeGlab(t, "777", "<!-- marker -->")
	c := NewClient("https://gitlab.example.com/example-org/example-repo", "42")

	if err := DeleteComments(c, "<!-- marker -->"); err != nil {
		t.Fatalf("DeleteComments: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "api projects/example-org%2Fexample-repo/merge_requests/42/notes/777 -X DELETE") {
		t.Errorf("expected DELETE API call with note ID 777, got log:\n%s", log)
	}
}
