package apiextensions

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// apiextensionsValidationPath is the CustomResourceDefinition validation in the
// apiextensions-apiserver staging module of kubernetes/kubernetes.
const apiextensionsValidationPath = "staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.37.0"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	"kubernetes/apiextensions/crd-storage-version-invalid": {
		Path:        apiextensionsValidationPath,
		Functions:   []string{"validateCustomResourceDefinitionSpec"},
		Digest:      "sha256:e8754bbba288215d52d4c15a9c0dde392aa03768abeead7eb0bdd302be21d828",
		ValidatedAt: validatedAt,
		Note: "Ports the storageFlagCount != 1 -> \"must have exactly one version marked as storage " +
			"version\" branch. Upstream reports one error on spec.versions for any count other than 1; " +
			"this check reports the zero case on spec.versions and each extra storage version on its own " +
			"spec.versions[i].storage path so the offending versions are named. The other spec branches " +
			"(names, scope, schema, conversion) are not ported.",
	},
}
