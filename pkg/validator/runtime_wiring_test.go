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
// ("kubernetes/batch/schedule-invalid"), so check.ByID missed on every finding.
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
		RuleID:    "kubernetes/batch/schedule-invalid",
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
		mk("kubernetes/batch/schedule-invalid"),
		mk("kubernetes/batch/parallelism-invalid"),
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

// TestRuntimeFindingsCarryTheirCategory pins the report's grouping keys for
// every check at once.
//
// Family and category are derived from the rule ID rather than stored, so the
// thing that can actually break is an ID without enough segments to derive
// them from: CategoryOf would hand back the family and the report would grow
// a group named after a whole family rather than one of its categories.
// Asserting the ID is "<family>/<category>/<rule>" and that both keys survive
// into Extra covers every half.
func TestRuntimeFindingsCarryTheirCategory(t *testing.T) {
	for _, c := range runtimeChecks(t) {
		id := c.ID()
		parts := strings.Split(id, "/")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			t.Errorf("check %q is not %q, so its findings would group under a family or the rule itself", id, "<family>/<category>/<rule>")
			continue
		}
		family, group := parts[0], parts[1]
		if got := runtimepkg.FamilyOf(id); got != family {
			t.Errorf("check %q: FamilyOf = %q, want %q", id, got, family)
		}
		if got := runtimepkg.CategoryOf(id); got != group {
			t.Errorf("check %q: CategoryOf = %q, want %q", id, got, group)
		}
		f := runtimepkg.Finding{RuleID: id, RuleTitle: c.Title()}.ToCheckFinding()
		if got := f.Extra["family"]; got != family {
			t.Errorf("check %q: finding family = %q, want %q; the report groups on this", id, got, family)
		}
		if got := f.Extra["category"]; got != group {
			t.Errorf("check %q: finding category = %q, want %q; the report groups on this", id, got, group)
		}
	}
}

// TestRuntimeSectionRendersRuleAndCitation guards the reporting half of the
// family's claim.
//
// The section is grouped by category, which has no TableSpec, so the generic
// compliance renderer fell through to a File/Message table: the resource, the
// field path, the rule that fired and the upstream citation were all dropped,
// and identical findings from two overlays were printed twice. A citation
// that never reaches the report is not a citation.
func TestRuntimeSectionRendersRuleAndCitation(t *testing.T) {
	mk := func(file string) check.Finding {
		return runtimepkg.Finding{
			RuleID:    "kubernetes/batch/schedule-invalid",
			RuleTitle: "CronJob Schedule Must Be Valid",
			Finding: check.Finding{
				File:    file,
				Kind:    "CronJob",
				Name:    "nightly",
				Path:    "spec.schedule",
				Message: "schedule: invalid cron expression",
			},
		}.ToCheckFinding()
	}

	sec := ComposeRuntimeValidationSection([]check.Finding{mk("a.yaml")})

	for _, want := range []string{
		"kubernetes/batch/schedule-invalid", // which rule fired
		"CronJob/nightly",                   // which resource
		"spec.schedule",                     // which field
		"pkg/apis/batch/validation",         // the upstream citation
		"invalid cron expression",           // the message
	} {
		if !strings.Contains(sec.Body, want) {
			t.Errorf("section body omits %q:\n%s", want, sec.Body)
		}
	}

	// The same finding reaching the report from two rendered overlays is one
	// problem, not two.
	dup := ComposeRuntimeValidationSection([]check.Finding{mk("a.yaml"), mk("a.yaml")})
	if n := strings.Count(dup.Body, "invalid cron expression"); n != 1 {
		t.Errorf("an identical finding rendered %d times, want 1:\n%s", n, dup.Body)
	}
}

