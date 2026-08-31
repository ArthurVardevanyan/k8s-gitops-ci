package k8scni

import (
	"strings"
	"testing"
)

func runOVNCheck(t *testing.T, yamlDoc string) []string {
	t.Helper()
	findings := newOVNNetConfInvalidCheck().Run([]byte(yamlDoc), "test.yaml")
	msgs := make([]string, len(findings))
	for i, f := range findings {
		msgs[i] = f.Message
	}
	return msgs
}

func TestOVNNetConfInvalidCheck_Valid(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","subnets":"10.100.200.0/24","role":"secondary"}`
	if got := runOVNCheck(t, nadYAML("my-network", cfg)); len(got) != 0 {
		t.Errorf("expected a valid OVN NAD to pass, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_PersistentIPsLayer3(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","allowPersistentIPs":true,"subnets":"10.100.200.0/24","role":"secondary"}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 || !strings.Contains(got[0], "layer3 topology does not allow persistent IPs") {
		t.Fatalf("expected the layer3+persistentIPs rejection, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_PersistentIPsRequiresSubnets(t *testing.T) {
	// Current upstream ValidateNetConf also requires the subnets attribute
	// whenever allowPersistentIPs is set (not just a non-layer3 topology) -
	// a real divergence from what the superseded pkg/validator/nad package
	// ported, caught by re-reading upstream's current source for this move.
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","allowPersistentIPs":true,"role":"secondary"}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 || !strings.Contains(got[0], "subnets attribute must be set") {
		t.Fatalf("expected the persistentIPs-requires-subnets rejection, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_InfrastructureSubnetsNonLayer2(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","infrastructureSubnets":"10.1.130.0/30","role":"secondary"}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 || !strings.Contains(got[0], "infrastructureSubnets is only supported for layer2 topology") {
		t.Fatalf("expected the infrastructureSubnets rejection, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_NameInconsistent(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"wrong/name","subnets":"10.100.200.0/24","role":"secondary"}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for inconsistent name, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_DHCPIPAMAllowedOnLocalnet(t *testing.T) {
	// Current upstream ValidateNetConf allows ipam.type "dhcp" specifically
	// on localnet topology (as long as subnets is unset) - our prior
	// hand-rolled port predated this and rejected any non-empty ipam.type
	// outright, which would have been a false positive here.
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"localnet","netAttachDefName":"myns/my-network","ipam":{"type":"dhcp"}}`
	if got := runOVNCheck(t, nadYAML("my-network", cfg)); len(got) != 0 {
		t.Errorf("expected dhcp ipam on localnet to be accepted, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_DHCPIPAMRejectedOffLocalnet(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","subnets":"10.100.200.0/24","role":"secondary","ipam":{"type":"dhcp"}}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 || !strings.Contains(got[0], "only supported with localnet topology") {
		t.Fatalf("expected dhcp ipam off localnet to be rejected, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_UnsupportedIPAMKey(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","subnets":"10.100.200.0/24","role":"secondary","ipam":{"type":"whereabouts"}}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 || !strings.Contains(got[0], "unsupported ipam key") {
		t.Fatalf("expected the unsupported-ipam-key rejection, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_NonOVNTypeSkipped(t *testing.T) {
	// A non-OVN NAD is not this check's concern at all - ovn-kubernetes's
	// own ParseNetConf treats it as a no-op skip (ErrorAttachDefNotOvnManaged),
	// never a failure. macvlan-specific fields (master/ipam) must never be
	// judged against OVN's semantic rules.
	cfg := `{"cniVersion":"0.3.1","type":"macvlan","master":"ens1","ipam":{"type":"whereabouts","range":"10.0.0.0/24"}}`
	if got := runOVNCheck(t, nadYAML("my-network", cfg)); len(got) != 0 {
		t.Errorf("expected a non-OVN NAD to be skipped entirely, got %v", got)
	}
}

// Regression for the reported false positive this package's predecessor
// (pkg/validator/nad) was rewritten to fix: ODF/OCS's openshift-storage/
// ocs-storagecluster Multus NAD uses macvlan, not OVN, and must never be
// judged by this check.
func TestOVNNetConfInvalidCheck_ODFStorageClusterMacvlanSkipped(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","type":"macvlan","master":"ens1f0","mode":"bridge","ipam":{"type":"whereabouts","range":"192.168.1.0/24"}}`
	if got := runOVNCheck(t, nadYAML("ocs-storagecluster", cfg)); len(got) != 0 {
		t.Errorf("expected the ODF macvlan NAD to be skipped, got %v", got)
	}
}

func TestOVNNetConfInvalidCheck_MalformedConfigSkipped(t *testing.T) {
	// A malformed spec.config is k8scni/net-attach-def/config-invalid's concern; this
	// check must not double-report it under a second rule ID.
	if got := runOVNCheck(t, nadYAML("my-network", `{not valid json`)); len(got) != 0 {
		t.Errorf("expected a malformed config to be left to k8scni/net-attach-def/config-invalid, got %v", got)
	}
}

// A config that the generic structural gate (k8scni/net-attach-def/config-invalid, backed
// by containernetworking/cni's own parser) accepts, but that
// ovn-kubernetes's own ParseNetConf rejects for an OVN-specific reason (here:
// declaring both a top-level single-plugin config and a "plugins" conflist,
// which ParseNetConf explicitly forbids), must be surfaced by this check -
// not silently skipped. Silently skipping it would mean a real, structurally-
// valid-looking OVN NAD that ovn-kubernetes itself would reject is never
// judged by anything.
func TestOVNNetConfInvalidCheck_OVNSpecificParseErrorSurfaced(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","subnets":"10.0.0.0/24","role":"secondary","plugins":[{"type":"ovn-k8s-cni-overlay"}]}`
	got := runOVNCheck(t, nadYAML("my-network", cfg))
	if len(got) != 1 || !strings.Contains(got[0], "cannot have both a plugin list and a single config") {
		t.Fatalf("expected the OVN-specific parse error to be surfaced, got %v", got)
	}
}

// A NAD with no metadata.namespace (e.g. one that relies on `kubectl apply
// -n <ns>` rather than a namespace field or a kustomize namespace
// transformer) must not be judged against a manufactured "/"+Name identity -
// netconf.NADName naming a real namespace/name pair would almost never equal
// that for a perfectly consistent NAD, which would be a false positive on
// every namespace-less NAD rather than a genuine inconsistency.
func TestOVNNetConfInvalidCheck_NoNamespaceSkipsNameConsistencyCheck(t *testing.T) {
	doc := "apiVersion: k8s.cni.cncf.io/v1\n" +
		"kind: NetworkAttachmentDefinition\n" +
		"metadata:\n" +
		"  name: my-network\n" +
		"spec:\n" +
		"  config: '{\"cniVersion\":\"0.3.1\",\"name\":\"mynet\",\"type\":\"ovn-k8s-cni-overlay\",\"topology\":\"layer2\",\"netAttachDefName\":\"default/my-network\",\"subnets\":\"10.0.0.0/24\",\"role\":\"secondary\"}'\n"
	if got := runOVNCheck(t, doc); len(got) != 0 {
		t.Errorf("expected the name-consistency check to be skipped for a namespace-less NAD, got %v", got)
	}
}
