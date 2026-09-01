// Package static holds the static-analysis validator family: engines that
// judge a manifest from its own content alone, with no cluster/runtime
// admission semantics involved (unlike pkg/validator/runtime, whose checks
// port what the cluster itself - the API server or, for k8scni, a project
// like OVN-Kubernetes - would reject).
//
// Membership in this package is not the same as being check.Register-driven:
// nad (the NetworkAttachmentDefinition non-blocking advisories) lives here
// too, even though it is a separate always-on validator over rendered
// overlay output, not part of the check.Register registry - see
// docs/CI.md#networkattachmentdefinition-nad-validation for why (they
// correspond to no specific upstream function, so they do not belong in the
// citation-required runtime family, but they are still static analysis).
//
// This file also provides the shared check-adapter registry and the
// cluster-identity wiring seam so that other packages (e.g. the engine
// package) can access cluster metadata without importing the top-level
// validator package directly. clusterid lives here for the same reason as
// the other engines: it is check.Register-driven (via ClusterIdentityAdapter,
// registered below), exactly like the other 9 engines in this package -
// it previously sat top-level as pkg/validator/clusterid despite that,
// which was the second (and equally unjustified) exception to "static/ is
// the registered-engine package" alongside nad's real one, before this
// package's scope was redefined to the description above.
//
// The adapter types themselves live in register_checks.go; this file
// only exports the registration function, the ClusterIndexProvider
// seam, and the ClusterMetadata→ClusterIndex wiring helper.
package static

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/clusterid"
)

// RegisterAll registers all static check adapters with the check registry.
func RegisterAll() {
	check.Register(NamespaceCheck{})
	check.Register(PSACheck{})
	check.Register(RBACReadonlyCheck{})
	check.Register(RBACWildcardCheck{})
	check.Register(CRBCheck{})
	check.Register(SyncOptsCheck{})
	check.Register(ImageCheck{})
	check.Register(ImageFQDNCheck{})
	check.Register(NamedPortCheck{})
	check.Register(PodspecCheck{})
	check.Register(PlaceholderCheck{})
	check.Register(ClusterIdentityAdapter{})
}

// ClusterIndexProvider is a seam allowing the engine to inject a ClusterIndex
// before running overlay checks. Set once at pipeline startup - nil means
// the cluster-identity check is disabled (see clusterIdentityAdapter.
// CheckOverlay), which is the correct default for a generic run with no
// org cluster-identity metadata configured.
var ClusterIndexProvider func() clusterid.ClusterIndex
