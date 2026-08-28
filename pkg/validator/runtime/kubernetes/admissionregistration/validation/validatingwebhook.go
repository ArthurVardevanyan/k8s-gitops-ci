package validation

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var validatingWebhookKinds = []string{"ValidatingWebhookConfiguration"}

// webhookNameInvalidCheck validates that each webhook has a valid DNS subdomain name.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingWebhookNameInvalidCheck struct{}

func (c validatingWebhookNameInvalidCheck) ID() string {
	return "admissionregistration/validating-webhook-name-invalid"
}

func (c validatingWebhookNameInvalidCheck) Title() string {
	return "Webhook Name Must Be Valid DNS Subdomain"
}

func (c validatingWebhookNameInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingWebhookNameInvalidCheck) Blocking() bool {
	return true
}

func (c validatingWebhookNameInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingWebhookNameInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingWebhookNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
					Kind:    "ValidatingWebhookConfiguration",
					Name:    vwc.GetName(),
				},
			})
		}
	}

	return findings
}

// clientConfigInvalidCheck validates that clientConfig specifies either url or service.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingClientConfigInvalidCheck struct{}

func (c validatingClientConfigInvalidCheck) ID() string {
	return "admissionregistration/validating-client-config-invalid"
}

func (c validatingClientConfigInvalidCheck) Title() string {
	return "Webhook ClientConfig Must Specify URL or Service"
}

func (c validatingClientConfigInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingClientConfigInvalidCheck) Blocking() bool {
	return true
}

func (c validatingClientConfigInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingClientConfigInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingClientConfigInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
					Kind:    "ValidatingWebhookConfiguration",
					Name:    vwc.GetName(),
				},
			})
		}
	}

	return findings
}

// serviceInvalidCheck validates that service reference is valid when specified.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingServiceInvalidCheck struct{}

func (c validatingServiceInvalidCheck) ID() string {
	return "admissionregistration/validating-service-invalid"
}

func (c validatingServiceInvalidCheck) Title() string {
	return "Webhook Service Reference Must Be Valid"
}

func (c validatingServiceInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingServiceInvalidCheck) Blocking() bool {
	return true
}

func (c validatingServiceInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingServiceInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingServiceInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
					Kind:    "ValidatingWebhookConfiguration",
					Name:    vwc.GetName(),
				},
			})
		}
	}

	return findings
}

// failurePolicyInvalidCheck validates failurePolicy is a recognized value.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingFailurePolicyInvalidCheck struct{}

func (c validatingFailurePolicyInvalidCheck) ID() string {
	return "admissionregistration/validating-failure-policy-invalid"
}

func (c validatingFailurePolicyInvalidCheck) Title() string {
	return "Webhook FailurePolicy Must Be Valid"
}

func (c validatingFailurePolicyInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingFailurePolicyInvalidCheck) Blocking() bool {
	return true
}

func (c validatingFailurePolicyInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingFailurePolicyInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingFailurePolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validFailurePolicies := map[admissionregistrationv1.FailurePolicyType]bool{
		admissionregistrationv1.Ignore: true,
		admissionregistrationv1.Fail:   true,
	}

	for i, webhook := range vwc.Webhooks {
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
					Kind:    "ValidatingWebhookConfiguration",
					Name:    vwc.GetName(),
				},
			})
		}
	}

	return findings
}

// sideEffectsInvalidCheck validates sideEffects is a recognized value.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingSideEffectsInvalidCheck struct{}

func (c validatingSideEffectsInvalidCheck) ID() string {
	return "admissionregistration/validating-side-effects-invalid"
}

func (c validatingSideEffectsInvalidCheck) Title() string {
	return "Webhook SideEffects Must Be Valid"
}

func (c validatingSideEffectsInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingSideEffectsInvalidCheck) Blocking() bool {
	return true
}

func (c validatingSideEffectsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingSideEffectsInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingSideEffectsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validSideEffects := map[admissionregistrationv1.SideEffectClass]bool{
		admissionregistrationv1.SideEffectClassUnknown:      true,
		admissionregistrationv1.SideEffectClassNone:         true,
		admissionregistrationv1.SideEffectClassNoneOnDryRun: true,
		admissionregistrationv1.SideEffectClassSome:         true,
	}

	for i, webhook := range vwc.Webhooks {
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
					Kind:    "ValidatingWebhookConfiguration",
					Name:    vwc.GetName(),
				},
			})
		}
	}

	return findings
}

