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
type policyTypeInvalidCheck struct{}

func (c policyTypeInvalidCheck) ID() string {
	return "network-policy/policy-type-invalid"
}

func (c policyTypeInvalidCheck) Title() string {
	return "NetworkPolicy policyTypes Must Be Valid"
}

func (c policyTypeInvalidCheck) Category() string {
	return "network-policy"
}

func (c policyTypeInvalidCheck) Blocking() bool {
	return true
}

func (c policyTypeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c policyTypeInvalidCheck) DocSkipper() []string {
	return networkPolicyKinds
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
				Category:  c.Category(),
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

// getIntValue extracts the integer value from an IntOrString, returning 0 if not an int.
func getIntValue(v intstr.IntOrString) int32 {
	if v.Type == intstr.Int {
		return v.IntVal
	}
	return 0
}

// validatePort checks a single port in a NetworkPolicy rule.
func validatePort(port networkingv1.NetworkPolicyPort, portPath *field.Path, npName string) []runtime.Finding {
	var findings []runtime.Finding

	if port.Port != nil {
		portInt := getIntValue(*port.Port)
		if portInt < 0 || portInt > 65535 {
			findings = append(findings, runtime.Finding{
				RuleID:    "network-policy/ingress-rule-invalid",
				RuleTitle: "NetworkPolicy Ingress Rules Must Be Valid",
				Category:  "network-policy",
				Finding: check.Finding{
					Path:    portPath.Child("port").String(),
					Message: fmt.Sprintf("invalid ingress rule: port must be between 0 and 65535: %d", portInt),
					Kind:    "NetworkPolicy",
					Name:    npName,
				},
			})
		}
	}

	if port.Protocol != nil {
		if *port.Protocol != corev1.ProtocolTCP && *port.Protocol != corev1.ProtocolUDP && *port.Protocol != corev1.ProtocolSCTP {
			findings = append(findings, runtime.Finding{
				RuleID:    "network-policy/ingress-rule-invalid",
				RuleTitle: "NetworkPolicy Ingress Rules Must Be Valid",
				Category:  "network-policy",
				Finding: check.Finding{
					Path:    portPath.Child("protocol").String(),
					Message: fmt.Sprintf("invalid ingress rule: protocol: Unsupported value: %q", string(*port.Protocol)),
					Kind:    "NetworkPolicy",
					Name:    npName,
				},
			})
		}
	}

	if port.EndPort != nil {
		if port.Port == nil {
			findings = append(findings, runtime.Finding{
				RuleID:    "network-policy/ingress-rule-invalid",
				RuleTitle: "NetworkPolicy Ingress Rules Must Be Valid",
				Category:  "network-policy",
				Finding: check.Finding{
					Path:    portPath.Child("endPort").String(),
					Message: "invalid ingress rule: endPort requires port to be specified",
					Kind:    "NetworkPolicy",
					Name:    npName,
				},
			})
		} else if *port.EndPort < int32(getIntValue(*port.Port)) {
			findings = append(findings, runtime.Finding{
				RuleID:    "network-policy/ingress-rule-invalid",
				RuleTitle: "NetworkPolicy Ingress Rules Must Be Valid",
				Category:  "network-policy",
				Finding: check.Finding{
					Path:    portPath.Child("endPort").String(),
					Message: fmt.Sprintf("port range end must be >= start: endPort %d < port %d", *port.EndPort, getIntValue(*port.Port)),
					Kind:    "NetworkPolicy",
					Name:    npName,
				},
			})
		} else if *port.EndPort > 65535 {
			findings = append(findings, runtime.Finding{
				RuleID:    "network-policy/ingress-rule-invalid",
				RuleTitle: "NetworkPolicy Ingress Rules Must Be Valid",
				Category:  "network-policy",
				Finding: check.Finding{
					Path:    portPath.Child("endPort").String(),
					Message: fmt.Sprintf("invalid ingress rule: endPort must be between 0 and 65535: %d", *port.EndPort),
					Kind:    "NetworkPolicy",
					Name:    npName,
				},
			})
		}
	}

	return findings
}

// ingressRuleInvalidCheck validates ingress rules have valid ports and peers.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type ingressRuleInvalidCheck struct{}

func (c ingressRuleInvalidCheck) ID() string {
	return "network-policy/ingress-rule-invalid"
}

func (c ingressRuleInvalidCheck) Title() string {
	return "NetworkPolicy Ingress Rules Must Be Valid"
}

func (c ingressRuleInvalidCheck) Category() string {
	return "network-policy"
}

func (c ingressRuleInvalidCheck) Blocking() bool {
	return true
}

func (c ingressRuleInvalidCheck) RenderSensitive() bool {
	return true
}

func (c ingressRuleInvalidCheck) DocSkipper() []string {
	return networkPolicyKinds
}

func (c ingressRuleInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal(data, &np); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range np.Spec.Ingress {
		rulePath := field.NewPath("spec").Child("ingress").Index(i)

		for j, port := range rule.Ports {
			portPath := rulePath.Child("ports").Index(j)
			findings = append(findings, validatePort(port, portPath, np.GetName())...)
		}

		for j, peer := range rule.From {
			peerPath := rulePath.Child("from").Index(j)

			hasPodSelector := peer.PodSelector != nil
			hasNamespaceSelector := peer.NamespaceSelector != nil
			hasIPBlock := peer.IPBlock != nil

			if !hasPodSelector && !hasNamespaceSelector && !hasIPBlock {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    peerPath.String(),
						Message: "invalid ingress rule: ingress rule must have at least one of pods, namespaces, or ipBlock",
						Kind:    "NetworkPolicy",
						Name:    np.GetName(),
					},
				})
			}

			if hasIPBlock {
				ipPath := peerPath.Child("ipBlock")
				if peer.IPBlock.CIDR == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    ipPath.Child("cidr").String(),
							Message: "invalid ingress rule: ipBlock must have a valid cidr",
							Kind:    "NetworkPolicy",
							Name:    np.GetName(),
						},
					})
				}
				for k, except := range peer.IPBlock.Except {
					exceptPath := ipPath.Child("except").Index(k)
					if except == "" {
						findings = append(findings, runtime.Finding{
							RuleID:    c.ID(),
							RuleTitle: c.Title(),
							Category:  c.Category(),
							Finding: check.Finding{
								Path:    exceptPath.String(),
								Message: "invalid ingress rule: ipBlock except must be a valid CIDR",
								Kind:    "NetworkPolicy",
								Name:    np.GetName(),
							},
						})
					}
				}
			}
		}
	}

	return findings
}

