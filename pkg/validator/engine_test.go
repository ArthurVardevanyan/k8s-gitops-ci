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
	res := runDocChecks([]string{}, nil, nil, 1, nil)
	if len(res.Findings) != 0 {
		t.Errorf("expected empty findings")
	}
}

// TestRunDocChecks_KyvernoPolicyDocsExcludedFromComplianceChecks is a
// regression/coverage test proving indexDocuments' blanket
// isKyvernoPolicyDoc skip already excludes Kyverno ClusterPolicy documents
// from every ScopeDoc check (podspec included), even though a
// ClusterPolicy's rule body can be shaped like a bare Pod spec (which would
// otherwise trip podspec's tree-walker).
func TestRunDocChecks_KyvernoPolicyDocsExcludedFromComplianceChecks(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "policy.yaml")
	policy := `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  rules:
  - name: autogen-cronjobs
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      pattern:
        spec:
          containers:
          - name: "*"
            securityContext:
              runAsNonRoot: true
`
	if err := os.WriteFile(f, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	for _, finding := range res.Findings {
		if finding.CheckID == "podspec-defaults" {
			t.Errorf("expected Kyverno policy documents to be excluded from podspec-defaults, got %+v", finding)
		}
	}
}

// TestEvaluateDoc_DocSkipperExcludesMatchingKind is a direct unit test of
// the DocSkipper wiring itself (rather than a specific check), so a future
// regression is caught even if every current DocSkipper implementation
// happens to also be covered some other way.
func TestEvaluateDoc_DocSkipperExcludesMatchingKind(t *testing.T) {
	doc := []byte("kind: Widget\nmetadata:\n  name: x\n")
	findings, _ := evaluateDoc(doc, []string{"a.yaml"}, []check.Check{skippingDocCheck{skipKind: "Widget"}}, nil)
	if len(findings) != 0 {
		t.Errorf("expected the matching kind to be skipped, got %+v", findings)
	}
	findings, _ = evaluateDoc(doc, []string{"a.yaml"}, []check.Check{skippingDocCheck{skipKind: "OtherKind"}}, nil)
	if len(findings) != 1 {
		t.Errorf("expected a non-matching kind not to be skipped, got %+v", findings)
	}
}

type skippingDocCheck struct{ skipKind string }

func (skippingDocCheck) ID() string                 { return "skipping-doc-check" }
func (skippingDocCheck) Title() string              { return "" }
func (skippingDocCheck) Section() string            { return "" }
func (skippingDocCheck) Blocking() bool             { return false }
func (skippingDocCheck) Scope() check.Scope         { return check.ScopeDoc }
func (s skippingDocCheck) SkipDoc(kind string) bool { return kind == s.skipKind }
func (skippingDocCheck) CheckDoc(data []byte, source string) []check.Finding {
	return []check.Finding{{CheckID: "skipping-doc-check", File: source}}
}

