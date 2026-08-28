package validation

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var roleKinds = []string{"Role"}

// nonResourceURLInvalidCheck validates that nonResourceURLs are valid paths
// starting with /.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type nonResourceURLInvalidCheck struct{}

func (c nonResourceURLInvalidCheck) ID() string {
	return "rbac/non-resource-url-invalid"
}

func (c nonResourceURLInvalidCheck) Title() string {
	return "Role nonResourceURLs Must Start With /"
}

func (c nonResourceURLInvalidCheck) Category() string {
	return "rbac"
}

func (c nonResourceURLInvalidCheck) Blocking() bool {
	return true
}

func (c nonResourceURLInvalidCheck) RenderSensitive() bool {
	return true
}

func (c nonResourceURLInvalidCheck) DocSkipper() []string {
	return roleKinds
}

func (c nonResourceURLInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var role rbacv1.Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range role.Rules {
		for j, url := range rule.NonResourceURLs {
			if url == "" {
				continue
			}
			if url[0] != '/' {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("rules").Index(i).Child("nonResourceURLs").Index(j).String(),
						Message: fmt.Sprintf("nonResourceURLs: invalid value: %q: must start with /", url),
						Kind:    "Role",
						Name:    role.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// verbsInvalidCheck validates that verbs are not wildcards when used with
// nonResourceURLs (per Kubernetes admission rules).
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type verbsInvalidCheck struct{}

func (c verbsInvalidCheck) ID() string {
	return "rbac/verbs-invalid"
}

func (c verbsInvalidCheck) Title() string {
	return "Role Verbs With nonResourceURLs Must Be Valid"
}

func (c verbsInvalidCheck) Category() string {
	return "rbac"
}

func (c verbsInvalidCheck) Blocking() bool {
	return true
}

func (c verbsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c verbsInvalidCheck) DocSkipper() []string {
	return roleKinds
}

func (c verbsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var role rbacv1.Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range role.Rules {
		if len(rule.NonResourceURLs) == 0 {
			continue
		}
		hasWildcard := false
		for _, v := range rule.Verbs {
			if v == "*" {
				hasWildcard = true
				break
			}
		}
		if hasWildcard {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("rules").Index(i).Child("verbs").String(),
					Message: "verbs: invalid value: wildcard verbs are not allowed with nonResourceURLs",
					Kind:    "Role",
					Name:    role.GetName(),
				},
			})
		}
	}

	return findings
}

// resourceNameInvalidCheck validates that resourceNames are valid names.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type resourceNameInvalidCheck struct{}

func (c resourceNameInvalidCheck) ID() string {
	return "rbac/resource-name-invalid"
}

func (c resourceNameInvalidCheck) Title() string {
	return "Role resourceNames Must Be Valid Names"
}

func (c resourceNameInvalidCheck) Category() string {
	return "rbac"
}

func (c resourceNameInvalidCheck) Blocking() bool {
	return true
}

func (c resourceNameInvalidCheck) RenderSensitive() bool {
	return true
}

func (c resourceNameInvalidCheck) DocSkipper() []string {
	return roleKinds
}

func (c resourceNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var role rbacv1.Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range role.Rules {
		for j, name := range rule.ResourceNames {
			if name == "" {
				continue
			}
			if len(name) > 253 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("rules").Index(i).Child("resourceNames").Index(j).String(),
						Message: fmt.Sprintf("resourceNames: invalid value: %q: must be a valid name (max 253 characters)", name),
						Kind:    "Role",
						Name:    role.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// rulesEmptyCheck validates that rules are not empty.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type rulesEmptyCheck struct{}

func (c rulesEmptyCheck) ID() string {
	return "rbac/rules-empty"
}

func (c rulesEmptyCheck) Title() string {
	return "Role Must Have At Least One Rule"
}

func (c rulesEmptyCheck) Category() string {
	return "rbac"
}

func (c rulesEmptyCheck) Blocking() bool {
	return true
}

func (c rulesEmptyCheck) RenderSensitive() bool {
	return true
}

func (c rulesEmptyCheck) DocSkipper() []string {
	return roleKinds
}

func (c rulesEmptyCheck) Run(data []byte, source string) []runtime.Finding {
	var role rbacv1.Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if len(role.Rules) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("rules").String(),
				Message: "rules: must have at least one rule",
				Kind:    "Role",
				Name:    role.GetName(),
			},
		})
	}

	return findings
}

// apiGroupInvalidCheck validates that apiGroups are valid values.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type apiGroupInvalidCheck struct{}

func (c apiGroupInvalidCheck) ID() string {
	return "rbac/api-group-invalid"
}

func (c apiGroupInvalidCheck) Title() string {
	return "Role apiGroups Must Be Valid"
}

func (c apiGroupInvalidCheck) Category() string {
	return "rbac"
}

func (c apiGroupInvalidCheck) Blocking() bool {
	return true
}

func (c apiGroupInvalidCheck) RenderSensitive() bool {
	return true
}

func (c apiGroupInvalidCheck) DocSkipper() []string {
	return roleKinds
}

func (c apiGroupInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var role rbacv1.Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range role.Rules {
		hasAPIGroups := len(rule.APIGroups) > 0
		hasNonResourceURLs := len(rule.NonResourceURLs) > 0
		hasResources := len(rule.Resources) > 0

		if hasNonResourceURLs && hasAPIGroups {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("rules").Index(i).Child("apiGroups").String(),
					Message: "apiGroups: invalid value: cannot specify apiGroups with nonResourceURLs",
					Kind:    "Role",
					Name:    role.GetName(),
				},
			})
		}

		if hasNonResourceURLs && hasResources {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("rules").Index(i).Child("apiGroups").String(),
					Message: "apiGroups: invalid value: cannot specify apiGroups with nonResourceURLs",
					Kind:    "Role",
					Name:    role.GetName(),
				},
			})
		}
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		nonResourceURLInvalidCheck{},
		verbsInvalidCheck{},
		resourceNameInvalidCheck{},
		rulesEmptyCheck{},
		apiGroupInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
