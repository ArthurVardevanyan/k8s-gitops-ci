package main

import "testing"

func TestResolveNoComment_DefaultOff(t *testing.T) {
	if !resolveNoComment(false, false) {
		t.Error("expected comments off by default (neither flag passed)")
	}
}

func TestResolveNoComment_CommentOptsIn(t *testing.T) {
	if resolveNoComment(true, false) {
		t.Error("expected --comment to opt in to posting")
	}
}

func TestResolveNoComment_NoCommentAlwaysWins(t *testing.T) {
	if !resolveNoComment(true, true) {
		t.Error("expected --no-comment to override --comment")
	}
	if !resolveNoComment(false, true) {
		t.Error("expected --no-comment to force comments off on its own")
	}
}

func TestStringSliceFlag_AccumulatesInOrder(t *testing.T) {
	var apps []string
	f := newStringSliceFlag(&apps)
	if err := f.Set("a"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := f.Set("b"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(apps) != 2 || apps[0] != "a" || apps[1] != "b" {
		t.Fatalf("expected [a b], got %v", apps)
	}
	if got := f.String(); got != "a,b" {
		t.Errorf("String() = %q, want %q", got, "a,b")
	}
}

func TestStringSliceFlag_EmptyString(t *testing.T) {
	var clusters []string
	f := newStringSliceFlag(&clusters)
	if got := f.String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
}

func TestSplitCommaList(t *testing.T) {
	cases := map[string][]string{
		"":                nil,
		"   ":             nil,
		"a":               {"a"},
		"a,b":             {"a", "b"},
		"a, b ,, c":       {"a", "b", "c"},
		"kubernetes/,ci/": {"kubernetes/", "ci/"},
	}
	for in, want := range cases {
		got := splitCommaList(in)
		if len(got) != len(want) {
			t.Fatalf("splitCommaList(%q) = %v, want %v", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("splitCommaList(%q) = %v, want %v", in, got, want)
			}
		}
	}
}
