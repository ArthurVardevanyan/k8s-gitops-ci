package validation

import (
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// init registers this package's checks when it is (blank) imported, matching
// every other runtime validation subpackage.
func init() {
	Register()
}

var registerOnce sync.Once

// Register registers every check in this package with the check registry.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
//
// It is guarded by sync.Once so that a caller invoking it after the init()
// above cannot trip check.Register's duplicate-ID panic.
func Register() {
	registerOnce.Do(register)
}

func register() {
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
		objectMetaNameInvalidCheck{},
		objectMetaNamespaceInvalidCheck{},

		// Core object checks (ConfigMap, LimitRange, ResourceQuota).
		configMapDataSizeExceededCheck{},
		limitRangeMaxMinInvalidCheck{},
		resourceQuotaHardInvalidCheck{},
		resourceQuotaHardNegativeCheck{},
	}

	runtime.RegisterAll(checks, upstreamRefs)
}
