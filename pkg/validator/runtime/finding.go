package runtime

import (
	"slices"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// Finding extends check.Finding with runtime validation metadata.
// Runtime checks are always blocking and never exempted — the cluster rejects
// non-compliant manifests regardless of any exemptions.
type Finding struct {
	// RuleID is the upstream Kubernetes validation rule ID.
	RuleID string `json:"ruleId,omitempty"`
	// RuleTitle is a human-readable title for the rule.
	RuleTitle string `json:"ruleTitle,omitempty"`
	check.Finding
}

// ToCheckFinding converts a Finding to a check.Finding.
//
// CheckID must be the registered rule ID. The dispatcher resolves a
// finding's section by looking CheckID up in the check registry, and checks
// register under their rule ID ("batch/schedule-invalid"), never under a
// category ("batch"). A finding whose CheckID does not resolve is not
// classified as runtime-validation, so it never reaches the Runtime
// Validation section and is dropped by the resource-compliance copy loop,
// which only emits IDs listed in complianceCheckOrder.
//
// The category is derived for grouping the rendered section and travels in
// Extra, rather than being a second identity that could disagree with the
// first.
func (f Finding) ToCheckFinding() check.Finding {
	cf := f.Finding
	if cf.CheckID == "" {
		cf.CheckID = f.RuleID
	}
	if cf.Extra == nil {
		cf.Extra = make(map[string]string)
	}
	cf.Extra["ruleId"] = f.RuleID
	cf.Extra["ruleTitle"] = f.RuleTitle
	if cat := CategoryOf(f.RuleID); cat != "" {
		cf.Extra["category"] = cat
	}
	// Surface the upstream rule this finding corresponds to, so a reviewer
	// can see which API-server validation function rejects the manifest
	// rather than having to take the finding's word for it.
	if ref, ok := Ref(f.RuleID); ok {
		cf.Extra["upstreamRef"] = ref.Path + ":" + strings.Join(ref.Functions, ",")
	}
	return cf
}

// CheckToRegistered converts a runtime Check to a registered check.Check.
func CheckToRegistered(c Check) check.Check {
	return adapter{c: c}
}

// Check is a runtime validation check that operates on Kubernetes resources.
type Check interface {
	// ID returns the unique identifier for this check.
	ID() string
	// Title returns a human-readable title.
	Title() string
	// Blocking returns whether findings cause pipeline failure.
	Blocking() bool
	// RenderSensitive returns whether this check should run on rendered output.
	RenderSensitive() bool
	// Kinds returns the resource kinds this check applies to. An empty
	// slice means "all kinds". The adapter inverts this into the
	// check.DocSkipper contract (SkipDoc), so a check is never handed a
	// document whose kind it doesn't care about.
	Kinds() []string
	// Run validates the given document data and returns findings.
	Run(data []byte, source string) []Finding
}

type adapter struct {
	c Check
}

func (a adapter) ID() string {
	return a.c.ID()
}

func (a adapter) Title() string {
	return a.c.Title()
}

func (a adapter) Section() string {
	return "runtime-validation"
}

func (a adapter) Blocking() bool {
	return a.c.Blocking()
}

func (a adapter) Scope() check.Scope {
	return check.ScopeDoc
}

func (a adapter) RenderSensitive() bool {
	return a.c.RenderSensitive()
}

// SkipDoc implements check.DocSkipper. Check.Kinds() declares the kinds a
// check applies to, so the dispatcher skips every other kind - inverting
// applies-to into skip-if. An empty Kinds() means "applies to everything"
// and never skips.
//
// This must stay a `SkipDoc(kind string) bool` method: check.DocSkipper is
// satisfied structurally, and an adapter that merely exposes the raw kind
// slice silently satisfies nothing, causing every check to be handed every
// document.
func (a adapter) SkipDoc(kind string) bool {
	kinds := a.c.Kinds()
	if len(kinds) == 0 {
		return false
	}
	return !slices.Contains(kinds, kind)
}

// NonExemptable implements check.NonExemptable. Runtime findings describe
// manifests the API server itself rejects, so they are never suppressible.
func (a adapter) NonExemptable() bool {
	return true
}

func (a adapter) CheckDoc(data []byte, source string) []check.Finding {
	findings := a.c.Run(data, source)
	out := make([]check.Finding, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.ToCheckFinding())
	}
	return out
}