// timeoutInvalidCheck validates timeoutSeconds is within the allowed range.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingTimeoutInvalidCheck struct{}

func (c validatingTimeoutInvalidCheck) ID() string {
	return "admissionregistration/validating-timeout-invalid"
}

func (c validatingTimeoutInvalidCheck) Title() string {
	return "Webhook TimeoutSeconds Must Be Between 1 and 30"
}

func (c validatingTimeoutInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingTimeoutInvalidCheck) Blocking() bool {
	return true
}

func (c validatingTimeoutInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingTimeoutInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingTimeoutInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
						Kind:    "ValidatingWebhookConfiguration",
						Name:    vwc.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// rulesInvalidCheck validates that each webhook has at least one rule.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingRulesInvalidCheck struct{}

func (c validatingRulesInvalidCheck) ID() string {
	return "admissionregistration/validating-rules-invalid"
}

func (c validatingRulesInvalidCheck) Title() string {
	return "Webhook Must Have At Least One Rule"
}

func (c validatingRulesInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingRulesInvalidCheck) Blocking() bool {
	return true
}

func (c validatingRulesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingRulesInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingRulesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
					Kind:    "ValidatingWebhookConfiguration",
					Name:    vwc.GetName(),
				},
			})
		}
	}

	return findings
}

// apiGroupsInvalidCheck validates apiGroups in webhook rules.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type validatingAPIGroupsInvalidCheck struct{}

func (c validatingAPIGroupsInvalidCheck) ID() string {
	return "admissionregistration/validating-api-groups-invalid"
}

func (c validatingAPIGroupsInvalidCheck) Title() string {
	return "Webhook API Groups Must Be Valid"
}

func (c validatingAPIGroupsInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingAPIGroupsInvalidCheck) Blocking() bool {
	return true
}

func (c validatingAPIGroupsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingAPIGroupsInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingAPIGroupsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
							Kind:    "ValidatingWebhookConfiguration",
							Name:    vwc.GetName(),
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
type validatingResourcesInvalidCheck struct{}

func (c validatingResourcesInvalidCheck) ID() string {
	return "admissionregistration/validating-resources-invalid"
}

func (c validatingResourcesInvalidCheck) Title() string {
	return "Webhook Resources Must Be Valid"
}

func (c validatingResourcesInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingResourcesInvalidCheck) Blocking() bool {
	return true
}

func (c validatingResourcesInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingResourcesInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingResourcesInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range vwc.Webhooks {
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
							Kind:    "ValidatingWebhookConfiguration",
							Name:    vwc.GetName(),
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
type validatingScopeInvalidCheck struct{}

func (c validatingScopeInvalidCheck) ID() string {
	return "admissionregistration/validating-scope-invalid"
}

func (c validatingScopeInvalidCheck) Title() string {
	return "Webhook Scope Must Be Valid"
}

func (c validatingScopeInvalidCheck) Category() string {
	return "admissionregistration"
}

func (c validatingScopeInvalidCheck) Blocking() bool {
	return true
}

func (c validatingScopeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c validatingScopeInvalidCheck) DocSkipper() []string {
	return validatingWebhookKinds
}

func (c validatingScopeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	validScopes := map[admissionregistrationv1.ScopeType]bool{
		admissionregistrationv1.ClusterScope:    true,
		admissionregistrationv1.NamespacedScope: true,
		admissionregistrationv1.AllScopes:       true,
		"":                                      true,
	}

	for i, webhook := range vwc.Webhooks {
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
						Kind:    "ValidatingWebhookConfiguration",
						Name:    vwc.GetName(),
					},
				})
			}
		}
	}

	return findings
}

func init() {
	checks := []runtime.Check{
		validatingWebhookNameInvalidCheck{},
		validatingClientConfigInvalidCheck{},
		validatingServiceInvalidCheck{},
		validatingFailurePolicyInvalidCheck{},
		validatingSideEffectsInvalidCheck{},
		validatingTimeoutInvalidCheck{},
		validatingRulesInvalidCheck{},
		validatingAPIGroupsInvalidCheck{},
		validatingResourcesInvalidCheck{},
		validatingScopeInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
