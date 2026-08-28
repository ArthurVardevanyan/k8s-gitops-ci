package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// coreValidationPath is pkg/apis/core/validation/validation.go in
// kubernetes/kubernetes, which holds the bulk of the rules ported here.
const coreValidationPath = "pkg/apis/core/validation/validation.go"

// objectMetaPath is apimachinery's shared object-metadata validation, applied
// by the API server to every object regardless of kind.
const objectMetaPath = "staging/src/k8s.io/apimachinery/pkg/api/validation/objectmeta.go"

// labelsPath is apimachinery's meta/v1 label validation, which the shared
// object-metadata path above delegates to for metadata.labels.
const labelsPath = "staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken
// at. It matches the tag derived from go.mod that
// `task verify:upstream-refs` pins to.
const validatedAt = "v1.37.0"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	// --- container ---------------------------------------------------------
	"container/name-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerCommon"},
		Digest:      "sha256:a4dbd3861f90d8ae24bebdabb6d2839506dc0d027f470059c534c0ead8d344be",
		ValidatedAt: validatedAt,
		Note: "Ports the namePath branch: an absent name is Required and a present one must pass " +
			"ValidateDNS1123Label. The two are mutually exclusive upstream and are kept that way here. " +
			"The rest of validateContainerCommon (image, lifecycle, probes, resources) is not ported.",
	},

	"container/port-name-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:1e4b6749f5bdbb16b08c779591cf18d912380cbc1300b2c11c5a0aaa6ca1213f",
		ValidatedAt: validatedAt,
		Note: "Ports the IsValidPortName branch only, which upstream applies solely when a port name is " +
			"set, since an unnamed port is legal. The Duplicate branch of the same function is covered " +
			"by container/duplicate-port-names, and the containerPort/hostPort/protocol branches by " +
			"container/port-number-range.",
	},

	"container/duplicate-container-names": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainers"},
		Digest:      "sha256:a27afba15f49a693820862049de0c5e4f40381f6de3e388649343b32b5a3c924",
		ValidatedAt: validatedAt,
		Note: "Ports the allNames/field.Duplicate branch. Upstream splits regular, init " +
			"and ephemeral containers across validateContainers, validateInitContainers " +
			"and validateEphemeralContainers to avoid duplicate messages; this check " +
			"walks all container lists at once and reports each colliding name once.",
	},
	"container/duplicate-port-names": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:1e4b6749f5bdbb16b08c779591cf18d912380cbc1300b2c11c5a0aaa6ca1213f",
		ValidatedAt: validatedAt,
		Note:        "Ports the allNames duplicate-port-name branch only; the sibling rules in the same function are covered by container/port-number-range.",
	},
	"container/port-number-range": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:1e4b6749f5bdbb16b08c779591cf18d912380cbc1300b2c11c5a0aaa6ca1213f",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidatePortNumOrName/containerPort range branch only; the duplicate-name branch is covered by container/duplicate-port-names.",
	},
	"container/image-pull-policy": {
		Path:        coreValidationPath,
		Functions:   []string{"validatePullPolicy"},
		Digest:      "sha256:863a3c3337c6ca5d02f13c9120f634fa2ae6aa2efb894c550e454d28d8f5f808",
		ValidatedAt: validatedAt,
		Note:        "Upstream additionally requires the field (empty is Required); this check skips empty because defaulting fills it in before validation and unrendered manifests legitimately omit it.",
	},
	"container/mount-propagation-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateMountPropagation"},
		Digest:      "sha256:74269abe2122561a5f2b0b3650c750a84e677edfea428c279c5007df28f81d46",
		ValidatedAt: validatedAt,
		Note:        "Ports the supported-value branch only. The Bidirectional-requires-privileged branch is not ported: it is a cross-field rule against the container securityContext.",
	},
	"container/termination-message-policy-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerCommon"},
		Digest:      "sha256:a4dbd3861f90d8ae24bebdabb6d2839506dc0d027f470059c534c0ead8d344be",
		ValidatedAt: validatedAt,
		Note:        "Ports the terminationMessagePolicy switch. The empty case is skipped rather than reported Required, because defaulting supplies it.",
	},
	"container/volume-mount-name-duplicate": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateVolumeMounts", "IsMatchedVolume"},
		Digest:      "sha256:557c21473e961112901aabff3714918e08bd449639d6c5685e30929a2bd389c2",
		ValidatedAt: validatedAt,
		Note:        "Ports the !IsMatchedVolume -> field.NotFound branch (a volumeMount must name a declared volume).",
	},

	// --- pod spec ----------------------------------------------------------
	"pod-spec/restart-policy-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateRestartPolicy"},
		Digest:      "sha256:bd9dd69734808b0f2de5c957e16c41adf51706310e3196757aeb21d829af6768",
		ValidatedAt: validatedAt,
	},
	"pod-spec/dns-policy-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateDNSPolicy"},
		Digest:      "sha256:647b87b7449ee64adc1503341952293477e04d4bc70b103cad702bf01a14b2d5",
		ValidatedAt: validatedAt,
	},
	"pod-spec/toleration-operator-value": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateTolerations"},
		Digest:      "sha256:26ba60c837f145eb446ab5cc88f2f24e3ac93b913527369bebd1fa1a3752c9df",
		ValidatedAt: validatedAt,
		Note:        "Ports the operator supported-value branch only; the key/value/effect branches of the same function are not ported.",
	},
	"pod-spec/affinity-node-selector-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePodSpec"},
		Digest:      "sha256:dc7bb989a2aa5e7bccc0b2dcef021764e0fc18f6f7e7338d223ded13048c307f",
		ValidatedAt: validatedAt,
		Note:        "Ports the unversionedvalidation.ValidateLabels(spec.NodeSelector, ...) call in ValidatePodSpec: nodeSelector keys must be qualified names and values valid label values.",
	},
	"pod-spec/pod-affinity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateAffinity", "ValidateNodeSelectorRequirement", "ValidatePodAffinityTermSelector"},
		Digest:      "sha256:80ffe01c4d97b46a6bb2b5d17ff0955a43bbcb7662e5205be33cb71ee126b4ec",
		ValidatedAt: validatedAt,
		Note:        "Ports the label-key/label-value branches reached from validateAffinity: required nodeAffinity matchExpressions/matchFields keys, and pod (anti-)affinity labelSelector keys and values.",
	},
	"pod-spec/topology-spread-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateTopologySpreadConstraints"},
		Digest:      "sha256:6e495685a8d1a7f0dc55ff9ae093bef92b4dfbdedba7521e0e9ee9fcfb8bfd8f",
		ValidatedAt: validatedAt,
		Note:        "Ports the labelSelector validation branch only; maxSkew/whenUnsatisfiable/topologyKey/minDomains branches are not ported.",
	},
	"pod-spec/service-account-name-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePodSpec"},
		Digest:      "sha256:dc7bb989a2aa5e7bccc0b2dcef021764e0fc18f6f7e7338d223ded13048c307f",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateServiceAccountName(spec.ServiceAccountName, false) call in ValidatePodSpec. ValidateServiceAccountName is an alias for apimachinery NameIsDNSSubdomain (generic.go), i.e. IsDNS1123Subdomain.",
	},
	"pod-spec/active-deadline-seconds-negative": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePodSpec"},
		Digest:      "sha256:dc7bb989a2aa5e7bccc0b2dcef021764e0fc18f6f7e7338d223ded13048c307f",
		ValidatedAt: validatedAt,
		Note:        "Ports the activeDeadlineSeconds InclusiveRangeError(1, math.MaxInt32) branch in ValidatePodSpec. Only the lower bound is reported here; the upper bound is unreachable through int32 decoding.",
	},
	"pod-spec/readiness-gate-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateReadinessGates"},
		Digest:      "sha256:757173fa3badacbac8c8bd02e74106863c1710e459574c1320d215bc18722c88",
		ValidatedAt: validatedAt,
	},

	// --- resources ---------------------------------------------------------
	"resources/resource-requests-greater-than-limits": {
		Path:        coreValidationPath,
		Functions:   []string{"validateResourceRequirements"},
		Digest:      "sha256:9661ff4744e2decbcc616e53c290a5f85574822c5a48e3aa05d4589a0eaf4308",
		ValidatedAt: validatedAt,
		Note:        "Ports the \"must be less than or equal to <resource> limit\" branch only. The stricter must-be-equal branch for non-overcommittable resources and the Required-limit branch are not ported, since both depend on helper.IsOvercommitAllowed.",
	},
	"resources/resource-quantity-negative": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateResourceQuantityValue", "ValidateNonnegativeQuantity"},
		Digest:      "sha256:09f5e2fd561041a660d4987328c7a228b5918bdc91e16e0557f0a77d97d2743f",
		ValidatedAt: validatedAt,
		Note:        "Ports ValidateNonnegativeQuantity, reached from validateResourceRequirements for every entry of resources.requests and resources.limits. The integer-resource branch of ValidateResourceQuantityValue is not ported.",
	},

	// --- volume ------------------------------------------------------------
	"volume/duplicate-volume-names": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateVolumes"},
		Digest:      "sha256:24672f720f50d50fcf7f45b1a24a2fc36b995bbffa4b731e45479e983ff1b0c4",
		ValidatedAt: validatedAt,
		Note:        "Ports the allNames/field.Duplicate branch on spec.volumes[].name.",
	},
	"volume/secret-name-required": {
		Path:        coreValidationPath,
		Functions:   []string{"validateSecretVolumeSource"},
		Digest:      "sha256:fc0888d9a77954b5901bfa011532f80363e4c757bbec470c93a37510f7f029c4",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(secretSource.SecretName) == 0 -> field.Required branch. It is a required-field rule, not a live lookup: whether the Secret exists in the cluster cannot be checked from a manifest.",
	},
	"volume/configmap-name-required": {
		Path:        coreValidationPath,
		Functions:   []string{"validateConfigMapVolumeSource"},
		Digest:      "sha256:a0573b41524df4ef32f8d50e05570cb3b64a567ee6c6ff957cda243186ec18cc",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(configMapSource.Name) == 0 -> field.Required branch. As with volume/secret-name-required this is a required-field rule, not a live lookup.",
	},

	// --- object metadata ---------------------------------------------------
	"core/object-meta-name-invalid": {
		Path:        objectMetaPath,
		Functions:   []string{"ValidateObjectMetaAccessor"},
		Digest:      "sha256:1ee0006f3c2e88f2728e5641f03cd5282af542416a87b2f879c7dffc6dfb3bfd",
		ValidatedAt: validatedAt,
		Note: "Ports the name/generateName half: generateName is validated with prefix=true whenever set, " +
			"and a name is required only when generateName is absent. The per-kind nameFn is resolved from " +
			"the kind's upstream call site and evaluates to apimachinery NameIsDNSSubdomain / NameIsDNSLabel " +
			"(generic.go, including maskTrailingDash) or content.IsPathSegmentName for RBAC kinds. " +
			"Deliberate divergence: Service uses the KEP-5311 relaxed DNS-1123 label rule (NameIsDNSLabel) " +
			"rather than the historical NameIsDNS1035Label, because the relaxed rule is beta and on by " +
			"default from 1.36 and is the permissive one, so a leading-digit Service name is never falsely " +
			"blocked. Kinds with no confirmed upstream call site are skipped rather than defaulted.",
	},
	"core/object-meta-namespace-invalid": {
		Path:        objectMetaPath,
		Functions:   []string{"validateObjectMetaAccessorWithOptsCommon"},
		Digest:      "sha256:353cac5eff6478c7fe4c4e9031ee216b69cb7e84077ab1ec571ff1577d2366c6",
		ValidatedAt: validatedAt,
		Note: "Ports the ValidateNamespaceName(meta.GetNamespace(), false) branch only; ValidateNamespaceName " +
			"is an alias for apimachinery NameIsDNSLabel (generic.go), i.e. IsDNS1123Label. " +
			"Deliberate divergence: the surrounding scope rules (a namespaced object must set a namespace, " +
			"a cluster-scoped object must not) are NOT ported here. They are owned by the exemptable " +
			"\"namespace\" static check, which carries the generated cluster resource-scope map; duplicating " +
			"them would double-report and would make an exemptable policy decision unexemptable.",
	},

	"core/object-meta-labels-invalid": {
		Path:        labelsPath,
		Functions:   []string{"ValidateLabels", "ValidateLabelName"},
		Digest:      "sha256:2d462c5689c01b48bb90b35becfba9415e19e56c05a4ecf6a299e13a8483fc3e",
		ValidatedAt: validatedAt,
		Note: "Ports both branches of ValidateLabels: the key is checked with IsQualifiedName via " +
			"ValidateLabelName, and the value with the narrower IsValidLabelValue. Unlike the name rules " +
			"in object_meta.go this is not per-kind, so the check declares no Kinds() and runs on every " +
			"document. Findings are emitted in sorted key order because upstream iterates a map and Go " +
			"randomizes that; ordering is not part of the upstream contract.",
	},

	"core/object-meta-annotations-invalid": {
		Path:        objectMetaPath,
		Functions:   []string{"ValidateAnnotations", "ValidateAnnotationsSize"},
		Digest:      "sha256:89a0f370e961b4895558f35f64622501b96c778d0aa0f7603abbe85f951b8e23",
		ValidatedAt: validatedAt,
		Note: "Ports the qualified-name branch, which lowercases the key first because annotation keys " +
			"are case-insensitive where label keys are not, and the ValidateAnnotationsSize branch, " +
			"which sums every key and value against the single 256 kB TotalAnnotationSizeLimitB and is " +
			"therefore reported once per object rather than per annotation. Like the labels check this " +
			"is kind-independent. Not ported: the field.TooLong error's empty value argument, an " +
			"upstream formatting quirk with no bearing on whether the object is rejected.",
	},

	// --- core objects ------------------------------------------------------
	"core/configmap-data-size-exceeded": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateConfigMap"},
		Digest:      "sha256:6093ed83cf1d9045aa732c0224d29b8f979becf7ec61a799583906783d7c1885",
		ValidatedAt: validatedAt,
		Note:        "Ports the totalSize > core.MaxSecretSize -> field.TooLong branch (data plus binaryData, 1 MiB). The key-name and duplicate-key branches are not ported.",
	},
	"core/limitrange-max-min-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateLimitRange"},
		Digest:      "sha256:e9cf91adcc0d363670085e3760afd4655184ecdcaf826a893b80f93d5a451d61",
		ValidatedAt: validatedAt,
		Note:        "Ports the \"min value %s is greater than max value %s\" branch. Upstream reports it on the spec.limits[i].min[k] path; this check reports the equivalent condition on the max path. The default/defaultRequest/maxLimitRequestRatio branches are not ported.",
	},
	"core/resourcequota-hard-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateResourceQuotaSpec", "ValidateResourceQuotaResourceName"},
		Digest:      "sha256:c189076c8a34be4496c148ded6261c5f12b7c339e4f1e338cd19c43b1cd56aca",
		ValidatedAt: validatedAt,
		Note:        "Ports the qualified-name half of ValidateResourceQuotaResourceName as applied to every key of spec.hard; the standard-resource-prefix branch is not ported.",
	},
	"core/resourcequota-hard-negative": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateResourceQuotaSpec", "ValidateResourceQuantityValue"},
		Digest:      "sha256:1f684551a899ad11ae80d01e226de8d1506cad72e7ac09f5309c0322edd2c159",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeQuantity path of ValidateResourceQuantityValue as applied to every value of spec.hard; the integer-resource branch is not ported.",
	},
}
