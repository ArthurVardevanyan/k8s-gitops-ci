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
