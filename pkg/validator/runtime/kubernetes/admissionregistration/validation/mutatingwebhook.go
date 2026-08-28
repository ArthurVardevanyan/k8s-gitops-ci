package validation

import (
	"fmt"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var mutatingWebhookKinds = []string{"MutatingWebhookConfiguration"}

// webhookNameInvalidCheck validates that each webhook has a valid DNS subdomain name.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type webhookNameInvalidCheck struct{}

func (c webhookNameInvalidCheck) ID() string {
	return "admissionregistration/webhook-name-invalid"
}

func (c webhookNameInvalidCheck) Title() string {
	return "Webhook Name Must Be Valid DNS Subdomain"
}

func (c webhookNameInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c webhookNameInvalidCheck) Blocking() bool {
	return true
}

func (c webhookNameInvalidCheck) RenderSensitive() bool {
	return true
}

func (c webhookNameInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c webhookNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		namePath := webhookPath.Child("name")
		name := webhook.Name

		if !isValidDNS1123Subdomain(name) {
			msg := "webhook: Invalid value: "
			if len(name) > 0 {
				msg += fmt.Sprintf("%q must be a valid DNS subdomain", name)
			} else {
				msg += "must be a valid DNS subdomain"
			}
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    namePath.String(),
					Message: msg,
					Kind:    "MutatingWebhookConfiguration",
					Name:    mwc.GetName(),
				},
			})
		}
	}

	return findings
}

// clientConfigInvalidCheck validates that clientConfig specifies either url or service.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type clientConfigInvalidCheck struct{}

func (c clientConfigInvalidCheck) ID() string {
	return "admissionregistration/client-config-invalid"
}

func (c clientConfigInvalidCheck) Title() string {
	return "Webhook ClientConfig Must Specify URL or Service"
}

func (c clientConfigInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c clientConfigInvalidCheck) Blocking() bool {
	return true
}

func (c clientConfigInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clientConfigInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c clientConfigInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		clientConfigPath := webhookPath.Child("clientConfig")

		if webhook.ClientConfig.URL == nil && webhook.ClientConfig.Service == nil {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    clientConfigPath.String(),
					Message: "clientConfig: must specify either URL or service",
					Kind:    "MutatingWebhookConfiguration",
					Name:    mwc.GetName(),
				},
			})
		}
	}

	return findings
}

// serviceInvalidCheck validates that service reference is valid when specified.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type serviceInvalidCheck struct{}

func (c serviceInvalidCheck) ID() string {
	return "admissionregistration/service-invalid"
}

func (c serviceInvalidCheck) Title() string {
	return "Webhook Service Reference Must Be Valid"
}

func (c serviceInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c serviceInvalidCheck) Blocking() bool {
	return true
}

func (c serviceInvalidCheck) RenderSensitive() bool {
	return true
}

func (c serviceInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c serviceInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		clientConfigPath := webhookPath.Child("clientConfig")

		if webhook.ClientConfig.Service == nil {
			continue
		}

		svc := webhook.ClientConfig.Service
		svcPath := clientConfigPath.Child("service")

		if svc.Name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    svcPath.Child("name").String(),
					Message: "service: invalid value: name must not be empty",
					Kind:    "MutatingWebhookConfiguration",
					Name:    mwc.GetName(),
				},
			})
		}
	}

	return findings
}

// failurePolicyInvalidCheck validates failurePolicy is a recognized value.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type failurePolicyInvalidCheck struct{}

func (c failurePolicyInvalidCheck) ID() string {
	return "admissionregistration/failure-policy-invalid"
}

func (c failurePolicyInvalidCheck) Title() string {
	return "Webhook FailurePolicy Must Be Valid"
}

func (c failurePolicyInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c failurePolicyInvalidCheck) Blocking() bool {
	return true
}

func (c failurePolicyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c failurePolicyInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c failurePolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validFailurePolicies := map[admissionregistrationv1.FailurePolicyType]bool{
		admissionregistrationv1.Ignore: true,
		admissionregistrationv1.Fail:   true,
	}

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		path := webhookPath.Child("failurePolicy")

		if webhook.FailurePolicy != nil && !validFailurePolicies[*webhook.FailurePolicy] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("failurePolicy: Unsupported value: %q", string(*webhook.FailurePolicy)),
					Kind:    "MutatingWebhookConfiguration",
					Name:    mwc.GetName(),
				},
			})
		}
	}

	return findings
}

