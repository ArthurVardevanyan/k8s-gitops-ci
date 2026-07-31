package rbac

import (
	"strings"
	"testing"
)

func TestValidateReader_ReadonlyAggregateWithBadVerbs(t *testing.T) {
	data := `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: reader
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
rules:
- verbs: ["get", "create"]
  resources: ["pods"]
  apiGroups: [""]
`
	errs := ValidateReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
	if !strings.Contains(errs[0].String(), "non-readonly verbs") {
		t.Errorf("unexpected error: %q", errs[0].String())
	}
}

func TestValidateReader_NoAggregateLabel(t *testing.T) {
	data := `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: cr
rules:
- verbs: ["create"]
  resources: ["pods"]
`
	errs := ValidateReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors without aggregate label: %v", errs)
	}
}

func TestValidateWildcards_VerbWildcard(t *testing.T) {
	data := `kind: ClusterRole
metadata:
  name: admin
rules:
- verbs: ["*"]
  resources: ["pods"]
`
	errs := ValidateWildcardsReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 1 || errs[0].Field != "verbs" {
		t.Errorf("expected verb wildcard: %v", errs)
	}
}

func TestValidateWildcards_ResourceWildcard(t *testing.T) {
	data := `kind: Role
metadata:
  name: admin
rules:
- verbs: ["get"]
  resources: ["*"]
`
	errs := ValidateWildcardsReader(strings.NewReader(data), "role.yaml")
	if len(errs) != 1 || errs[0].Field != "resources" {
		t.Errorf("expected resource wildcard: %v", errs)
	}
}

func TestFormatWildcardComment(t *testing.T) {
	s := FormatWildcardComment([]WildcardError{{Kind: "ClusterRole", Resource: "admin", RuleIndex: 0, Field: "verbs"}})
	if !strings.Contains(s, WildcardMarker) {
		t.Errorf("expected marker: %q", s)
	}
}

// ── isExemptVerb ──────────────────────────────────────────────────────────
//
// readOnlyExempt currently has two entries (metrics.k8s.io, monitoring.coreos.com).
// The original implementation iterated the whole map and bailed on the
// first entry whose key didn't match the apiGroup being checked - since Go
// intentionally randomizes map iteration order per call, a genuinely exempt
// verb would be rejected on roughly half of all calls. Every test below
// that exercises a real allowlisted entry runs isExemptVerb many times in a
// loop specifically to catch that class of regression; a single call could
// pass by chance even with the bug present.

const exemptCheckIterations = 200

func TestIsExemptVerb_MetricsAllowed(t *testing.T) {
	for i := 0; i < exemptCheckIterations; i++ {
		if !isExemptVerb("create", []string{"metrics.k8s.io"}, []string{"pods"}) {
			t.Fatalf("iteration %d: expected metrics.k8s.io/pods:create to be exempt", i)
		}
	}
}

func TestIsExemptVerb_PrometheusAllowed(t *testing.T) {
	for i := 0; i < exemptCheckIterations; i++ {
		if !isExemptVerb("update", []string{"monitoring.coreos.com"}, []string{"prometheuses/api"}) {
			t.Fatalf("iteration %d: expected monitoring.coreos.com/prometheuses/api:update to be exempt", i)
		}
	}
}

func TestIsExemptVerb_WrongVerbNotExempt(t *testing.T) {
	for i := 0; i < exemptCheckIterations; i++ {
		if isExemptVerb("delete", []string{"metrics.k8s.io"}, []string{"pods"}) {
			t.Fatalf("iteration %d: expected metrics.k8s.io/pods:delete to NOT be exempt", i)
		}
	}
}

func TestIsExemptVerb_WrongResourceNotExempt(t *testing.T) {
	for i := 0; i < exemptCheckIterations; i++ {
		if isExemptVerb("create", []string{"metrics.k8s.io"}, []string{"nodes"}) {
			t.Fatalf("iteration %d: expected metrics.k8s.io/nodes:create to NOT be exempt", i)
		}
	}
}

