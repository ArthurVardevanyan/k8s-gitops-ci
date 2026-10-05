package convention

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsKnownNonManifestFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"Taskfile.yml", true},
		{"repo/Taskfile.yml", true},
		{".golangci.yml", true},
		{".golangci.yaml", true},
		{".goreleaser.yaml", true},
		{".goreleaser.yml", true},
		{".pre-commit-config.yaml", true},
		{".gitlab-ci.yml", true},
		{".gitlab-ci.yaml", true},
		{"kubernetes/tekton/overlays/operator/deployment.yaml", false},

		{"Taskfile.yml.bak", false},
	}
	for _, c := range cases {
		if got := IsKnownNonManifestFile(c.path); got != c.want {
			t.Errorf("IsKnownNonManifestFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsHelmChartTemplate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x: 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("app/base/Chart.yaml")
	chartTpl := write("app/base/templates/deploy.yaml")
	nested := write("app/base/templates/sub/dir/cm.yaml")
	chartValues := write("app/base/values.yaml")
	noChart := write("plain/templates/deploy.yaml")
	chartNoTemplates := write("other/Chart.yaml")
	chartOtherDir := write("app/base/files/deploy.yaml")
	// A `templates` dir further up that is not a chart's templates dir.
	deepNoChart := write("plain/templates/x/overlays/k.yaml")

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"chart template", chartTpl, true},
		{"nested chart template", nested, true},
		{"chart values file", chartValues, false},
		{"templates dir without Chart.yaml", noChart, false},
		{"Chart.yaml itself", chartNoTemplates, false},
		{"other dir inside chart", chartOtherDir, false},
		{"deep path under templates without Chart.yaml", deepNoChart, false},
	}
	for _, c := range cases {
		if got := IsHelmChartTemplate(c.path); got != c.want {
			t.Errorf("%s: IsHelmChartTemplate(%q) = %v, want %v", c.name, c.path, got, c.want)
		}
	}
}

func TestIsHelmChartTemplateRelative(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "chart", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "chart", "Chart.yaml"), []byte("name: c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if !IsHelmChartTemplate("chart/templates/a.yaml") {
		t.Error("relative chart template not detected")
	}
	t.Chdir(filepath.Join(root, "chart"))
	if !IsHelmChartTemplate("templates/a.yaml") {
		t.Error("chart-root-relative template not detected")
	}
}
