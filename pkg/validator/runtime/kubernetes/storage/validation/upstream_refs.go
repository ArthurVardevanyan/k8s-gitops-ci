package validation

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
	"storage-class/provisioner-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateProvisioner"},
		Digest:      "sha256:9099cca3d73276b96e2072b386dc8584c9587212100db107cb3a656c1570d5a7",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(provisioner) == 0 -> field.Required branch. The IsQualifiedName branch on a non-empty provisioner is not ported.",
	},
	"storage-class/reclaim-policy-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateReclaimPolicy"},
		Digest:      "sha256:791ebb715d65b5a33bfbfb742a971db7513c53b2a343d5e3610a8c874fe50064",
		ValidatedAt: validatedAt,
		Note:        "Upstream additionally reports a nil/empty reclaimPolicy as Required; this check skips absent values because defaulting supplies Delete before validation.",
	},
	"storage-class/volume-binding-mode-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateVolumeBindingMode"},
		Digest:      "sha256:bec218feef5c924cdde9be09c75d44df565ca1a769a1700c291cc7e9c4889ab7",
		ValidatedAt: validatedAt,
		Note:        "Upstream additionally reports an absent volumeBindingMode as Required; this check skips absent values because defaulting supplies Immediate before validation.",
	},
	"storage-class/allowed-topology-range-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidateTopologySelectorTerm", "validateTopologySelectorLabelRequirement"},
		Digest:      "sha256:452b415090852737d6a299b6f6bc0c568ca505079a2bb14a13325ff9666588a3",
		ValidatedAt: validatedAt,
		Note: "Ports the ValidateLabelName(rq.Key, ...) branch of validateTopologySelectorLabelRequirement, " +
			"reached from storage validateAllowedTopologies -> ValidateTopologySelectorTerm for every " +
			"spec.allowedTopologies[].matchLabelExpressions[].key. ValidateLabelName is apimachinery " +
			"IsQualifiedName. The empty-key case is reported with its own message here; upstream reaches " +
			"it through the same IsQualifiedName call. The values Required/Duplicate branches and the " +
			"duplicate-term branch of validateAllowedTopologies are not ported.",
	},

	// --- PersistentVolume --------------------------------------------------
	"persistent-volume/access-modes-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeSpec"},
		Digest:      "sha256:1fff24fb51a902e080f973a47107553cda679fdb80ca56d1414ee7a46864cc05",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedAccessModes.Has(mode) -> field.NotSupported branch on spec.accessModes. The Required (empty list) and ReadWriteOncePod-with-other-modes branches are not ported.",
	},
	"persistent-volume/capacity-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeSpec"},
		Digest:      "sha256:1fff24fb51a902e080f973a47107553cda679fdb80ca56d1414ee7a46864cc05",
		ValidatedAt: validatedAt,
		Note:        "Ports the len(pvSpec.Capacity) == 0 -> field.Required branch. The sibling NotSupported branch requiring exactly the \"storage\" resource key is not ported.",
	},

	// --- PersistentVolumeClaim ---------------------------------------------
	"persistent-volume-claim/access-modes-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeClaimSpec"},
		Digest:      "sha256:a22a9b4b8eef2bc3fad9b07e34d35db8b0f1070ce5f3c7d7f583a8be3938a981",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedAccessModes.Has(mode) -> field.NotSupported branch on spec.accessModes. The Required (empty list) and ReadWriteOncePod-with-other-modes branches are not ported.",
	},
	"persistent-volume-claim/volume-mode-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeClaimSpec"},
		Digest:      "sha256:a22a9b4b8eef2bc3fad9b07e34d35db8b0f1070ce5f3c7d7f583a8be3938a981",
		ValidatedAt: validatedAt,
		Note:        "Ports the !supportedVolumeModes.Has(*spec.VolumeMode) -> field.NotSupported branch (Filesystem or Block).",
	},
}
