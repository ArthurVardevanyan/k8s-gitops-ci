package validation

import "sync"

var registerOnce sync.Once

// init registers all batch (Job, CronJob) validation checks with the check
// registry. Both Register and registerCronJob are invoked here: this package
// is blank-imported by pkg/validator/runtime/kubernetes/register.go purely for
// this side effect, so without an init() none of the batch checks would ever
// reach the registry.
func init() {
	registerOnce.Do(func() {
		Register()
		registerCronJob()
	})
}
