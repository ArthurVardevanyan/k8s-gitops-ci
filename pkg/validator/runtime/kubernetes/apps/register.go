package apps

import (
	"sort"
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var registerOnce sync.Once

// allChecks is the single list of this package's checks, sorted by ID. Both
// Register and the tests derive from it so neither can drift from the other.
func allChecks() []runtime.Check {
	checks := []runtime.Check{
		newDeploymentSelectorInvalidCheck(),
		newDeploymentStrategyTypeInvalidCheck(),
		newDeploymentReplicasInvalidCheck(),
		newDeploymentMinReadySecondsInvalidCheck(),
		newStatefulSetReplicasInvalidCheck(),
		newStatefulSetPodManagementPolicyInvalidCheck(),
		newStatefulSetUpdateStrategyInvalidCheck(),
		newDaemonSetSelectorInvalidCheck(),
		newDaemonSetUpdateStrategyInvalidCheck(),
		newDaemonSetMinReadySecondsInvalidCheck(),
		newReplicaSetSelectorInvalidCheck(),
		newReplicaSetReplicasInvalidCheck(),
	}
	sort.Slice(checks, func(i, j int) bool {
		return checks[i].ID() < checks[j].ID()
	})

	return checks
}

// Register registers all apps validation checks with the check registry.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
func Register() {
	registerOnce.Do(func() {
		runtime.RegisterAll(allChecks(), upstreamRefs)
	})
}
