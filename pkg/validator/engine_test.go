package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

func TestIndexDocuments(t *testing.T) {
	d := t.TempDir()
	f1 := filepath.Join(d, "a.yaml")
	_ = os.WriteFile(f1, []byte("kind: Pod\nmetadata:\n  name: x\n"), 0o644)
	f2 := filepath.Join(d, "b.yaml")
	_ = os.WriteFile(f2, []byte("kind: Service\nmetadata:\n  name: y\n"), 0o644)
	docs := indexDocuments([]string{f1, f2})
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestSplitDocuments(t *testing.T) {
	in := []byte("kind: Pod\n---\nkind: Service\n")
	docs := splitDocuments(in)
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestUniqueStrings(t *testing.T) {
	in := []string{"a", "a", "b"}
	out := uniqueStrings(in)
	if len(out) != 2 {
		t.Errorf("expected 2 unique, got %d", len(out))
	}
}

func TestFinalizeCompliance(t *testing.T) {
	findings := []check.Finding{
		{CheckID: "x", File: "a.yaml"},
		{CheckID: "x", File: "b.yaml", ForcedDirect: true},
	}
	changed := map[string]bool{"a.yaml": true}
	direct, indirect := finalizeCompliance(findings, changed)
	if len(direct) != 2 || len(indirect) != 0 {
		t.Errorf("unexpected split: direct=%d indirect=%d", len(direct), len(indirect))
	}
}

func TestRunDocChecks(t *testing.T) {
	res := runDocChecks([]string{}, nil, 1, nil)
	if len(res.Findings) != 0 {
		t.Errorf("expected empty findings")
	}
}

func TestRunOverlayChecks(t *testing.T) {
	res := runOverlayChecks([]string{"overlay"}, "cluster", nil, 1, nil)
	if len(res.Findings) != 0 {
		t.Errorf("expected empty findings")
	}
}

func TestFilterDisabled(t *testing.T) {
	all := check.ByScope(check.ScopeDoc)
	if len(all) == 0 {
		t.Skip("no ScopeDoc checks registered")
	}
	target := all[0].ID()

	// No disabled set: everything passes through.
	if out := filterDisabled(all, nil); len(out) != len(all) {
		t.Errorf("expected all %d checks, got %d", len(all), len(out))
	}

	// Disabling one ID removes exactly that check.
	disabled := map[string]bool{target: true}
	out := filterDisabled(all, disabled)
	if len(out) != len(all)-1 {
		t.Fatalf("expected %d checks after disabling one, got %d", len(all)-1, len(out))
	}
	for _, c := range out {
		if c.ID() == target {
			t.Errorf("disabled check %q still present", target)
		}
	}
}

func TestRunDocChecks_DisabledCheckExcluded(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "x.yaml")
	_ = os.WriteFile(f, []byte("kind: ArgoCD\napiVersion: argoproj.io/v1alpha1\nmetadata:\n  name: cd\n"), 0o644)

	// sync-options should normally flag this doc.
	base := runDocChecks([]string{f}, nil, 1, nil)
	found := false
	for _, fnd := range base.Findings {
		if fnd.CheckID == "sync-options" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sync-options finding without disabling, got %+v", base.Findings)
	}

	// Disabling sync-options should exclude it entirely.
	out := runDocChecks([]string{f}, nil, 1, map[string]bool{"sync-options": true})
	for _, fnd := range out.Findings {
		if fnd.CheckID == "sync-options" {
			t.Errorf("expected sync-options findings to be excluded, got %+v", out.Findings)
		}
	}
}

func TestRunDocChecks_ImageChecksumAnnotationExemption_RecordsAudit(t *testing.T) {
	// End-to-end regression covering two related fixes together: the
	// image-checksum check adapter now routes through the shared
	// check/exempt engine (previously it decided annotation exemptions
	// internally, before a Finding ever existed), and fanOut now actually
	// records the resulting exemption instead of discarding it.
	d := t.TempDir()
	f := filepath.Join(d, "x.yaml")
	doc := `kind: Deployment
metadata:
  name: d
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: registry.io/repo:tag
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo:tag
`
	if err := os.WriteFile(f, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	res := runDocChecks([]string{f}, nil, 1, nil)
	for _, fnd := range res.Findings {
		if fnd.CheckID == "image-checksum" {
			t.Errorf("expected the annotation-exempted image finding to be excluded, got %+v", fnd)
		}
	}
	found := false
	for _, ex := range res.Exempted {
		if ex.CheckID == "image-checksum" && ex.Value == "registry.io/repo:tag" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an audit-trail entry for the image-checksum exemption, got %+v", res.Exempted)
	}
}

func TestFanOutExemption(t *testing.T) {
	findings := []check.Finding{{CheckID: "namespace", File: "", Value: "skip-me"}}
	selectors := []exempt.Selector{{Check: "namespace", Value: "skip-me"}}
	out, exempted := fanOut(findings, []string{"a.yaml"}, selectors)
	if len(out) != 0 {
		t.Errorf("expected exemption to remove finding")
	}
	// Regression: fanOut previously discarded the exempt.Applied value
	// entirely (`if ok, _ := exempt.Evaluate(...)`), so no check ever got
	// an audit-trail entry for its exemptions.
	if len(exempted) != 1 {
		t.Fatalf("expected 1 recorded exemption, got %d: %v", len(exempted), exempted)
	}
	if exempted[0].CheckID != "namespace" || exempted[0].Value != "skip-me" {
		t.Errorf("unexpected recorded exemption: %+v", exempted[0])
	}
}

func TestFanOutExemption_NoMatch_NoRecordedExemption(t *testing.T) {
	findings := []check.Finding{{CheckID: "namespace", File: "", Value: "keep-me"}}
	selectors := []exempt.Selector{{Check: "namespace", Value: "skip-me"}}
	out, exempted := fanOut(findings, []string{"a.yaml"}, selectors)
	if len(out) != 1 {
		t.Errorf("expected the non-matching finding to survive")
	}
	if len(exempted) != 0 {
		t.Errorf("expected no recorded exemptions, got %v", exempted)
	}
}
