package forge

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
