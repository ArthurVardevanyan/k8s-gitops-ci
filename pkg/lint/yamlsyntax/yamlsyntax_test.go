package yamlsyntax

import "testing"

func TestFilter(t *testing.T) {
	in := []string{"a.yaml", "b.yml", "c.json"}
	out := Filter(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 yaml files, got %d", len(out))
	}
}

func TestCheckBytes_Valid(t *testing.T) {
	v := CheckBytes("x.yaml", []byte("hello: world\n"))
	if len(v) != 0 {
		t.Errorf("expected no violations: %+v", v)
	}
}

func TestCheckBytes_Invalid(t *testing.T) {
	v := CheckBytes("x.yaml", []byte("hello: ["))
	if len(v) != 1 {
		t.Errorf("expected one violation: %+v", v)
	}
}
