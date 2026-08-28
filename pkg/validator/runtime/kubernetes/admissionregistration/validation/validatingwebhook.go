package validation

// validatingWebhookKinds is the Kinds() list for
// ValidatingWebhookConfiguration checks.
var validatingWebhookKinds = []string{"ValidatingWebhookConfiguration"}

// validatingWebhookBase parameterizes the shared webhook checks (see
// webhook.go) for ValidatingWebhookConfiguration. Its checks keep the
// "validating-" prefixed check IDs:
// admissionregistration/validating-service-invalid,
// admissionregistration/validating-failure-policy-invalid and
// admissionregistration/validating-timeout-invalid.
var validatingWebhookBase = webhookCheckBase{
	idPrefix: "validating-",
	kind:     "ValidatingWebhookConfiguration",
	kinds:    validatingWebhookKinds,
	parse:    parseValidatingWebhookConfiguration,
}
