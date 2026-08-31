package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var networkPolicyKinds = []string{"NetworkPolicy"}

// policyTypeInvalidCheck validates that policyTypes only contains valid values.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type policyTypeInvalidCheck struct{ runtime.Meta }

func newPolicyTypeInvalidCheck() policyTypeInvalidCheck {
	return policyTypeInvalidCheck{runtime.Meta{
		RuleID:    "network-policy/policy-type-invalid",
		RuleTitle: "NetworkPolicy policyTypes Must Be Valid",
		AppliesTo: networkPolicyKinds,
	}}
}

func (c policyTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal(data, &np); err != nil {
		return nil
	}

	var findings []runtime.Finding
	policyTypesPath := field.NewPath("spec").Child("policyTypes")

	for _, pt := range np.Spec.PolicyTypes {
		valid := false
		switch pt {
		case networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress:
			valid = true
		}
		if !valid {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    policyTypesPath.String(),
					Message: fmt.Sprintf("policyTypes: Unsupported value: %q", string(pt)),
					Kind:    "NetworkPolicy",
					Name:    np.GetName(),
				},
			})
		}
	}

	return findings
}

// portRangeInvalidCheck validates that port range end is >= start.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type portRangeInvalidCheck struct{ runtime.Meta }

func newPortRangeInvalidCheck() portRangeInvalidCheck {
	return portRangeInvalidCheck{runtime.Meta{
		RuleID:    "network-policy/port-range-invalid",
		RuleTitle: "NetworkPolicy Port Range Must Be Valid",
		AppliesTo: networkPolicyKinds,
	}}
}

func (c portRangeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal(data, &np); err != nil {
		return nil
	}

	var findings []runtime.Finding

	validatePortRanges := func(ports []networkingv1.NetworkPolicyPort, pathPrefix *field.Path) {
		for i, port := range ports {
			portPath := pathPrefix.Child("ports").Index(i)
			report := func(msg string) {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Finding: check.Finding{
						Path:    portPath.Child("endPort").String(),
						Message: msg,
						Kind:    "NetworkPolicy",
						Name:    np.GetName(),
					},
				})
			}

			if port.EndPort == nil {
				continue
			}
			switch {
			case port.Port == nil:
				report("endPort may not be specified when port is not specified")
			case port.Port.Type != intstr.Int:
				// A named port has no numeric value to bound, so upstream
				// rejects the pairing outright. Coercing the name to an
				// int instead read it as port 0, which any positive
				// endPort clears, so this shape was silently accepted.
				report(fmt.Sprintf("endPort may not be specified when port is non-numeric (port %q)", port.Port.StrVal))
			case *port.EndPort < port.Port.IntVal:
				report(fmt.Sprintf("port range end must be >= start: endPort %d < port %d", *port.EndPort, port.Port.IntVal))
			}
		}
	}

	for i, rule := range np.Spec.Ingress {
		rulePath := field.NewPath("spec").Child("ingress").Index(i)
		validatePortRanges(rule.Ports, rulePath)
	}

	for i, rule := range np.Spec.Egress {
		rulePath := field.NewPath("spec").Child("egress").Index(i)
		validatePortRanges(rule.Ports, rulePath)
	}

	return findings
}

// protocolInvalidCheck validates that protocol is one of TCP, UDP, SCTP.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type protocolInvalidCheck struct{ runtime.Meta }

func newProtocolInvalidCheck() protocolInvalidCheck {
	return protocolInvalidCheck{runtime.Meta{
		RuleID:    "network-policy/protocol-invalid",
		RuleTitle: "NetworkPolicy Protocol Must Be Valid",
		AppliesTo: networkPolicyKinds,
	}}
}

func (c protocolInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal(data, &np); err != nil {
		return nil
	}

	var findings []runtime.Finding

	validateProtocol := func(ports []networkingv1.NetworkPolicyPort, pathPrefix *field.Path) {
		for i, port := range ports {
			portPath := pathPrefix.Child("ports").Index(i)
			if port.Protocol != nil {
				if *port.Protocol != corev1.ProtocolTCP && *port.Protocol != corev1.ProtocolUDP && *port.Protocol != corev1.ProtocolSCTP {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Finding: check.Finding{
							Path:    portPath.Child("protocol").String(),
							Message: fmt.Sprintf("protocol: Unsupported value: %q", string(*port.Protocol)),
							Kind:    "NetworkPolicy",
							Name:    np.GetName(),
						},
					})
				}
			}
		}
	}

	for i, rule := range np.Spec.Ingress {
		rulePath := field.NewPath("spec").Child("ingress").Index(i)
		validateProtocol(rule.Ports, rulePath)
	}

	for i, rule := range np.Spec.Egress {
		rulePath := field.NewPath("spec").Child("egress").Index(i)
		validateProtocol(rule.Ports, rulePath)
	}

	return findings
}
