package kubeconform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ansibleInventory mirrors the shape of an Ansible inventory (a top-level
// group name mapping to vars:/hosts:), which is valid YAML but not a
// Kubernetes manifest - no root apiVersion/kind.
const ansibleInventory = `webservers:
  vars:
    http_port: 80
    max_clients: 200
  hosts:
    web-0:
      ansible_host: web-0.example.com
      node_labels:
        topology.kubernetes.io/zone: zone-a
`

// nmstateConfig mirrors an NMState network config (top-level hostname:/
// dns-resolver:/routes:/interfaces:), also valid YAML but not a manifest.
const nmstateConfig = `hostname:
  config: node-0.example.com
dns-resolver:
  config:
    server:
      - 192.0.2.10
interfaces:
  - name: bond0
    type: vlan
    state: up
`

func TestIsManifestYAML(t *testing.T) {
	cases := []struct {
		name string
		data string
		want bool
	}{
		{"deployment", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n", true},
		{"kind-only", "kind: ConfigMap\ndata:\n  a: b\n", true},
		{"apiversion-only", "apiVersion: v1\nmetadata:\n  name: foo\n", true},
		{"kustomization", "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - deploy.yaml\n", true},
		{"ansible-inventory", ansibleInventory, false},
		{"nmstate-config", nmstateConfig, false},
		{"helm-values", "replicaCount: 2\nimage:\n  repository: nginx\n  tag: latest\n", false},
		// A nested `kind:` (buried inside a value, not at the document root)
		// must NOT make a non-manifest file look like a manifest.
		{"nested-kind-not-root", "servers:\n  web:\n    kind: frontend\n    apiVersion: v2\n", false},
		// Multi-document: a stream containing at least one manifest counts.
		{"multidoc-with-manifest", "foo: bar\n---\napiVersion: v1\nkind: Service\nmetadata:\n  name: s\n", true},
		{"multidoc-no-manifest", "foo: bar\n---\nbaz: qux\n", false},
		// Empty / comments-only / whitespace: not classified as non-manifest
		// (left to the validator's existing empty handling).
		{"empty", "", true},
		{"comments-only", "# just a comment\n", true},
		{"whitespace-only", "   \n\n", true},
		// A top-level YAML sequence is never a manifest document.
		{"root-sequence", "- a\n- b\n", false},
		// Malformed YAML biases toward validating (fail-safe) so the
		// validator / YAML-syntax check surfaces the problem.
		{"malformed", "foo: : : bar\n\t- broken\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsManifestYAML([]byte(c.data)); got != c.want {
				t.Errorf("IsManifestYAML(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

// TestValidateFiles_SkipsNonManifest verifies that flat, raw-validated YAML
// files with no root apiVersion/kind are recorded in SkippedNonManifest and
// never reach kubeconform (so no "missing 'kind' key" error), while a file
// that does carry a kind is still routed through validation.
func TestValidateFiles_SkipsNonManifest(t *testing.T) {
	d := t.TempDir()
	inv := filepath.Join(d, "inventory.yml")
	nms := filepath.Join(d, "node-0.example.com.yaml")
	// A real manifest whose kind is in DefaultOptions.SkipKinds, so it is
	// routed to kubeconform (proving it was NOT treated as non-manifest) but
	// resolves to Skipped without needing an on-disk schema in the test.
	es := filepath.Join(d, "secret.yaml")
	mustWriteFile(t, inv, ansibleInventory)
	mustWriteFile(t, nms, nmstateConfig)
	mustWriteFile(t, es, "apiVersion: external-secrets.io/v1beta1\nkind: ExternalSecret\nmetadata:\n  name: s\n")

	res, err := ValidateFiles([]string{inv, nms, es}, DefaultOptions())
	if err != nil {
		t.Fatalf("ValidateFiles: %v", err)
	}
	if res.Errors != 0 || res.Invalid != 0 {
		t.Errorf("expected no errors/invalid for non-manifest skips, got %d errors, %d invalid:\n%s", res.Errors, res.Invalid, res.Summary())
	}
	if len(res.SkippedNonManifest) != 2 {
		t.Fatalf("expected 2 skipped non-manifest files, got %d: %v", len(res.SkippedNonManifest), res.SkippedNonManifest)
	}
	got := strings.Join(res.SkippedNonManifest, ",")
	if !strings.Contains(got, "inventory.yml") || !strings.Contains(got, "node-0.example.com.yaml") {
		t.Errorf("skipped list missing expected files: %v", res.SkippedNonManifest)
	}
	if strings.Contains(got, "secret.yaml") {
		t.Errorf("a real manifest (ExternalSecret) must not be classified non-manifest: %v", res.SkippedNonManifest)
	}
	if res.Skipped != 1 {
		t.Errorf("expected the ExternalSecret to be SkipKinds-skipped by kubeconform (Skipped=1), got %d", res.Skipped)
	}
}

// TestValidateDir_SkipsNonManifest verifies the same gate applies to the
// standalone directory-walk entry point (kubeconform --dir).
func TestValidateDir_SkipsNonManifest(t *testing.T) {
	d := t.TempDir()
	mustWriteFile(t, filepath.Join(d, "inventory.yml"), ansibleInventory)

	// Set SchemaDir so ValidateDir skips embedded-schema extraction (which
	// needs the embedschemas build tag); the inventory is skipped before any
	// schema lookup, so the dir's contents are irrelevant.
	opts := DefaultOptions()
	opts.SchemaDir = t.TempDir()
	res, cleanup, err := ValidateDir(d, opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("ValidateDir: %v", err)
	}
	if res.Errors != 0 {
		t.Errorf("expected no errors, got %d:\n%s", res.Errors, res.Summary())
	}
	if len(res.SkippedNonManifest) != 1 {
		t.Errorf("expected 1 skipped non-manifest file, got %d: %v", len(res.SkippedNonManifest), res.SkippedNonManifest)
	}
	if !strings.Contains(res.Summary(), "non-manifest YAML file(s)") {
		t.Errorf("Summary should mention skipped non-manifest files, got:\n%s", res.Summary())
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
