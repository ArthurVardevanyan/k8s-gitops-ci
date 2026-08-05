package nad

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFiles_ValidNAD(t *testing.T) {
	tmp := t.TempDir()
	nadPath := filepath.Join(tmp, "nad.yaml")
	_ = os.WriteFile(nadPath, []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
spec:
  config: '{"cniVersion":"0.3.1","type":"ovn-k8s-cni-overlay"}'
`), 0o600)

	errs, warns := ValidateFiles([]string{nadPath})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid NAD, got %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings for valid NAD, got %v", warns)
	}
}

func TestValidateFiles_MissingConfig(t *testing.T) {
	tmp := t.TempDir()
	nadPath := filepath.Join(tmp, "nad.yaml")
	_ = os.WriteFile(nadPath, []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
spec:
  plugins: []
`), 0o600)

	errs, _ := ValidateFiles([]string{nadPath})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing config, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "spec.config is empty") {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}

func TestValidateFiles_EmptyConfig(t *testing.T) {
	tmp := t.TempDir()
	nadPath := filepath.Join(tmp, "nad.yaml")
	_ = os.WriteFile(nadPath, []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
spec:
  config:
`), 0o600)

	errs, _ := ValidateFiles([]string{nadPath})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for empty config, got %d: %v", len(errs), errs)
	}
	if errs[0].File != nadPath {
		t.Errorf("expected file %q, got %q", nadPath, errs[0].File)
	}
}

func TestValidateFiles_EmptyStringConfig(t *testing.T) {
	tmp := t.TempDir()
	nadPath := filepath.Join(tmp, "nad.yaml")
	_ = os.WriteFile(nadPath, []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
spec:
  config: ''
`), 0o600)

	errs, _ := ValidateFiles([]string{nadPath})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for empty string config, got %d: %v", len(errs), errs)
	}
}

func TestValidateFiles_NonNADFilesSkipped(t *testing.T) {
	tmp := t.TempDir()
	// Regular ConfigMap - should be skipped
	cm := filepath.Join(tmp, "configmap.yaml")
	_ = os.WriteFile(cm, []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`), 0o600)

	// Non-YAML file
	txt := filepath.Join(tmp, "notes.txt")
	_ = os.WriteFile(txt, []byte("just notes"), 0o600)

	errs, warns := ValidateFiles([]string{cm, txt})
	if len(errs) != 0 || len(warns) != 0 {
		t.Errorf("expected no findings for non-NAD files, got errs=%v warns=%v", errs, warns)
	}
}

func TestValidateFiles_NonexistentFile(t *testing.T) {
	errs, _ := ValidateFiles([]string{"/nonexistent/nad.yaml"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for nonexistent file (skipped), got %v", errs)
	}
}

func TestValidateFiles_EmptyList(t *testing.T) {
	errs, _ := ValidateFiles(nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil input, got %v", errs)
	}
}

func TestValidateDir(t *testing.T) {
	tmp := t.TempDir()

	// Valid NAD (bridge). No cniVersion -> a non-gating advisory, not an error.
	_ = os.WriteFile(filepath.Join(tmp, "valid-nad.yaml"), []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: good
spec:
  config: '{"type":"bridge"}'
`), 0o600)

	// Invalid NAD (empty config) -> hard error.
	_ = os.WriteFile(filepath.Join(tmp, "bad-nad.yaml"), []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: bad
spec:
  config: ""
`), 0o600)

	// Non-NAD file (should be ignored)
	_ = os.WriteFile(filepath.Join(tmp, "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
`), 0o600)

	errs, _ := ValidateDir(tmp)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (bad NAD), got %d: %v", len(errs), errs)
	}
}

