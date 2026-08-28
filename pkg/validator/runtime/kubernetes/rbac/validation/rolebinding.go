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
type roleBindingRoleRefInvalidCheck struct{}

func (c roleBindingRoleRefInvalidCheck) ID() string {
	return "rbac/role-ref-invalid"
}

func (c roleBindingRoleRefInvalidCheck) Title() string {
	return "RoleBinding roleRef Must Be Valid"
}

func (c roleBindingRoleRefInvalidCheck) Category() string {
	return "rbac"
}

func (c roleBindingRoleRefInvalidCheck) Blocking() bool {
	return true
}

func (c roleBindingRoleRefInvalidCheck) RenderSensitive() bool {
	return true
}

func (c roleBindingRoleRefInvalidCheck) DocSkipper() []string {
	return roleBindingKinds
}

func (c roleBindingRoleRefInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal(data, &rb); err != nil {
		return nil
	}

	var findings []runtime.Finding

	roleRefPath := field.NewPath("roleRef")

	if rb.RoleRef.Kind != "Role" && rb.RoleRef.Kind != "ClusterRole" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
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
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    roleRefPath.Child("name").String(),
				Message: "roleRef: invalid value: name is required",
				Kind:    "RoleBinding",
				Name:    rb.GetName(),
			},
		})
	}

	if rb.RoleRef.APIGroup != rbacv1.GroupName {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
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

// roleBindingSubjectsInvalidCheck validates that subjects are not empty.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type roleBindingSubjectsInvalidCheck struct{}

func (c roleBindingSubjectsInvalidCheck) ID() string {
	return "rbac/subjects-invalid"
}

func (c roleBindingSubjectsInvalidCheck) Title() string {
	return "RoleBinding subjects Must Not Be Empty"
}

func (c roleBindingSubjectsInvalidCheck) Category() string {
	return "rbac"
}

func (c roleBindingSubjectsInvalidCheck) Blocking() bool {
	return true
}

func (c roleBindingSubjectsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c roleBindingSubjectsInvalidCheck) DocSkipper() []string {
	return roleBindingKinds
}

func (c roleBindingSubjectsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal(data, &rb); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if len(rb.Subjects) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("subjects").String(),
				Message: "subjects: must not be empty",
				Kind:    "RoleBinding",
				Name:    rb.GetName(),
			},
		})
	}

	return findings
}

// roleBindingNamespaceInvalidCheck validates that namespace is required when
// roleRef is a Role.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type roleBindingNamespaceInvalidCheck struct{}

func (c roleBindingNamespaceInvalidCheck) ID() string {
	return "rbac/namespace-invalid"
}

func (c roleBindingNamespaceInvalidCheck) Title() string {
	return "RoleBinding namespace Required When roleRef Is Role"
}

func (c roleBindingNamespaceInvalidCheck) Category() string {
	return "rbac"
}

func (c roleBindingNamespaceInvalidCheck) Blocking() bool {
	return true
}

func (c roleBindingNamespaceInvalidCheck) RenderSensitive() bool {
	return true
}

func (c roleBindingNamespaceInvalidCheck) DocSkipper() []string {
	return roleBindingKinds
}

func (c roleBindingNamespaceInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal(data, &rb); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if rb.RoleRef.Kind == "Role" && rb.Namespace == "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("metadata").Child("namespace").String(),
				Message: "namespace: required when roleRef is Role",
				Kind:    "RoleBinding",
				Name:    rb.GetName(),
			},
		})
	}

	return findings
}

// roleBindingSubjectInvalidCheck validates each subject has valid kind, name, and namespace.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type roleBindingSubjectInvalidCheck struct{}

func (c roleBindingSubjectInvalidCheck) ID() string {
	return "rbac/subject-invalid"
}

func (c roleBindingSubjectInvalidCheck) Title() string {
	return "RoleBinding subjects Must Be Valid"
}

func (c roleBindingSubjectInvalidCheck) Category() string {
	return "rbac"
}

func (c roleBindingSubjectInvalidCheck) Blocking() bool {
	return true
}

func (c roleBindingSubjectInvalidCheck) RenderSensitive() bool {
	return true
}

func (c roleBindingSubjectInvalidCheck) DocSkipper() []string {
	return roleBindingKinds
}

func (c roleBindingSubjectInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal(data, &rb); err != nil {
		return nil
	}

	var findings []runtime.Finding

	validSubjectKinds := map[string]bool{
		"User":           true,
		"Group":          true,
		"ServiceAccount": true,
	}

	for i, subject := range rb.Subjects {
		subjectPath := field.NewPath("subjects").Index(i)

		if !validSubjectKinds[subject.Kind] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    subjectPath.Child("kind").String(),
					Message: fmt.Sprintf("subjects: invalid value: kind %q is not supported, must be User, Group, or ServiceAccount", subject.Kind),
					Kind:    "RoleBinding",
					Name:    rb.GetName(),
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
					Kind:    "RoleBinding",
					Name:    rb.GetName(),
				},
			})
		}

		if subject.Kind == "ServiceAccount" && subject.Namespace == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    subjectPath.Child("namespace").String(),
					Message: "subjects: invalid value: namespace is required for ServiceAccount subjects",
					Kind:    "RoleBinding",
					Name:    rb.GetName(),
				},
			})
		}
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		roleBindingRoleRefInvalidCheck{},
		roleBindingSubjectsInvalidCheck{},
		roleBindingNamespaceInvalidCheck{},
		roleBindingSubjectInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
