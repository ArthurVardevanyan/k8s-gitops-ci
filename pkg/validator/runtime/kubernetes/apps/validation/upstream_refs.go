package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// appsValidationPath is pkg/apis/apps/validation/validation.go in
// kubernetes/kubernetes, which holds every rule ported by this package.
const appsValidationPath = "pkg/apis/apps/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken
// at. It matches the tag derived from go.mod that
// `task verify:upstream-refs` pins to.
const validatedAt = "v1.36.3"

// selectorNote is shared by the workload selector checks. Each upstream
// *Spec validator hands spec.selector to apimachinery's ValidateLabelSelector;
// this package ports the qualified-name half of that (matchLabels keys and
// matchExpressions keys).
const selectorNote = "Ports the unversionedvalidation.ValidateLabelSelector(spec.Selector, ...) call. " +
	"Only the qualified-name rule on matchLabels keys and matchExpressions keys is ported; " +
	"the nil-selector Required branch, the empty-selector Invalid branch, the operator/values " +
	"branches and the selector-matches-template-labels branch are not."

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	// --- Deployment --------------------------------------------------------
	"apps/deployment-selector-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentSpec"},
		Digest:      "sha256:e3dc5b5e520f1b808ce45922c4e21405da0a1fc8fd443038b578be3fcd601444",
		ValidatedAt: validatedAt,
		Note:        selectorNote,
	},
	"apps/deployment-strategy-type-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentStrategy"},
		Digest:      "sha256:ae7bc51ed073a4b3e7da92cda923aa6edb3915044495c6ff0b64e9f38c133e62",
		ValidatedAt: validatedAt,
		Note: "Ports the default -> field.NotSupported branch on strategy.type. " +
			"Deliberate divergence: an empty type is skipped rather than reported, because " +
			"defaulting sets RollingUpdate before validation and unrendered manifests legitimately " +
			"omit it. The rollingUpdate sub-branches are not ported.",
	},
	"apps/deployment-replicas-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentSpec"},
		Digest:      "sha256:e3dc5b5e520f1b808ce45922c4e21405da0a1fc8fd443038b578be3fcd601444",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.Replicas, ...) call in ValidateDeploymentSpec.",
	},
	"apps/deployment-min-ready-seconds-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentSpec"},
		Digest:      "sha256:e3dc5b5e520f1b808ce45922c4e21405da0a1fc8fd443038b578be3fcd601444",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.MinReadySeconds, ...) call in ValidateDeploymentSpec.",
	},

	// --- StatefulSet -------------------------------------------------------
	"apps/statefulset-replicas-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateStatefulSetSpec"},
		Digest:      "sha256:49cafc1bc27a4840cfbe86266db3595b1e4abd7f37e1327433430267360403d6",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.Replicas, ...) call in ValidateStatefulSetSpec.",
	},
	"apps/statefulset-pod-management-policy-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateStatefulSetSpec"},
		Digest:      "sha256:49cafc1bc27a4840cfbe86266db3595b1e4abd7f37e1327433430267360403d6",
		ValidatedAt: validatedAt,
		Note: "Ports the default branch of the spec.podManagementPolicy switch " +
			"(must be 'OrderedReady' or 'Parallel'). Deliberate divergence: the empty case, which " +
			"upstream reports Required, is skipped because defaulting supplies OrderedReady.",
	},
	"apps/statefulset-update-strategy-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateStatefulSetSpec"},
		Digest:      "sha256:49cafc1bc27a4840cfbe86266db3595b1e4abd7f37e1327433430267360403d6",
		ValidatedAt: validatedAt,
		Note: "Ports the default branch of the spec.updateStrategy.type switch " +
			"(must be 'RollingUpdate' or 'OnDelete'). Deliberate divergence: the empty case, which " +
			"upstream reports Required, is skipped because defaulting supplies RollingUpdate. " +
			"The rollingUpdate sub-branches are not ported.",
	},

	// --- DaemonSet ---------------------------------------------------------
	"apps/daemonset-selector-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDaemonSetSpec"},
		Digest:      "sha256:854de045a62b38b1e3423bc962fb7ebf7c9ddfb1aeffbe429c308bcb084b3aed",
		ValidatedAt: validatedAt,
		Note:        selectorNote,
	},
	"apps/daemonset-update-strategy-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDaemonSetUpdateStrategy"},
		Digest:      "sha256:97d4333627ad4f027549e2885800a14f425df48634dc8459d575621177f688ec",
		ValidatedAt: validatedAt,
		Note: "Ports the default -> field.NotSupported branch on updateStrategy.type. " +
			"Deliberate divergence: an empty type, which upstream reaches through the same default " +
			"branch, is skipped because defaulting sets RollingUpdate before validation. " +
			"The rollingUpdate sub-branch is not ported.",
	},
	"apps/daemonset-min-ready-seconds-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDaemonSetSpec"},
		Digest:      "sha256:854de045a62b38b1e3423bc962fb7ebf7c9ddfb1aeffbe429c308bcb084b3aed",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.MinReadySeconds, ...) call in ValidateDaemonSetSpec.",
	},

	// --- ReplicaSet --------------------------------------------------------
	"apps/replicaset-selector-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateReplicaSetSpec"},
		Digest:      "sha256:f2e36085b90d34c4912750bf30c76c6325eaed55f65fe56d6a2512fc12edcfe9",
		ValidatedAt: validatedAt,
		Note:        selectorNote,
	},
	"apps/replicaset-replicas-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateReplicaSetSpec"},
		Digest:      "sha256:f2e36085b90d34c4912750bf30c76c6325eaed55f65fe56d6a2512fc12edcfe9",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.Replicas, ...) call in ValidateReplicaSetSpec.",
	},
}
