package policy

import (
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// init registers all policy (PodDisruptionBudget) validation checks with the
// check registry. This package is blank-imported by
// pkg/validator/runtime/kubernetes/register.go purely for this side effect,
// so without an init() calling Register() none of its checks would ever
// reach the registry - their unit tests would still pass in isolation while
// the pipeline validated nothing.
func init() {
	Register()
}

var registerOnce sync.Once

// Register registers all PodDisruptionBudget validation checks with the
// check registry.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
//
// It is guarded by sync.Once so that a caller invoking it after the init()
// in register.go cannot trip check.Register's duplicate-ID panic.
func Register() {
	registerOnce.Do(register)
}

func register() {
	checks := []runtime.Check{
		newSelectorInvalidCheck(),
		newMinAvailableInvalidCheck(),
		newMaxUnavailableInvalidCheck(),
		newMinAndMaxSpecifiedCheck(),
	}

	runtime.RegisterAll(checks, upstreamRefs)
}
