package github

import "testing"

func TestClient_Repo(t *testing.T) {
	c := NewClient("https://github.com/oakwood-commons/scafctl", "3")
	if c.Repo() != "oakwood-commons/scafctl" {
		t.Errorf("Repo() = %q", c.Repo())
	}
}

func TestClient_RepoEdgeCases(t *testing.T) {
	cases := map[string]string{
		"":                                "",
		"https://github.com/org/repo.git": "org/repo",
		"git@github.com:org/repo.git":     "", // unsupported
		"github.com/org/repo":             "org/repo",
	}
	for in, want := range cases {
		if got := NewClient(in, "1").Repo(); got != want {
			t.Errorf("NewClient(%q).Repo() = %q, want %q", in, got, want)
		}
	}
}

func TestClient_IsAvailable_MissingInfo(t *testing.T) {
	if NewDisabledClient().IsAvailable() {
		t.Error("disabled client should be unavailable")
	}
	if NewClient("", "1").IsAvailable() {
		t.Error("empty repo should be unavailable")
	}
}

func TestValidatePRTitle_Patterns(t *testing.T) {
	ok := []string{"feat: add thing", "fix(scope): bug", "docs: update"}
	bad := []string{"adding thing", "", "WIP: foo"}
	for _, s := range ok {
		if err := ValidatePRTitleString(s); err != nil {
			t.Errorf("%q should be valid: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := ValidatePRTitleString(s); err == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestValidatePRChecklist_Logic(t *testing.T) {
	body := `- [x] a
- [ ] b
- [x] c
- [ ] d
`
	spec := ChecklistSpec{
		Required:  []string{"a"},
		SelectOne: [][]string{{"b", "c"}},
	}
	if err := ValidatePRChecklistString(body, spec); err == nil {
		t.Error("expected error for unchecked required and one-of")
	}
}

func TestGetUnsignedCommits_NoGH(t *testing.T) {
	_, err := GetUnsignedCommits(NewDisabledClient())
	if err != nil {
		t.Error("disabled client should not error")
	}
}

func TestParseUnsignedCommits_AllVerified(t *testing.T) {
	data := `[
		{"sha": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "commit": {"message": "feat: a", "verification": {"verified": true}}},
		{"sha": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "commit": {"message": "fix: b", "verification": {"verified": true}}}
	]`
	got, err := parseUnsignedCommits([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no unsigned commits, got %v", got)
	}
}

func TestParseUnsignedCommits_MixedVerification(t *testing.T) {
	data := `[
		{"sha": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "commit": {"message": "feat: a", "verification": {"verified": true}}},
		{"sha": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "commit": {"message": "fix: b\n\nlonger body text", "verification": {"verified": false}}}
	]`
	got, err := parseUnsignedCommits([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 unsigned commit, got %v", got)
	}
	if got[0] != "bbbbbbb fix: b" {
		t.Errorf("got %q, want %q", got[0], "bbbbbbb fix: b")
	}
}

func TestParseUnsignedCommits_Empty(t *testing.T) {
	got, err := parseUnsignedCommits([]byte(`[]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no unsigned commits, got %v", got)
	}
}

func TestParseUnsignedCommits_MalformedJSON(t *testing.T) {
	if _, err := parseUnsignedCommits([]byte(`not json`)); err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseUnsignedCommits_UnverifiedTreatedAsUnsigned(t *testing.T) {
	// verification.verified defaults to false (Go zero value) when the
	// verification object is entirely absent - GitHub omits it for some
	// commit types. Absence must be treated as unsigned, not skipped.
	data := `[{"sha": "cccccccccccccccccccccccccccccccccccccccc", "commit": {"message": "chore: c"}}]`
	got, err := parseUnsignedCommits([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 unsigned commit, got %v", got)
	}
}
