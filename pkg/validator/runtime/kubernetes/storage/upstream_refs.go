package storage

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// storageValidationPath is pkg/apis/storage/validation/validation.go in
// kubernetes/kubernetes, which holds the StorageClass rules ported here.
const storageValidationPath = "pkg/apis/storage/validation/validation.go"

// coreValidationPath is pkg/apis/core/validation/validation.go. PersistentVolume
// and PersistentVolumeClaim are core types, so their validation lives there
// rather than in the storage API group.
const coreValidationPath = "pkg/apis/core/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.37.0"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	// --- StorageClass ------------------------------------------------------
	"kubernetes/storage-class/provisioner-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateProvisioner"},
		Digest:      "sha256:9099cca3d73276b96e2072b386dc8584c9587212100db107cb3a656c1570d5a7",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(provisioner) == 0 -> field.Required branch. The IsQualifiedName branch on a non-empty provisioner is not ported.",
	},
	"kubernetes/storage-class/reclaim-policy-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateReclaimPolicy"},
		Digest:      "sha256:791ebb715d65b5a33bfbfb742a971db7513c53b2a343d5e3610a8c874fe50064",
		ValidatedAt: validatedAt,
		Note: "Ports the !supportedReclaimPolicy.Has(*reclaimPolicy) -> field.NotSupported branch " +
			"(Delete or Retain). Neither an absent nor an explicitly-empty reclaimPolicy is " +
			"reported, matching upstream rather than diverging from it: validateReclaimPolicy " +
			"returns early on nil and wraps the NotSupported branch in its own " +
			"len(string(*reclaimPolicy)) > 0 guard, so the API server accepts reclaimPolicy: \"\". " +
			"There is no Required branch in the cited function. Note this guard is specific to " +
			"this field - validateVolumeBindingMode in the same file has none, so " +
			"kubernetes/storage-class/volume-binding-mode-invalid correctly reports an empty value.",
		Additional: []runtime.UpstreamRef{{
			Path:        storageValidationPath,
			Functions:   []string{"supportedReclaimPolicy"},
			Digest:      "sha256:30bdbfc17205c7038d1db6a7660f105b3657490db3db82dce5098af67fb93881",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"kubernetes/storage-class/volume-binding-mode-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateVolumeBindingMode"},
		Digest:      "sha256:4d6f232bcacebc818c011eef7d9e7fd277ae8a47e8a7b85b839888ab3d6ca29b",
		ValidatedAt: validatedAt,
		Note:        "Upstream additionally reports an absent volumeBindingMode as Required; this check skips absent values because defaulting supplies Immediate before validation.",
		Additional: []runtime.UpstreamRef{{
			Path:        storageValidationPath,
			Functions:   []string{"supportedVolumeBindingModes"},
			Digest:      "sha256:4ddbdf41e687b86ce14be034868d9c5e661e75fd74cc0d5906ae2733653269d1",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"kubernetes/storage-class/allowed-topology-range-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateTopologySelectorTerm", "validateTopologySelectorLabelRequirement"},
		Digest:      "sha256:ac518bc1edccd06c88fd6110f27819116b6e6fa752e122b7651615e786100b94",
		ValidatedAt: validatedAt,
		Note: "Ports the ValidateLabelName(rq.Key, ...) branch of validateTopologySelectorLabelRequirement, " +
			"reached from storage validateAllowedTopologies -> ValidateTopologySelectorTerm for every " +
			"spec.allowedTopologies[].matchLabelExpressions[].key. ValidateLabelName is apimachinery " +
			"IsQualifiedName. The empty-key case is reported with its own message here; upstream reaches " +
			"it through the same IsQualifiedName call. The values Required/Duplicate branches and the " +
			"duplicate-term branch of validateAllowedTopologies are not ported.",
	},

	// --- PersistentVolume --------------------------------------------------
	"kubernetes/persistent-volume/access-modes-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeSpec"},
		Digest:      "sha256:6d6f8faa74e72b0624928a93d4d965ede0be1767d12329bf11fb8dd2669d20dc",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedAccessModes.Has(mode) -> field.NotSupported branch on spec.accessModes. The Required (empty list) and ReadWriteOncePod-with-other-modes branches are not ported.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedAccessModes"},
			Digest:      "sha256:cf4be75ee7c1338fc1de9621314457aaca1c721a51fda9ae7e9db0237cb4e107",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"kubernetes/persistent-volume/volume-mode-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeSpec"},
		Digest:      "sha256:6d6f8faa74e72b0624928a93d4d965ede0be1767d12329bf11fb8dd2669d20dc",
		ValidatedAt: validatedAt,
		Note: "Ports the !supportedVolumeModes.Has(*pvSpec.VolumeMode) -> field.NotSupported branch " +
			"(Filesystem or Block), the same set ValidatePersistentVolumeClaimSpec applies to a claim. " +
			"The sibling Forbidden branch guarded by validateInlinePersistentVolumeSpec is not ported: " +
			"it rejects a non-Filesystem mode on a PV embedded inline in a pod spec, and a standalone " +
			"PersistentVolume document is validated with that flag false. A nil volumeMode is not " +
			"reported, because defaulting is guarded on nil and supplies Filesystem.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedVolumeModes"},
			Digest:      "sha256:bf946d503a1ee64253d0d0320abe6f330400cbe19ebc06dea30e422465dccc90",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"kubernetes/persistent-volume/capacity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeSpec"},
		Digest:      "sha256:6d6f8faa74e72b0624928a93d4d965ede0be1767d12329bf11fb8dd2669d20dc",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(pvSpec.Capacity) == 0 -> field.Required branch. The sibling NotSupported branch requiring exactly the \"storage\" resource key is not ported.",
	},

	// --- PersistentVolumeClaim ---------------------------------------------
	"kubernetes/persistent-volume-claim/access-modes-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeClaimSpec"},
		Digest:      "sha256:9ca3d57d0c139d824644173c0daa8d64e1a23dabee01c9a132d60769d667eba2",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedAccessModes.Has(mode) -> field.NotSupported branch on spec.accessModes. The Required (empty list) and ReadWriteOncePod-with-other-modes branches are not ported.",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedAccessModes"},
			Digest:      "sha256:cf4be75ee7c1338fc1de9621314457aaca1c721a51fda9ae7e9db0237cb4e107",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"kubernetes/persistent-volume-claim/volume-mode-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeClaimSpec"},
		Digest:      "sha256:9ca3d57d0c139d824644173c0daa8d64e1a23dabee01c9a132d60769d667eba2",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedVolumeModes.Has(*spec.VolumeMode) -> field.NotSupported branch (Filesystem or Block).",
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedVolumeModes"},
			Digest:      "sha256:bf946d503a1ee64253d0d0320abe6f330400cbe19ebc06dea30e422465dccc90",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
}
