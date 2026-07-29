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

// ComposeKustomizeBuildSection renders the Kustomize Build section.
func ComposeKustomizeBuildSection(overlayCount int, buildErrors map[string]string, hookLines []string, fixNeeded []string) Section {
	var b strings.Builder
	hasError := false

	// Overlay build summary
	if len(buildErrors) == 0 {
		fmt.Fprintf(&b, "- ✅ **Overlay Build** — %d overlay(s) built successfully\n", overlayCount)
	} else {
		hasError = true
		b.WriteString("- ❌ **Overlay Build**\n")
		for ov, msg := range buildErrors {
			fmt.Fprintf(&b, "\n%s\n", RenderSubDropdown(ov, "```\n"+strings.TrimSpace(msg)+"\n```"))
		}
	}

	// Hook results
	if len(hookLines) > 0 {
		b.WriteString(RenderSubDropdown("Hook Results", strings.Join(hookLines, "\n")))
		b.WriteString("\n")
	} else {
		b.WriteString("- ✅ **Hooks** — no hooks or all passed\n")
	}

	// Kustomize fix
	if len(fixNeeded) > 0 {
		hasError = true
		b.WriteString("- ❌ **Kustomize Fix** — the following files need `kustomize edit fix`:\n")
		for _, f := range fixNeeded {
			fmt.Fprintf(&b, "  - `%s`\n", f)
		}
	} else {
		b.WriteString("- ✅ **Kustomize Fix** — all kustomization.yaml files are up to date\n")
	}

	return Section{Name: "Kustomize Build", Body: b.String(), Error: hasError}
}

// ComposeScaffoldValidationSection renders scaffold validation results.
func ComposeScaffoldValidationSection(driftSummary string, execErrors []string, missingClusters []string) Section {
	var b strings.Builder
	hasError := false

	// Drift
	if driftSummary == "" {
		b.WriteString("- ✅ **Scaffold Drift** — no drift detected\n")
	} else {
		hasError = true
		b.WriteString("- ❌ **Scaffold Drift**\n\n")
		b.WriteString(RenderSubDropdown("Drift Details", driftSummary))
		b.WriteString("\n")
	}

	// Exec errors
	if len(execErrors) == 0 {
		b.WriteString("- ✅ **Scaffold Exec** — all scaffold runs succeeded\n")
	} else {
		hasError = true
		b.WriteString("- ❌ **Scaffold Exec**\n")
		for _, e := range execErrors {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}

	// Missing clusters
	if len(missingClusters) == 0 {
		b.WriteString("- ✅ **Cluster Coverage** — all clusters accounted for\n")
	} else {
		hasError = true
		b.WriteString("- ❌ **Missing Clusters**\n")
		for _, c := range missingClusters {
			fmt.Fprintf(&b, "  - `%s`\n", c)
		}
	}

	return Section{Name: "Scaffold Validation", Body: b.String(), Error: hasError}
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
