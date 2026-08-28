package validation

import (
	"sort"
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var registerOnce sync.Once

// Register registers all apps validation checks with the check registry.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
func Register() {
	registerOnce.Do(func() {
		checks := []runtime.Check{
			deploymentSelectorInvalidCheck{},
			deploymentStrategyTypeInvalidCheck{},
			deploymentReplicasInvalidCheck{},
			deploymentMinReadySecondsInvalidCheck{},
			statefulSetReplicasInvalidCheck{},
			statefulSetPodManagementPolicyInvalidCheck{},
			statefulSetUpdateStrategyInvalidCheck{},
			daemonSetSelectorInvalidCheck{},
			daemonSetUpdateStrategyInvalidCheck{},
			daemonSetMinReadySecondsInvalidCheck{},
			replicaSetSelectorInvalidCheck{},
			replicaSetReplicasInvalidCheck{},
		}
		sort.Slice(checks, func(i, j int) bool {
			return checks[i].ID() < checks[j].ID()
		})

		runtime.RegisterAll(checks, upstreamRefs)
	})
}
