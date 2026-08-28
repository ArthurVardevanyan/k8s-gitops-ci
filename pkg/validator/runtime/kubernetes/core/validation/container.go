package validation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Check 1: container-name
// Container names must be 1-63 characters and conform to DNS-1123 subdomain.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:1765-1773

type containerNameCheck struct{}

func (c containerNameCheck) ID() string {
	return "container/container-name"
}

func (c containerNameCheck) Title() string {
	return "Container Name Must Be Valid DNS-1123 Subdomain"
}

func (c containerNameCheck) Category() string {
	return "container"
}

func (c containerNameCheck) Blocking() bool {
	return true
}

func (c containerNameCheck) RenderSensitive() bool {
	return true
}

func (c containerNameCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c containerNameCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		name := ctr.Container.Name
		if name == "" {
			continue
		}

		if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      ctr.Path.Child("name").String(),
					Message:   fmt.Sprintf("container name %q: %s", name, strings.Join(errs, "; ")),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Container: ctr.Container.Name,
				},
			})
		}
	}

	return findings
}

// Check 2: duplicate-container-names
// All containers (regular + init) must have unique names within a Pod.

type duplicateContainerNamesCheck struct{}

func (c duplicateContainerNamesCheck) ID() string {
	return "container/duplicate-container-names"
}

func (c duplicateContainerNamesCheck) Title() string {
	return "Duplicate Container Names Not Allowed"
}

func (c duplicateContainerNamesCheck) Category() string {
	return "container"
}

func (c duplicateContainerNamesCheck) Blocking() bool {
	return true
}

func (c duplicateContainerNamesCheck) RenderSensitive() bool {
	return true
}

func (c duplicateContainerNamesCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c duplicateContainerNamesCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	names := make(map[string]int)
	for _, ctr := range containers {
		if ctr.Container.Name == "" {
			continue
		}
		names[ctr.Container.Name]++
	}

	for name, count := range names {
		if count > 1 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("containers").Key(name).String(),
					Message:   fmt.Sprintf("duplicate container name %q appears %d times", name, count),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// Check 3: port-name-format
// Named ports must be 1-63 characters, alphanumeric + hyphen + underscore,
// starting with letter, ending with alphanumeric.

var portNameRegex = regexp.MustCompile(`^[a-z]([a-z0-9_-]*[a-z0-9])?$`)

type portNameFormatCheck struct{}

func (c portNameFormatCheck) ID() string {
	return "container/port-name-format"
}

func (c portNameFormatCheck) Title() string {
	return "Port Name Must Follow Valid Format"
}

func (c portNameFormatCheck) Category() string {
	return "container"
}

func (c portNameFormatCheck) Blocking() bool {
	return true
}

func (c portNameFormatCheck) RenderSensitive() bool {
	return true
}

func (c portNameFormatCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c portNameFormatCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		for _, port := range ctr.Container.Ports {
			name := port.Name
			if name == "" {
				continue
			}

			if len(name) > 63 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Key(name).Child("name").String(),
						Message:   fmt.Sprintf("port name %q in container %q: length must be 1-63 characters (%d)", name, ctr.Container.Name, len(name)),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     name,
					},
				})
				continue
			}

			if !portNameRegex.MatchString(name) {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Key(name).Child("name").String(),
						Message:   fmt.Sprintf("port name %q in container %q: a valid port name must be an empty string or consist of alphanumeric characters or '-!_'", name, ctr.Container.Name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     name,
					},
				})
			}
		}
	}

	return findings
}

// Check 4: duplicate-port-names
// All ports must have unique names within a container.

type duplicatePortNamesCheck struct{}

func (c duplicatePortNamesCheck) ID() string {
	return "container/duplicate-port-names"
}

func (c duplicatePortNamesCheck) Title() string {
	return "Duplicate Port Names Not Allowed"
}

func (c duplicatePortNamesCheck) Category() string {
	return "container"
}

func (c duplicatePortNamesCheck) Blocking() bool {
	return true
}

func (c duplicatePortNamesCheck) RenderSensitive() bool {
	return true
}

func (c duplicatePortNamesCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c duplicatePortNamesCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		seen := make(map[string]bool)
		for _, port := range ctr.Container.Ports {
			name := port.Name
			if name == "" {
				continue
			}
			if seen[name] {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Key(name).Child("name").String(),
						Message:   fmt.Sprintf("duplicate port name %q in container %q", name, ctr.Container.Name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     name,
					},
				})
				break
			}
			seen[name] = true
		}
	}

	return findings
}

// Check 5: port-number-range
// Port numbers must be in range 1-65535.

type portNumberRangeCheck struct{}

func (c portNumberRangeCheck) ID() string {
	return "container/port-number-range"
}

