package validator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/logger"
)

func TestRunNADValidation_NoOutputsOmitsSection(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	_, present := runNADValidation(nil, false, log)
	if present {
		t.Error("expected no section (present=false) with no rendered overlays to validate")
	}
}

func TestRunNADValidation_NoNADResourcesOmitsSection(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n")},
	}
	_, present := runNADValidation(outputs, false, log)
	if present {
		t.Error("expected no section (present=false) for a batch with no NAD resources")
	}
}

func TestRunNADValidation_ValidNADShowsPassingSection(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: good\nspec:\n  config: '{\"cniVersion\":\"0.3.1\"}'\n")},
	}
	s, present := runNADValidation(outputs, false, log)
	if !present {
		t.Fatal("expected a section (present=true) when a NAD is in the chain, even when it passes")
	}
	if s.Name != "NetworkAttachmentDefinition Validation" {
		t.Errorf("Name = %q, want %q", s.Name, "NetworkAttachmentDefinition Validation")
	}
	if s.Status == StatusError {
		t.Errorf("expected a passing (non-error) section for a valid NAD, got: %+v", s)
	}
}

func TestRunNADValidation_StructuralFindingRemapsToOverlay(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: bad\nspec:\n  config: ''\n")},
	}
	s, present := runNADValidation(outputs, false, log)
	if !present {
		t.Fatal("expected a section (present=true) when a NAD is in the chain")
	}
	if s.Status != StatusError {
		t.Fatalf("expected an error section for an empty spec.config, got: %+v", s)
	}
	if !strings.Contains(s.Body, "myapp/overlays/prod") {
		t.Errorf("expected the finding remapped back to the overlay path, got body: %s", s.Body)
	}
}

func TestRunNADValidation_OVNTierAppliedWhenAssumeOpenShift(t *testing.T) {
	t.Parallel()
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

	if s, present := runNADValidation(outputs, false, log); !present || s.Status == StatusError {
		t.Errorf("expected structural tier to surface a passing section, got present=%v s=%+v", present, s)
	}
	if s, present := runNADValidation(outputs, true, log); !present || s.Status != StatusError {
		t.Errorf("expected OVN-aware tier to catch the persistent-IPs-on-layer3 violation, got present=%v s=%+v", present, s)
	}
}

// TestRunAll_NADSectionOmittedWhenNoNAD guards the omit-when-absent behavior:
// an overlay chain with no NetworkAttachmentDefinition resource produces no
// NAD section at all (rather than an empty "0 NADs, all good" stub). NAD
// validation is still always-on - it just has nothing to report here.
func TestRunAll_NADSectionOmittedWhenNoNAD(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources: []\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	for _, s := range res.Sections {
		if s.Name == "NetworkAttachmentDefinition Validation" {
			t.Error("expected no NetworkAttachmentDefinition Validation section when no NAD is in the chain")
		}
	}
}

// TestRunAll_NADSectionPresentWhenNADInChain is the companion to the
// omit-when-absent test: once a NAD resource is actually rendered by an
// overlay, the section shows up (here as a passing section, since the NAD is
// structurally valid) - "shown when present, good or bad".
func TestRunAll_NADSectionPresentWhenNADInChain(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "nad.yaml"), "apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: good\nspec:\n  config: '{\"cniVersion\":\"0.3.1\"}'\n")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources:\n  - nad.yaml\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	found := false
	for _, s := range res.Sections {
		if s.Name == "NetworkAttachmentDefinition Validation" {
			found = true
			if s.Status == StatusError {
				t.Errorf("expected a passing NAD section for a valid NAD, got: %+v", s)
			}
		}
	}
	if !found {
		t.Error("expected a NetworkAttachmentDefinition Validation section when a NAD is in the chain")
	}
}
