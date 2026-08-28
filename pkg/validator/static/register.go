// Package static provides the shared check-adapter registry and the
// cluster-identity wiring seam so that other packages (e.g. the engine
// package) can access cluster metadata without importing the top-level
// validator package directly.
//
// The adapter types themselves live in register_checks.go; this file
// only exports the registration function, the ClusterIndexProvider
// seam, and the ClusterMetadata→ClusterIndex wiring helper.
package static

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/clusterid"
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
