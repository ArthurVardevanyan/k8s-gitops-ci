package nad

import (
	"os"
	"path/filepath"
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
`), 0o644)

	errs := ValidateFiles([]string{nadPath}, false)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid NAD, got %v", errs)
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
`), 0o644)

	errs := ValidateFiles([]string{nadPath}, false)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing config, got %d: %v", len(errs), errs)
	}
	if errs[0].Message != "spec.config field not found in NetworkAttachmentDefinition" {
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
`), 0o644)

	errs := ValidateFiles([]string{nadPath}, false)
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
`), 0o644)

	errs := ValidateFiles([]string{nadPath}, false)
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
`), 0o644)

	// Non-YAML file
	txt := filepath.Join(tmp, "notes.txt")
	_ = os.WriteFile(txt, []byte("just notes"), 0o644)

	errs := ValidateFiles([]string{cm, txt}, false)
	if len(errs) != 0 {
		t.Errorf("expected no errors for non-NAD files, got %v", errs)
	}
}

func TestValidateFiles_NonexistentFile(t *testing.T) {
	errs := ValidateFiles([]string{"/nonexistent/nad.yaml"}, false)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nonexistent file (skipped), got %v", errs)
	}
}

func TestValidateFiles_EmptyList(t *testing.T) {
	errs := ValidateFiles(nil, false)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil input, got %v", errs)
	}
}

func TestValidateDir(t *testing.T) {
	tmp := t.TempDir()

	// Valid NAD
	_ = os.WriteFile(filepath.Join(tmp, "valid-nad.yaml"), []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: good
spec:
  config: '{"type":"bridge"}'
`), 0o644)

	// Invalid NAD (empty config)
	_ = os.WriteFile(filepath.Join(tmp, "bad-nad.yaml"), []byte(`apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: bad
spec:
  config: ""
`), 0o644)

	// Non-NAD file (should be ignored)
	_ = os.WriteFile(filepath.Join(tmp, "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
`), 0o644)

	errs := ValidateDir(tmp, false)
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
  config: '{"type":"macvlan"}'
`), 0o644)

	errs := ValidateDir(tmp, false)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid deep NAD, got %v", errs)
	}
}

func TestValidateDir_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	errs := ValidateDir(tmp, false)
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
			_ = os.WriteFile(path, []byte(tc.content), 0o644)
			got := IsNADFile(path)
			if got != tc.expected {
				t.Errorf("IsNADFile(%q) = %v, want %v", tc.filename, got, tc.expected)
			}
		})
	}
}

// --- OVN-aware validation (assumeOpenshift = true) ---

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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateFiles_OVN_Valid(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"myns/my-network","subnets":"10.100.200.0/24","role":"secondary"}`
	path := writeNAD(t, cfg)
	if errs := ValidateFiles([]string{path}, true); len(errs) != 0 {
		t.Errorf("expected valid OVN NAD, got %v", errs)
	}
}

func TestValidateFiles_OVN_PersistentIPsLayer3(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","allowPersistentIPs":true,"role":"secondary"}`
	path := writeNAD(t, cfg)
	errs := ValidateFiles([]string{path}, true)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Message != "layer3 topology does not allow persistent IPs" {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}

func TestValidateFiles_OVN_InfrastructureSubnetsNonLayer2(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","infrastructureSubnets":"10.1.130.0/30","role":"secondary"}`
	path := writeNAD(t, cfg)
	errs := ValidateFiles([]string{path}, true)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Message != "infrastructureSubnets is only supported for layer2 topology" {
		t.Errorf("unexpected message: %s", errs[0].Message)
	}
}

func TestValidateFiles_OVN_MalformedJSON(t *testing.T) {
	path := writeNAD(t, `{not valid json`)
	errs := ValidateFiles([]string{path}, true)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for malformed config, got %d: %v", len(errs), errs)
	}
}

func TestValidateFiles_OVN_NameInconsistent(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer2","netAttachDefName":"wrong/name","subnets":"10.100.200.0/24","role":"secondary"}`
	path := writeNAD(t, cfg)
	errs := ValidateFiles([]string{path}, true)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for inconsistent name, got %d: %v", len(errs), errs)
	}
}

// A config that is invalid for OVN must still pass the structural tier, which
// only checks that spec.config is present and non-empty.
func TestValidateFiles_StructuralIgnoresOVNRules(t *testing.T) {
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","allowPersistentIPs":true}`
	path := writeNAD(t, cfg)
	if errs := ValidateFiles([]string{path}, false); len(errs) != 0 {
		t.Errorf("structural tier should ignore OVN semantics, got %v", errs)
	}
}

// A rendered overlay file commonly contains resources other than NADs whose
// spec.config is an object (for example an OLM Subscription). Those documents
// must not break OVN-aware decoding — only the NAD document is validated.
func TestValidateFiles_OVN_IgnoresNonNADObjectConfig(t *testing.T) {
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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := ValidateFiles([]string{path}, true); len(errs) != 0 {
		t.Errorf("expected no errors when a non-NAD object spec.config is present, got %v", errs)
	}
}

// A NAD whose spec.config is authored as an object (rather than a stringified
// netconf) is malformed and must be reported.
func TestValidateFiles_OVN_ObjectConfigOnNAD(t *testing.T) {
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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	errs := ValidateFiles([]string{path}, true)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for object spec.config on a NAD, got %d: %v", len(errs), errs)
	}
}
