package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// rbacValidationPath is pkg/apis/rbac/validation/validation.go in
// kubernetes/kubernetes, which holds the RoleBinding/ClusterRoleBinding rules
// ported here.
const rbacValidationPath = "pkg/apis/rbac/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.37.0"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	"rbac/role-ref-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateRoleBinding"},
		Digest:      "sha256:0edb35117d9aefca6833b8535206f9d4a4785c9ab105441ee06154e42c91ca0e",
		ValidatedAt: validatedAt,
		Note: "Ports the three roleRef branches of ValidateRoleBinding: apiGroup must be rbac.GroupName, " +
			"kind must be Role or ClusterRole, and name is Required. The ValidateRBACName check on a " +
			"non-empty roleRef.name and the per-subject branches are not ported here.",
	},
	"rbac/clusterrole-ref-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateClusterRoleBinding"},
		Digest:      "sha256:759c184539ae73e38a5e224dc77a81d23b19daaf6157ce4cdf1e0ecc499c9963",
		ValidatedAt: validatedAt,
		Note: "Ports the three roleRef branches of ValidateClusterRoleBinding: apiGroup must be " +
			"rbac.GroupName, kind must be ClusterRole (a ClusterRoleBinding may not reference a " +
			"namespaced Role), and name is Required. The ValidateRBACName check on a non-empty " +
			"roleRef.name and the per-subject branches are not ported here.",
	},
	"rbac/clusterrolebinding-subject-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateClusterRoleBinding", "ValidateRoleBindingSubject"},
		Digest:      "sha256:d025704ba9ca71753122018db9d68df9ac990e45d7360c51350b2dbd2daa042a",
		ValidatedAt: validatedAt,
		Note: "Ports the subject branches reached from ValidateClusterRoleBinding's " +
			"ValidateRoleBindingSubject loop: the default: field.NotSupported branch on subject.kind " +
			"(ServiceAccount, User or Group) and the len(subject.Name) == 0 -> field.Required branch. " +
			"The per-kind apiGroup, ServiceAccount name and cluster-scoped namespace-required branches " +
			"are not ported.",
	},
}
