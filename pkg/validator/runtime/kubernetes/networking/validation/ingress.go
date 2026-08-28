package validation

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var ingressKinds = []string{"Ingress"}

// classInvalidCheck validates that ingress class name is valid.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type classInvalidCheck struct{}

func (c classInvalidCheck) ID() string {
	return "ingress/class-invalid"
}

func (c classInvalidCheck) Title() string {
	return "Ingress Class Must Be Valid"
}

func (c classInvalidCheck) Category() string {
	return "ingress"
}

func (c classInvalidCheck) Blocking() bool {
	return true
}

func (c classInvalidCheck) RenderSensitive() bool {
	return true
}

func (c classInvalidCheck) DocSkipper() []string {
	return ingressKinds
}

func (c classInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(data, &ing); err != nil {
		return nil
	}

	var findings []runtime.Finding

	if ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
		path := field.NewPath("spec").Child("ingressClassName")
		cls := *ing.Spec.IngressClassName

		// Validate ingress class name: must be a valid DNS subdomain or empty
		isValid := true
		if len(cls) > 253 {
			isValid = false
		}

		// Check each label segment
		if isValid {
			for _, label := range cls {
				if (label < 'a' || label > 'z') && (label < 'A' || label > 'Z') && (label < '0' || label > '9') && label != '.' && label != '-' {
					isValid = false
					break
				}
			}
		}

		// Check that segments don't start or end with hyphen
		if isValid {
			segs := splitByDots(cls)
			for _, seg := range segs {
				if len(seg) > 0 && (seg[0] == '-' || seg[0] == '.') {
					isValid = false
					break
				}
				lastIdx := len(seg) - 1
				if lastIdx >= 0 && (seg[lastIdx] == '-' || seg[lastIdx] == '.') {
					isValid = false
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
					Message: fmt.Sprintf("ingressClassName: invalid value: %q must be a valid DNS subdomain", cls),
					Kind:    "Ingress",
					Name:    ing.GetName(),
				},
			})
		}
	}

	return findings
}

// ruleHostInvalidCheck validates that host is a valid DNS hostname.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type ruleHostInvalidCheck struct{}

func (c ruleHostInvalidCheck) ID() string {
	return "ingress/rule-host-invalid"
}

func (c ruleHostInvalidCheck) Title() string {
	return "Ingress Rule Host Must Be Valid"
}

func (c ruleHostInvalidCheck) Category() string {
	return "ingress"
}

func (c ruleHostInvalidCheck) Blocking() bool {
	return true
}

func (c ruleHostInvalidCheck) RenderSensitive() bool {
	return true
}

func (c ruleHostInvalidCheck) DocSkipper() []string {
	return ingressKinds
}

func (c ruleHostInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(data, &ing); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range ing.Spec.Rules {
		rulePath := field.NewPath("spec").Child("rules").Index(i)

		if rule.Host == "" {
			continue
		}

		host := rule.Host

		isValid := true

		// Check length
		if len(host) > 253 {
			isValid = false
		}

		// Check characters
		if isValid {
			for _, label := range host {
				if (label < 'a' || label > 'z') && (label < 'A' || label > 'Z') && (label < '0' || label > '9') && label != '.' && label != '-' {
					isValid = false
					break
				}
			}
		}

		// Check that each label is <= 63 chars and doesn't start/end with hyphen
		if isValid {
			labels := splitByDots(host)
			for _, label := range labels {
				if len(label) > 63 {
					isValid = false
					break
				}
				if len(label) > 0 && (label[0] == '-' || label[0] == '.') {
					isValid = false
					break
				}
				lastIdx := len(label) - 1
				if lastIdx >= 0 && (label[lastIdx] == '-' || label[lastIdx] == '.') {
					isValid = false
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
					Path:    rulePath.Child("host").String(),
					Message: fmt.Sprintf("host: invalid value: %q must be a valid DNS hostname", host),
					Kind:    "Ingress",
					Name:    ing.GetName(),
				},
			})
		}
	}

	return findings
}

// backendServiceInvalidCheck validates that backend service name is valid.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type backendServiceInvalidCheck struct{}

func (c backendServiceInvalidCheck) ID() string {
	return "ingress/backend-service-invalid"
}

func (c backendServiceInvalidCheck) Title() string {
	return "Ingress Backend Service Must Be Valid"
}

func (c backendServiceInvalidCheck) Category() string {
	return "ingress"
}

func (c backendServiceInvalidCheck) Blocking() bool {
	return true
}

func (c backendServiceInvalidCheck) RenderSensitive() bool {
	return true
}

func (c backendServiceInvalidCheck) DocSkipper() []string {
	return ingressKinds
}

