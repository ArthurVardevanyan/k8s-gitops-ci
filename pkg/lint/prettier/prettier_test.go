package prettier

import "testing"

func TestFilter(t *testing.T) {
	in := []string{"a.yaml", "b.go", "c.json", "d.md"}
	out := Filter(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 prettier files, got %d", len(out))
	}
}
