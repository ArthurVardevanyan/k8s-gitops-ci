package admissionregistration

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// admissionregistrationValidationPath is
// pkg/apis/admissionregistration/validation/validation.go in
// kubernetes/kubernetes, which holds the webhook rules ported here.
const admissionregistrationValidationPath = "pkg/apis/admissionregistration/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.37.0"

// Digests over validateMutatingWebhook and validateValidatingWebhook. The two
// functions are near-identical but not equal (only the mutating variant
// validates reinvocationPolicy), so each variant of a shared check cites and
// digests its own upstream function rather than sharing one ref.
const (
	mutatingWebhookDigest   = "sha256:a921ba2615a614386c350ad5c6f020dfca2c3acbfd5d364a5e93e8834d06141c"
	validatingWebhookDigest = "sha256:f032b44225340e7948aadc8a67e6a4154c4fe1639d1e3f30f196fa773d1ad2c9"
)

// Notes shared between the mutating and validating variants of each rule.
const (
	serviceNote = "Ports the cc.Service != nil branch of the clientConfig switch, which delegates to " +
		"webhook.ValidateWebhookService (staging/src/k8s.io/apiserver/pkg/util/webhook/validation.go); " +
		"only that function's len(name) == 0 -> field.Required branch is reproduced. The namespace, " +
		"port and path branches, and the sibling url/exactly-one-of branches, are not ported."
	failurePolicyNote = "Ports the !supportedFailurePolicies.Has(*hook.FailurePolicy) -> field.NotSupported " +
		"branch (Ignore or Fail). An absent failurePolicy is skipped, matching upstream's nil guard."
	timeoutNote = "Ports the *hook.TimeoutSeconds > 30 || *hook.TimeoutSeconds < 1 -> field.Invalid " +
		"branch (\"the timeout value must be between 1 and 30 seconds\"). An absent timeoutSeconds is " +
		"skipped, matching upstream's nil guard."
)

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
//
// The keys are the composed check IDs built by webhookCheckBase.id: the
// MutatingWebhookConfiguration variants carry no prefix and the
// ValidatingWebhookConfiguration variants carry the "validating-" prefix.
var upstreamRefs = map[string]runtime.UpstreamRef{
	// --- MutatingWebhookConfiguration --------------------------------------
	"admissionregistration/service-invalid": {
		Path:        admissionregistrationValidationPath,
		Functions:   []string{"validateMutatingWebhook"},
		Digest:      mutatingWebhookDigest,
		ValidatedAt: validatedAt,
		Note:        serviceNote,
	},
	"admissionregistration/failure-policy-invalid": {
		Path:        admissionregistrationValidationPath,
		Functions:   []string{"validateMutatingWebhook"},
		Digest:      mutatingWebhookDigest,
		ValidatedAt: validatedAt,
		Note:        failurePolicyNote,
		Additional: []runtime.UpstreamRef{{
			Path:        admissionregistrationValidationPath,
			Functions:   []string{"supportedFailurePolicies"},
			Digest:      "sha256:dd0059f1e133b857360b03a0a562deaac33cd75689305b1d1a7aec30f732497a",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"admissionregistration/timeout-invalid": {
		Path:        admissionregistrationValidationPath,
		Functions:   []string{"validateMutatingWebhook"},
		Digest:      mutatingWebhookDigest,
		ValidatedAt: validatedAt,
		Note:        timeoutNote,
	},

	// --- ValidatingWebhookConfiguration ------------------------------------
	"admissionregistration/validating-service-invalid": {
		Path:        admissionregistrationValidationPath,
		Functions:   []string{"validateValidatingWebhook"},
		Digest:      validatingWebhookDigest,
		ValidatedAt: validatedAt,
		Note:        serviceNote,
	},
	"admissionregistration/validating-failure-policy-invalid": {
		Path:        admissionregistrationValidationPath,
		Functions:   []string{"validateValidatingWebhook"},
		Digest:      validatingWebhookDigest,
		ValidatedAt: validatedAt,
		Note:        failurePolicyNote,
		Additional: []runtime.UpstreamRef{{
			Path:        admissionregistrationValidationPath,
			Functions:   []string{"supportedFailurePolicies"},
			Digest:      "sha256:dd0059f1e133b857360b03a0a562deaac33cd75689305b1d1a7aec30f732497a",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"admissionregistration/validating-timeout-invalid": {
		Path:        admissionregistrationValidationPath,
		Functions:   []string{"validateValidatingWebhook"},
		Digest:      validatingWebhookDigest,
		ValidatedAt: validatedAt,
		Note:        timeoutNote,
	},
}
