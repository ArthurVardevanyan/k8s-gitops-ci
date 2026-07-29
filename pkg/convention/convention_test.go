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