func (c portNumberRangeCheck) Title() string {
	return "Port Number Must Be In Range 1-65535"
}

func (c portNumberRangeCheck) Category() string {
	return "container"
}

func (c portNumberRangeCheck) Blocking() bool {
	return true
}

func (c portNumberRangeCheck) RenderSensitive() bool {
	return true
}

func (c portNumberRangeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c portNumberRangeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		for _, port := range ctr.Container.Ports {
			if port.ContainerPort < 1 || port.ContainerPort > 65535 {
				idx := getPortIndex(ctr.Container.Ports, port)
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Index(idx).Child("containerPort").String(),
						Message:   fmt.Sprintf("invalid port %d in container %q: port must be 1-65535", port.ContainerPort, ctr.Container.Name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     fmt.Sprintf("%d", port.ContainerPort),
					},
				})
			}
		}
	}

	return findings
}

func getPortIndex(ports []corev1.ContainerPort, target corev1.ContainerPort) int {
	for i, p := range ports {
		if p.ContainerPort == target.ContainerPort && p.Protocol == target.Protocol {
			return i
		}
	}
	return 0
}

// Check 6: duplicate-port-numbers
// All ports must have unique numbers within a container (except for
// TCP/UDP protocol variants).

type duplicatePortNumbersCheck struct{}

func (c duplicatePortNumbersCheck) ID() string {
	return "container/duplicate-port-numbers"
}

func (c duplicatePortNumbersCheck) Title() string {
	return "Duplicate Port Numbers Not Allowed"
}

func (c duplicatePortNumbersCheck) Category() string {
	return "container"
}

func (c duplicatePortNumbersCheck) Blocking() bool {
	return true
}

func (c duplicatePortNumbersCheck) RenderSensitive() bool {
	return true
}

func (c duplicatePortNumbersCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c duplicatePortNumbersCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		portMap := make(map[string]int)
		for _, port := range ctr.Container.Ports {
			proto := string(port.Protocol)
			if proto == "" {
				proto = string(corev1.ProtocolTCP)
			}
			key := fmt.Sprintf("%d/%s", port.ContainerPort, proto)
			portMap[key]++
			if portMap[key] > 1 {
				idx := getPortIndexWithProtocol(ctr.Container.Ports, port.ContainerPort, proto)
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Index(idx).Child("containerPort").String(),
						Message:   fmt.Sprintf("duplicate port number %d/%s in container %q", port.ContainerPort, proto, ctr.Container.Name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     fmt.Sprintf("%d", port.ContainerPort),
					},
				})
				break
			}
		}
	}

	return findings
}

func getPortIndexWithProtocol(ports []corev1.ContainerPort, portNum int32, proto string) int {
	for i, p := range ports {
		pProto := string(p.Protocol)
		if pProto == "" {
			pProto = string(corev1.ProtocolTCP)
		}
		if p.ContainerPort == portNum && pProto == proto {
			return i
		}
	}
	return 0
}

// Check 7: port-name-unique
// If any port has a name, ALL ports in the container must have names
// (Kubernetes enforcement rule).

type portNameUniqueCheck struct{}

func (c portNameUniqueCheck) ID() string {
	return "container/port-name-unique"
}

func (c portNameUniqueCheck) Title() string {
	return "All Ports Must Have Names If Any Port Is Named"
}

func (c portNameUniqueCheck) Category() string {
	return "container"
}

func (c portNameUniqueCheck) Blocking() bool {
	return true
}

func (c portNameUniqueCheck) RenderSensitive() bool {
	return true
}

func (c portNameUniqueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c portNameUniqueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		ports := ctr.Container.Ports
		if len(ports) == 0 {
			continue
		}

		hasAnyNamed := false
		for _, port := range ports {
			if port.Name != "" {
				hasAnyNamed = true
				break
			}
		}

		if !hasAnyNamed {
			continue
		}

		for _, port := range ports {
			if port.Name == "" {
				idx := 0
				for i, p := range ports {
					if p.ContainerPort == port.ContainerPort {
						idx = i
						break
					}
				}
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Index(idx).Child("name").String(),
						Message:   fmt.Sprintf("container %q: if one port is named, all ports must have unique names; port %d is unnamed", ctr.Container.Name, port.ContainerPort),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
					},
				})
			}
		}
	}

	return findings
}

// Check 8: image-pull-policy
// imagePullPolicy must be one of: Always, Never, IfNotPresent.

type imagePullPolicyCheck struct{}

func (c imagePullPolicyCheck) ID() string {
	return "container/image-pull-policy"
}

func (c imagePullPolicyCheck) Title() string {
	return "ImagePullPolicy Must Be Valid"
}

func (c imagePullPolicyCheck) Category() string {
	return "container"
}

func (c imagePullPolicyCheck) Blocking() bool {
	return true
}

