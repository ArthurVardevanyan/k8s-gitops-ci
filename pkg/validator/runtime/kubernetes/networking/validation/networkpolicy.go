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

func (c policyTypeInvalidCheck) Kinds() []string {
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

func (c portRangeInvalidCheck) Kinds() []string {
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

func (c protocolInvalidCheck) Kinds() []string {
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