// TestRuntimeSectionCellsSurviveHostileValues pins that values copied out of a
// manifest cannot break the report table.
//
// Resource names and field values are attacker-adjacent in the sense that they
// come from the file under test rather than from this tool. A newline in one
// of them splits a single row into several, so the rest of the table renders
// as prose and the remaining findings become unreadable. A pipe does the same
// to the column count. The renderer previously escaped only the pipe.
func TestRuntimeSectionCellsSurviveHostileValues(t *testing.T) {
	f := runtimepkg.Finding{
		RuleID:    "core/object-name-invalid",
		RuleTitle: "Object Name Must Be Valid",
		Finding: check.Finding{
			File:    "a.yaml",
			Kind:    "Pod",
			Name:    "evil\n| x | y |\nmore",
			Path:    "metadata.name",
			Message: "name: invalid | value\nsecond line",
		},
	}.ToCheckFinding()

	sec := ComposeRuntimeValidationSection([]check.Finding{f})

	// A table row is one line. If a value's newline survives, the row's later
	// cells are pushed onto following lines, so locate the row by its rule ID
	// and require every other cell to still be on it.
	var row string
	for _, line := range strings.Split(sec.Body, "\n") {
		if strings.Contains(line, "core/object-name-invalid") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatalf("no table row for the finding:\n%s", sec.Body)
	}
	for _, want := range []string{"Pod/evil", "a.yaml", "metadata.name", "second line"} {
		if !strings.Contains(row, want) {
			t.Errorf("cell %q is not on the finding's row - an embedded newline split it:\n%s",
				want, sec.Body)
		}
	}
	if strings.Contains(sec.Body, "invalid | value") {
		t.Errorf("an embedded pipe was left unescaped:\n%s", sec.Body)
	}
}

// TestRuntimeSectionGroupsByFamily pins both halves of the family level:
// it is omitted for a single family and rendered for more than one.
//
// The omission is the half worth guarding. Wrapping a lone family in a
// <details> whose only child is the section's entire content adds a click and
// says nothing, and every run today has exactly one family - so a regression
// here would degrade every report while the multi-family path, which nothing
// yet exercises, stayed green.
func TestRuntimeSectionGroupsByFamily(t *testing.T) {
	mk := func(ruleID, kind string) check.Finding {
		return runtimepkg.Finding{
			RuleID:    ruleID,
			RuleTitle: "Some Rule",
			Finding: check.Finding{
				File: "a.yaml", Kind: kind, Name: "x",
				Path: "spec", Message: "boom",
			},
		}.ToCheckFinding()
	}

	single := ComposeRuntimeValidationSection([]check.Finding{
		mk("kubernetes/batch/schedule-invalid", "CronJob"),
	})
	if strings.Contains(single.Body, "rejected by the API server") {
		t.Errorf("single family rendered a family wrapper:\n%s", single.Body)
	}

	// The whole claim of the family change is that a single-family run - which
	// is every run today, since every registered check is in the kubernetes
	// family - renders exactly as it did before. A family wrapper is the
	// obvious way to break that, but so is quietly generalising the intro to
	// suit families that do not exist yet. Pin the sentence verbatim: it is
	// accurate precisely because the only family shipping is one the API
	// server does enforce, and it should not change until that stops being
	// true.
	const intro = "These are structural/runtime Kubernetes validation rules enforced by the cluster API server. Findings here indicate manifests that the cluster would reject."
	if !strings.Contains(single.Body, intro) {
		t.Errorf("single-family intro changed; a report from this branch no longer\ndiffers from its base only in check IDs.\nwant: %s\ngot:\n%s", intro, single.Body)
	}

	multi := ComposeRuntimeValidationSection([]check.Finding{
		mk("kubernetes/batch/schedule-invalid", "CronJob"),
		mk("example/net-attach-def/config-invalid", "NetworkAttachmentDefinition"),
	})
	for _, want := range []string{
		"Kubernetes — rejected by the API server", // the qualifier itself
		"Example", // a family with no bespoke title
	} {
		if !strings.Contains(multi.Body, want) {
			t.Errorf("multi-family body omits %q:\n%s", want, multi.Body)
		}
	}

	// Both families' findings must survive the extra nesting level.
	for _, want := range []string{"CronJob/x", "NetworkAttachmentDefinition/x"} {
		if !strings.Contains(multi.Body, want) {
			t.Errorf("multi-family body dropped %q:\n%s", want, multi.Body)
		}
	}
}