func TestIsExemptVerb_UnknownGroupNotExempt(t *testing.T) {
	for i := 0; i < exemptCheckIterations; i++ {
		if isExemptVerb("create", []string{"apps"}, []string{"pods"}) {
			t.Fatalf("iteration %d: expected an unlisted apiGroup to never be exempt", i)
		}
	}
}

func TestIsExemptVerb_WildcardGroupNotExempt(t *testing.T) {
	// Regression: a wildcard apiGroup must never be treated as exempt - it
	// would bypass the allowlist's narrow scoping entirely.
	for i := 0; i < exemptCheckIterations; i++ {
		if isExemptVerb("create", []string{"*"}, []string{"pods"}) {
			t.Fatalf("iteration %d: expected a wildcard apiGroup to never be exempt", i)
		}
	}
}

func TestIsExemptVerb_MultipleGroupsOneMismatchNotExempt(t *testing.T) {
	// Every listed apiGroup must be individually exempt; mixing an exempt
	// group with a non-exempt one must not be exempt overall.
	for i := 0; i < exemptCheckIterations; i++ {
		if isExemptVerb("create", []string{"metrics.k8s.io", "apps"}, []string{"pods"}) {
			t.Fatalf("iteration %d: expected mixed apiGroups to NOT be exempt", i)
		}
	}
}

func TestIsExemptVerb_EmptyInputsNotExempt(t *testing.T) {
	if isExemptVerb("create", nil, []string{"pods"}) {
		t.Error("expected no apiGroups to never be exempt")
	}
	if isExemptVerb("create", []string{"metrics.k8s.io"}, nil) {
		t.Error("expected no resources to never be exempt")
	}
}

// ── end-to-end: the exemption wired through badVerbs/ValidateReader ───────

func TestValidateReader_ExemptReadonlyVerbsNotFlagged(t *testing.T) {
	data := `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: metrics-reader
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
rules:
- verbs: ["get", "list", "watch", "create"]
  resources: ["pods"]
  apiGroups: ["metrics.k8s.io"]
`
	for i := 0; i < exemptCheckIterations; i++ {
		errs := ValidateReader(strings.NewReader(data), "cr.yaml")
		if len(errs) != 0 {
			t.Fatalf("iteration %d: expected metrics.k8s.io/pods:create to be exempt, got: %v", i, errs)
		}
	}
}

func TestValidateReader_ExemptMixedRuleStillFails(t *testing.T) {
	// A rule exempting one apiGroup/resource pair must not accidentally
	// exempt a sibling, non-allowlisted apiGroup in the same rule.
	data := `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: mixed
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
rules:
- verbs: ["get", "create"]
  resources: ["pods"]
  apiGroups: ["metrics.k8s.io", "apps"]
`
	for i := 0; i < exemptCheckIterations; i++ {
		errs := ValidateReader(strings.NewReader(data), "cr.yaml")
		if len(errs) != 1 {
			t.Fatalf("iteration %d: expected 1 error for a non-exempt mixed apiGroup rule, got: %v", i, errs)
		}
	}
}

func TestDeduplicate_DifferentBadVerbsNotMerged(t *testing.T) {
	// Same Kind/Resource/RuleIndex/AggLabel, but different offending verb
	// sets, must produce two separate DeduplicatedError entries - not one
	// merged entry with an inflated count and only the first-seen
	// BadVerbs.
	errs := []ValidationError{
		{Kind: "ClusterRole", Resource: "reader", RuleIndex: 0, AggLabel: "view", BadVerbs: []string{"create"}},
		{Kind: "ClusterRole", Resource: "reader", RuleIndex: 0, AggLabel: "view", BadVerbs: []string{"delete"}},
	}
	out := Deduplicate(errs)
	if len(out) != 2 {
		t.Fatalf("expected 2 distinct deduplicated entries for different BadVerbs, got %d: %+v", len(out), out)
	}
	for _, d := range out {
		if d.Count != 1 {
			t.Errorf("expected each distinct-BadVerbs entry to have Count 1, got %d for %+v", d.Count, d)
		}
	}
}

