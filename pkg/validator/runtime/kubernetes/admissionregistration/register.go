package admissionregistration

import (
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

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
	checks := append(
		webhookChecks(mutatingWebhookBase),
		webhookChecks(validatingWebhookBase)...,
	)

	runtime.RegisterAll(checks, upstreamRefs)
}

// init registers this package's checks. The package is blank-imported by
// pkg/validator/runtime/kubernetes/register.go purely for this side effect.
func init() {
	Register()
}
