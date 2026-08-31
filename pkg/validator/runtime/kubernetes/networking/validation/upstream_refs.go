package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// networkingValidationPath is pkg/apis/networking/validation/validation.go in
// kubernetes/kubernetes, which holds the Ingress and NetworkPolicy rules.
const networkingValidationPath = "pkg/apis/networking/validation/validation.go"

// coreValidationPath is pkg/apis/core/validation/validation.go. Service is a
// core type, so its validation lives there rather than in the networking API
// group.
const coreValidationPath = "pkg/apis/core/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.37.0"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	// --- Ingress -----------------------------------------------------------
	"ingress/path-type-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"validateHTTPIngressPath"},
		Digest:      "sha256:1f1d5b2706899fa638f06ca9d47e43fdb88a5ba0954d6da02c49f01dfe5126bd",
		ValidatedAt: validatedAt,
		Note: "Ports the default: field.NotSupported branch of the pathType switch. Upstream additionally " +
			"reports a nil pathType as Required; this check skips absent/empty values because defaulting " +
			"supplies ImplementationSpecific and unrendered manifests legitimately omit it. The path-shape " +
			"branches (absolute path, invalid sequences/suffixes) are not ported.",
		Additional: []runtime.UpstreamRef{{
			Path:        networkingValidationPath,
			Functions:   []string{"supportedPathTypes"},
			Digest:      "sha256:321ec83f450fb296ec5701ae0fbb586889850c16f0a6a15909dc81ea6b54454b",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},

	// --- NetworkPolicy -----------------------------------------------------
	"network-policy/policy-type-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"ValidateNetworkPolicySpec"},
		Digest:      "sha256:8c5cb74a1b8b6305913af7959b4617b8fae62d8dbb507aa48ad06b14103b2cea",
		ValidatedAt: validatedAt,
		Note:        "Ports the !allowed.Has(string(pType)) -> field.NotSupported branch on spec.policyTypes. The \"may not specify more than two policyTypes\" branch and the podSelector/peer branches are not ported.",
	},
	"network-policy/port-range-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"ValidateNetworkPolicyPort"},
		Digest:      "sha256:170d64ffe974b384932f1c67058e0acc0f2feb3cbad540924a90b500f9859996",
		ValidatedAt: validatedAt,
		Note: "Ports the three endPort-pairing branches: *port.EndPort < port.Port.IntVal (\"must be " +
			"greater than or equal to `port`\"), endPort set with no port (\"may not be specified when " +
			"`port` is not specified\"), and endPort set with a named port (\"may not be specified when " +
			"`port` is non-numeric\"). Not ported: the two IsValidPortNum branches, on port and on " +
			"endPort, and the IsValidPortName branch on a named port. No other check covers those " +
			"three, so a NetworkPolicy port outside 1-65535 is currently unreported.",
	},
	"network-policy/protocol-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"ValidateNetworkPolicyPort"},
		Digest:      "sha256:170d64ffe974b384932f1c67058e0acc0f2feb3cbad540924a90b500f9859996",
		ValidatedAt: validatedAt,
		Note: "Ports the protocol field.NotSupported branch (TCP, UDP or SCTP) only. " +
			"network-policy/port-range-invalid covers the endPort-pairing branches of the same " +
			"function, but not its IsValidPortNum or IsValidPortName branches, which no check covers.",
	},

	// --- Service -----------------------------------------------------------
	"service/type-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateService"},
		Digest:      "sha256:dda4013238c5d4cc19046be1a5fdfa8ae764452fdfadac074d1b49b436144674",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedServiceType.Has(service.Spec.Type) -> field.NotSupported branch. Upstream additionally reports an empty type as Required; this check skips empty because defaulting supplies ClusterIP.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedServiceType"},
			Digest:      "sha256:8902d9b5c9e3a6acc7f51200a2795e13980426892388637f4e28cc994fd06f97",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"service/session-affinity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateService"},
		Digest:      "sha256:dda4013238c5d4cc19046be1a5fdfa8ae764452fdfadac074d1b49b436144674",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedSessionAffinityType.Has(service.Spec.SessionAffinity) -> field.NotSupported branch (ClientIP or None). Upstream additionally reports an empty sessionAffinity as Required; this check skips empty because defaulting supplies None. The sessionAffinityConfig branches are not ported.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedSessionAffinityType"},
			Digest:      "sha256:2ace1c89396d3d8c0a441277701b8fc47d9151e590be10e4a8a85ff86d7b2c09",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
}
