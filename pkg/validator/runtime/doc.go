// Package runtime provides validation checks that mirror what the cluster
// itself rejects but that an OpenAPI schema cannot express, so kubeconform
// alone cannot catch them. Most checks in this family port the Kubernetes
// API server's own admission-time validation (see the kubernetes/
// subpackage); a check may instead port or call a different upstream
// project's code where that project's own controller is the thing that
// rejects the manifest - e.g. k8scni/net-attach-def/ovn-netconf-invalid ports
// OVN-Kubernetes' network-controller validation, which rejects a
// NetworkAttachmentDefinition after admission rather than during it. What
// unifies the family is not "the API server admission path" specifically,
// but that every check cites a specific, verified upstream function (see
// upstream.go) for a rule that really is enforced somewhere the manifest
// will actually run - never this tool's own invented policy.
//
// This package holds only the shared machinery; the checks themselves live in
// per-upstream-family subpackages such as kubernetes/ (kubernetes/kubernetes'
// own API groups) and k8scni/ (the k8s.cni.cncf.io CRD and, for its OVN tier,
// ovn-kubernetes).
//
//   - walker.go — extracts a PodSpec and its container list from any
//     pod-bearing kind, with field paths for accurate finding locations.
//   - finding.go — the Finding type and the adapter that exposes a Check to
//     the check registry, including kind filtering via Kinds/SkipDoc and the
//     NonExemptable marker.
//   - upstream.go — the UpstreamRef citation type and RegisterAll, which
//     panics if a check is registered without a valid citation.
//
// Checks parse manifests with typed structs and reimplement the upstream rule
// (RefKindRewrite) or call the upstream code directly (RefKindImport) rather
// than importing kubernetes/kubernetes's own pkg/apis/*/validation, which is
// not a usable library dependency.
//
// Because this family is always-blocking and non-exemptable, every check must
// cite the exact upstream function it ports or calls via an UpstreamRef in
// its package's upstreamRefs table. For a RefKindRewrite citation, the
// citation records a file path, the function names, and a digest of their
// normalized bodies at a pinned upstream version, so `task verify:upstream-
// refs` fails when upstream changes and the port has not been re-reviewed;
// for a RefKindImport citation, the compiler and go.mod already verify
// exactly what runs, so only the file/functions are confirmed to still
// exist. See docs/CI.md for the full standard and for the known
// version-skew and feature-gate limitations.
package runtime
