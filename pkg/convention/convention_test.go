package convention

import (
	"strings"
	"testing"
)

func TestPrefixes(t *testing.T) {
	if got := ScaffoldConfigsPrefix(); got != ".scafctl/configs/" {
		t.Errorf("ScaffoldConfigsPrefix() = %q", got)
	}
	if got := ScaffoldTemplatesPrefix(); got != ".scafctl/templates/" {
		t.Errorf("ScaffoldTemplatesPrefix() = %q", got)
	}
}

func TestPrefixesRecompute(t *testing.T) {
	old := ScaffoldDir
	ScaffoldDir = "custom"
	t.Cleanup(func() { ScaffoldDir = old })

	if got := ScaffoldConfigsPrefix(); got != "custom/configs/" {
		t.Errorf("after override ScaffoldConfigsPrefix() = %q", got)
	}
	if got := ScaffoldTemplatesPrefix(); got != "custom/templates/" {
		t.Errorf("after override ScaffoldTemplatesPrefix() = %q", got)
	}
	if !strings.HasSuffix(ScaffoldTemplatesPrefix(), "/") {
		t.Error("prefix should end with /")
	}
}

func TestScaffoldArtifactClassification(t *testing.T) {
	// Default ScaffoldDir (.scafctl) and an org override (.custom-scaffold)
	// must both classify correctly - the generic exclusion must cover
	// whatever ScaffoldDir an org sets.
	for _, dir := range []string{".scafctl", ".custom-scaffold"} {
		old := ScaffoldDir
		ScaffoldDir = dir
		t.Run(dir, func(t *testing.T) {
			cfg := dir + "/configs/myapp.yaml"
			tpl := dir + "/templates/myapp/overlays/kustomization.yaml"
			manifest := "myapp/overlays/dev/kustomization.yaml"

			if !IsScaffoldConfig(cfg) || !IsScaffoldArtifact(cfg) {
				t.Errorf("%q should be a scaffold config artifact", cfg)
			}
			if IsScaffoldTemplate(cfg) {
				t.Errorf("%q is a config, not a template", cfg)
			}
			if !IsScaffoldTemplate(tpl) || !IsScaffoldArtifact(tpl) {
				t.Errorf("%q should be a scaffold template artifact", tpl)
			}
			if IsScaffoldArtifact(manifest) {
				t.Errorf("%q is a real manifest, not a scaffold artifact", manifest)
			}
		})
		ScaffoldDir = old
	}
}
