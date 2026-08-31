package k8scni

import (
	"strings"
	"testing"
)

func runConfigCheck(t *testing.T, yamlDoc string) []string {
	t.Helper()
	findings := newConfigInvalidCheck().Run([]byte(yamlDoc), "test.yaml")
	msgs := make([]string, len(findings))
	for i, f := range findings {
		msgs[i] = f.Message
	}
	return msgs
}

func nadYAML(name, config string) string {
	return "apiVersion: k8s.cni.cncf.io/v1\n" +
		"kind: NetworkAttachmentDefinition\n" +
		"metadata:\n" +
		"  name: " + name + "\n" +
		"  namespace: myns\n" +
		"spec:\n" +
		"  config: '" + config + "'\n"
}

func TestConfigInvalidCheck_ValidConfigs(t *testing.T) {
	cfgs := []string{
		`{"cniVersion":"0.3.1","type":"ovn-k8s-cni-overlay"}`,
		`{"cniVersion":"0.3.1","type":"macvlan","master":"ens1"}`,
		`{"cniVersion":"0.3.1","type":"bridge","bridge":"br0"}`,
		`{"cniVersion":"0.3.1","name":"n","plugins":[{"type":"macvlan"},{"type":"tuning"}]}`,
	}
	for _, cfg := range cfgs {
		if got := runConfigCheck(t, nadYAML("my-network", cfg)); len(got) != 0 {
			t.Errorf("cfg=%s: expected no findings, got %v", cfg, got)
		}
	}
}

func TestConfigInvalidCheck_MissingConfig(t *testing.T) {
	doc := "apiVersion: k8s.cni.cncf.io/v1\n" +
		"kind: NetworkAttachmentDefinition\n" +
		"metadata:\n  name: my-network\n" +
		"spec:\n  plugins: []\n"
	got := runConfigCheck(t, doc)
	if len(got) != 1 || !strings.Contains(got[0], "must not be empty") {
		t.Fatalf("expected 1 empty-config finding, got %v", got)
	}
}

func TestConfigInvalidCheck_EmptyStringConfig(t *testing.T) {
	got := runConfigCheck(t, nadYAML("my-network", ""))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for empty string config, got %v", got)
	}
}

func TestConfigInvalidCheck_MalformedJSON(t *testing.T) {
	got := runConfigCheck(t, nadYAML("my-network", `{"cniVersion":"0.3.1","type":"macvlan"`))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for malformed JSON, got %v", got)
	}
}

func TestConfigInvalidCheck_MissingType(t *testing.T) {
	got := runConfigCheck(t, nadYAML("my-network", `{"cniVersion":"0.3.1"}`))
	if len(got) != 1 || !strings.Contains(got[0], "missing") {
		t.Fatalf("expected 1 missing-type finding, got %v", got)
	}
}

func TestConfigInvalidCheck_ConflistPluginMissingType(t *testing.T) {
	got := runConfigCheck(t, nadYAML("my-network", `{"cniVersion":"0.3.1","name":"n","plugins":[{"master":"ens1"}]}`))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for a conflist plugin missing type, got %v", got)
	}
}

func TestConfigInvalidCheck_ConflistMissingName(t *testing.T) {
	// containernetworking/cni's own NetworkConfFromBytes requires a
	// top-level "name" for a conflist - a real CNI-spec requirement our
	// prior homegrown probe never enforced.
	got := runConfigCheck(t, nadYAML("my-network", `{"cniVersion":"0.3.1","plugins":[{"type":"macvlan"}]}`))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for a conflist with no name, got %v", got)
	}
}

func TestConfigInvalidCheck_ObjectConfigOnNAD(t *testing.T) {
	doc := "apiVersion: k8s.cni.cncf.io/v1\n" +
		"kind: NetworkAttachmentDefinition\n" +
		"metadata:\n  name: my-network\n" +
		"spec:\n  config:\n    cniVersion: \"0.3.1\"\n    type: ovn-k8s-cni-overlay\n"
	got := runConfigCheck(t, doc)
	if len(got) != 1 || !strings.Contains(got[0], "must be a string") {
		t.Fatalf("expected 1 finding for object spec.config, got %v", got)
	}
}

func TestConfigInvalidCheck_KindsDeclaresNADOnly(t *testing.T) {
	// Run itself never re-checks kind - like every other runtime check, it
	// relies on the dispatcher's SkipDoc (driven by Kinds()) to guarantee it
	// is never handed a document of any other kind. Guard that contract
	// here rather than asserting Run's behavior on an input the real
	// pipeline never produces.
	got := newConfigInvalidCheck().Kinds()
	if len(got) != 1 || got[0] != "NetworkAttachmentDefinition" {
		t.Errorf("Kinds() = %v, want [NetworkAttachmentDefinition]", got)
	}
}
