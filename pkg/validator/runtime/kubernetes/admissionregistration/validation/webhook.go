package validation

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// webhookInfo is the subset of a webhook entry inspected by the checks in this
// package. MutatingWebhook and ValidatingWebhook are distinct Go types with
// identical shapes for these fields, so both are normalized into this struct.
type webhookInfo struct {
	Service        *admissionregistrationv1.ServiceReference
	FailurePolicy  *admissionregistrationv1.FailurePolicyType
	TimeoutSeconds *int32
}

// webhookConfiguration is the normalized form of a
// Mutating/ValidatingWebhookConfiguration.
type webhookConfiguration struct {
	Name     string
	Webhooks []webhookInfo
}

// webhookCheckBase carries everything that varies between the
// MutatingWebhookConfiguration and ValidatingWebhookConfiguration variants of
// each check: the check-ID prefix, the kind name reported on findings, the
// Kinds() list, and the decoder for the concrete API type.
type webhookCheckBase struct {
	idPrefix string
	kind     string
	kinds    []string
	parse    func(data []byte) (*webhookConfiguration, bool)
}

func (b webhookCheckBase) Category() string      { return "admissionregistration" }
func (b webhookCheckBase) Blocking() bool        { return true }
func (b webhookCheckBase) RenderSensitive() bool { return true }
func (b webhookCheckBase) Kinds() []string       { return b.kinds }

// id builds the fully-qualified check ID for the given suffix.
func (b webhookCheckBase) id(suffix string) string {
	return "admissionregistration/" + b.idPrefix + suffix
}

// finding builds a finding for this configuration kind.
func (b webhookCheckBase) finding(c runtime.Check, cfg *webhookConfiguration, path, message string) runtime.Finding {
	return runtime.Finding{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    path,
			Message: message,
			Kind:    b.kind,
			Name:    cfg.Name,
		},
	}
}

func parseMutatingWebhookConfiguration(data []byte) (*webhookConfiguration, bool) {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &mwc); err != nil {
		return nil, false
	}

	cfg := &webhookConfiguration{Name: mwc.GetName()}
	for _, webhook := range mwc.Webhooks {
		cfg.Webhooks = append(cfg.Webhooks, webhookInfo{
			Service:        webhook.ClientConfig.Service,
			FailurePolicy:  webhook.FailurePolicy,
			TimeoutSeconds: webhook.TimeoutSeconds,
		})
	}

	return cfg, true
}

func parseValidatingWebhookConfiguration(data []byte) (*webhookConfiguration, bool) {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &vwc); err != nil {
		return nil, false
	}

	cfg := &webhookConfiguration{Name: vwc.GetName()}
	for _, webhook := range vwc.Webhooks {
		cfg.Webhooks = append(cfg.Webhooks, webhookInfo{
			Service:        webhook.ClientConfig.Service,
			FailurePolicy:  webhook.FailurePolicy,
			TimeoutSeconds: webhook.TimeoutSeconds,
		})
	}

	return cfg, true
}

// serviceInvalidCheck validates that service reference is valid when specified.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type serviceInvalidCheck struct {
	webhookCheckBase
}

func (c serviceInvalidCheck) ID() string {
	return c.id("service-invalid")
}

func (c serviceInvalidCheck) Title() string {
	return "Webhook Service Reference Must Be Valid"
}

func (c serviceInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	cfg, ok := c.parse(data)
	if !ok {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range cfg.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		clientConfigPath := webhookPath.Child("clientConfig")

		if webhook.Service == nil {
			continue
		}

		svc := webhook.Service
		svcPath := clientConfigPath.Child("service")

		if svc.Name == "" {
			findings = append(findings, c.finding(c, cfg,
				svcPath.Child("name").String(),
				"service: invalid value: name must not be empty"))
		}
	}

	return findings
}

// failurePolicyInvalidCheck validates failurePolicy is a recognized value.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type failurePolicyInvalidCheck struct {
	webhookCheckBase
}

func (c failurePolicyInvalidCheck) ID() string {
	return c.id("failure-policy-invalid")
}

func (c failurePolicyInvalidCheck) Title() string {
	return "Webhook FailurePolicy Must Be Valid"
}

func (c failurePolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	cfg, ok := c.parse(data)
	if !ok {
		return nil
	}

	var findings []runtime.Finding
	validFailurePolicies := map[admissionregistrationv1.FailurePolicyType]bool{
		admissionregistrationv1.Ignore: true,
		admissionregistrationv1.Fail:   true,
	}

	for i, webhook := range cfg.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		path := webhookPath.Child("failurePolicy")

		if webhook.FailurePolicy != nil && !validFailurePolicies[*webhook.FailurePolicy] {
			findings = append(findings, c.finding(c, cfg, path.String(),
				fmt.Sprintf("failurePolicy: Unsupported value: %q", string(*webhook.FailurePolicy))))
		}
	}

	return findings
}

// timeoutInvalidCheck validates timeoutSeconds is within the allowed range.
// Source: k8s.io/kubernetes/pkg/apis/admissionregistration/validation/validation.go
type timeoutInvalidCheck struct {
	webhookCheckBase
}

func (c timeoutInvalidCheck) ID() string {
	return c.id("timeout-invalid")
}

func (c timeoutInvalidCheck) Title() string {
	return "Webhook TimeoutSeconds Must Be Between 1 and 30"
}

func (c timeoutInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	cfg, ok := c.parse(data)
	if !ok {
		return nil
	}

	var findings []runtime.Finding

	for i, webhook := range cfg.Webhooks {
		webhookPath := field.NewPath("webhooks").Index(i)
		path := webhookPath.Child("timeoutSeconds")

		if webhook.TimeoutSeconds != nil {
			ts := *webhook.TimeoutSeconds
			if ts < 1 || ts > 30 {
				findings = append(findings, c.finding(c, cfg, path.String(),
					fmt.Sprintf("timeoutSeconds: must be between 1 and 30, got %d", ts)))
			}
		}
	}

	return findings
}

// webhookChecks returns the three shared checks for one configuration kind.
func webhookChecks(base webhookCheckBase) []runtime.Check {
	return []runtime.Check{
		serviceInvalidCheck{webhookCheckBase: base},
		failurePolicyInvalidCheck{webhookCheckBase: base},
		timeoutInvalidCheck{webhookCheckBase: base},
	}
}