// egressRuleInvalidCheck validates egress rules have valid ports and peers.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type egressRuleInvalidCheck struct{}

func (c egressRuleInvalidCheck) ID() string {
	return "network-policy/egress-rule-invalid"
}

func (c egressRuleInvalidCheck) Title() string {
	return "NetworkPolicy Egress Rules Must Be Valid"
}

func (c egressRuleInvalidCheck) Category() string {
	return "network-policy"
}

func (c egressRuleInvalidCheck) Blocking() bool {
	return true
}

func (c egressRuleInvalidCheck) RenderSensitive() bool {
	return true
}

func (c egressRuleInvalidCheck) DocSkipper() []string {
	return networkPolicyKinds
}

func (c egressRuleInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal(data, &np); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range np.Spec.Egress {
		rulePath := field.NewPath("spec").Child("egress").Index(i)

		for j, port := range rule.Ports {
			portPath := rulePath.Child("ports").Index(j)
			findings = append(findings, validatePort(port, portPath, np.GetName())...)
		}

		for j, peer := range rule.To {
			peerPath := rulePath.Child("to").Index(j)

			hasPodSelector := peer.PodSelector != nil
			hasNamespaceSelector := peer.NamespaceSelector != nil
			hasIPBlock := peer.IPBlock != nil

			if !hasPodSelector && !hasNamespaceSelector && !hasIPBlock {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    peerPath.String(),
						Message: "invalid egress rule: egress rule must have at least one of pods, namespaces, or ipBlock",
						Kind:    "NetworkPolicy",
						Name:    np.GetName(),
					},
				})
			}

			if hasIPBlock {
				ipPath := peerPath.Child("ipBlock")
				if peer.IPBlock.CIDR == "" {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    ipPath.Child("cidr").String(),
							Message: "invalid egress rule: ipBlock must have a valid cidr",
							Kind:    "NetworkPolicy",
							Name:    np.GetName(),
						},
					})
				}
				for k, except := range peer.IPBlock.Except {
					exceptPath := ipPath.Child("except").Index(k)
					if except == "" {
						findings = append(findings, runtime.Finding{
							RuleID:    c.ID(),
							RuleTitle: c.Title(),
							Category:  c.Category(),
							Finding: check.Finding{
								Path:    exceptPath.String(),
								Message: "invalid egress rule: ipBlock except must be a valid CIDR",
								Kind:    "NetworkPolicy",
								Name:    np.GetName(),
							},
						})
					}
				}
			}
		}
	}

	return findings
}

// portRangeInvalidCheck validates that port range end is >= start.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type portRangeInvalidCheck struct{}

func (c portRangeInvalidCheck) ID() string {
	return "network-policy/port-range-invalid"
}

func (c portRangeInvalidCheck) Title() string {
	return "NetworkPolicy Port Range Must Be Valid"
}

func (c portRangeInvalidCheck) Category() string {
	return "network-policy"
}

func (c portRangeInvalidCheck) Blocking() bool {
	return true
}

func (c portRangeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c portRangeInvalidCheck) DocSkipper() []string {
	return networkPolicyKinds
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
			if port.Port != nil && port.EndPort != nil {
				portInt := getIntValue(*port.Port)
				if *port.EndPort < int32(portInt) {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    portPath.Child("endPort").String(),
							Message: fmt.Sprintf("port range end must be >= start: endPort %d < port %d", *port.EndPort, portInt),
							Kind:    "NetworkPolicy",
							Name:    np.GetName(),
						},
					})
				}
			} else if port.EndPort != nil {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    portPath.Child("endPort").String(),
						Message: "port range end must be >= start: endPort requires port to be specified",
						Kind:    "NetworkPolicy",
						Name:    np.GetName(),
					},
				})
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
type protocolInvalidCheck struct{}

func (c protocolInvalidCheck) ID() string {
	return "network-policy/protocol-invalid"
}

func (c protocolInvalidCheck) Title() string {
	return "NetworkPolicy Protocol Must Be Valid"
}

func (c protocolInvalidCheck) Category() string {
	return "network-policy"
}

func (c protocolInvalidCheck) Blocking() bool {
	return true
}

func (c protocolInvalidCheck) RenderSensitive() bool {
	return true
}

func (c protocolInvalidCheck) DocSkipper() []string {
	return networkPolicyKinds
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
						Category:  c.Category(),
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

func init() {
	checks := []runtime.Check{
		policyTypeInvalidCheck{},
		ingressRuleInvalidCheck{},
		egressRuleInvalidCheck{},
		portRangeInvalidCheck{},
		protocolInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