func TestValidateDir_Subdirectories(t *testing.T) {
	tmp := t.TempDir()
	subDir := filepath.Join(tmp, "cluster-a")
	_ = os.MkdirAll(subDir, 0o755)

	_ = os.WriteFile(filepath.Join(subDir, "nad.yml"), []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: deep-nad
spec:
  config: '{"cniVersion":"0.3.1","type":"macvlan"}'
`), 0o600)

	errs, warns := ValidateDir(tmp)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid deep NAD, got %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("expected no advisories for a macvlan NAD with cniVersion, got %v", warns)
	}
}

func TestValidateDir_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	errs, _ := ValidateDir(tmp)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty dir, got %v", errs)
	}
}

func TestIsNADFile(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		expected bool
	}{
		{
			name:     "valid NAD yaml",
			filename: "nad.yaml",
			content:  "kind: NetworkAttachmentDefinition\nmetadata:\n  name: test",
			expected: true,
		},
		{
			name:     "valid NAD yml",
			filename: "nad.yml",
			content:  "kind: NetworkAttachmentDefinition\nmetadata:\n  name: test",
			expected: true,
		},
		{
			name:     "non-NAD yaml",
			filename: "deployment.yaml",
			content:  "kind: Deployment\nmetadata:\n  name: test",
			expected: false,
		},
		{
			name:     "no space after colon",
			filename: "nad.yaml",
			content:  "kind:NetworkAttachmentDefinition\nmetadata:\n  name: test",
			expected: true,
		},
		{
			name:     "extra whitespace and double-quoted value",
			filename: "nad.yaml",
			content:  "kind:   \"NetworkAttachmentDefinition\"  \nmetadata:\n  name: test",
			expected: true,
		},
		{
			name:     "single-quoted value",
			filename: "nad.yaml",
			content:  "kind: 'NetworkAttachmentDefinition'\nmetadata:\n  name: test",
			expected: true,
		},
		{
			name:     "substring in another value is not matched",
			filename: "cm.yaml",
			content:  "kind: ConfigMap\ndata:\n  note: kind NetworkAttachmentDefinition example",
			expected: false,
		},
		{
			name:     "non-yaml file",
			filename: "notes.txt",
			content:  "kind: NetworkAttachmentDefinition",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(tmp, tc.filename)
			_ = os.WriteFile(path, []byte(tc.content), 0o600)
			got := IsNADFile(path)
			if got != tc.expected {
				t.Errorf("IsNADFile(%q) = %v, want %v", tc.filename, got, tc.expected)
			}
		})
	}
}

// writeNAD writes a single-NAD file (namespace myns, name my-network) whose
// spec.config is the given stringified netconf. The config must not contain a
// single quote (it is wrapped in single quotes in YAML).
func writeNAD(t *testing.T, config string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nad.yaml")
	content := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
  namespace: myns
spec:
  config: '` + config + `'
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- JSON / structural gating (all CNI types) ---

func TestValidateFiles_MalformedJSONIsError(t *testing.T) {
	path := writeNAD(t, `{"cniVersion":"0.3.1","type":"macvlan"`) // missing closing brace
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for malformed JSON, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "not valid JSON") {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}

func TestValidateFiles_MissingTypeIsError(t *testing.T) {
	path := writeNAD(t, `{"cniVersion":"0.3.1"}`)
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for a config with no CNI type, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, `missing a CNI "type"`) {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}

func TestValidateFiles_ConflistPluginMissingTypeIsError(t *testing.T) {
	path := writeNAD(t, `{"cniVersion":"0.3.1","name":"n","plugins":[{"master":"ens1"}]}`)
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for a conflist plugin with no type, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "plugins[0] is missing a CNI") {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}

// --- Non-OVN dispatch (advisory-only) ---

func TestValidateFiles_NonOVNKnownTypesPass(t *testing.T) {
	cfgs := []string{
		`{"cniVersion":"0.3.1","type":"macvlan","master":"ens1"}`,
		`{"cniVersion":"0.3.1","type":"bridge","bridge":"br0"}`,
		`{"cniVersion":"0.3.1","type":"ipvlan","master":"ens1"}`,
		`{"cniVersion":"0.3.1","type":"host-device","device":"ens1"}`,
		`{"cniVersion":"0.3.1","type":"sriov"}`,
	}
	for _, cfg := range cfgs {
		path := writeNAD(t, cfg)
		errs, warns := ValidateFiles([]string{path})
		if len(errs) != 0 {
			t.Errorf("known non-OVN type must not error: cfg=%s errs=%v", cfg, errs)
		}
		if len(warns) != 0 {
			t.Errorf("known non-OVN type with cniVersion should have no advisories: cfg=%s warns=%v", cfg, warns)
		}
	}
}

// A non-OVN NAD must NOT be subjected to OVN semantic rules. macvlan-specific
// fields (master/mode/ipam) are the plugin's concern and must pass.
func TestValidateFiles_NonOVNSkipsOVNRules(t *testing.T) {
	path := writeNAD(t, `{"cniVersion":"0.3.1","type":"macvlan","master":"ens1","mode":"bridge","ipam":{"type":"whereabouts","range":"10.0.0.0/24"}}`)
	errs, warns := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Errorf("a non-OVN NAD must not be subject to OVN rules, got: %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("a well-formed macvlan NAD should have no advisories, got: %v", warns)
	}
}

func TestValidateFiles_ConflistDispatchesOnFirstPlugin(t *testing.T) {
	// First plugin (macvlan) is the dispatch type; a chained tuning meta-plugin
	// is left alone (only that it declares a type is checked).
	path := writeNAD(t, `{"cniVersion":"0.3.1","name":"mynet","plugins":[{"type":"macvlan","master":"ens1"},{"type":"tuning"}]}`)
	errs, warns := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Errorf("a valid conflist must not error, got: %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("a valid conflist should have no advisories, got: %v", warns)
	}
}

func TestValidateFiles_UnknownTypeWarns(t *testing.T) {
	path := writeNAD(t, `{"cniVersion":"0.3.1","type":"mcvlan"}`)
	errs, warns := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Fatalf("an unrecognized CNI type must not gate, got errs: %v", errs)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "unrecognized CNI type") {
		t.Fatalf("expected 1 unrecognized-type warning, got: %v", warns)
	}
	if !warns[0].Warning {
		t.Error("finding must be marked as a Warning")
	}
}

func TestValidateFiles_UnknownIPAMWarns(t *testing.T) {
	path := writeNAD(t, `{"cniVersion":"0.3.1","type":"macvlan","ipam":{"type":"whereabout"}}`)
	errs, warns := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Fatalf("an unrecognized IPAM type must not gate, got: %v", errs)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "unrecognized IPAM type") {
		t.Fatalf("expected 1 unrecognized-ipam warning, got: %v", warns)
	}
}

func TestValidateFiles_MissingCNIVersionWarns(t *testing.T) {
	path := writeNAD(t, `{"type":"macvlan"}`)
	errs, warns := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Fatalf("a missing cniVersion must not gate, got: %v", errs)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "cniVersion") {
		t.Fatalf("expected 1 missing-cniVersion warning, got: %v", warns)
	}
}

// Regression for the reported false positive: ODF/OCS's openshift-storage/
// ocs-storagecluster Multus NAD uses macvlan, not OVN, and must pass rather
// than fail with "net-attach-def not managed by OVN".
func TestValidateFiles_ODFStorageClusterMacvlanPasses(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nad.yaml")
	content := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ocs-storagecluster
  namespace: openshift-storage
spec:
  config: '{"cniVersion":"0.3.1","type":"macvlan","master":"ens1f0","mode":"bridge","ipam":{"type":"whereabouts","range":"192.168.1.0/24"}}'
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	errs, warns := ValidateFiles([]string{path})
	if len(errs) != 0 {
		t.Fatalf("ODF macvlan NAD must not be a hard error (regression for 'net-attach-def not managed by OVN'), got: %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("a well-formed macvlan NAD should have no advisories, got: %v", warns)
	}
}

// --- OVN-aware validation (type ovn-k8s-cni-overlay) ---

func TestValidateFiles_OVN_Valid(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","subnets":"10.100.200.0/24","role":"secondary"}`
	path := writeNAD(t, cfg)
	if errs, _ := ValidateFiles([]string{path}); len(errs) != 0 {
		t.Errorf("expected valid OVN NAD, got %v", errs)
	}
}

func TestValidateFiles_OVN_PersistentIPsLayer3(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","allowPersistentIPs":true,"role":"secondary"}`
	path := writeNAD(t, cfg)
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	// The message is scoped to the NAD (namespace/name) so it stays unambiguous
	// in a multi-NAD file.
	if !strings.Contains(errs[0].Message, "layer3 topology does not allow persistent IPs") {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
	if !strings.Contains(errs[0].Message, "myns/my-network") {
		t.Errorf("OVN error must be scoped to the NAD namespace/name, got: %s", errs[0].Message)
	}
}

func TestValidateFiles_OVN_InfrastructureSubnetsNonLayer2(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","infrastructureSubnets":"10.1.130.0/30","role":"secondary"}`
	path := writeNAD(t, cfg)
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "infrastructureSubnets is only supported for layer2 topology") {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
	if !strings.Contains(errs[0].Message, "myns/my-network") {
		t.Errorf("OVN error must be scoped to the NAD namespace/name, got: %s", errs[0].Message)
	}
}

func TestValidateFiles_OVN_MalformedJSON(t *testing.T) {
	path := writeNAD(t, `{not valid json`)
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for malformed config, got %d: %v", len(errs), errs)
	}
}

func TestValidateFiles_OVN_NameInconsistent(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"wrong/name","subnets":"10.100.200.0/24","role":"secondary"}`
	path := writeNAD(t, cfg)
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for inconsistent name, got %d: %v", len(errs), errs)
	}
}

// When a single rendered file carries multiple NAD documents (the common case
// for overlay output), an OVN semantic error must identify the specific failing
// NAD by namespace/name so it is not ambiguous.
func TestValidateFiles_OVN_ErrorScopedInMultiNADFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nads.yaml")
	content := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: good-net
  namespace: ns-a
spec:
  config: '{"cniVersion":"0.3.1","name":"good","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"ns-a/good-net","subnets":"10.0.0.0/24","role":"secondary"}'
---
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: bad-net
  namespace: ns-b
spec:
  config: '{"cniVersion":"0.3.1","name":"bad","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"ns-b/bad-net","allowPersistentIPs":true,"role":"secondary"}'
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (only bad-net), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "ns-b/bad-net") {
		t.Errorf("error must name the failing NAD (ns-b/bad-net), got: %s", errs[0].Message)
	}
	if strings.Contains(errs[0].Message, "ns-a/good-net") {
		t.Errorf("error must not reference the passing NAD, got: %s", errs[0].Message)
	}
}

// A rendered overlay file commonly contains resources other than NADs whose
// spec.config is an object (for example an OLM Subscription). Those documents
// must not break decoding — only the NAD document is validated.
func TestValidateFiles_IgnoresNonNADObjectConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "pp24.yaml")
	content := `apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: kubernetes-nmstate-operator
  namespace: openshift-nmstate
spec:
  channel: stable
  config:
    resources:
      limits:
        cpu: 500m
        memory: 4Gi
      requests:
        cpu: 100m
        memory: 2Gi
  name: kubernetes-nmstate-operator
---
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
  namespace: myns
spec:
  config: |-
    {
      "cniVersion": "0.3.1",
      "name": "mynet",
      "type": "ovn-k8s-cni-overlay",
      "topology": "layer2",
      "netAttachDefName": "myns/my-network",
      "subnets": "10.100.200.0/24",
      "role": "secondary"
    }
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if errs, _ := ValidateFiles([]string{path}); len(errs) != 0 {
		t.Errorf("expected no errors when a non-NAD object spec.config is present, got %v", errs)
	}
}

// A NAD whose spec.config is authored as an object (rather than a stringified
// netconf) is malformed and must be reported.
func TestValidateFiles_ObjectConfigOnNAD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nad.yaml")
	content := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
  namespace: myns
spec:
  config:
    cniVersion: "0.3.1"
    type: ovn-k8s-cni-overlay
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	errs, _ := ValidateFiles([]string{path})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for object spec.config on a NAD, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "spec.config must be a string") {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}
