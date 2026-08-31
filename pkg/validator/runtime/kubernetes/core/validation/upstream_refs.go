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
		Digest:      "sha256:8f708a22858be520690bd972483bfcc649bcc37ee757c38dd24113e2bfa19ae0",
		ValidatedAt: validatedAt,
		Note: "Ports the namePath branch: an absent name is Required and a present one must pass " +
			"ValidateDNS1123Label. The two are mutually exclusive upstream and are kept that way here. " +
			"The rest of validateContainerCommon (image, terminationMessagePolicy, ports, env/envFrom, " +
			"volumeMounts/volumeDevices, imagePullPolicy, resources, resizePolicy, securityContext) is " +
			"not ported. lifecycle and probes are validated in validateContainers/validateInitContainers/" +
			"validateEphemeralContainers, not in validateContainerCommon.",
	},

	"container/port-name-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:0c583e14a5bf64699161f2202eeb01c4d80f95a7a3525b2cf60ac1070f15fa51",
		ValidatedAt: validatedAt,
		Note: "Ports the IsValidPortName branch only, which upstream applies solely when a port name is " +
			"set, since an unnamed port is legal. The other branches of validateContainerPorts are " +
			"covered by container/duplicate-port-names (Duplicate), container/port-number-range " +
			"(containerPort), container/host-port-range (hostPort) and container/port-protocol-invalid " +
			"(protocol).",
	},

	"core/replicationcontroller-replicas-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateReplicationControllerSpec"},
		Digest:      "sha256:97e83579d67da9c5f839b93d0d438c2fb09d143f7d5d4ee12f54e82d5b93f29d",
		ValidatedAt: validatedAt,
		Note: "Ports the replicas ValidateNonnegativeField branch. Deliberate divergence: the " +
			"Required branch for a nil replicas is skipped, because defaulting sets 1 before " +
			"validation and an unrendered manifest legitimately omits the field.",
	},
	"core/replicationcontroller-selector-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateReplicationControllerSpec", "ValidateNonEmptySelector"},
		Digest:      "sha256:4c3c38b503c5572c18b2b6de6353a031599200c4f9428f72e064488f94255f7a",
		ValidatedAt: validatedAt,
		Note: "Ports the ValidateNonEmptySelector Required branch. The selector is not read as " +
			"written: SetDefaults_ReplicationController copies spec.template.metadata.labels into " +
			"an empty selector before validation, so an omitted selector is valid and is the " +
			"ordinary way to write the kind. That defaulting is cited below and reproduced here, " +
			"so the rule fires only when the template has no labels to supply one either.",
		Additional: []runtime.UpstreamRef{{
			Path:        "pkg/apis/core/v1/defaults.go",
			Functions:   []string{"SetDefaults_ReplicationController"},
			Digest:      "sha256:2383065a64ea4d28fea31907fb5619507cbf06f8346f80770105a33c6e2827ab",
			ValidatedAt: validatedAt,
			Note: "Supplies the selector this rule tests. Without it the check reports every " +
				"ReplicationController that omits spec.selector, which the API server accepts.",
		}},
	},
	"core/replicationcontroller-min-ready-seconds-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateReplicationControllerSpec"},
		Digest:      "sha256:97e83579d67da9c5f839b93d0d438c2fb09d143f7d5d4ee12f54e82d5b93f29d",
		ValidatedAt: validatedAt,
		Note:        "Ports the minReadySeconds ValidateNonnegativeField branch.",
	},
	"container/duplicate-container-names": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainers", "validateInitContainers", "validateEphemeralContainers"},
		Digest:      "sha256:9e9ae5e3675f60e293157886944d3e8870cd1abd6754392acb2075b43f7a7e2e",
		ValidatedAt: validatedAt,
		Note: "Ports the allNames/field.Duplicate branch. Upstream splits regular, init " +
			"and ephemeral containers across validateContainers, validateInitContainers " +
			"and validateEphemeralContainers to avoid duplicate messages; this check " +
			"walks all container lists at once and reports each colliding name once. " +
			"All three are cited because the rule is ported from all three: citing only " +
			"validateContainers left changes to the init and ephemeral branches invisible.",
	},
	"container/duplicate-port-names": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:0c583e14a5bf64699161f2202eeb01c4d80f95a7a3525b2cf60ac1070f15fa51",
		ValidatedAt: validatedAt,
		Note: "Ports the allNames Duplicate branch only. The sibling branches of validateContainerPorts " +
			"are covered by container/port-name-invalid (IsValidPortName), container/port-number-range " +
			"(containerPort), container/host-port-range (hostPort) and container/port-protocol-invalid " +
			"(protocol).",
	},
	"container/port-number-range": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:0c583e14a5bf64699161f2202eeb01c4d80f95a7a3525b2cf60ac1070f15fa51",
		ValidatedAt: validatedAt,
		Note: "Ports the containerPort range branch, which upstream expresses as " +
			"validation.IsValidPortNum(int(port.ContainerPort)). Deliberate divergence: upstream splits " +
			"zero out first and reports it as field.Required; this check folds 0 into the range finding, " +
			"since both say the same thing about a manifest and a separate Required rule would " +
			"double-report. The other branches of the same function are covered by " +
			"container/duplicate-port-names, container/port-name-invalid, container/host-port-range and " +
			"container/port-protocol-invalid.",
	},
	"container/host-port-range": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:0c583e14a5bf64699161f2202eeb01c4d80f95a7a3525b2cf60ac1070f15fa51",
		ValidatedAt: validatedAt,
		Note: "Ports the hostPort range branch, which upstream expresses as " +
			"validation.IsValidPortNum(int(port.HostPort)) behind its own port.HostPort != 0 guard. " +
			"That guard is upstream's, not a defaulting concession: a zero hostPort means the port was " +
			"not requested, so it is skipped here for the same reason. The other branches of the same " +
			"function are covered by container/duplicate-port-names, container/port-name-invalid, " +
			"container/port-number-range and container/port-protocol-invalid.",
	},
	"container/port-protocol-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerPorts"},
		Digest:      "sha256:0c583e14a5bf64699161f2202eeb01c4d80f95a7a3525b2cf60ac1070f15fa51",
		ValidatedAt: validatedAt,
		Note: "Ports the !supportedPortProtocols.Has(port.Protocol) -> field.NotSupported branch " +
			"(TCP, UDP or SCTP). Deliberate divergence: the sibling len(port.Protocol) == 0 -> " +
			"field.Required branch is skipped, because ContainerPort.Protocol carries a +default=\"TCP\" " +
			"marker and is defaulted before validation runs; reporting it would fail nearly every " +
			"manifest, since omitting the protocol is the ordinary way to write a port. That marker is " +
			"cited below. The other branches of the same function are covered by " +
			"container/duplicate-port-names, container/port-name-invalid, container/port-number-range " +
			"and container/host-port-range.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedPortProtocols"},
			Digest:      "sha256:7025c6fa0000f2450b43be9f90f212c6c5fc60d4a0d981e9f0a4d4e827efadd4",
			ValidatedAt: validatedAt,
			Note: "The set this rule tests. Cited so that adding a protocol upstream moves the " +
				"digest instead of silently leaving this check rejecting a value the API server accepts.",
		}, {
			Path:        "pkg/apis/core/v1/zz_generated.defaults.go",
			Functions:   []string{"SetObjectDefaults_Pod"},
			Digest:      "sha256:4be5038e07f9acd07b07a4a570079362f4c06bf14d2ccc5cbf75643872436a84",
			ValidatedAt: validatedAt,
			Note: "Supplies the protocol this rule skips. It applies the ContainerPort.Protocol " +
				"+default=\"TCP\" marker as generated code, so an omitted protocol never reaches " +
				"upstream's Required branch. The marker itself is a comment, which the digest " +
				"mechanism strips; this generated function is the closest citable form of it. Pod is " +
				"cited as the representative kind: the generator emits the same protocol block into " +
				"every workload kind's defaulter.",
		}},
	},

	"container/image-pull-policy": {
		Path:        coreValidationPath,
		Functions:   []string{"validatePullPolicy"},
		Digest:      "sha256:a0277f0d5fd7ff385f46d34f6d3927331b11ff37d17b92f7bf2deec2b3d2f2c8",
		ValidatedAt: validatedAt,
		Note:        "Upstream additionally requires the field (empty is Required); this check skips empty because defaulting fills it in before validation and unrendered manifests legitimately omit it.",
	},
	"container/mount-propagation-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateMountPropagation"},
		Digest:      "sha256:e5f40212f89ab7553916d10cd676ccbcf2829d83c3d16cc175a5c1309f39d05b",
		ValidatedAt: validatedAt,
		Note:        "Ports the supported-value branch only. The Bidirectional-requires-privileged branch is not ported: it is a cross-field rule against the container securityContext.",
	},
	"container/termination-message-policy-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateContainerCommon"},
		Digest:      "sha256:8f708a22858be520690bd972483bfcc649bcc37ee757c38dd24113e2bfa19ae0",
		ValidatedAt: validatedAt,
		Note:        "Ports the terminationMessagePolicy switch. The empty case is skipped rather than reported Required, because defaulting supplies it.",
	},
	"container/volume-mount-name-undefined": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateVolumeMounts", "IsMatchedVolume"},
		Digest:      "sha256:8a642b90311df5fc19224ab138c610ff5258698207a9a0430e2ca7f9117bcf32",
		ValidatedAt: validatedAt,
		Note: "Ports the !IsMatchedVolume -> field.NotFound branch (a volumeMount must name a declared volume). " +
			"The set of declared volumes is the caller's, not the pod spec's alone: for a StatefulSet the API " +
			"server synthesizes a volume per volumeClaimTemplate and drops any pod-template volume of the same " +
			"name before validating the template, so a mount naming a claim template resolves. That merge is " +
			"reproduced in the walker and cited below; without it this rule rejects nearly every real StatefulSet.",
		Additional: []runtime.UpstreamRef{{
			Path:        "pkg/apis/apps/validation/validation.go",
			Functions:   []string{"volumesToAddForTemplates", "ValidateStatefulSetSpec"},
			Digest:      "sha256:7555e6f4f97cd7946d82c95c278c85b64dde81bd20597d22a0ebaea3f6e845a0",
			ValidatedAt: validatedAt,
			Note: "Supplies the volume set the mount rule is evaluated against. volumesToAddForTemplates builds " +
				"one volume per claim template; ValidateStatefulSetSpec is cited with it because the precedence " +
				"(claim templates first, same-named pod-template volumes dropped) lives in the caller.",
		}},
	},

	// --- pod spec ----------------------------------------------------------
	"pod-spec/restart-policy-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateRestartPolicy"},
		Digest:      "sha256:46741962bb02ecee1a5dd104bb757d723cd31d72a5141fbcbc59e8c01901b55c",
		ValidatedAt: validatedAt,
		Note: "Ports the default -> field.NotSupported branch on restartPolicy. Deliberate " +
			"divergence: the empty-value Required branch is skipped, because defaulting sets " +
			"Always before validation and an unrendered manifest legitimately omits the field.",
	},
	"pod-spec/dns-policy-value": {
		Path:        coreValidationPath,
		Functions:   []string{"validateDNSPolicy"},
		Digest:      "sha256:647b87b7449ee64adc1503341952293477e04d4bc70b103cad702bf01a14b2d5",
		ValidatedAt: validatedAt,
		Note: "Ports the default -> field.NotSupported branch on dnsPolicy. Deliberate " +
			"divergence: the empty-value Required branch is skipped, because defaulting sets " +
			"ClusterFirst before validation and an unrendered manifest legitimately omits the " +
			"field.",
	},
	"pod-spec/toleration-operator-value": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateTolerations"},
		Digest:      "sha256:081875fd127be4a599df5ae122dd3c4078276e84c656e4b8917ce7108882f367",
		ValidatedAt: validatedAt,
		Note: "Ports the operator supported-value branch only; the key/value/effect branches of the same function are not ported. " +
			"Lt and Gt are accepted: upstream rejects them only when AllowTaintTolerationComparisonOperators is off, " +
			"and a gate that widens what the API server accepts is ported on its permissive branch, since this tool " +
			"cannot see the target cluster's gates and the finding would be non-exemptable.",
	},
	"pod-spec/affinity-node-selector-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePodSpec"},
		Digest:      "sha256:29b2c14aa676080fcdd79baf32563b9b20e3c616763a2eb42dd07ab58ad490b6",
		ValidatedAt: validatedAt,
		Note:        "Ports the unversionedvalidation.ValidateLabels(spec.NodeSelector, ...) call in ValidatePodSpec: nodeSelector keys must be qualified names and values valid label values.",
	},
	"pod-spec/pod-affinity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateAffinity", "ValidateNodeSelectorRequirement", "ValidatePodAffinityTermSelector"},
		Digest:      "sha256:ce4fef4c89ca006441e9a50a632b95f57d7a4097d3a83b739f8c35e2b74c1c13",
		ValidatedAt: validatedAt,
		Note:        "Ports the label-key/label-value branches reached from validateAffinity: required nodeAffinity matchExpressions/matchFields keys, and pod (anti-)affinity labelSelector keys and values.",
	},
	"pod-spec/topology-spread-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateTopologySpreadConstraints"},
		Digest:      "sha256:68a04e8603d2bd346fb134f3b07c78dde0e2f8bb145cc9e2e4484cb936bbfe7c",
		ValidatedAt: validatedAt,
		Note:        "Ports the labelSelector validation branch only; maxSkew/whenUnsatisfiable/topologyKey/minDomains branches are not ported.",
	},
	"pod-spec/service-account-name-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePodSpec"},
		Digest:      "sha256:29b2c14aa676080fcdd79baf32563b9b20e3c616763a2eb42dd07ab58ad490b6",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateServiceAccountName(spec.ServiceAccountName, false) call in ValidatePodSpec. ValidateServiceAccountName is an alias for apimachinery NameIsDNSSubdomain (generic.go), i.e. IsDNS1123Subdomain.",
	},
	"pod-spec/active-deadline-seconds-negative": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePodSpec"},
		Digest:      "sha256:29b2c14aa676080fcdd79baf32563b9b20e3c616763a2eb42dd07ab58ad490b6",
		ValidatedAt: validatedAt,
		Note: "Ports the activeDeadlineSeconds InclusiveRangeError(1, math.MaxInt32) branch in ValidatePodSpec, " +
			"both bounds. The note here previously claimed the upper bound was unreachable through int32 " +
			"decoding; the field is an *int64, so a manifest can carry a value above MaxInt32 and the API " +
			"server rejects it.",
	},
	"pod-spec/readiness-gate-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"validateReadinessGates"},
		Digest:      "sha256:757173fa3badacbac8c8bd02e74106863c1710e459574c1320d215bc18722c88",
		ValidatedAt: validatedAt,
		Note: "Ports the ValidateQualifiedName call on each readinessGates[i].conditionType. " +
			"No divergence: the upstream function contains only this rule.",
	},

	// --- resources ---------------------------------------------------------
	"resources/resource-requests-greater-than-limits": {
		Path:        coreValidationPath,
		Functions:   []string{"validateResourceRequirements"},
		Digest:      "sha256:ebd45fa41579856ed97f11415251a6cf9664e0f1c24da20bdfce141cc5b1b155",
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
		Digest:      "sha256:e450f197ceafe7b4881598cc69fac729929f6cb99801e9a263d2b1c0a7dc5e6f",
		ValidatedAt: validatedAt,
		Note:        "Ports the allNames/field.Duplicate branch on spec.volumes[].name.",
	},
	"volume/secret-name-required": {
		Path:        coreValidationPath,
		Functions:   []string{"validateSecretVolumeSource"},
		Digest:      "sha256:0e113cdc5ab9c008ad9db36223e94de4651918f4708df956cd358b66bed7badc",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(secretSource.SecretName) == 0 -> field.Required branch. It is a required-field rule, not a live lookup: whether the Secret exists in the cluster cannot be checked from a manifest.",
	},
	"volume/configmap-name-required": {
		Path:        coreValidationPath,
		Functions:   []string{"validateConfigMapVolumeSource"},
		Digest:      "sha256:a75fc50ffc885294d3229e8cd29f6f1931e3dacde347a20c9a7c6a93c5157a69",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(configMapSource.Name) == 0 -> field.Required branch. As with volume/secret-name-required this is a required-field rule, not a live lookup.",
	},

	// --- object metadata ---------------------------------------------------
	"core/object-meta-name-invalid": {
		Path:        objectMetaPath,
		Functions:   []string{"ValidateObjectMetaAccessor"},
		Digest:      "sha256:011fc471f77a7b587b23815678cbd425116995d69cd4777631cb5662e41aa4e3",
		ValidatedAt: validatedAt,
		Note: "Ports the name/generateName half: generateName is validated with prefix=true whenever set. " +
			"Deliberate divergence on the name-Required branch: upstream requires a name whenever it is " +
			"empty, regardless of generateName, because ValidateObjectMetaAccessor runs after the API " +
			"server has already generated one. This check reads the manifest as written, before that " +
			"generation, so it requires a name only when generateName is absent too. The per-kind nameFn is resolved from " +
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
		Digest:      "sha256:feddb0b049625d7337a51a56e16edfb736b1e5faff7e35325ba1cd7aebeba3f6",
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
		Digest:      "sha256:ecb199a418fee63df7b67bd1395ca179ca467a468832c2e21bdd19e7997c8760",
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
		Digest:      "sha256:68ca05c6b2c16e575a3f69b8f91df2a521aef82847031cbf26356d3dea32428d",
		ValidatedAt: validatedAt,
		Note:        "Ports the totalSize > core.MaxSecretSize -> field.TooLong branch (data plus binaryData, 1 MiB). The key-name and duplicate-key branches are not ported.",
	},
	"core/limitrange-max-min-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateLimitRange"},
		Digest:      "sha256:cf0cc0f55f463f84b530e2c26f7571df82fcd888b1f63fb94c1ae619005f5503",
		ValidatedAt: validatedAt,
		Note:        "Ports the \"min value %s is greater than max value %s\" branch. Upstream reports it on the spec.limits[i].min[k] path; this check reports the equivalent condition on the max path. The default/defaultRequest/maxLimitRequestRatio branches are not ported.",
	},
	"core/resourcequota-hard-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateResourceQuotaSpec", "ValidateResourceQuotaResourceName"},
		Digest:      "sha256:af9133fdbf0ed46bab48e778d94f5c180c2df434749ae893cfc66c5087ea7019",
		ValidatedAt: validatedAt,
		Note:        "Ports the qualified-name half of ValidateResourceQuotaResourceName as applied to every key of spec.hard; the standard-resource-prefix branch is not ported.",
	},
	"core/resourcequota-hard-negative": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateResourceQuotaSpec", "ValidateResourceQuantityValue"},
		Digest:      "sha256:22aa55408c0e636bd89987ad9b44672f8d9f375fec3708d6faa90d52ab921266",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeQuantity path of ValidateResourceQuantityValue as applied to every value of spec.hard; the integer-resource branch is not ported.",
	},
}
