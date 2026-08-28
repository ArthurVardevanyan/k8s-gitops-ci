package validator

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/clusterid"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes" // registers runtime validation checks
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static"
)

// ClusterIndexProvider is a seam allowing the engine to inject a ClusterIndex
// before running overlay checks. Set once at pipeline startup - nil means
// the cluster-identity check is disabled (see clusterIdentityAdapter.
// CheckOverlay), which is the correct default for a generic run with no
// org cluster-identity metadata configured.
var ClusterIndexProvider func() clusterid.ClusterIndex

// configureClusterIdentityFromProviders wires opts.Providers.ProjectIdentity()
// (the provider.Providers.ClusterMetadata seam - see pkg/provider) into
// ClusterIndexProvider for the duration of this run, bridging
// cluster.ProjectIndex to clusterid.ClusterIndex. Called once from
// RunAll. Leaves (or resets) ClusterIndexProvider to nil - disabling the
// cluster-identity check entirely, rather than running it against an
// empty index - whenever no ClusterMetadata provider is configured,
// ProjectIdentity errors, or it reports itself as not enabled: a generic
// run with no org cluster-identity plugin installed must never produce
// cluster-identity findings (including its always-on infraID-mismatch and
// invalid-JSON structural findings, which don't otherwise depend on the
// index's contents at all).
//
// Also syncs with static.ClusterIndexProvider so the check adapter
// (registered via static.RegisterAll) can read the same index.
func configureClusterIdentityFromProviders(opts Options) {
	projectIdx, knownClusters, ok, err := opts.Providers.ProjectIdentity()
	if !ok || err != nil {
		ClusterIndexProvider = nil
		static.ClusterIndexProvider = nil
		return
	}
	idx := clusterid.ClusterIndex{
		IDToCluster:     projectIdx.IDToCluster,
		NumberToCluster: projectIdx.NumberToCluster,
		KnownClusters:   knownClusters,
	}
	ClusterIndexProvider = func() clusterid.ClusterIndex { return idx }
	static.ClusterIndexProvider = ClusterIndexProvider
}

func init() {
	// kubeconform is a standalone lint step (not a check.Register entry) but
	// is registered as exemptable so EXEMPTIONS=(check=kubeconform,...) entries
	// in test.sh can suppress kubeconform validation for specific files or
	// directories (e.g. non-Kubernetes YAML that has no kind/apiVersion).
	exempt.RegisterExemptable("kubeconform")

	// Register all static check adapters from the static package.
	static.RegisterAll()
}

// psaMissingLabelsExtraKey is the check.Finding.Extra key psaCheck.CheckDoc
// (register_checks.go) uses to carry a PSA-labels finding's raw,
// comma-separated MissingLabels, so filterCommentedPSAFindings
// (psa_wiring.go) can check each missing label individually against
// psa.FindCommentedNamespaces without re-parsing the rendered message string.
//
// Kept here (rather than only in static/) so psa_wiring.go can reference it
// without importing static.
const psaMissingLabelsExtraKey = "missing_labels"
