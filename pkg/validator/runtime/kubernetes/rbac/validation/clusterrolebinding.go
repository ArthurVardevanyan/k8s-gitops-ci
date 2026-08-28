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
type clusterRoleBindingRoleRefInvalidCheck struct{}

func (c clusterRoleBindingRoleRefInvalidCheck) ID() string {
	return "rbac/clusterrole-ref-invalid"
}

func (c clusterRoleBindingRoleRefInvalidCheck) Title() string {
	return "ClusterRoleBinding roleRef Must Reference a ClusterRole"
}

func (c clusterRoleBindingRoleRefInvalidCheck) Category() string {
	return "rbac"
}

func (c clusterRoleBindingRoleRefInvalidCheck) Blocking() bool {
	return true
}

func (c clusterRoleBindingRoleRefInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleBindingRoleRefInvalidCheck) Kinds() []string {
	return clusterRoleBindingKinds
}

func (c clusterRoleBindingRoleRefInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crb rbacv1.ClusterRoleBinding
	if err := yaml.Unmarshal(data, &crb); err != nil {
		return nil
	}

	var findings []runtime.Finding

	roleRefPath := field.NewPath("roleRef")

	if crb.RoleRef.Kind != "ClusterRole" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
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
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("name").String(),
				Message: "roleRef: invalid value: name is required",
				Kind:    "ClusterRoleBinding",
				Name:    crb.GetName(),
			},
		})
	}

	if crb.RoleRef.APIGroup != rbacv1.GroupName {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
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
type clusterRoleBindingSubjectInvalidCheck struct{}

func (c clusterRoleBindingSubjectInvalidCheck) ID() string {
	return "rbac/clusterrolebinding-subject-invalid"
}

func (c clusterRoleBindingSubjectInvalidCheck) Title() string {
	return "ClusterRoleBinding subjects Must Be Valid"
}

func (c clusterRoleBindingSubjectInvalidCheck) Category() string {
	return "rbac"
}

func (c clusterRoleBindingSubjectInvalidCheck) Blocking() bool {
	return true
}

func (c clusterRoleBindingSubjectInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleBindingSubjectInvalidCheck) Kinds() []string {
	return clusterRoleBindingKinds
}

func (c clusterRoleBindingSubjectInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var crb rbacv1.ClusterRoleBinding
	if err := yaml.Unmarshal(data, &crb); err != nil {
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
				Category:  c.Category(),
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
				Category:  c.Category(),
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

func init() {
	checks := []runtime.Check{
		clusterRoleBindingRoleRefInvalidCheck{},
		clusterRoleBindingSubjectInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
