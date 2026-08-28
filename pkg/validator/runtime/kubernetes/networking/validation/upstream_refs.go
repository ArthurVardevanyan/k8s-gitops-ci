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
const validatedAt = "v1.36.3"

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
		Digest:      "sha256:b1bf82dca313a3889114358245ddc8a9f9e0bd0e5da7feca2aa0b5cc702cb9ed",
		ValidatedAt: validatedAt,
		Note: "Ports the endPort branches: *port.EndPort < port.Port.IntVal (\"must be greater than or equal " +
			"to `port`\") and endPort set with no port (\"may not be specified when `port` is not " +
			"specified\"). The port-number-range and named-port branches are not ported.",
	},
	"network-policy/protocol-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"ValidateNetworkPolicyPort"},
		Digest:      "sha256:b1bf82dca313a3889114358245ddc8a9f9e0bd0e5da7feca2aa0b5cc702cb9ed",
		ValidatedAt: validatedAt,
		Note:        "Ports the protocol field.NotSupported branch (TCP, UDP or SCTP) only; the port/endPort branches of the same function are covered by network-policy/port-range-invalid.",
	},

	// --- Service -----------------------------------------------------------
	"service/type-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateService"},
		Digest:      "sha256:dda4013238c5d4cc19046be1a5fdfa8ae764452fdfadac074d1b49b436144674",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedServiceType.Has(service.Spec.Type) -> field.NotSupported branch. Upstream additionally reports an empty type as Required; this check skips empty because defaulting supplies ClusterIP.",
	},
	"service/session-affinity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateService"},
		Digest:      "sha256:dda4013238c5d4cc19046be1a5fdfa8ae764452fdfadac074d1b49b436144674",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedSessionAffinityType.Has(service.Spec.SessionAffinity) -> field.NotSupported branch (ClientIP or None). Upstream additionally reports an empty sessionAffinity as Required; this check skips empty because defaulting supplies None. The sessionAffinityConfig branches are not ported.",
	},
}