// sideEffectsInvalidCheck validates sideEffects is a recognized value.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type sideEffectsInvalidCheck struct{}

func (c sideEffectsInvalidCheck) ID() string {
	return "admissionregistration/side-effects-invalid"
}

func (c sideEffectsInvalidCheck) Title() string {
	return "Webhook SideEffects Must Be Valid"
}

func (c sideEffectsInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c sideEffectsInvalidCheck) Blocking() bool {
	return true
}

func (c sideEffectsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c sideEffectsInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c sideEffectsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validSideEffects := map[admissionregistrationv1.SideEffectClass]bool{
		admissionregistrationv1.SideEffectClassUnknown:      true,
		admissionregistrationv1.SideEffectClassNone:         true,
		admissionregistrationv1.SideEffectClassNoneOnDryRun: true,
		admissionregistrationv1.SideEffectClassSome:         true,
	}

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		path := webhookPath.Child("sideEffects")

		if webhook.SideEffects != nil && !validSideEffects[*webhook.SideEffects] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("sideEffects: Unsupported value: %q", string(*webhook.SideEffects)),
					Kind:    "MutatingWebhookConfiguration",
					Name:    mwc.GetName(),
				},
			})
		}
	}

	return findings
}

// timeoutInvalidCheck validates timeoutSeconds is within the allowed range.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type timeoutInvalidCheck struct{}

func (c timeoutInvalidCheck) ID() string {
	return "admissionregistration/timeout-invalid"
}

func (c timeoutInvalidCheck) Title() string {
	return "Webhook TimeoutSeconds Must Be Between 1 and 30"
}

func (c timeoutInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c timeoutInvalidCheck) Blocking() bool {
	return true
}

func (c timeoutInvalidCheck) RenderSensitive() bool {
	return true
}

func (c timeoutInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c timeoutInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		path := webhookPath.Child("timeoutSeconds")

		if webhook.TimeoutSeconds != nil {
			ts := *webhook.TimeoutSeconds
			if ts < 1 || ts > 30 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    path.String(),
						Message: fmt.Sprintf("timeoutSeconds: must be between 1 and 30, got %d", ts),
						Kind:    "MutatingWebhookConfiguration",
						Name:    mwc.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// rulesInvalidCheck validates that each webhook has at least one rule.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type rulesInvalidCheck struct{}

func (c rulesInvalidCheck) ID() string {
	return "admissionregistration/rules-invalid"
}

func (c rulesInvalidCheck) Title() string {
	return "Webhook Must Have At Least One Rule"
}

func (c rulesInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c rulesInvalidCheck) Blocking() bool {
	return true
}

func (c rulesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c rulesInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c rulesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		path := webhookPath.Child("rules")

		if len(webhook.Rules) == 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: "rules: must have at least one rule",
					Kind:    "MutatingWebhookConfiguration",
					Name:    mwc.GetName(),
				},
			})
		}
	}

	return findings
}

// apiGroupsInvalidCheck validates apiGroups in webhook rules.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type apiGroupsInvalidCheck struct{}

func (c apiGroupsInvalidCheck) ID() string {
	return "admissionregistration/api-groups-invalid"
}

func (c apiGroupsInvalidCheck) Title() string {
	return "Webhook API Groups Must Be Valid"
}

func (c apiGroupsInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c apiGroupsInvalidCheck) Blocking() bool {
	return true
}

func (c apiGroupsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c apiGroupsInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c apiGroupsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)

		for j, rule := range webhook.Rules {
			rulePath := webhookPath.Child("rules").Index(j)

			if len(rule.APIGroups) == 0 {
				continue
			}

			for k, apiGroup := range rule.APIGroups {
				groupPath := rulePath.Child("apiGroups").Index(k)

				if apiGroup == "*" {
					continue
				}

				if !isValidAPIGroup(apiGroup) {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    groupPath.String(),
							Message: fmt.Sprintf("apiGroups: invalid value: %q", apiGroup),
							Kind:    "MutatingWebhookConfiguration",
							Name:    mwc.GetName(),
						},
					})
				}
			}
		}
	}

	return findings
}

// resourcesInvalidCheck validates resources in webhook rules.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type resourcesInvalidCheck struct{}

func (c resourcesInvalidCheck) ID() string {
	return "admissionregistration/resources-invalid"
}

func (c resourcesInvalidCheck) Title() string {
	return "Webhook Resources Must Be Valid"
}