func TestFilterKyvernoTestFixtureDirs(t *testing.T) {
	d := t.TempDir()
	fixtureDir := filepath.Join(d, "fixtures")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "kyverno-test.yaml"), []byte("name: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureResource := filepath.Join(fixtureDir, "bad-pod.yaml")
	if err := os.WriteFile(fixtureResource, []byte("kind: Pod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalDir := filepath.Join(d, "app")
	if err := os.MkdirAll(normalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	normalResource := filepath.Join(normalDir, "deployment.yaml")
	if err := os.WriteFile(normalResource, []byte("kind: Deployment\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filterKyvernoTestFixtureDirs([]string{fixtureResource, normalResource})
	if len(out) != 1 || out[0] != normalResource {
		t.Errorf("expected only %q to survive, got %v", normalResource, out)
	}
}

func TestFilterKyvernoTestFixtureDirs_NoFixtureDirsIsNoOp(t *testing.T) {
	in := []string{"a.yaml", "b.yaml"}
	out := filterKyvernoTestFixtureDirs(in)
	if len(out) != 2 {
		t.Errorf("expected no filtering when no kyverno-test.yaml exists, got %v", out)
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
	base := runDocChecks([]string{f}, nil, nil, 1, nil)
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
	out := runDocChecks([]string{f}, nil, nil, 1, map[string]bool{"sync-options": true})
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

	res := runDocChecks([]string{f}, nil, nil, 1, nil)
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

func TestRunDocChecks_ImageChecksumRepoLevelAnnotationExemption(t *testing.T) {
	// End-to-end regression for a reported false-negative-in-reverse: a
	// user annotated a resource with a repo-level exemption (no tag) to
	// survive future Renovate tag bumps
	// ("docker.io/linuxserver/heimdall"), but the finding's exact Image
	// value is the tagged reference ("docker.io/linuxserver/heimdall:2.8.2"),
	// so an exact-only match never applied the exemption. The
	// image-checksum adapter now also sets MatchAliases to the tag/
	// digest-independent repo key, so this now exempts correctly.
	d := t.TempDir()
	f := filepath.Join(d, "statefulset.yaml")
	doc := `kind: StatefulSet
metadata:
  name: heimdall
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: docker.io/linuxserver/heimdall
spec:
  template:
    spec:
      containers:
      - name: heimdall
        image: docker.io/linuxserver/heimdall:2.8.2
`
	if err := os.WriteFile(f, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	for _, fnd := range res.Findings {
		if fnd.CheckID == "image-checksum" {
			t.Errorf("expected the repo-level-exempted tagged image finding to be excluded, got %+v", fnd)
		}
	}
	found := false
	for _, ex := range res.Exempted {
		if ex.CheckID == "image-checksum" && ex.Value == "docker.io/linuxserver/heimdall:2.8.2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an audit-trail entry for the repo-level image-checksum exemption, got %+v", res.Exempted)
	}

	// A different repo must NOT be exempted by this same annotation - the
	// alias match is anchored to the exact repo key, not a prefix/substring.
	fOther := filepath.Join(d, "other.yaml")
	docOther := `kind: StatefulSet
metadata:
  name: other
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: docker.io/linuxserver/heimdall
spec:
  template:
    spec:
      containers:
      - name: other
        image: docker.io/linuxserver/heimdall-extra:1.0
`
	if err := os.WriteFile(fOther, []byte(docOther), 0o644); err != nil {
		t.Fatal(err)
	}
	resOther := runDocChecks([]string{fOther}, nil, nil, 1, nil)
	stillFlagged := false
	for _, fnd := range resOther.Findings {
		if fnd.CheckID == "image-checksum" {
			stillFlagged = true
		}
	}
	if !stillFlagged {
		t.Error("expected an unrelated repo sharing a name prefix to still be flagged, not exempted")
	}
}

func TestRunDocChecks_ImageFQDNNotExemptable_AnnotationAndSelectorIgnored(t *testing.T) {
	// image-fqdn is deliberately non-exemptable (see exempt.Exemptable),
	// but check.Register unconditionally calls
	// exempt.RegisterExemptable(c.ID()) for every registered check - this
	// is a regression guard proving the hardcoded exception in
	// exempt.Exemptable actually wins end-to-end through the real
	// registration in this package's init(), for both an annotation and
	// an EXEMPTIONS=(...) selector attempt.
	if exempt.Exemptable("image-fqdn") {
		t.Fatal("image-fqdn must not be exemptable after real check registration")
	}

	d := t.TempDir()
	f := filepath.Join(d, "x.yaml")
	doc := `kind: Deployment
metadata:
  name: d
  annotations:
    gitops-ci.k8s.io/exempt-image-fqdn: nginx:latest
spec:
  template:
    spec:
      containers:
      - image: nginx:latest
`
	if err := os.WriteFile(f, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	selectors := []exempt.Selector{{Check: "image-fqdn", Value: "nginx:latest"}}
	res := runDocChecks([]string{f}, nil, selectors, 1, nil)
	found := false
	for _, fnd := range res.Findings {
		if fnd.CheckID == "image-fqdn" {
			found = true
		}
	}
	if !found {
		t.Error("expected the image-fqdn finding to still fire despite a matching annotation and selector")
	}
	for _, ex := range res.Exempted {
		if ex.CheckID == "image-fqdn" {
			t.Errorf("expected no image-fqdn exemption to ever be recorded, got %+v", ex)
		}
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

func TestRunDocChecks_TektonPACPipelineRun_ExemptFromSyncOptionsAndNamespace(t *testing.T) {
	// PipelineRun manifests under a top-level .tekton/ directory are
	// managed directly by the Tekton Pipelines-as-code controller, not
	// synced by Argo CD - so they're exempt from sync-options and
	// namespace by default (see tekton_exemptions.go).
	d := t.TempDir()
	t.Chdir(d)

	doc := `kind: PipelineRun
apiVersion: tekton.dev/v1
metadata:
  name: overlay-test
`
	if err := os.MkdirAll(".tekton", 0o755); err != nil {
		t.Fatal(err)
	}
	tektonFile := filepath.Join(".tekton", "overlay-test.yaml")
	if err := os.WriteFile(tektonFile, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	// Same doc under a non-.tekton path must still be flagged - guards
	// against the exemption leaking beyond the .tekton/ directory.
	elsewhereFile := filepath.Join("apps", "foo", "base.yaml")
	if err := os.MkdirAll(filepath.Join("apps", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(elsewhereFile, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	res := runDocChecks([]string{tektonFile, elsewhereFile}, nil, builtinExemptSelectors(), 1, nil)

	byFileAndCheck := map[string]bool{}
	for _, f := range res.Findings {
		byFileAndCheck[f.File+"/"+f.CheckID] = true
	}
	for _, checkID := range []string{"sync-options", "namespace"} {
		if byFileAndCheck[tektonFile+"/"+checkID] {
			t.Errorf("expected %s under .tekton/ to be exempt, got a finding", checkID)
		}
		if !byFileAndCheck[elsewhereFile+"/"+checkID] {
			t.Errorf("expected %s outside .tekton/ to still be flagged", checkID)
		}
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
