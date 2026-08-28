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
			deploymentSelectorMustMatchCheck{},
			deploymentSelectorInvalidCheck{},
			deploymentStrategyUndefinedCheck{},
			deploymentStrategyTypeInvalidCheck{},
			deploymentReplicasInvalidCheck{},
			deploymentMinReadySecondsInvalidCheck{},
			deploymentMaxUnavailableInvalidCheck{},
			deploymentMaxSurgeInvalidCheck{},
			statefulSetReplicasInvalidCheck{},
			statefulSetSelectorMustMatchCheck{},
			statefulSetPodManagementPolicyInvalidCheck{},
			statefulSetUpdateStrategyInvalidCheck{},
			statefulSetServiceNameInvalidCheck{},
			statefulSetVolumeClaimTemplatesEmptyCheck{},
			daemonSetSelectorMustMatchCheck{},
			daemonSetSelectorInvalidCheck{},
			daemonSetUpdateStrategyInvalidCheck{},
			daemonSetMinReadySecondsInvalidCheck{},
			replicaSetSelectorMustMatchCheck{},
			replicaSetSelectorInvalidCheck{},
			replicaSetReplicasInvalidCheck{},
			replicaSetRestartPolicyInvalidCheck{},
		}
		sort.Slice(checks, func(i, j int) bool {
			return checks[i].ID() < checks[j].ID()
		})
		for _, c := range checks {
			check.Register(runtime.CheckToRegistered(c))
		}
	})
}
