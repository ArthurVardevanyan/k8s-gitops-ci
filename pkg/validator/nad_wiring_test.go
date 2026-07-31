package validator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

func TestRunNADValidation_NoOutputsPassesTrivially(t *testing.T) {
	log := logger.NewLogger(false, "")
	s := runNADValidation(nil, false, log)
	if s.Name != "NetworkAttachmentDefinition Validation" {
		t.Errorf("Name = %q, want %q", s.Name, "NetworkAttachmentDefinition Validation")
	}
	if s.Error {
		t.Error("expected no error with no rendered overlays to validate")
	}
}

func TestRunNADValidation_NoNADResourcesPasses(t *testing.T) {
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n")},
	}
	s := runNADValidation(outputs, false, log)
	if s.Error {
		t.Errorf("expected no error for a batch with no NAD resources, got: %+v", s)
	}
}

func TestRunNADValidation_StructuralFindingRemapsToOverlay(t *testing.T) {
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: bad\nspec:\n  config: ''\n")},
	}
	s := runNADValidation(outputs, false, log)
	if !s.Error {
		t.Fatalf("expected an error section for an empty spec.config, got: %+v", s)
	}
	if !strings.Contains(s.Body, "myapp/overlays/prod") {
		t.Errorf("expected the finding remapped back to the overlay path, got body: %s", s.Body)
	}
}

func TestRunNADValidation_OVNTierAppliedWhenAssumeOpenShift(t *testing.T) {
	log := logger.NewLogger(false, "")
	// Valid structurally (non-empty config) but invalid under OVN semantics:
	// layer3 topology does not allow persistent IPs.
	cfg := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
  namespace: myns
spec:
  config: '{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","allowPersistentIPs":true,"role":"secondary"}'
`
	outputs := []renderedOverlay{{overlay: "myapp/overlays/prod", data: []byte(cfg)}}

	if s := runNADValidation(outputs, false, log); s.Error {
		t.Errorf("expected structural tier to ignore OVN semantics, got: %+v", s)
	}
	if s := runNADValidation(outputs, true, log); !s.Error {
		t.Errorf("expected OVN-aware tier to catch the persistent-IPs-on-layer3 violation, got: %+v", s)
	}
}

func TestRunAll_NADSectionAlwaysPresent(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	found := false
	for _, s := range res.Sections {
		if s.Name == "NetworkAttachmentDefinition Validation" {
			found = true
		}
	}
	if !found {
		t.Error("expected a NetworkAttachmentDefinition Validation section unconditionally (NAD's structural tier is always on)")
	}
}