func (c resourcesInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c resourcesInvalidCheck) Blocking() bool {
	return true
}

func (c resourcesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c resourcesInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c resourcesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)

		for j, rule := range webhook.Rules {
			rulePath := webhookPath.Child("rules").Index(j)

			if len(rule.Resources) == 0 {
				continue
			}

			for k, resource := range rule.Resources {
				resourcePath := rulePath.Child("resources").Index(k)

				if !isValidSubresource(resource) {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    resourcePath.String(),
							Message: fmt.Sprintf("resources: invalid value: %q", resource),
							Kind:    "MutatingWebhookConfiguration",
							Name:    mwc.GetName(),
						},
					})
				}
			}
		}
	}

	return findings
}

// scopeInvalidCheck validates scope in webhook rules.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type scopeInvalidCheck struct{}

func (c scopeInvalidCheck) ID() string {
	return "admissionregistration/scope-invalid"
}

func (c scopeInvalidCheck) Title() string {
	return "Webhook Scope Must Be Valid"
}

func (c scopeInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c scopeInvalidCheck) Blocking() bool {
	return true
}

func (c scopeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scopeInvalidCheck) DocSkipper() []string {
	return mutatingWebhookKinds
}

func (c scopeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validScopes := map[admissionregistrationv1.ScopeType]bool{
		admissionregistrationv1.ClusterScope:    true,
		admissionregistrationv1.NamespacedScope: true,
		admissionregistrationv1.AllScopes:       true,
		"":                                      true,
	}

	for i, webhook := range mwc.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)

		for j, rule := range webhook.Rules {
			rulePath := webhookPath.Child("rules").Index(j)

			if rule.Scope != nil && !validScopes[*rule.Scope] {
				scopePath := rulePath.Child("scope")
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    scopePath.String(),
						Message: fmt.Sprintf("scope: Unsupported value: %q", string(*rule.Scope)),
						Kind:    "MutatingWebhookConfiguration",
						Name:    mwc.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// isValidDNS1123Subdomain validates a DNS-1123 subdomain string.
func isValidDNS1123Subdomain(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	labels := splitByDots(name)
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !isDNS1123LabelChar(c) {
				return false
			}
		}
	}
	return true
}

// isValidDNS1123Label validates a single DNS-1123 label.
func isValidDNS1123Label(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for _, c := range name {
		if !isDNS1123LabelChar(c) {
			return false
		}
	}
	return true
}

// isDNS1123LabelChar returns true if the rune is a valid DNS-1123 label character.
func isDNS1123LabelChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-'
}

// isValidAPIGroup validates an API group string.
func isValidAPIGroup(group string) bool {
	if len(group) == 0 {
		return false
	}
	if group == "*" {
		return true
	}
	// Split into prefix and suffix
	parts := strings.SplitN(group, "/", 2)
	if len(parts) == 1 {
		// Core API group - must match "kubernetes" or be empty (empty is not valid in rules)
		return isValidDNS1123Label(parts[0])
	}
	// Has a slash - first part must be a valid DNS subdomain
	prefix := parts[0]
	if len(prefix) == 0 || !isValidDNS1123Subdomain(prefix) {
		return false
	}
	// Second part must be a valid DNS label
	suffix := parts[1]
	if len(suffix) == 0 || !isValidDNS1123Label(suffix) {
		return false
	}
	return true
}

// splitByDots splits a string by '.' and returns the parts.
func splitByDots(s string) []string {
	var parts []string
	var current string
	for _, b := range s {
		if b == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(b)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// isValidSubresource validates a resource string.
func isValidSubresource(resource string) bool {
	if len(resource) == 0 {
		return false
	}
	if resource == "*" {
		return true
	}
	// Check if it's a subresource (contains /)
	if strings.Contains(resource, "/") {
		parts := strings.SplitN(resource, "/", 2)
		return isValidDNS1123Label(parts[0]) && isValidDNS1123Label(parts[1])
	}
	return isValidDNS1123Label(resource)
}

func init() {
	checks := []runtime.Check{
		webhookNameInvalidCheck{},
		clientConfigInvalidCheck{},
		serviceInvalidCheck{},
		failurePolicyInvalidCheck{},
		sideEffectsInvalidCheck{},
		timeoutInvalidCheck{},
		rulesInvalidCheck{},
		apiGroupsInvalidCheck{},
		resourcesInvalidCheck{},
		scopeInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