func (c backendServiceInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(data, &ing); err != nil {
		return nil
	}

	var findings []runtime.Finding

	validateBackendService := func(pathPrefix *field.Path, service networkingv1.IngressServiceBackend) {
		path := pathPrefix.Child("service").Child("name")
		name := service.Name

		if name == "" {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    path.String(),
					Message: "backend service: invalid value: service name must not be empty",
					Kind:    "Ingress",
					Name:    ing.GetName(),
				},
			})
			return
		}

		// Validate service name: must be a valid DNS subdomain name
		isValid := len(name) <= 253

		if isValid {
			for _, label := range name {
				if (label < 'a' || label > 'z') && (label < 'A' || label > 'Z') && (label < '0' || label > '9') && label != '-' {
					isValid = false
					break
				}
			}
		}

		if isValid {
			segs := splitByDots(name)
			for _, seg := range segs {
				if len(seg) > 0 && seg[0] == '-' {
					isValid = false
					break
				}
				lastIdx := len(seg) - 1
				if lastIdx >= 0 && seg[lastIdx] == '-' {
					isValid = false
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
					Message: fmt.Sprintf("backend service: invalid value: %q must be a valid DNS subdomain name", name),
					Kind:    "Ingress",
					Name:    ing.GetName(),
				},
			})
		}
	}

	// Check path-based rules
	for i, rule := range ing.Spec.Rules {
		rulePath := field.NewPath("spec").Child("rules").Index(i)

		if rule.HTTP == nil {
			continue
		}

		for j, path := range rule.HTTP.Paths {
			pathPath := rulePath.Child("http").Child("paths").Index(j)
			if path.Backend.Service != nil {
				validateBackendService(pathPath, *path.Backend.Service)
			}
		}
	}

	// Check default backend
	if ing.Spec.DefaultBackend != nil {
		if ing.Spec.DefaultBackend.Service != nil {
			validateBackendService(field.NewPath("spec").Child("defaultBackend"), *ing.Spec.DefaultBackend.Service)
		}
	}

	return findings
}

// pathTypeInvalidCheck validates that pathType is one of the allowed values.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type pathTypeInvalidCheck struct{}

func (c pathTypeInvalidCheck) ID() string {
	return "ingress/path-type-invalid"
}

func (c pathTypeInvalidCheck) Title() string {
	return "Ingress PathType Must Be Valid"
}

func (c pathTypeInvalidCheck) Category() string {
	return "ingress"
}

func (c pathTypeInvalidCheck) Blocking() bool {
	return true
}

func (c pathTypeInvalidCheck) RenderSensitive() bool {
	return true
}

func (c pathTypeInvalidCheck) DocSkipper() []string {
	return ingressKinds
}

func (c pathTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(data, &ing); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, rule := range ing.Spec.Rules {
		rulePath := field.NewPath("spec").Child("rules").Index(i)

		if rule.HTTP == nil {
			continue
		}

		for j, path := range rule.HTTP.Paths {
			pathPath := rulePath.Child("http").Child("paths").Index(j)

			if path.PathType == nil || string(*path.PathType) == "" {
				continue
			}

			validPathTypes := map[networkingv1.PathType]bool{
				networkingv1.PathTypeExact:                  true,
				networkingv1.PathTypePrefix:                 true,
				networkingv1.PathTypeImplementationSpecific: true,
			}

			if !validPathTypes[*path.PathType] {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    pathPath.Child("pathType").String(),
						Message: fmt.Sprintf("pathType: Unsupported value: %q", string(*path.PathType)),
						Kind:    "Ingress",
						Name:    ing.GetName(),
					},
				})
			}
		}
	}

	return findings
}

// tlsHostInvalidCheck validates that TLS hosts are valid DNS hostnames.
// Source: k8s.io/kubernetes/pkg/apis/networking/validation/validation.go
type tlsHostInvalidCheck struct{}

func (c tlsHostInvalidCheck) ID() string {
	return "ingress/tls-host-invalid"
}

func (c tlsHostInvalidCheck) Title() string {
	return "Ingress TLS Hosts Must Be Valid"
}

func (c tlsHostInvalidCheck) Category() string {
	return "ingress"
}

func (c tlsHostInvalidCheck) Blocking() bool {
	return true
}

func (c tlsHostInvalidCheck) RenderSensitive() bool {
	return true
}

func (c tlsHostInvalidCheck) DocSkipper() []string {
	return ingressKinds
}

func (c tlsHostInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(data, &ing); err != nil {
		return nil
	}

	var findings []runtime.Finding

	for i, tls := range ing.Spec.TLS {
		tlsPath := field.NewPath("spec").Child("tls").Index(i)

		for j, host := range tls.Hosts {
			hostPath := tlsPath.Child("hosts").Index(j)

			if host == "" {
				continue
			}

			isValid := true

			// Check length
			if len(host) > 253 {
				isValid = false
			}

			// Check characters
			if isValid {
				for _, label := range host {
					if (label < 'a' || label > 'z') && (label < 'A' || label > 'Z') && (label < '0' || label > '9') && label != '.' && label != '-' {
						isValid = false
						break
					}
				}
			}

			// Check that each label is <= 63 chars and doesn't start/end with hyphen
			if isValid {
				labels := splitByDots(host)
				for _, label := range labels {
					if len(label) > 63 {
						isValid = false
						break
					}
					if len(label) > 0 && (label[0] == '-' || label[0] == '.') {
						isValid = false
						break
					}
					lastIdx := len(label) - 1
					if lastIdx >= 0 && (label[lastIdx] == '-' || label[lastIdx] == '.') {
						isValid = false
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
						Path:    hostPath.String(),
						Message: fmt.Sprintf("tls hosts: invalid value: %q must be a valid DNS hostname", host),
						Kind:    "Ingress",
						Name:    ing.GetName(),
					},
				})
			}
		}
	}

	return findings
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

func init() {
	checks := []runtime.Check{
		classInvalidCheck{},
		ruleHostInvalidCheck{},
		backendServiceInvalidCheck{},
		pathTypeInvalidCheck{},
		tlsHostInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
