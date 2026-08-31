package validation

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var roleBindingKinds = []string{"RoleBinding"}

// roleBindingRoleRefInvalidCheck validates that roleRef specifies a valid Role or ClusterRole.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type roleBindingRoleRefInvalidCheck struct{ runtime.Meta }

func newRoleBindingRoleRefInvalidCheck() roleBindingRoleRefInvalidCheck {
	return roleBindingRoleRefInvalidCheck{runtime.Meta{
		RuleID:    "rbac/role-ref-invalid",
		RuleTitle: "RoleBinding roleRef Must Be Valid",
		AppliesTo: roleBindingKinds,
	}}
}

func (c roleBindingRoleRefInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal(data, &rb); err != nil {
		return nil
	}
	if rb.Kind != "RoleBinding" {
		return nil
	}

	var findings []runtime.Finding

	roleRefPath := field.NewPath("roleRef")

	if rb.RoleRef.Kind != "Role" && rb.RoleRef.Kind != "ClusterRole" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("kind").String(),
				Message: fmt.Sprintf("roleRef: invalid value: kind %q is not supported, must be Role or ClusterRole", rb.RoleRef.Kind),
				Kind:    "RoleBinding",
				Name:    rb.GetName(),
			},
		})
	}

	if rb.RoleRef.Name == "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("name").String(),
				Message: "roleRef: invalid value: name is required",
				Kind:    "RoleBinding",
				Name:    rb.GetName(),
			},
		})
	}

	// SetDefaults_RoleBinding/SetDefaults_ClusterRoleBinding replace an empty
	// roleRef.apiGroup with the rbac group before validation runs, so an
	// explicitly-empty apiGroup is accepted by the API server.
	if rb.RoleRef.APIGroup != "" && rb.RoleRef.APIGroup != rbacv1.GroupName {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("apiGroup").String(),
				Message: fmt.Sprintf("roleRef: invalid value: apiGroup %q does not match expected group %q", rb.RoleRef.APIGroup, rbacv1.GroupName),
				Kind:    "RoleBinding",
				Name:    rb.GetName(),
			},
		})
	}

	return findings
}
