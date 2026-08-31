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
		Digest:      "sha256:b080f2e353f20f1fc91b9e31b93c664238b8f70683cd8a3816441e67c266d43b",
		ValidatedAt: validatedAt,
		Note: "Ports the default: field.NotSupported branch of the pathType switch. Upstream additionally " +
			"reports a nil pathType as Required. Deliberate divergence: an omitted pathType is not " +
			"reported, because an unrendered manifest legitimately omits it. An explicitly-empty " +
			"pathType IS reported: networking.k8s.io/v1 has no pathType defaulter - only the legacy " +
			"v1beta1 one does, and it guards on nil - so \"\" reaches the switch as a non-nil pointer " +
			"and upstream returns NotSupported. The path-shape branches (absolute path, invalid " +
			"sequences/suffixes) are not ported.",
		Additional: []runtime.UpstreamRef{{
			Path:        networkingValidationPath,
			Functions:   []string{"supportedPathTypes"},
			Digest:      "sha256:cca5ee444bcc0489ce31d3bfebf981dcf6c1b656615575b2703d924f34bc5e4d",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},

	// --- NetworkPolicy -----------------------------------------------------
	"network-policy/policy-type-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"ValidateNetworkPolicySpec"},
		Digest:      "sha256:8d49962ef76cfba94b53e58a687cc3d56d9b67512ae1b730ea91afcef6f798e0",
		ValidatedAt: validatedAt,
		Note:        "Ports the !allowed.Has(string(pType)) -> field.NotSupported branch on spec.policyTypes. The \"may not specify more than two policyTypes\" branch and the podSelector/peer branches are not ported.",
	},
	"network-policy/port-range-invalid": {
		Path:        networkingValidationPath,
		Functions:   []string{"ValidateNetworkPolicyPort"},
		Digest:      "sha256:44de03ec43f9534a14917e64cf75a17ed94dd8b5c726781610e0969d3f5b5132",
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
		Digest:      "sha256:44de03ec43f9534a14917e64cf75a17ed94dd8b5c726781610e0969d3f5b5132",
		ValidatedAt: validatedAt,
		Note: "Ports the protocol field.NotSupported branch (TCP, UDP or SCTP) only. " +
			"network-policy/port-range-invalid covers the endPort-pairing branches of the same " +
			"function, but not its IsValidPortNum or IsValidPortName branches, which no check covers.",
	},

	// --- Service -----------------------------------------------------------
	"service/type-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateService"},
		Digest:      "sha256:021ecdc235c0d6920a1f8e94a91d9e4e62f6b572fbf554c5a1158ad4243b3149",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedServiceType.Has(service.Spec.Type) -> field.NotSupported branch. Upstream additionally reports an empty type as Required; this check skips empty because defaulting supplies ClusterIP.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedServiceType"},
			Digest:      "sha256:45b41734a8b04054f17cb3288bdfbef297f3f1e367776a7621e819b8af82b234",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"service/session-affinity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateService"},
		Digest:      "sha256:021ecdc235c0d6920a1f8e94a91d9e4e62f6b572fbf554c5a1158ad4243b3149",
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
