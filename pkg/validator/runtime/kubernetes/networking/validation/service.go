package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var serviceKinds = []string{"Service"}

// typeInvalidCheck validates that service type is one of the allowed values.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type typeInvalidCheck struct{}

func (c typeInvalidCheck) ID() string {
	return "service/type-invalid"
}

func (c typeInvalidCheck) Title() string {
	return "Service Type Must Be Valid"
}

func (c typeInvalidCheck) Category() string {
	return "service"
}

func (c typeInvalidCheck) Blocking() bool {
	return true
}

func (c typeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c typeInvalidCheck) DocSkipper() []string {
	return serviceKinds
}

func (c typeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	path := field.NewPath("spec").Child("type")

	validTypes := map[corev1.ServiceType]bool{
		corev1.ServiceTypeClusterIP:    true,
		corev1.ServiceTypeNodePort:     true,
		corev1.ServiceTypeLoadBalancer: true,
		corev1.ServiceTypeExternalName: true,
	}

	if !validTypes[svc.Spec.Type] && string(svc.Spec.Type) != "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    path.String(),
				Message: fmt.Sprintf("type: Unsupported value: %q", string(svc.Spec.Type)),
				Kind:    "Service",
				Name:    svc.GetName(),
			},
		})
	}

	return findings
}

// externalNameInvalidCheck validates that ExternalName services have valid hostnames.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type externalNameInvalidCheck struct{}

func (c externalNameInvalidCheck) ID() string {
	return "service/external-name-invalid"
}

func (c externalNameInvalidCheck) Title() string {
	return "Service ExternalName Must Be Valid"
}

func (c externalNameInvalidCheck) Category() string {
	return "service"
}

func (c externalNameInvalidCheck) Blocking() bool {
	return true
}

func (c externalNameInvalidCheck) RenderSensitive() bool {
	return true
}

func (c externalNameInvalidCheck) DocSkipper() []string {
	return serviceKinds
}

func (c externalNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		path := field.NewPath("spec").Child("externalName")
		externalName := svc.Spec.ExternalName

		if externalName == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: "externalName: invalid value: must be a valid DNS hostname for ExternalName services",
					Kind:    "Service",
					Name:    svc.GetName(),
				},
			})
		} else {
			// Check for invalid characters and structure
			for _, label := range externalName {
				if label != '.' && label != '-' && (label < 'a' || label > 'z') && (label < 'A' || label > 'Z') && (label < '0' || label > '9') {
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    path.String(),
							Message: "externalName: invalid value: must be a valid DNS hostname",
							Kind:    "Service",
							Name:    svc.GetName(),
						},
					})
					break
				}
			}
			if len(externalName) > 253 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    path.String(),
						Message: "externalName: invalid value: must be a valid DNS hostname (max 253 characters)",
						Kind:    "Service",
						Name:    svc.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// clusterIPInvalidCheck validates that clusterIP is a valid IP address.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type clusterIPInvalidCheck struct{}

func (c clusterIPInvalidCheck) ID() string {
	return "service/cluster-ip-invalid"
}

func (c clusterIPInvalidCheck) Title() string {
	return "Service ClusterIP Must Be Valid"
}

func (c clusterIPInvalidCheck) Category() string {
	return "service"
}

func (c clusterIPInvalidCheck) Blocking() bool {
	return true
}

func (c clusterIPInvalidCheck) RenderSensitive() bool {
	return true
}

func (c clusterIPInvalidCheck) DocSkipper() []string {
	return serviceKinds
}

