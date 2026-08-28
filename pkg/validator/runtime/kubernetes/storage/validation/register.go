package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Register registers every check in this package with the check registry.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
func Register() {
	checks := []runtime.Check{
		// StorageClass.
		scProvisionerInvalidCheck{},
		scReclaimPolicyInvalidCheck{},
		scVolumeBindingModeInvalidCheck{},
		scAllowedTopologyRangeInvalidCheck{},

		// PersistentVolume.
		pvAccessModesInvalidCheck{},
		pvCapacityInvalidCheck{},

		// PersistentVolumeClaim.
		pvcAccessModesInvalidCheck{},
		pvcVolumeModeInvalidCheck{},
	}

	runtime.RegisterAll(checks, upstreamRefs)
}

// init registers this package's checks. The package is blank-imported by
// pkg/validator/runtime/kubernetes/register.go purely for this side effect.
func init() {
	Register()
}
