package forge

import (
	"fmt"
	"regexp"
)

// ChecklistItem defines a single checkbox in the PR template.
type ChecklistItem struct {
	ID           string // unique identifier referenced by SelectOneGroups and Conditionals
	LabelPattern string // regex-safe label, matched against `- [x] <LabelPattern>`
}

// SelectOneGroup defines a group where exactly one option must be checked.
type SelectOneGroup struct {
	Name    string   // human-readable group name (e.g. "Change Type")
	Options []string // IDs of ChecklistItems in this group
}

// ConditionalRequire defines a conditional requirement: if the WhenID item is
// checked, all RequireIDs items must also be checked.
type ConditionalRequire struct {
	WhenID     string   // must be checked for the condition to apply
	RequireIDs []string // must all be checked when WhenID is checked
	Message    string   // error message when the condition is not met
}

// ChecklistSpec defines the PR checklist validation rules.
// An empty (zero-value) spec skips all validation.
type ChecklistSpec struct {
	Items        []ChecklistItem      // all known checkboxes (ID → LabelPattern)
	Required     []string             // IDs of items that must always be checked
	SelectOne    []SelectOneGroup     // exactly-one-of constraints
	Conditionals []ConditionalRequire // conditional dependencies
}

// ValidateChecklistString validates a checklist body against a ChecklistSpec
// without any forge API interaction.
func ValidateChecklistString(body string, spec ChecklistSpec) error {
	return validateChecklistBody(body, spec)
}

// buildCheckedSet builds a set of checked item IDs from the body.
func buildCheckedSet(body string, items []ChecklistItem) map[string]bool {
	checked := map[string]bool{}
	for _, item := range items {
		re := regexp.MustCompile(`(?im)^\s*-\s*\[x\]\s*` + regexp.QuoteMeta(item.LabelPattern))
		if re.MatchString(body) {
			checked[item.ID] = true
		}
	}
	return checked
}

// validateChecklistBody validates the body content against a ChecklistSpec.
func validateChecklistBody(body string, spec ChecklistSpec) error {
	// Empty spec = no validation.
	if len(spec.Items) == 0 && len(spec.Required) == 0 &&
		len(spec.SelectOne) == 0 && len(spec.Conditionals) == 0 {
		return nil
	}

	if body == "" {
		return fmt.Errorf("PR description is empty")
	}

	checked := buildCheckedSet(body, spec.Items)

	// Required items.
	for _, id := range spec.Required {
		if !checked[id] {
			label := labelForItem(id, spec.Items)
			return fmt.Errorf("required checkbox not checked: %s", label)
		}
	}

	// Select-one groups.
	for _, group := range spec.SelectOne {
		var selected []string
		for _, optID := range group.Options {
			if checked[optID] {
				selected = append(selected, labelForItem(optID, spec.Items))
			}
		}
		if len(selected) == 0 {
			return fmt.Errorf("no %s selected in PR description", group.Name)
		}
		if len(selected) > 1 {
			return fmt.Errorf("multiple %s selected in PR description", group.Name)
		}
	}

	// Conditional requirements.
	for _, cond := range spec.Conditionals {
		if !checked[cond.WhenID] {
			continue
		}
		for _, reqID := range cond.RequireIDs {
			if !checked[reqID] {
				if cond.Message != "" {
					return fmt.Errorf("%s", cond.Message)
				}
				return fmt.Errorf("%s requires %s", labelForItem(cond.WhenID, spec.Items), labelForItem(reqID, spec.Items))
			}
		}
	}

	return nil
}

func labelForItem(id string, items []ChecklistItem) string {
	for _, item := range items {
		if item.ID == id {
			return item.LabelPattern
		}
	}
	return id
}
