package validator

import (
	"fmt"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// Compliance check IDs, defined as constants once rather than comparing to
// string literals throughout the codebase.
const (
	IDPodspec      = "podspec-defaults"
	IDPSALabels    = "psa-labels"
	IDNamespace    = "namespace"
	IDRBACWildcard = "rbac-wildcards"
	IDRBACReadonly = "rbac-readonly"
	IDSyncOptions  = "sync-options"
	IDCRB          = "crb"
	IDNamedPorts   = "named-ports"
	IDPlaceholder  = "placeholder"
)

// complianceCheckOrder is the fixed sub-section order for the Resource
// Compliance section. Image SHA Pinning (exempt.IDImageChecksum) is first.
// Ghost Patches and Kyverno are rendered separately by their own section
// composers, not through this ordered list.
var complianceCheckOrder = []string{
	exempt.IDImageChecksum,
	IDPodspec,
	IDNamedPorts,
	IDPSALabels,
	IDRBACWildcard,
	IDCRB,
	IDRBACReadonly,
	IDNamespace,
	IDSyncOptions,
	// placeholder and cluster-identity were previously omitted here, which
	// silently dropped their findings from the composed Resource Compliance
	// section's direct/indirect classification (phases.go only copies checks
	// present in this list) - so an "Unresolved Placeholders: N finding(s)"
	// live-log line had no matching table in the section body, and a no-diff
	// test-all run reported the section as passed while still logging the
	// warning count. Including them here makes the composed section and the
	// live per-check log agree, and surfaces the per-finding table.
	IDPlaceholder,
	exempt.IDClusterIdentity,
}

// indexOfComplianceCheck returns the sort key for id within
// complianceCheckOrder. Unknown IDs sort to the end (large sentinel).
func indexOfComplianceCheck(id string) int {
	for i, cid := range complianceCheckOrder {
		if cid == id {
			return i
		}
	}
	return 999
}

// classifyResourceCompliance splits findings by compliance check ID and
// classifies each as blocking (the specific resource was directly modified in
// this PR) or non-blocking (pre-existing, surfaced for visibility). Uses the
// resource-level attribution model via complianceAttributionCtx, so a
// finding from an overlay whose kustomization.yaml was touched but whose
// resource definition (Kind/Name) wasn't changed is correctly a warning, not
// blocking. This matches the behavior described in the CI docs: "If the
// affected resource is being modified in this PR, these issues must be
// corrected. Otherwise, these are non-blocking warnings for pre-existing
// issues."
func classifyResourceCompliance(findings []check.Finding, ctx *complianceAttributionCtx) (blockingByCheck, nonblockingByCheck map[string][]check.Finding) {
	blockingByCheck = make(map[string][]check.Finding)
	nonblockingByCheck = make(map[string][]check.Finding)

	for _, f := range findings {
		// ForcedDirect findings (from raw-source checks that override the
		// resource-level model, or from the raw fallback pass) stay blocking.
		if f.ForcedDirect {
			blockingByCheck[f.CheckID] = append(blockingByCheck[f.CheckID], f)
			continue
		}

		// Resource-level classification for render-sensitive or overlay-scope
		// findings. A finding is blocking when its specific resource was directly
		// modified in a source file that feeds this overlay's build chain.
		id := f.CheckID
		spec, ok := checkTableSpecs[id]
		if !ok || ctx == nil {
			// No table spec or no attribution ctx — use file-based fallback.
			nonblockingByCheck[id] = append(nonblockingByCheck[id], f)
			continue
		}
		resourceKey := ""
		if spec.ResourceKey != nil {
			resourceKey = spec.ResourceKey(f)
		}
		if resourceKey != "" && ctx.changedKeys != nil && ctx.changedKeys[resourceKey] != nil && isResourceAffected(resourceKey, ctx, f.File) {
			blockingByCheck[id] = append(blockingByCheck[id], f)
		} else {
			nonblockingByCheck[id] = append(nonblockingByCheck[id], f)
		}
	}

	return blockingByCheck, nonblockingByCheck
}

// checkTableSpecs defines TableSpec for each registered check.
//
// Resource-compliance checks (those with a ResourceKey) omit a raw "File"
// column: the deduped renderer (renderResourceComplianceTable) appends an
// "Overlays" column (count or single built-file) and, for blocking rows, a
// "Source File(s)" column derived via SourceKey. File-based checks
// (placeholder, cluster-identity) keep their File column and legacy rendering.
var checkTableSpecs = map[string]check.TableSpec{
	"namespace": {
		Title:    "Namespace Scope",
		Preamble: "Resources that are missing a `namespace` field or have incorrect cluster scope.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"psa-labels": {
		Title:    "PSA Namespace Labels",
		Preamble: "Namespaces missing required Pod Security Admission labels.",
		Columns: []check.Column{
			{Header: "Namespace", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Missing", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return "Namespace", f.Name },
		ResourceKey: func(f check.Finding) string { return "Namespace/" + f.Name },
	},
	"rbac-readonly": {
		Title:    "RBAC Read-Only Aggregate",
		Preamble: "ClusterRoles with the read-only aggregate label but non-readonly verbs.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"rbac-wildcards": {
		Title:    "RBAC Wildcards",
		Preamble: "RBAC rules using wildcard verbs or resources.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"crb": {
		Title:    "ClusterRoleBinding Subject Namespace",
		Preamble: "ClusterRoleBinding ServiceAccount subjects missing a namespace.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"sync-options": {
		Title:    "Argo CD Sync Options",
		Preamble: "CRD-based resources must include `SkipDryRunOnMissingResource=true` in their `argocd.argoproj.io/sync-options` annotation.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"image-checksum": {
		Title:    "Image Digest Pinning",
		Preamble: "Container images not pinned to a SHA256 digest.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Image", Cell: func(f check.Finding) string { return f.Value }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"named-ports": {
		Title:    "Named Ports",
		Preamble: "Workload, Service or Ingress resources using numeric ports.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Container", Cell: func(f check.Finding) string { return f.Container }},
			{Header: "Issue", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
	},
	"podspec-defaults": {
		Title:    "PodSpec Defaults",
		Preamble: "Pods missing required resource requests/limits or security context fields.",
		Columns: []check.Column{
			{Header: "Kind", Cell: func(f check.Finding) string { return f.Kind }},
			{Header: "Name", Cell: func(f check.Finding) string { return f.Name }},
			{Header: "Container", Cell: func(f check.Finding) string { return f.Container }},
			{Header: "Missing", Cell: func(f check.Finding) string { return f.Message }},
		},
		SourceKey:   func(f check.Finding) (string, string) { return f.Kind, f.Name },
		ResourceKey: kindNameKey,
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
