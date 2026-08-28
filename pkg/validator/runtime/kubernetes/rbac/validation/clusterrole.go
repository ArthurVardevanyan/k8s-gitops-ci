package validation

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var clusterRoleKinds = []string{"ClusterRole"}

// clusterRoleNonResourceURLInvalidCheck validates that nonResourceURLs are valid paths
// starting with /.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleNonResourceURLInvalidCheck struct{}

func (c clusterRoleNonResourceURLInvalidCheck) ID() string {
	return "rbac/clusterrole-non-resource-url-invalid"
}

func (c clusterRoleNonResourceURLInvalidCheck) Title() string {
	return "ClusterRole nonResourceURLs Must Start With /"
}

func (c clusterRoleNonResourceURLInvalidCheck) Category() string {
	return "rbac"
}

func (c clusterRoleNonResourceURLInvalidCheck) Blocking() bool {
	return true
}

func (c clusterRoleNonResourceURLInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleNonResourceURLInvalidCheck) DocSkipper() []string {
	return clusterRoleKinds
}

func (c clusterRoleNonResourceURLInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cr rbacv1.ClusterRole
	if err := yaml.Unmarshal(data, &cr); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range cr.Rules {
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
						Kind:    "ClusterRole",
						Name:    cr.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// clusterRoleVerbsInvalidCheck validates that verbs are not wildcards when used with
// nonResourceURLs.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleVerbsInvalidCheck struct{}

func (c clusterRoleVerbsInvalidCheck) ID() string {
	return "rbac/clusterrole-verbs-invalid"
}

func (c clusterRoleVerbsInvalidCheck) Title() string {
	return "ClusterRole Verbs With nonResourceURLs Must Be Valid"
}

func (c clusterRoleVerbsInvalidCheck) Category() string {
	return "rbac"
}

func (c clusterRoleVerbsInvalidCheck) Blocking() bool {
	return true
}

func (c clusterRoleVerbsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleVerbsInvalidCheck) DocSkipper() []string {
	return clusterRoleKinds
}

func (c clusterRoleVerbsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cr rbacv1.ClusterRole
	if err := yaml.Unmarshal(data, &cr); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range cr.Rules {
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
					Kind:    "ClusterRole",
					Name:    cr.GetName(),
				},
			})
		}
	}

	return findings
}

// clusterRoleResourceNameInvalidCheck validates that resourceNames are valid names.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleResourceNameInvalidCheck struct{}

func (c clusterRoleResourceNameInvalidCheck) ID() string {
	return "rbac/clusterrole-resource-name-invalid"
}

func (c clusterRoleResourceNameInvalidCheck) Title() string {
	return "ClusterRole resourceNames Must Be Valid Names"
}

func (c clusterRoleResourceNameInvalidCheck) Category() string {
	return "rbac"
}

func (c clusterRoleResourceNameInvalidCheck) Blocking() bool {
	return true
}

func (c clusterRoleResourceNameInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleResourceNameInvalidCheck) DocSkipper() []string {
	return clusterRoleKinds
}

func (c clusterRoleResourceNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cr rbacv1.ClusterRole
	if err := yaml.Unmarshal(data, &cr); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range cr.Rules {
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
						Kind:    "ClusterRole",
						Name:    cr.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// clusterRoleRulesEmptyCheck validates that rules are not empty.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleRulesEmptyCheck struct{}

func (c clusterRoleRulesEmptyCheck) ID() string {
	return "rbac/clusterrole-rules-empty"
}

func (c clusterRoleRulesEmptyCheck) Title() string {
	return "ClusterRole Must Have At Least One Rule"
}

func (c clusterRoleRulesEmptyCheck) Category() string {
	return "rbac"
}

func (c clusterRoleRulesEmptyCheck) Blocking() bool {
	return true
}

func (c clusterRoleRulesEmptyCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleRulesEmptyCheck) DocSkipper() []string {
	return clusterRoleKinds
}

func (c clusterRoleRulesEmptyCheck) Run(data []byte, source string) []runtime.Finding {
	var cr rbacv1.ClusterRole
	if err := yaml.Unmarshal(data, &cr); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if len(cr.Rules) == 0 {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("rules").String(),
				Message: "rules: must have at least one rule",
				Kind:    "ClusterRole",
				Name:    cr.GetName(),
			},
		})
	}

	return findings
}

// clusterRoleAPIGroupInvalidCheck validates that apiGroups are valid values.
// Source: k8s.io/kubernetes/pkg/apis/rbac/validation/validation.go
type clusterRoleAPIGroupInvalidCheck struct{}

func (c clusterRoleAPIGroupInvalidCheck) ID() string {
	return "rbac/clusterrole-api-group-invalid"
}

func (c clusterRoleAPIGroupInvalidCheck) Title() string {
	return "ClusterRole apiGroups Must Be Valid"
}

func (c clusterRoleAPIGroupInvalidCheck) Category() string {
	return "rbac"
}

func (c clusterRoleAPIGroupInvalidCheck) Blocking() bool {
	return true
}

func (c clusterRoleAPIGroupInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterRoleAPIGroupInvalidCheck) DocSkipper() []string {
	return clusterRoleKinds
}

func (c clusterRoleAPIGroupInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cr rbacv1.ClusterRole
	if err := yaml.Unmarshal(data, &cr); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range cr.Rules {
		hasAPIGroups := len(rule.APIGroups) > 0
		hasNonResourceURLs := len(rule.NonResourceURLs) > 0
		hasResources := len(rule.Resources) > 0

		if hasNonResourceURLs && hasAPIGroups {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("rules").Index(i).Child("apiGroups").String(),
					Message: "apiGroups: invalid value: cannot specify apiGroups with nonResourceURLs",
					Kind:    "ClusterRole",
					Name:    cr.GetName(),
				},
			})
		}

		if hasNonResourceURLs && hasResources {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("rules").Index(i).Child("apiGroups").String(),
					Message: "apiGroups: invalid value: cannot specify apiGroups with nonResourceURLs",
					Kind:    "ClusterRole",
					Name:    cr.GetName(),
				},
			})
		}
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		clusterRoleNonResourceURLInvalidCheck{},
		clusterRoleVerbsInvalidCheck{},
		clusterRoleResourceNameInvalidCheck{},
		clusterRoleRulesEmptyCheck{},
		clusterRoleAPIGroupInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
