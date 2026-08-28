package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// policyValidationPath is pkg/apis/policy/validation/validation.go in
// kubernetes/kubernetes, which holds the PodDisruptionBudget rules ported here.
const policyValidationPath = "pkg/apis/policy/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.36.3"

// pdbFunctions is the upstream call chain every check in this package ports:
// ValidatePodDisruptionBudget delegates the whole spec to
// ValidatePodDisruptionBudgetSpec, which carries each individual rule.
var pdbFunctions = []string{"ValidatePodDisruptionBudget", "ValidatePodDisruptionBudgetSpec"}

// pdbDigest is the digest over pdbFunctions at validatedAt.
const pdbDigest = "sha256:f0d2898b603e66847a297d2dca92cf8bb1126edc77aa8b95c8bb3c429c04ebc4"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	"policy/selector-invalid": {
		Path:        policyValidationPath,
		Functions:   pdbFunctions,
		Digest:      pdbDigest,
		ValidatedAt: validatedAt,
		Note: "Ports the unversionedvalidation.ValidateLabelSelector(spec.Selector, ...) call in " +
			"ValidatePodDisruptionBudgetSpec. An absent selector is skipped rather than reported, " +
			"because upstream tolerates a nil selector here.",
	},
	"policy/min-available-invalid": {
		Path:        policyValidationPath,
		Functions:   pdbFunctions,
		Digest:      pdbDigest,
		ValidatedAt: validatedAt,
		Note: "Ports the appsvalidation.ValidatePositiveIntOrPercent(*spec.MinAvailable, ...) call in " +
			"ValidatePodDisruptionBudgetSpec, restricted to its non-negative-integer branch. The " +
			"percentage-format and IsNotMoreThan100Percent branches are not ported.",
	},
	"policy/max-unavailable-invalid": {
		Path:        policyValidationPath,
		Functions:   pdbFunctions,
		Digest:      pdbDigest,
		ValidatedAt: validatedAt,
		Note: "Ports the appsvalidation.ValidatePositiveIntOrPercent(*spec.MaxUnavailable, ...) call in " +
			"ValidatePodDisruptionBudgetSpec, restricted to its non-negative-integer branch. The " +
			"percentage-format and IsNotMoreThan100Percent branches are not ported.",
	},
	"policy/min-and-max-specified": {
		Path:        policyValidationPath,
		Functions:   pdbFunctions,
		Digest:      pdbDigest,
		ValidatedAt: validatedAt,
		Note:        "Ports the spec.MinAvailable != nil && spec.MaxUnavailable != nil -> \"minAvailable and maxUnavailable cannot be both set\" branch of ValidatePodDisruptionBudgetSpec.",
	},
}
