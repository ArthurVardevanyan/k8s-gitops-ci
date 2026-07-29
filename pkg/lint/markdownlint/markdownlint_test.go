package markdownlint

import "testing"

func TestFilter(t *testing.T) {
	in := []string{"a.md", "b.yaml", "c.MD", "d.markdown"}
	out := Filter(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 md files, got %d", len(out))
	}
}
