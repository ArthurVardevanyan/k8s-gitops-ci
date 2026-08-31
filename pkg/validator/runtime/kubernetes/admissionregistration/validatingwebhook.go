package admissionregistration

// validatingWebhookKinds is the Kinds() list for
// ValidatingWebhookConfiguration checks.
var validatingWebhookKinds = []string{"ValidatingWebhookConfiguration"}

// validatingWebhookBase parameterizes the shared webhook checks (see
// webhook.go) for ValidatingWebhookConfiguration. Its checks keep the
// "validating-" prefixed check IDs:
// kubernetes/admissionregistration/validating-service-invalid,
// kubernetes/admissionregistration/validating-failure-policy-invalid and
// kubernetes/admissionregistration/validating-timeout-invalid.
var validatingWebhookBase = webhookCheckBase{
	idPrefix: "validating-",
	kind:     "ValidatingWebhookConfiguration",
	kinds:    validatingWebhookKinds,
	parse:    parseValidatingWebhookConfiguration,
}
