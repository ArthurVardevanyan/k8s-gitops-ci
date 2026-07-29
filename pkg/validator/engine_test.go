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
	res := runDocChecks([]string{}, nil, 1)
	if len(res.Findings) != 0 {
		t.Errorf("expected empty findings")
	}
}

func TestRunOverlayChecks(t *testing.T) {
	res := runOverlayChecks([]string{"overlay"}, "cluster", nil, 1)
	if len(res.Findings) != 0 {
		t.Errorf("expected empty findings")
	}
}

func TestFanOutExemption(t *testing.T) {
	findings := []check.Finding{{CheckID: "namespace", File: "", Value: "skip-me"}}
	selectors := []exempt.Selector{{Check: "namespace", Value: "skip-me"}}
	out := fanOut(findings, []string{"a.yaml"}, selectors)
	if len(out) != 0 {
		t.Errorf("expected exemption to remove finding")
	}
}
