package github

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

// installFakeGH writes an executable "gh" shim to a temp dir, prepends it to
// PATH for the duration of the test, and returns the path to a log file the
// shim appends one record to per invocation (args + stdin content). This
// lets tests assert exactly what gh was called with - including stdin,
// which is the thing the UpsertComment/postComment bugs this test guards
// against got wrong (body silently never reaching gh's stdin).
//
// If commentID is non-empty, the shim simulates an existing comment already
// matching the marker (the "list comments" lookup call returns commentID);
// otherwise it simulates no existing comment.
func installFakeGH(t *testing.T, commentID string) string {
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

# Simulate the "find existing comment by marker" list-comments lookup: the
# real call is: gh api repos/OWNER/REPO/issues/PR/comments --jq EXPR
case "$1 $2" in
  "api "*)
    case "$2" in
      */comments) printf '%%s\n' %q ;;
    esac
    ;;
esac
exit 0
`, logPath, commentID)
	shPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(shPath, []byte(script), 0o755); err != nil { //nolint:gosec // test-only executable shim
		t.Fatalf("writing fake gh: %v", err)
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
	logPath := installFakeGH(t, "") // no existing comment -> postComment path
	c := NewClient("https://github.com/example-org/example-repo", "42")

	const body = "## Report\n\nsome findings here"
	if err := UpsertComment(c, "<!-- marker -->", body); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "ARGS: pr comment 42 --body-file -") {
		t.Errorf("expected pr comment --body-file - call, got log:\n%s", log)
	}
	if !strings.Contains(log, "STDIN:"+body) {
		t.Errorf("expected body to reach stdin, got log:\n%s", log)
	}
}

func TestUpsertComment_ExistingComment_PatchesByID(t *testing.T) {
	logPath := installFakeGH(t, "999") // existing comment id 999
	c := NewClient("https://github.com/example-org/example-repo", "42")

	const body = "## Updated Report\n\nnew findings"
	if err := UpsertComment(c, "<!-- marker -->", body); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "issues/comments/999") || !strings.Contains(log, "PATCH") {
		t.Errorf("expected a PATCH to the resolved comment id, got log:\n%s", log)
	}
	if !strings.Contains(log, "body=@-") {
		t.Errorf("expected -F body=@- (stdin), not an inline body argument, got log:\n%s", log)
	}
	if !strings.Contains(log, "STDIN:"+body) {
		t.Errorf("expected body to reach stdin, got log:\n%s", log)
	}
	// The bug this test guards against: UpsertComment must never fall back
	// to gh pr comment --edit-last for an existing comment.
	if strings.Contains(log, "--edit-last") {
		t.Errorf("must not use --edit-last, got log:\n%s", log)
	}
}
