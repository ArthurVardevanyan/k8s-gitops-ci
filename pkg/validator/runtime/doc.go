// Package runtime provides validation checks that mirror what the Kubernetes
// API server rejects at admission time but that an OpenAPI schema cannot
// express, so kubeconform alone cannot catch them.
//
// This package holds only the shared machinery; the checks themselves live in
// the per-API-group subpackages under kubernetes/.
//
//   - walker.go — extracts a PodSpec and its container list from any
//     pod-bearing kind, with field paths for accurate finding locations.
//   - finding.go — the Finding type and the adapter that exposes a Check to
//     the check registry, including kind filtering via Kinds/SkipDoc and the
//     NonExemptable marker.
//   - upstream.go — the UpstreamRef citation type and RegisterAll, which
//     panics if a check is registered without a valid citation.
//
// Checks parse manifests with the k8s.io/api typed structs and reimplement the
// upstream rule rather than importing k8s.io/kubernetes, which is not a usable
// library dependency.
//
// Because this family is always-blocking and non-exemptable, every check must
// cite the exact upstream function it ports via an UpstreamRef in its
// package's upstreamRefs table. The citation records a file path, the function
// names, and a digest of their normalized bodies at a pinned Kubernetes tag,
// so `task verify:upstream-refs` fails when upstream changes and the port has
// not been re-reviewed. See docs/CI.md for the full standard and for the
// known version-skew and feature-gate limitations.
package runtime
