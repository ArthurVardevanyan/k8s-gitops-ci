package validator

import (
	"fmt"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// checkTableSpecs defines TableSpec for each registered check.
var checkTableSpecs = map[string]check.TableSpec{
	"namespace": {
		Title:    "Namespace Scope",
		Preamble: "Resources that are missing a `namespace` field or have incorrect cluster scope.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"psa-labels": {
		Title:    "PSA Namespace Labels",
		Preamble: "Namespaces missing required Pod Security Admission labels.",
		Columns: []check.Column{
			{Header: "Namespace", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Missing", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"rbac-readonly": {
		Title:    "RBAC Read-Only Aggregate",
		Preamble: "ClusterRoles with the read-only aggregate label but non-readonly verbs.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"rbac-wildcards": {
		Title:    "RBAC Wildcards",
		Preamble: "RBAC rules using wildcard verbs or resources.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"crb": {
		Title:    "ClusterRoleBinding Subject Namespace",
		Preamble: "ClusterRoleBinding ServiceAccount subjects missing a namespace.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"sync-options": {
		Title:    "Argo CD Sync Options",
		Preamble: "CRDs missing the required ServerSideApply sync annotation.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
		},
	},
	"image-checksum": {
		Title:    "Image Digest Pinning",
		Preamble: "Container images not pinned to a SHA256 digest.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Image", Cell: func(f check.Finding) string { return f.Value }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
		},
	},
	"named-ports": {
		Title:    "Named Ports",
		Preamble: "Workload, Service or Ingress resources using numeric ports.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Container", Cell: func(f check.Finding) string { return f.Container }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"podspec-defaults": {
		Title:    "PodSpec Defaults",
		Preamble: "Pods missing required resource requests/limits or security context fields.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Container", Cell: func(f check.Finding) string { return f.Container }},
			{Header: "Missing", Cell: func(f check.Finding) string { return f.Message }},
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
		},
	},
	"placeholder": {
		Title:    "Unresolved Placeholders",
		Preamble: "Files containing angle-bracket or sentinel placeholder tokens (e.g. <UPPER>, <a-b-c>, CHANGEME). AVP-scheme references (<path:...>, <vault:...>) are resolved at deploy time and are not flagged.",
		Columns: []check.Column{
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Placeholder", Cell: func(f check.Finding) string { return f.Value }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
	"cluster-identity": {
		Title:    "Cluster Identity",
		Preamble: "Overlay files referencing project IDs or cluster tokens from another cluster.",
		Columns: []check.Column{
			{Header: "File", Cell: func(f check.Finding) string { return f.File }},
			{Header: "Field", Cell: func(f check.Finding) string { return f.Path }},
			{Header: "Value", Cell: func(f check.Finding) string { return f.Value }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
	},
}

// TableSpecForCheck returns the TableSpec for a check id, if registered.
func TableSpecForCheck(id string) (check.TableSpec, bool) {
	ts, ok := checkTableSpecs[id]
	return ts, ok
}

// RenderColumnedTable renders findings for a single check as a markdown table.
func RenderColumnedTable(findings []check.Finding, id string) string {
	ts, ok := TableSpecForCheck(id)
	if !ok || len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	if ts.Preamble != "" {
		fmt.Fprintf(&b, "%s\n\n", ts.Preamble)
	}
	// Header row
	headers := make([]string, len(ts.Columns))
	seps := make([]string, len(ts.Columns))
	for i, col := range ts.Columns {
		headers[i] = col.Header
		seps[i] = "---"
	}
	fmt.Fprintf(&b, "| %s |\n| %s |\n", strings.Join(headers, " | "), strings.Join(seps, " | "))
	for _, f := range findings {
		cells := make([]string, len(ts.Columns))
		for i, col := range ts.Columns {
			cells[i] = sanitizeCell(col.Cell(f))
		}
		fmt.Fprintf(&b, "| %s |\n", strings.Join(cells, " | "))
	}
	return b.String()
}

// BuildComplianceSubs groups findings by check id and renders a sub-dropdown per check.
func BuildComplianceSubs(findings []check.Finding) string {
	// Group findings by checkID preserving order.
	order := []string{}
	grouped := map[string][]check.Finding{}
	for _, f := range findings {
		if _, ok := grouped[f.CheckID]; !ok {
			order = append(order, f.CheckID)
		}
		grouped[f.CheckID] = append(grouped[f.CheckID], f)
	}
	var b strings.Builder
	for _, id := range order {
		fs := grouped[id]
		ts, ok := TableSpecForCheck(id)
		title := id
		if ok {
			title = ts.Title
		}
		table := RenderColumnedTable(fs, id)
		if table == "" {
			continue
		}
		b.WriteString(RenderSubDropdown(
			fmt.Sprintf("%s (%d)", title, len(fs)),
			table,
		))
		b.WriteString("\n")
	}
	return b.String()
}

func sanitizeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
