package validator

import (
	"fmt"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// ComposePRChecksSection renders PR-check results.
func ComposePRChecksSection(titleErr, signErr, checklistErr error) Section {
	var b strings.Builder
	var hasError bool
	if titleErr != nil {
		hasError = true
		fmt.Fprintf(&b, "- ❌ **PR Title** — %s\n", titleErr)
	} else {
		b.WriteString("- ✅ **PR Title**\n")
	}
	if signErr != nil {
		hasError = true
		fmt.Fprintf(&b, "- ❌ **Signed Commits** — %s\n", signErr)
	} else {
		b.WriteString("- ✅ **Signed Commits**\n")
	}
	if checklistErr != nil {
		hasError = true
		fmt.Fprintf(&b, "- ❌ **PR Checklist** — %s\n", checklistErr)
	} else {
		b.WriteString("- ✅ **PR Checklist**\n")
	}
	return Section{Name: "PR Checks", Body: b.String(), Error: hasError}
}

// ComposeLintingSection renders lint subsection.
func ComposeLintingSection(reports map[string]string) Section {
	var b strings.Builder
	var hasError bool
	for _, name := range []string{"markdownlint", "prettier", "shellcheck", "golangci", "kubeconform"} {
		out := reports[name]
		icon := "✅"
		if out != "" {
			hasError = true
			icon = "⚠️"
		}
		fmt.Fprintf(&b, "- %s **%s**\n", icon, strings.Title(name))
		if out != "" {
			fmt.Fprintf(&b, "\n```\n%s\n```\n", strings.TrimSpace(out))
		}
	}
	return Section{Name: "Linting", Body: b.String(), Error: hasError}
}

// ComposeStaticChecksSection renders static checks subsection.
func ComposeStaticChecksSection(reports map[string]string) Section {
	var b strings.Builder
	var hasError bool
	for _, name := range []string{"large-file", "YAML-syntax", "config-sort", "startingCSV", "placeholder", "cluster-identity"} {
		out := reports[name]
		icon := "✅"
		if out != "" {
			hasError = true
			icon = "⚠️"
		}
		fmt.Fprintf(&b, "- %s **%s**\n", icon, name)
		if out != "" {
			fmt.Fprintf(&b, "\n```\n%s\n```\n", strings.TrimSpace(out))
		}
	}
	return Section{Name: "Static Checks", Body: b.String(), Error: hasError}
}

// ComposeResourceComplianceSection renders generic compliance tables.
func ComposeResourceComplianceSection(findings []check.Finding) Section {
	if len(findings) == 0 {
		return Section{Name: "Resource Compliance", Body: "No compliance findings."}
	}
	var b strings.Builder
	b.WriteString("| Check | File | Message |\n| --- | --- | --- |\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", f.CheckID, f.File, f.Message)
	}
	return Section{Name: "Resource Compliance", Body: b.String(), Error: true}
}

// ComposeKyvernoSection renders the Kyverno subsection.
func ComposeKyvernoSection(body string) Section {
	if body == "" {
		return Section{Name: "Kyverno Policies", Body: "No Kyverno findings."}
	}
	return Section{Name: "Kyverno Policies", Body: body, Error: true}
}

// ComposeCINotesSection renders CI notes.
func ComposeCINotesSection(body string) Section {
	return Section{Name: "CI Notes", Body: body}
}

// RenderSubDropdown wraps a section in a nested dropdown.
func RenderSubDropdown(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n\n</details>", title, body)
}
