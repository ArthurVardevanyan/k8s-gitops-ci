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
	_, present := runNADValidation(nil, log)
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
	_, present := runNADValidation(outputs, log)
	if present {
		t.Error("expected no section (present=false) for a batch with no NAD resources")
	}
}

func TestRunNADValidation_ValidNADShowsPassingSection(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: good\nspec:\n  config: '{\"cniVersion\":\"0.3.1\",\"type\":\"macvlan\"}'\n")},
	}
	s, present := runNADValidation(outputs, log)
	if !present {
		t.Fatal("expected a section (present=true) when a NAD is in the chain, even when it passes")
	}
	if s.Name != "NetworkAttachmentDefinition Validation" {
		t.Errorf("Name = %q, want %q", s.Name, "NetworkAttachmentDefinition Validation")
	}
	if s.Status == StatusError {
		t.Errorf("expected a passing (non-error) section for a valid NAD, got: %+v", s)
	}
	if log.HasFailures() {
		t.Error("a valid NAD must not record a failure")
	}
}

func TestRunNADValidation_AdvisoryRemapsToOverlayAndDoesNotGate(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: bad\nspec:\n  config: '{\"cniVersion\":\"0.3.1\",\"type\":\"not-a-real-cni-plugin\"}'\n")},
	}
	s, present := runNADValidation(outputs, log)
	if !present {
		t.Fatal("expected a section (present=true) when a NAD is in the chain")
	}

	// Findings are produced against a temp file written per rendered overlay,
	// so the path a reader sees has to be mapped back to the overlay it came
	// from. That remapping is what this test is for, and it has to keep
	// working now that the only findings this section produces are advisories.
	if !strings.Contains(s.Body, "myapp/overlays/prod") {
		t.Errorf("expected the finding remapped back to the overlay path, got body: %s", s.Body)
	}
	if !strings.Contains(s.Body, "unrecognized CNI type") {
		t.Errorf("expected the advisory in the section body, got: %s", s.Body)
	}

	// This section no longer gates. An empty spec.config used to fail the run
	// from here; it is now reported by the config-invalid runtime check, which
	// gates through the normal doc-check dispatch instead.
	if log.HasFailures() {
		t.Error("an advisory must not fail the run")
	}
}

// An OVN semantic violation (a structurally valid config that nonetheless
// violates OVN-Kubernetes' own rules, e.g. persistent IPs on a layer3
// topology) is no longer this package's concern: it now lives in
// pkg/validator/runtime/k8scni's "k8scni/net-attach-def/ovn-netconf-invalid" check, part of
// the Runtime Validation family. runNADValidation's structural gate has
// nothing to say about it, so the NAD section must pass and the run must not
// be gated through this path (the runtime check gates it separately, via the
// normal doc-check dispatch - see runtime_wiring_test.go).
func TestRunNADValidation_OVNSemanticsNoLongerCheckedHere(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	cfg := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: my-network
  namespace: myns
spec:
  config: '{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"myns/my-network","allowPersistentIPs":true,"role":"secondary"}'
`
	outputs := []renderedOverlay{{overlay: "myapp/overlays/prod", data: []byte(cfg)}}
	s, present := runNADValidation(outputs, log)
	if !present || s.Status != StatusPassed {
		t.Errorf("expected the structural gate to pass an OVN semantic violation (that's k8scni/net-attach-def/ovn-netconf-invalid's concern now), got present=%v s=%+v", present, s)
	}
	if log.HasFailures() {
		t.Error("the structural gate must not itself gate on an OVN semantic violation")
	}
}

// A non-OVN NAD with an unrecognized CNI type is advisory-only: the section
// rolls up to ⚠️ (StatusWarning) but the run must NOT gate.
func TestRunNADValidation_UnknownTypeWarnsButDoesNotGate(t *testing.T) {
	t.Parallel()
	log := logger.NewLogger(false, "")
	outputs := []renderedOverlay{
		{overlay: "myapp/overlays/prod", data: []byte("apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: typo\nspec:\n  config: '{\"cniVersion\":\"0.3.1\",\"type\":\"mcvlan\"}'\n")},
	}
	s, present := runNADValidation(outputs, log)
	if !present {
		t.Fatal("expected a section for a NAD with an unrecognized type")
	}
	if s.Status != StatusWarning {
		t.Errorf("expected StatusWarning for an unrecognized CNI type, got %v:\n%s", s.Status, s.Body)
	}
	if log.HasFailures() {
		t.Error("an advisory warning must not gate the run")
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
	mustWrite(t, filepath.Join(app, "overlays", "prod", "nad.yaml"), "apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: good\nspec:\n  config: '{\"cniVersion\":\"0.3.1\",\"type\":\"macvlan\"}'\n")
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

// TestRunAll_OVNSemanticViolationGatesViaRuntimeValidation is the end-to-end
// proof that moving OVN's semantic rules into pkg/validator/runtime/k8scni
// didn't just relocate the code but actually wires it into the real
// pipeline: a NAD with an OVN semantic violation renders its
// NetworkAttachmentDefinition Validation section as passing (the structural
// gate has nothing to say about it - see
// TestRunNADValidation_OVNSemanticsNoLongerCheckedHere) while still gating
// the run, via a Runtime Validation section finding under
// k8scni/net-attach-def/ovn-netconf-invalid.
func TestRunAll_OVNSemanticViolationGatesViaRuntimeValidation(t *testing.T) {
	d := t.TempDir()
	app := filepath.Join(d, "myapp")
	cfg := `{"cniVersion":"0.3.1","name":"mynet","type":"ovn-k8s-cni-overlay","topology":"layer3","netAttachDefName":"default/bad-net","allowPersistentIPs":true,"role":"secondary"}`
	mustWrite(t, filepath.Join(app, "overlays", "prod", "nad.yaml"),
		"apiVersion: k8s.cni.cncf.io/v1\nkind: NetworkAttachmentDefinition\nmetadata:\n  name: bad-net\n  namespace: default\nspec:\n  config: '"+cfg+"'\n")
	mustWrite(t, filepath.Join(app, "overlays", "prod", "kustomization.yaml"), "resources:\n  - nad.yaml\n")

	res, err := RunAll(Options{Dirs: []string{d}})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}

	if !res.Blocking {
		t.Error("expected the OVN semantic violation to block the run")
	}

	var nadSectionPassed, runtimeSectionFound bool
	for _, s := range res.Sections {
		switch {
		case s.Name == "NetworkAttachmentDefinition Validation":
			nadSectionPassed = s.Status != StatusError
		case strings.Contains(s.Name, "Runtime"):
			runtimeSectionFound = true
		}
	}
	if !nadSectionPassed {
		t.Error("expected the NAD section's structural gate to pass despite the OVN semantic violation")
	}
	if !runtimeSectionFound {
		t.Error("expected a Runtime Validation section for the OVN semantic violation")
	}

	var found bool
	for _, f := range res.Check.Findings {
		if f.CheckID == "k8scni/net-attach-def/ovn-netconf-invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a k8scni/net-attach-def/ovn-netconf-invalid finding in Check.Findings, got %d finding(s): %+v",
			len(res.Check.Findings), res.Check.Findings)
	}
}
