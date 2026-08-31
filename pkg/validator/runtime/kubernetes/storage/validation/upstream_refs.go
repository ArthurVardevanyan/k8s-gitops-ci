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
		Note: "Ports the !supportedReclaimPolicy.Has(*reclaimPolicy) -> field.NotSupported branch " +
			"(Delete or Retain). Neither an absent nor an explicitly-empty reclaimPolicy is " +
			"reported, matching upstream rather than diverging from it: validateReclaimPolicy " +
			"returns early on nil and wraps the NotSupported branch in its own " +
			"len(string(*reclaimPolicy)) > 0 guard, so the API server accepts reclaimPolicy: \"\". " +
			"There is no Required branch in the cited function. Note this guard is specific to " +
			"this field - validateVolumeBindingMode in the same file has none, so " +
			"storage-class/volume-binding-mode-invalid correctly reports an empty value.",
		Additional: []runtime.UpstreamRef{{
			Path:        storageValidationPath,
			Functions:   []string{"supportedReclaimPolicy"},
			Digest:      "sha256:30bdbfc17205c7038d1db6a7660f105b3657490db3db82dce5098af67fb93881",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"storage-class/volume-binding-mode-invalid": {
		Path:        storageValidationPath,
		Functions:   []string{"validateVolumeBindingMode"},
		Digest:      "sha256:bec218feef5c924cdde9be09c75d44df565ca1a769a1700c291cc7e9c4889ab7",
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
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedAccessModes"},
			Digest:      "sha256:7cfa2280610fbfb3b20f52cef8426bf64386fea5117334cd2f1c7db62d029d3a",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"persistent-volume/volume-mode-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeSpec"},
		Digest:      "sha256:1fff24fb51a902e080f973a47107553cda679fdb80ca56d1414ee7a46864cc05",
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
		Additional: []runtime.UpstreamRef{{
			Path:        coreValidationPath,
			Functions:   []string{"supportedAccessModes"},
			Digest:      "sha256:7cfa2280610fbfb3b20f52cef8426bf64386fea5117334cd2f1c7db62d029d3a",
			ValidatedAt: validatedAt,
			Note:        "The set the ported branch tests membership against. Upstream decides acceptance here, not in the function body, so a value added to this set alone would leave the function digest unchanged while this check went on rejecting a manifest the API server accepts.",
		}},
	},
	"persistent-volume-claim/volume-mode-invalid": {
		Path:        coreValidationPath,
		Functions:   []string{"ValidatePersistentVolumeClaimSpec"},
		Digest:      "sha256:a22a9b4b8eef2bc3fad9b07e34d35db8b0f1070ce5f3c7d7f583a8be3938a981",
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
