package admissionregistration

// mutatingWebhookKinds is the Kinds() list for MutatingWebhookConfiguration
// checks.
var mutatingWebhookKinds = []string{"MutatingWebhookConfiguration"}

// mutatingWebhookBase parameterizes the shared webhook checks (see webhook.go)
// for MutatingWebhookConfiguration. Its checks keep the un-prefixed check IDs:
// admissionregistration/service-invalid,
// admissionregistration/failure-policy-invalid and
// admissionregistration/timeout-invalid.
var mutatingWebhookBase = webhookCheckBase{
	idPrefix: "",
	kind:     "MutatingWebhookConfiguration",
	kinds:    mutatingWebhookKinds,
	parse:    parseMutatingWebhookConfiguration,
}
