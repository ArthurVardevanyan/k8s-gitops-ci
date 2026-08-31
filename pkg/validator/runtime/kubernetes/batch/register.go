package batch

import (
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var registerOnce sync.Once

// Register registers every batch (Job and CronJob) validation check with the
// check registry, exactly once.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
func Register() {
	registerOnce.Do(func() {
		checks := []runtime.Check{
			// Job (pkg/apis/batch/validation validateJobSpec).
			newParallelismInvalidCheck(),
			newBackoffLimitInvalidCheck(),

			// CronJob (validateCronJobSpec and the helpers it calls).
			newScheduleInvalidCheck(),
			newConcurrencyPolicyInvalidCheck(),
			newFailedJobsHistoryLimitInvalidCheck(),
			newSuccessfulJobsHistoryLimitInvalidCheck(),
			newStartingDeadlineSecondsInvalidCheck(),
		}

		runtime.RegisterAll(checks, upstreamRefs)
	})
}

// init registers all batch validation checks. This package is blank-imported
// by pkg/validator/runtime/kubernetes/register.go purely for this side effect,
// so without an init() none of the batch checks would ever reach the registry.
func init() {
	Register()
}