func (c imagePullPolicyCheck) RenderSensitive() bool {
	return true
}

func (c imagePullPolicyCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c imagePullPolicyCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validPolicies := map[corev1.PullPolicy]bool{
		corev1.PullAlways:       true,
		corev1.PullNever:        true,
		corev1.PullIfNotPresent: true,
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		policy := ctr.Container.ImagePullPolicy
		if policy == "" || validPolicies[policy] {
			continue
		}

		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      ctr.Path.Child("imagePullPolicy").String(),
				Message:   fmt.Sprintf("container %q: imagePullPolicy: Unsupported value: %q: supported values: 'Always', 'Never', 'IfNotPresent'", ctr.Container.Name, string(policy)),
				Kind:      info.Kind,
				Name:      info.Name,
				Namespace: info.Namespace,
				Container: ctr.Container.Name,
				Value:     string(policy),
			},
		})
	}

	return findings
}

// Check 9: mount-propagation-value
// mountPropagation must be one of: None, HostToContainer, Bidirectional.

type mountPropagationValueCheck struct{}

func (c mountPropagationValueCheck) ID() string {
	return "container/mount-propagation-value"
}

func (c mountPropagationValueCheck) Title() string {
	return "MountPropagation Must Be Valid"
}

func (c mountPropagationValueCheck) Category() string {
	return "container"
}

func (c mountPropagationValueCheck) Blocking() bool {
	return true
}

func (c mountPropagationValueCheck) RenderSensitive() bool {
	return true
}

func (c mountPropagationValueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c mountPropagationValueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validPropagations := map[corev1.MountPropagationMode]bool{
		corev1.MountPropagationNone:            true,
		corev1.MountPropagationHostToContainer: true,
		corev1.MountPropagationBidirectional:   true,
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		for i, vm := range ctr.Container.VolumeMounts {
			if vm.MountPropagation == nil || validPropagations[*vm.MountPropagation] {
				continue
			}

			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      ctr.Path.Child("volumeMounts").Index(i).Child("mountPropagation").String(),
					Message:   fmt.Sprintf("container %q volumeMount %q: mountPropagation: Unsupported value: %q", ctr.Container.Name, vm.Name, string(*vm.MountPropagation)),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Container: ctr.Container.Name,
					Value:     string(*vm.MountPropagation),
				},
			})
		}
	}

	return findings
}

// Check 10: restart-policy-value
// restartPolicy must be one of: Always, OnFailure, Never.

type restartPolicyValueCheck struct{}

func (c restartPolicyValueCheck) ID() string {
	return "container/restart-policy-value"
}

func (c restartPolicyValueCheck) Title() string {
	return "RestartPolicy Must Be Valid"
}

func (c restartPolicyValueCheck) Category() string {
	return "container"
}

func (c restartPolicyValueCheck) Blocking() bool {
	return true
}

func (c restartPolicyValueCheck) RenderSensitive() bool {
	return true
}

func (c restartPolicyValueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c restartPolicyValueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validPolicies := map[corev1.RestartPolicy]bool{
		corev1.RestartPolicyAlways:    true,
		corev1.RestartPolicyOnFailure: true,
		corev1.RestartPolicyNever:     true,
	}

	findings := make([]runtime.Finding, 0, 1)
	policy := info.PodSpec.RestartPolicy

	if policy == "" || validPolicies[policy] {
		return findings
	}

	findings = append(findings, runtime.Finding{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:      field.NewPath("spec").Child("restartPolicy").String(),
			Message:   fmt.Sprintf("restartPolicy: Unsupported value: %q", string(policy)),
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     string(policy),
		},
	})

	return findings
}

// Check 11: termination-message-path
// terminationMessagePath must be a valid absolute path.

type terminationMessagePathCheck struct{}

func (c terminationMessagePathCheck) ID() string {
	return "container/termination-message-path"
}

func (c terminationMessagePathCheck) Title() string {
	return "TerminationMessagePath Must Be Valid Absolute Path"
}

func (c terminationMessagePathCheck) Category() string {
	return "container"
}

func (c terminationMessagePathCheck) Blocking() bool {
	return true
}

func (c terminationMessagePathCheck) RenderSensitive() bool {
	return true
}

func (c terminationMessagePathCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c terminationMessagePathCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		path := ctr.Container.TerminationMessagePath
		if path == "" {
			continue
		}

		if !filepath.IsAbs(path) {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      ctr.Path.Child("terminationMessagePath").String(),
					Message:   fmt.Sprintf("container %q: terminationMessagePath: invalid path %q: must be an absolute path", ctr.Container.Name, path),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Container: ctr.Container.Name,
					Value:     path,
				},
			})
		}
	}

	return findings
}

// Check 12: termination-message-policy-value
// terminationMessagePolicy must be one of: File, FallbackToLogsOnError.

