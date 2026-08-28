package validation

import (
	"sort"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var registerOnce sync.Once

// Register registers all apps validation checks with the check registry.
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
		for _, c := range checks {
			check.Register(runtime.CheckToRegistered(c))
		}
	})
}
