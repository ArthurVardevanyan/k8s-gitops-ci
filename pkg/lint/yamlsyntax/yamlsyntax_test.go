package yamlsyntax

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestFilter_SkipsHelmChartTemplates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	chartTpl := filepath.Join(dir, "chart", "templates", "deploy.yaml")
	values := filepath.Join(dir, "chart", "values.yaml")
	for p, body := range map[string]string{
		filepath.Join(dir, "chart", "Chart.yaml"): "name: c\n",
		chartTpl: "{{ range .Values.items }}\nkind: X\n{{ end }}\n",
		values:   "a: 1\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := Filter([]string{chartTpl, values})
	if len(out) != 1 || out[0] != values {
		t.Fatalf("expected only the values file, got %v", out)
	}
	vs, err := CheckFiles([]string{chartTpl, values})
	if err != nil || len(vs) != 0 {
		t.Fatalf("chart template must not raise violations: %v %v", vs, err)
	}
}
