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
const validatedAt = "v1.37.0"

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
		Digest:      "sha256:70932c3277e5d6a84aa10f1fff44202f55e229d8ee603320cd39782cfd3109b8",
		ValidatedAt: validatedAt,
		Note:        selectorNote,
	},
	"apps/deployment-strategy-type-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentStrategy"},
		Digest:      "sha256:0d3d7684994250c53b8da905d9b76c7cacc2508e21b3811df37c99d5c68e8f64",
		ValidatedAt: validatedAt,
		Note: "Ports the default -> field.NotSupported branch on strategy.type. " +
			"Deliberate divergence: an empty type is skipped rather than reported, because " +
			"defaulting sets RollingUpdate before validation and unrendered manifests legitimately " +
			"omit it. The rollingUpdate sub-branches are not ported.",
	},
	"apps/deployment-replicas-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentSpec"},
		Digest:      "sha256:70932c3277e5d6a84aa10f1fff44202f55e229d8ee603320cd39782cfd3109b8",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.Replicas, ...) call in ValidateDeploymentSpec.",
	},
	"apps/deployment-min-ready-seconds-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDeploymentSpec"},
		Digest:      "sha256:70932c3277e5d6a84aa10f1fff44202f55e229d8ee603320cd39782cfd3109b8",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.MinReadySeconds, ...) call in ValidateDeploymentSpec.",
	},

	// --- StatefulSet -------------------------------------------------------
	"apps/statefulset-replicas-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateStatefulSetSpec"},
		Digest:      "sha256:38d87077ae011f16a6f6227cd2372a8fcb37c519a7deea1ece414fa59ef52566",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.Replicas, ...) call in ValidateStatefulSetSpec.",
	},
	"apps/statefulset-pod-management-policy-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateStatefulSetSpec"},
		Digest:      "sha256:38d87077ae011f16a6f6227cd2372a8fcb37c519a7deea1ece414fa59ef52566",
		ValidatedAt: validatedAt,
		Note: "Ports the default branch of the spec.podManagementPolicy switch " +
			"(must be 'OrderedReady' or 'Parallel'). Deliberate divergence: the empty case, which " +
			"upstream reports Required, is skipped because defaulting supplies OrderedReady. That " +
			"defaulting is cited below, and an explicitly-empty value is accepted for the same reason.",
		Additional: []runtime.UpstreamRef{{
			Path:        "pkg/apis/apps/v1/defaults.go",
			Functions:   []string{"SetDefaults_StatefulSet"},
			Digest:      "sha256:56ffaaae3883b2d894a962815a0cdf3ab8f0d868aabcb61e650be57f8c38b175",
			ValidatedAt: validatedAt,
			Note: "Guards podManagementPolicy on len()==0, so an explicitly-empty value is " +
				"defaulted to OrderedReady and must not be reported.",
		}},
	},
	"apps/statefulset-update-strategy-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateStatefulSetSpec"},
		Digest:      "sha256:38d87077ae011f16a6f6227cd2372a8fcb37c519a7deea1ece414fa59ef52566",
		ValidatedAt: validatedAt,
		Note: "Ports the default branch of the spec.updateStrategy.type switch. " +
			"Deliberate divergence: the empty case, which upstream reports Required, is skipped " +
			"because defaulting supplies RollingUpdate. The rollingUpdate sub-branches are not ported. " +
			"Deliberate divergence: 'Recreate' is accepted. Upstream gates it behind the " +
			"StatefulSetRecreateStrategy feature gate, which reaches the validator as the " +
			"setOpts.AllowStatefulSetRecreateStrategy option, and this tool cannot see a cluster's feature " +
			"gates, so it takes the permissive branch rather than risk blocking a valid manifest " +
			"with a non-exemptable check; a cluster with the gate off rejects it at apply time.",
	},

	// --- DaemonSet ---------------------------------------------------------
	"apps/daemonset-selector-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDaemonSetSpec"},
		Digest:      "sha256:acc13d2b128b269650f1c2d39b36bb87cd6c5e601087c4928e691962c3e2ca65",
		ValidatedAt: validatedAt,
		Note:        selectorNote,
	},
	"apps/daemonset-update-strategy-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDaemonSetUpdateStrategy"},
		Digest:      "sha256:97aaae89ad60a0885be03a03c81fb038f2fc3c0913196ab30e3de26f3bb2db11",
		ValidatedAt: validatedAt,
		Note: "Ports the default -> field.NotSupported branch on updateStrategy.type. " +
			"Deliberate divergence: an empty type, which upstream reaches through the same default " +
			"branch, is skipped because defaulting sets RollingUpdate before validation. " +
			"The rollingUpdate sub-branch is not ported.",
	},
	"apps/daemonset-min-ready-seconds-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateDaemonSetSpec"},
		Digest:      "sha256:acc13d2b128b269650f1c2d39b36bb87cd6c5e601087c4928e691962c3e2ca65",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.MinReadySeconds, ...) call in ValidateDaemonSetSpec.",
	},

	// --- ReplicaSet --------------------------------------------------------
	"apps/replicaset-selector-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateReplicaSetSpec"},
		Digest:      "sha256:fd638e965d961324c568d3d5e4e0948ad1bf977583ee72f905b37d5185d8cec5",
		ValidatedAt: validatedAt,
		Note:        selectorNote,
	},
	"apps/replicaset-replicas-invalid": {
		Path:        appsValidationPath,
		Functions:   []string{"ValidateReplicaSetSpec"},
		Digest:      "sha256:fd638e965d961324c568d3d5e4e0948ad1bf977583ee72f905b37d5185d8cec5",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(spec.Replicas, ...) call in ValidateReplicaSetSpec.",
	},
}