func TestDeduplicate_SameBadVerbsRegardlessOfOrderStillMerged(t *testing.T) {
	// The same set of offending verbs, differently ordered, must still
	// collapse into a single entry with an incremented Count - the fix
	// sorts BadVerbs before building the key specifically so this holds.
	errs := []ValidationError{
		{Kind: "ClusterRole", Resource: "reader", RuleIndex: 0, AggLabel: "view", BadVerbs: []string{"create", "delete"}},
		{Kind: "ClusterRole", Resource: "reader", RuleIndex: 0, AggLabel: "view", BadVerbs: []string{"delete", "create"}},
	}
	out := Deduplicate(errs)
	if len(out) != 1 {
		t.Fatalf("expected 1 merged entry for the same BadVerbs set in different order, got %d: %+v", len(out), out)
	}
	if out[0].Count != 2 {
		t.Errorf("expected Count 2, got %d", out[0].Count)
	}
}

// --- previously-untested wildcard / non-RBAC / empty-comment cases -------

func TestValidateWildcards_NoWildcards(t *testing.T) {
	data := `kind: ClusterRole
metadata:
  name: fine
rules:
- verbs: ["get", "list"]
  resources: ["pods"]
  apiGroups: [""]
`
	errs := ValidateWildcardsReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no wildcard findings, got: %v", errs)
	}
}

func TestValidateWildcards_NonRBAC(t *testing.T) {
	data := `kind: ConfigMap
metadata:
  name: cm
data:
  key: value
`
	errs := ValidateWildcardsReader(strings.NewReader(data), "cm.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no wildcard findings for a non-RBAC kind, got: %v", errs)
	}
}

func TestFormatWildcardComment_Empty(t *testing.T) {
	if s := FormatWildcardComment(nil); s != "" {
		t.Errorf("expected empty string for no findings, got: %q", s)
	}
}

// --- testdata-fixture-driven tests ----------------------------------------

func TestValidateFile_WildcardAPIGroups(t *testing.T) {
	errs := ValidateWildcards("testdata/wildcard-apigroups.yaml")
	if len(errs) != 1 || errs[0].Field != "apiGroups" {
		t.Fatalf("expected 1 apiGroups wildcard finding, got: %v", errs)
	}
}

func TestValidateFile_MultiDoc(t *testing.T) {
	errs := ValidateFile("testdata/multi-doc.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 1 finding from each of the 2 docs in the multi-doc stream, got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_NotClusterRole(t *testing.T) {
	errs := ValidateFile("testdata/not-clusterrole.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for a RoleBinding, got: %v", errs)
	}
}

func TestValidateWildcards_RoleMultiDoc(t *testing.T) {
	errs := ValidateWildcards("testdata/wildcard-role-multidoc.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 wildcard findings (one per doc: Role verbs, ClusterRole resources), got %d: %v", len(errs), errs)
	}
	var sawRole, sawClusterRole bool
	for _, e := range errs {
		if e.Kind == "Role" && e.Field == "verbs" {
			sawRole = true
		}
		if e.Kind == "ClusterRole" && e.Field == "resources" {
			sawClusterRole = true
		}
	}
	if !sawRole || !sawClusterRole {
		t.Errorf("expected findings from both the Role and ClusterRole docs, got: %v", errs)
	}
}

func TestValidateFile_GoodClusterRole(t *testing.T) {
	errs := ValidateFile("testdata/good-clusterrole.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for an aggregate ClusterRole with all-readonly verbs, got: %v", errs)
	}
}

func TestValidateFile_ClusterReaderAggLabel(t *testing.T) {
	errs := ValidateFile("testdata/cluster-reader.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(errs), errs)
	}
	if errs[0].AggLabel != "rbac.authorization.k8s.io/aggregate-to-cluster-reader" {
		t.Errorf("expected the cluster-reader AggLabel specifically, got: %q", errs[0].AggLabel)
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	errs := ValidateFile("testdata/does-not-exist.yaml")
	if errs != nil {
		t.Errorf("expected nil for a nonexistent file, got: %v", errs)
	}
}

func TestValidateWildcards_MissingFile(t *testing.T) {
	errs := ValidateWildcards("testdata/does-not-exist.yaml")
	if errs != nil {
		t.Errorf("expected nil for a nonexistent file, got: %v", errs)
	}
}
