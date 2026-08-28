package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// rbacValidationPath is pkg/apis/rbac/validation/validation.go in
// kubernetes/kubernetes, which holds the RoleBinding/ClusterRoleBinding rules
// ported here.
const rbacValidationPath = "pkg/apis/rbac/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken at.
const validatedAt = "v1.36.3"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	"rbac/role-ref-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateRoleBinding"},
		Digest:      "sha256:1a1f1a247e55bfdc3a2c6ab8bce3a424bf86a4eaf82eb3d6ae1bab604fd15ea6",
		ValidatedAt: validatedAt,
		Note: "Ports the three roleRef branches of ValidateRoleBinding: apiGroup must be rbac.GroupName, " +
			"kind must be Role or ClusterRole, and name is Required. The ValidateRBACName check on a " +
			"non-empty roleRef.name and the per-subject branches are not ported here.",
	},
	"rbac/clusterrole-ref-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateClusterRoleBinding"},
		Digest:      "sha256:5d2bd79ebe82acc461f4b8a91e73f1df7c69aa950158b7f8bbf2ba5e7a34fbaa",
		ValidatedAt: validatedAt,
		Note: "Ports the three roleRef branches of ValidateClusterRoleBinding: apiGroup must be " +
			"rbac.GroupName, kind must be ClusterRole (a ClusterRoleBinding may not reference a " +
			"namespaced Role), and name is Required. The ValidateRBACName check on a non-empty " +
			"roleRef.name and the per-subject branches are not ported here.",
	},
	"rbac/clusterrolebinding-subject-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateClusterRoleBinding", "ValidateRoleBindingSubject"},
		Digest:      "sha256:d1ec0a8da15d9b7a0d76804dca6104d68465cb8a1e9f3b7921ffe9e295172c77",
		ValidatedAt: validatedAt,
		Note: "Ports the subject branches reached from ValidateClusterRoleBinding's " +
			"ValidateRoleBindingSubject loop: the default: field.NotSupported branch on subject.kind " +
			"(ServiceAccount, User or Group) and the len(subject.Name) == 0 -> field.Required branch. " +
			"The per-kind apiGroup, ServiceAccount name and cluster-scoped namespace-required branches " +
			"are not ported.",
	},
}
