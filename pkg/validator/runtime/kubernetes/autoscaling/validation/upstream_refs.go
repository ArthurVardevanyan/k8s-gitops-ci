package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// autoscalingValidationPath is pkg/apis/autoscaling/validation/validation.go
// in kubernetes/kubernetes, which holds every rule ported by this package.
const autoscalingValidationPath = "pkg/apis/autoscaling/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken
// at. It matches the tag derived from go.mod that
// `task verify:upstream-refs` pins to.
const validatedAt = "v1.36.3"

// scalingRulesNote is shared by the scaleUp/scaleDown checks: both port the
// same upstream function, which upstream reaches twice from validateBehavior
// (once per direction).
const scalingRulesNote = "Ports the StabilizationWindowSeconds < 0 branch of validateScalingRules, " +
	"which upstream reaches once per direction from validateBehavior " +
	"(behavior.scaleUp and behavior.scaleDown). The MaxStabilizationWindowSeconds upper-bound, " +
	"selectPolicy, policies and tolerance branches of the same function are not ported."

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	"autoscaling/max-replicas-invalid": {
		Path:        autoscalingValidationPath,
		Functions:   []string{"validateHorizontalPodAutoscalerSpec"},
		Digest:      "sha256:40627b2929c0f43c2e5fbb518c020ae43850124bbdc2a8a10fb66a10618f25f9",
		ValidatedAt: validatedAt,
		Note: "Ports the spec.maxReplicas must-be-greater-than-0 rule. Deliberate divergence: " +
			"a missing spec.maxReplicas is not reported here even though upstream reports it " +
			"Required, because maxReplicas is in the `required` array of every HPA schema variant " +
			"and kubeconform already rejects it; reporting it here would double-report. " +
			"The minReplicas lower-bound and maxReplicas >= minReplicas branches are not ported.",
	},
	"autoscaling/scale-up-invalid": {
		Path:        autoscalingValidationPath,
		Functions:   []string{"validateScalingRules"},
		Digest:      "sha256:5822565c8971b326f431e10e1a0532985a8c6b0c6249d23a2a48fcab24c66b43",
		ValidatedAt: validatedAt,
		Note:        scalingRulesNote,
	},
	"autoscaling/scale-down-invalid": {
		Path:        autoscalingValidationPath,
		Functions:   []string{"validateScalingRules"},
		Digest:      "sha256:5822565c8971b326f431e10e1a0532985a8c6b0c6249d23a2a48fcab24c66b43",
		ValidatedAt: validatedAt,
		Note:        scalingRulesNote,
	},
}
