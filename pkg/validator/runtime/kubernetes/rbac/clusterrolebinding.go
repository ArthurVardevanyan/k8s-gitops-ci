package validation

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var clusterRoleBindingKinds = []string{"ClusterRoleBinding"}

// clusterRoleBindingRoleRefInvalidCheck validates that roleRef specifies a ClusterRole.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleBindingRoleRefInvalidCheck struct{ runtime.Meta }

func newClusterRoleBindingRoleRefInvalidCheck() clusterRoleBindingRoleRefInvalidCheck {
	return clusterRoleBindingRoleRefInvalidCheck{runtime.Meta{
		RuleID:    "rbac/clusterrole-ref-invalid",
		RuleTitle: "ClusterRoleBinding roleRef Must Reference a ClusterRole",
		AppliesTo: clusterRoleBindingKinds,
	}}
}

func (c clusterRoleBindingRoleRefInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crb rbacv1.ClusterRoleBinding
	if err := yaml.Unmarshal(data, &crb); err != nil {
		return nil
	}
	if crb.Kind != "ClusterRoleBinding" {
		return nil
	}

	var findings []runtime.Finding

	roleRefPath := field.NewPath("roleRef")

	if crb.RoleRef.Kind != "ClusterRole" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("kind").String(),
				Message: fmt.Sprintf("roleRef: invalid value: kind %q is not supported, must be ClusterRole", crb.RoleRef.Kind),
				Kind:    "ClusterRoleBinding",
				Name:    crb.GetName(),
			},
		})
	}

	if crb.RoleRef.Name == "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("name").String(),
				Message: "roleRef: invalid value: name is required",
				Kind:    "ClusterRoleBinding",
				Name:    crb.GetName(),
			},
		})
	}

	// SetDefaults_ClusterRoleBinding replaces an empty roleRef.apiGroup with
	// the rbac group before validation runs, so an explicitly-empty apiGroup
	// is accepted by the API server.
	if crb.RoleRef.APIGroup != "" && crb.RoleRef.APIGroup != rbacv1.GroupName {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("apiGroup").String(),
				Message: fmt.Sprintf("roleRef: invalid value: apiGroup %q does not match expected group %q", crb.RoleRef.APIGroup, rbacv1.GroupName),
				Kind:    "ClusterRoleBinding",
				Name:    crb.GetName(),
			},
		})
	}

	return findings
}

// clusterRoleBindingSubjectInvalidCheck validates each subject has valid kind and name.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleBindingSubjectInvalidCheck struct{ runtime.Meta }

func newClusterRoleBindingSubjectInvalidCheck() clusterRoleBindingSubjectInvalidCheck {
	return clusterRoleBindingSubjectInvalidCheck{runtime.Meta{
		RuleID:    "rbac/clusterrolebinding-subject-invalid",
		RuleTitle: "ClusterRoleBinding subjects Must Be Valid",
		AppliesTo: clusterRoleBindingKinds,
	}}
}

func (c clusterRoleBindingSubjectInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crb rbacv1.ClusterRoleBinding
	if err := yaml.Unmarshal(data, &crb); err != nil {
		return nil
	}
	if crb.Kind != "ClusterRoleBinding" {
		return nil
	}

	var findings []runtime.Finding

	validSubjectKinds := map[string]bool{
		"User":           true,
		"Group":          true,
		"ServiceAccount": true,
	}

	for i, subject := range crb.Subjects {
		subjectPath := field.NewPath("subjects").Index(i)

		if !validSubjectKinds[subject.Kind] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    subjectPath.Child("kind").String(),
					Message: fmt.Sprintf("subjects: invalid value: kind %q is not supported, must be User, Group, or ServiceAccount", subject.Kind),
					Kind:    "ClusterRoleBinding",
					Name:    crb.GetName(),
				},
			})
		}

		if subject.Name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    subjectPath.Child("name").String(),
					Message: "subjects: invalid value: name is required",
					Kind:    "ClusterRoleBinding",
					Name:    crb.GetName(),
				},
			})
		}
	}

	return findings
}