func (c clusterIPInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != "None" {
		path := field.NewPath("spec").Child("clusterIP")

		// Parse the IP to check validity
		clusterIP := svc.Spec.ClusterIP

		// Check for valid IPv4 or IPv6 addresses
		isValid := false

		// Try IPv4 parsing
		parts := splitIP(clusterIP)
		if len(parts) == 4 {
			allValid := true
			for _, part := range parts {
				val := 0
				for i := 0; i < len(part); i++ {
					digit := int(part[i] - '0')
					if digit < 0 || digit > 9 {
						allValid = false
						break
					}
					val = val*10 + digit
				}
				if !allValid || val > 255 {
					allValid = false
					break
				}
			}
			if allValid {
				isValid = true
			}
		}

		// Try IPv6 parsing (simple check for colons)
		if !isValid {
			for _, b := range clusterIP {
				if b == ':' {
					isValid = true
					break
				}
			}
		}

		if !isValid {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: fmt.Sprintf("clusterIP: invalid value: %q", clusterIP),
					Kind:    "Service",
					Name:    svc.GetName(),
				},
			})
		}
	}

	return findings
}

// sessionAffinityInvalidCheck validates that sessionAffinity is one of ClientIP, None.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type sessionAffinityInvalidCheck struct{}

func (c sessionAffinityInvalidCheck) ID() string {
	return "service/session-affinity-invalid"
}

func (c sessionAffinityInvalidCheck) Title() string {
	return "Service SessionAffinity Must Be Valid"
}

func (c sessionAffinityInvalidCheck) Category() string {
	return "service"
}

func (c sessionAffinityInvalidCheck) Blocking() bool {
	return true
}

func (c sessionAffinityInvalidCheck) RenderSensitive() bool {
	return true
}

func (c sessionAffinityInvalidCheck) DocSkipper() []string {
	return serviceKinds
}

func (c sessionAffinityInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding
	path := field.NewPath("spec").Child("sessionAffinity")

	validAffinity := map[corev1.ServiceAffinity]bool{
		corev1.ServiceAffinityClientIP: true,
		corev1.ServiceAffinityNone:     true,
	}

	if !validAffinity[svc.Spec.SessionAffinity] && string(svc.Spec.SessionAffinity) != "" {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    path.String(),
				Message: fmt.Sprintf("sessionAffinity: Unsupported value: %q", string(svc.Spec.SessionAffinity)),
				Kind:    "Service",
				Name:    svc.GetName(),
			},
		})
	}

	return findings
}

// allocateLoadBalancerIPsInvalidCheck validates that allocateLoadBalancerIPs is a boolean.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type allocateLoadBalancerIPsInvalidCheck struct{}

func (c allocateLoadBalancerIPsInvalidCheck) ID() string {
	return "service/allocate-load-balancer-ips-invalid"
}

func (c allocateLoadBalancerIPsInvalidCheck) Title() string {
	return "Service allocateLoadBalancerIPs Must Be Valid"
}

func (c allocateLoadBalancerIPsInvalidCheck) Category() string {
	return "service"
}

func (c allocateLoadBalancerIPsInvalidCheck) Blocking() bool {
	return true
}

func (c allocateLoadBalancerIPsInvalidCheck) RenderSensitive() bool {
	return true
}

func (c allocateLoadBalancerIPsInvalidCheck) DocSkipper() []string {
	return serviceKinds
}

func (c allocateLoadBalancerIPsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var svc corev1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil
	}

	var findings []runtime.Finding

	// If allocateLoadBalancerIPs is explicitly set in the raw YAML,
	// verify it's actually a boolean by re-parsing with unstructured
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err == nil {
		if spec, ok := raw["spec"].(map[string]interface{}); ok {
			if val, ok := spec["allocateLoadBalancerIPs"]; ok {
				if _, ok := val.(bool); !ok {
					path := field.NewPath("spec").Child("allocateLoadBalancerIPs")
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:    path.String(),
							Message: "allocateLoadBalancerIPs: invalid value: must be a boolean",
							Kind:    "Service",
							Name:    svc.GetName(),
						},
					})
				}
			}
		}
	}

	return findings
}

// splitIP splits an IP string by '.' and returns the parts.
// Returns a single-element slice if no '.' found.
func splitIP(ip string) []string {
	var parts []string
	var current string
	for _, b := range ip {
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

func init() {
	checks := []runtime.Check{
		typeInvalidCheck{},
		externalNameInvalidCheck{},
		clusterIPInvalidCheck{},
		sessionAffinityInvalidCheck{},
		allocateLoadBalancerIPsInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
