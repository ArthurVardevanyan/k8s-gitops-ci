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
		Digest:      "sha256:191879d6004f1eca9c8d862d162a98bab11ebcb2ece40c8682f151421311d40b",
		ValidatedAt: validatedAt,
		Note: "Ports the three roleRef branches of ValidateRoleBinding: apiGroup must be rbac.GroupName, " +
			"kind must be Role or ClusterRole, and name is Required. The ValidateRBACName check on a " +
			"non-empty roleRef.name and the per-subject branches are not ported here. Deliberate " +
			"divergence: an explicitly-empty apiGroup is accepted, because SetDefaults_RoleBinding " +
			"replaces it with the rbac group before validation runs.",
		Additional: []runtime.UpstreamRef{{
			Path:        "pkg/apis/rbac/v1/defaults.go",
			Functions:   []string{"SetDefaults_RoleBinding"},
			Digest:      "sha256:0a0c780a9d5ae4eb93c729f541e4d5fc56f50077c0161e2fd965fc61d8b00ec6",
			ValidatedAt: validatedAt,
			Note: "Supplies the apiGroup this rule tests. Without it the check reports every " +
				"binding written with an explicitly-empty roleRef.apiGroup, which the API server accepts.",
		}},
	},
	"rbac/clusterrole-ref-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateClusterRoleBinding"},
		Digest:      "sha256:5f3744c1af9fad177a77630c270b6a010d1efc3e9de2449f0b415870d34da6d0",
		ValidatedAt: validatedAt,
		Note: "Ports the three roleRef branches of ValidateClusterRoleBinding: apiGroup must be " +
			"rbac.GroupName, kind must be ClusterRole (a ClusterRoleBinding may not reference a " +
			"namespaced Role), and name is Required. The ValidateRBACName check on a non-empty " +
			"roleRef.name and the per-subject branches are not ported here. Deliberate divergence: " +
			"an explicitly-empty apiGroup is accepted, because SetDefaults_ClusterRoleBinding " +
			"replaces it with the rbac group before validation runs.",
		Additional: []runtime.UpstreamRef{{
			Path:        "pkg/apis/rbac/v1/defaults.go",
			Functions:   []string{"SetDefaults_ClusterRoleBinding"},
			Digest:      "sha256:4e31786f30e8e5d304316df07d5e09d3af85a39d4f67eca41e8518736a0ea84b",
			ValidatedAt: validatedAt,
			Note: "Supplies the apiGroup this rule tests. Without it the check reports every " +
				"binding written with an explicitly-empty roleRef.apiGroup, which the API server accepts.",
		}},
	},
	"rbac/clusterrolebinding-subject-invalid": {
		Path:        rbacValidationPath,
		Functions:   []string{"ValidateClusterRoleBinding", "ValidateRoleBindingSubject"},
		Digest:      "sha256:b5b3e38b103d7aacd762ef12cc3a4aa88d429b18750101df5d8d004a88d59fe3",
		ValidatedAt: validatedAt,
		Note: "Ports the subject branches reached from ValidateClusterRoleBinding's " +
			"ValidateRoleBindingSubject loop: the default: field.NotSupported branch on subject.kind " +
			"(ServiceAccount, User or Group) and the len(subject.Name) == 0 -> field.Required branch. " +
			"The per-kind apiGroup, ServiceAccount name and cluster-scoped namespace-required branches " +
			"are not ported.",
	},
}
