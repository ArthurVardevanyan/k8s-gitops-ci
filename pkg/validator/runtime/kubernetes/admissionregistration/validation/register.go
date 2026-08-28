package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Register registers every check in this package with the check registry.
//
// The six checks are the three shared webhook checks (see webhook.go)
// instantiated once for MutatingWebhookConfiguration and once for
// ValidatingWebhookConfiguration; their IDs are composed from the base's
// idPrefix, so all six composed IDs must appear as keys in upstreamRefs.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef citing the exact upstream Kubernetes function it ports;
// RegisterAll panics on a check with no valid citation.
func Register() {
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