type terminationMessagePolicyValueCheck struct{}

func (c terminationMessagePolicyValueCheck) ID() string {
	return "container/termination-message-policy-value"
}

func (c terminationMessagePolicyValueCheck) Title() string {
	return "TerminationMessagePolicy Must Be Valid"
}

func (c terminationMessagePolicyValueCheck) Category() string {
	return "container"
}

func (c terminationMessagePolicyValueCheck) Blocking() bool {
	return true
}

func (c terminationMessagePolicyValueCheck) RenderSensitive() bool {
	return true
}

func (c terminationMessagePolicyValueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c terminationMessagePolicyValueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validPolicies := map[corev1.TerminationMessagePolicy]bool{
		corev1.TerminationMessageReadFile:              true,
		corev1.TerminationMessageFallbackToLogsOnError: true,
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		policy := ctr.Container.TerminationMessagePolicy
		if policy == "" || validPolicies[policy] {
			continue
		}

		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      ctr.Path.Child("terminationMessagePolicy").String(),
				Message:   fmt.Sprintf("container %q: terminationMessagePolicy: Unsupported value: %q", ctr.Container.Name, string(policy)),
				Kind:      info.Kind,
				Name:      info.Name,
				Namespace: info.Namespace,
				Container: ctr.Container.Name,
				Value:     string(policy),
			},
		})
	}

	return findings
}

// Check 13: env-name-duplicate
// Environment variable names must be unique within a container.

type envNameDuplicateCheck struct{}

func (c envNameDuplicateCheck) ID() string {
	return "container/env-name-duplicate"
}

func (c envNameDuplicateCheck) Title() string {
	return "Duplicate Environment Variable Names Not Allowed"
}

func (c envNameDuplicateCheck) Category() string {
	return "container"
}

func (c envNameDuplicateCheck) Blocking() bool {
	return true
}

func (c envNameDuplicateCheck) RenderSensitive() bool {
	return true
}

func (c envNameDuplicateCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c envNameDuplicateCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		seen := make(map[string]bool)
		for i, env := range ctr.Container.Env {
			if seen[env.Name] {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("env").Index(i).Child("name").String(),
						Message:   fmt.Sprintf("duplicate environment variable name %q in container %q", env.Name, ctr.Container.Name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     env.Name,
					},
				})
				break
			}
			seen[env.Name] = true
		}
	}

	return findings
}

// Check 14: env-name-format
// Environment variable names must be valid (not empty, valid identifier).

// validEnvName matches POSIX shell variable names: must start with letter or
// underscore, followed by letters, digits, or underscores.
var validEnvName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type envNameFormatCheck struct{}

func (c envNameFormatCheck) ID() string {
	return "container/env-name-format"
}

func (c envNameFormatCheck) Title() string {
	return "Environment Variable Name Must Be Valid Identifier"
}

func (c envNameFormatCheck) Category() string {
	return "container"
}

func (c envNameFormatCheck) Blocking() bool {
	return true
}

func (c envNameFormatCheck) RenderSensitive() bool {
	return true
}

func (c envNameFormatCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c envNameFormatCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		for i, env := range ctr.Container.Env {
			name := env.Name
			if name == "" || !validEnvName.MatchString(name) {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("env").Index(i).Child("name").String(),
						Message:   fmt.Sprintf("container %q: environment variable name %q must be a valid identifier", ctr.Container.Name, name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     name,
					},
				})
			}
		}
	}

	return findings
}

// Check 15: volume-mount-name-duplicate
// Volume mount names must match a volume definition name.

type volumeMountNameDuplicateCheck struct{}

func (c volumeMountNameDuplicateCheck) ID() string {
	return "container/volume-mount-name-duplicate"
}

func (c volumeMountNameDuplicateCheck) Title() string {
	return "VolumeMount Name Must Match a Volume Definition"
}

func (c volumeMountNameDuplicateCheck) Category() string {
	return "container"
}

func (c volumeMountNameDuplicateCheck) Blocking() bool {
	return true
}

func (c volumeMountNameDuplicateCheck) RenderSensitive() bool {
	return true
}

func (c volumeMountNameDuplicateCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c volumeMountNameDuplicateCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	// Build set of defined volume names
	volumeNames := make(map[string]bool)
	for _, vol := range info.PodSpec.Volumes {
		volumeNames[vol.Name] = true
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		for i, vm := range ctr.Container.VolumeMounts {
			if !volumeNames[vm.Name] {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("volumeMounts").Index(i).Child("name").String(),
						Message:   fmt.Sprintf("container %q volumeMount %q: volumeMounts.name: not found \u2014 no volume named %q is defined", ctr.Container.Name, vm.Name, vm.Name),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     vm.Name,
					},
				})
			}
		}
	}
	return findings
}
