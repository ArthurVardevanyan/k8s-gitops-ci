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
