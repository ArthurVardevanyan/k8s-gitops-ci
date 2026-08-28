package validator

import (
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtimepkg "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"

	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes"
)

// The runtime check family is documented as always-blocking and
// non-exemptable. That guarantee is not implemented by any single check - it
// depends on a chain of wiring: a finding's CheckID must resolve in the check
// registry, so the dispatcher classifies it as runtime-validation, so it
// renders in its own section and marks the result blocking.
//
// Every link in that chain was broken at once and no test noticed, because
// each check's unit tests only asserted "Run() returned a finding" and
// stopped there. These tests cover the wiring itself.

// runtimeChecks returns every registered check in the runtime-validation
// section.
func runtimeChecks(t *testing.T) []check.Check {
	t.Helper()
	var out []check.Check
	for _, c := range check.All() {
		if c.Section() == "runtime-validation" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		t.Fatal("no runtime-validation checks registered; the family's blank imports are missing")
	}
	return out
}

// TestRuntimeFindingCheckIDResolvesInRegistry is the direct regression test
// for the family being inert. Findings carried the broad Category ("batch")
// as their CheckID, but checks register under their rule ID
// ("batch/schedule-invalid"), so check.ByID missed on every finding.
func TestRuntimeFindingCheckIDResolvesInRegistry(t *testing.T) {
	for _, c := range runtimeChecks(t) {
		f := runtimepkg.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
		}.ToCheckFinding()

		if f.CheckID != c.ID() {
			t.Errorf("check %q: finding CheckID = %q, want the registered rule ID; a category is not an identity", c.ID(), f.CheckID)
		}
		got, ok := check.ByID(f.CheckID)
		if !ok {
			t.Errorf("check %q: finding CheckID %q does not resolve via check.ByID, so the finding is classified as resource-compliance and then dropped", c.ID(), f.CheckID)
			continue
		}
		if got.Section() != "runtime-validation" {
			t.Errorf("check %q: resolved section = %q, want runtime-validation", c.ID(), got.Section())
		}
	}
}

// TestRuntimeFindingsAreClassifiedAsRuntime asserts the dispatcher actually
// routes runtime findings into the runtime bucket rather than the compliance
// one.
func TestRuntimeFindingsAreClassifiedAsRuntime(t *testing.T) {
	all := runtimeChecks(t)
	findings := make([]check.Finding, 0, len(all))
	for _, c := range all {
		findings = append(findings, runtimepkg.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			// Deliberately a category, not the rule ID. Using c.ID() here
			// would make the finding resolve even under the old
			// CheckID-falls-back-to-Category behavior, so the test would
			// pass against the very bug it exists to catch.
		}.ToCheckFinding())
	}

	runtimeFindings, complianceFindings := separateFindingsBySection(findings)

	if len(runtimeFindings) != len(all) {
		t.Errorf("separateFindingsBySection routed %d/%d findings to runtime-validation", len(runtimeFindings), len(all))
	}
	if len(complianceFindings) != 0 {
		// These would be silently discarded: the compliance copy loop only
		// emits IDs present in complianceCheckOrder.
		t.Errorf("%d runtime finding(s) leaked into resource-compliance, where they are dropped entirely; first: %q",
			len(complianceFindings), complianceFindings[0].CheckID)
	}
}

// TestRuntimeValidationSectionRenders guards the reporting half: the section
// must actually contain the finding. ComposeRuntimeValidationSection had no
// test at all, which is why nobody noticed it was never being called.
func TestRuntimeValidationSectionRenders(t *testing.T) {
	f := runtimepkg.Finding{
		RuleID:    "batch/schedule-invalid",
		RuleTitle: "CronJob Schedule Must Be Valid",
		Finding: check.Finding{
			File:    "overlays/prod/cronjob.yaml",
			Kind:    "CronJob",
			Name:    "nightly",
			Message: "schedule: invalid cron expression",
		},
	}.ToCheckFinding()

	sec := ComposeRuntimeValidationSection([]check.Finding{f})

	if sec.Status != StatusError {
		t.Errorf("section Status = %v, want StatusError", sec.Status)
	}
	if !strings.Contains(sec.Body, "overlays/prod/cronjob.yaml") {
		t.Errorf("section body omits the offending file:\n%s", sec.Body)
	}
	if !strings.Contains(sec.Body, "invalid cron expression") {
		t.Errorf("section body omits the finding message:\n%s", sec.Body)
	}
}

// TestRuntimeSectionGroupsByCategory pins the grouping key. CheckID is the
// rule ID, so grouping the report on it would emit one <details> block per
// rule; the section groups on Extra["category"] instead.
func TestRuntimeSectionGroupsByCategory(t *testing.T) {
	mk := func(rule string) check.Finding {
		return runtimepkg.Finding{
			RuleID: rule,
			Finding: check.Finding{
				File:    "a.yaml",
				Message: "boom " + rule,
			},
		}.ToCheckFinding()
	}
	sec := ComposeRuntimeValidationSection([]check.Finding{
		mk("batch/schedule-invalid"),
		mk("batch/parallelism-invalid"),
	})

	if n := strings.Count(sec.Body, "<details>"); n != 1 {
		t.Errorf("got %d <details> blocks for two findings in one category, want 1 (grouping key is not the category)", n)
	}
}

// TestRuntimeChecksAreNonExemptable backs the "never suppressible" half of
// the family's contract.
func TestRuntimeChecksAreNonExemptable(t *testing.T) {
	for _, c := range runtimeChecks(t) {
		ne, ok := c.(interface{ NonExemptable() bool })
		if !ok || !ne.NonExemptable() {
			t.Errorf("check %q is exemptable, but the runtime family is documented as non-exemptable", c.ID())
		}
		if !c.Blocking() {
			t.Errorf("check %q is non-blocking, but the runtime family is documented as always-blocking", c.ID())
		}
		// The API server sees the rendered object, so these rules must be
		// evaluated against it. Asserted here rather than recorded per-check
		// in the identity snapshot, where it would be the same value 81
		// times.
		rs, ok := c.(interface{ RenderSensitive() bool })
		if !ok || !rs.RenderSensitive() {
			t.Errorf("check %q is not render-sensitive, but runtime rules apply to the rendered object", c.ID())
		}
	}
}
