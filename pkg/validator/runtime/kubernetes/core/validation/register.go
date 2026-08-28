package validation

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Register registers all container, pod-spec, resource, and volume validation
// checks with the check registry.
func Register() {
	checks := []runtime.Check{
		duplicateContainerNamesCheck{},
		duplicatePortNamesCheck{},
		portNumberRangeCheck{},
		imagePullPolicyCheck{},
		mountPropagationValueCheck{},
		terminationMessagePolicyValueCheck{},
		volumeMountNameDuplicateCheck{},
		duplicateVolumeNamesCheck{},
		secretVolumeCheck{},
		configmapVolumeCheck{},
		resourceRequestsGreaterThanLimitsCheck{},
		resourceQuantityNegativeCheck{},
		podSpecRestartPolicyValueCheck{},
		podSpecDNSPolicyValueCheck{},
		podSpecTolerationOperatorValueCheck{},
		podSpecNodeSelectorInvalidCheck{},
		podSpecAffinityInvalidCheck{},
		podSpecTopologySpreadInvalidCheck{},
		podSpecServiceAccountNameInvalidCheck{},
		podSpecActiveDeadlineSecondsNegativeCheck{},
		podSpecReadinessGateInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
