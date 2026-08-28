package validation

import "sync"

var registerOnce sync.Once

// init registers all policy (PodDisruptionBudget) validation checks with the
// check registry. This package is blank-imported by
// pkg/validator/runtime/kubernetes/register.go purely for this side effect,
// so without an init() calling Register() none of its checks would ever
// reach the registry - their unit tests would still pass in isolation while
// the pipeline validated nothing.
func init() {
	registerOnce.Do(Register)
}
