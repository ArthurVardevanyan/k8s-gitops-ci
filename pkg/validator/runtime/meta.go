package runtime

import (
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// Meta carries the identity a runtime check declares. Embed it in a check
// struct and the accessor half of the Check interface is satisfied:
//
//	type duplicateVolumeNamesCheck struct{ runtime.Meta }
//
// Every check previously spelled out six accessors, four of which said the
// same thing for all of them. Blocking and RenderSensitive are true for every
// check with no exception - a check that is neither does not belong in a
// family defined as always-blocking and evaluated against rendered output -
// and Category was always the ID prefix. Only the ID, title and kind list carry
// information, so only those are declared.
type Meta struct {
	// RuleID is the registered check ID, "<category>/<rule>". Findings are
	// keyed by it and reports group by its prefix, so it is the check's
	// identity, not a label.
	RuleID string
	// RuleTitle is the human-readable heading used in reports.
	RuleTitle string
	// AppliesTo lists the kinds this check is dispatched for. Nil means
	// every kind.
	AppliesTo []string
}

func (m Meta) ID() string      { return m.RuleID }
func (m Meta) Title() string   { return m.RuleTitle }
func (m Meta) Kinds() []string { return m.AppliesTo }

// Blocking is true for every runtime check. These findings describe
// manifests the API server itself rejects, so a non-blocking one would
// report a failure that is going to happen anyway and let it merge.
func (m Meta) Blocking() bool { return true }

// RenderSensitive is true for every runtime check. The API server sees the
// rendered object, so that is what these rules must be evaluated against.
func (m Meta) RenderSensitive() bool { return true }

// CategoryOf returns the report grouping for a rule ID: the segment before
// the first "/", so "volume/duplicate-volume-names" gives "volume".
//
// This is derived rather than declared because the two were identical for
// every check, and a hand-maintained copy of a value that must equal the ID
// prefix is only an opportunity for them to disagree - at which point a
// finding would file itself under a heading that matches no check.
func CategoryOf(ruleID string) string {
	if i := strings.Index(ruleID, "/"); i >= 0 {
		return ruleID[:i]
	}
	return ruleID
}

// NewFinding builds a Finding attributed to the given check.
//
// It exists so a check body states only what it found. The RuleID/RuleTitle
// pair was repeated verbatim in all 78 finding literals, and the category
// and upstream-reference metadata that reporting depends on is derived here
// rather than being something each new check has to remember to attach.
func NewFinding(c Check, f check.Finding) Finding {
	return Finding{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Finding:   f,
	}
}
