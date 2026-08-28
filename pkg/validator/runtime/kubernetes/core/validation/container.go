package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

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

func (c duplicateContainerNamesCheck) Kinds() []string {
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

func (c duplicatePortNamesCheck) Kinds() []string {
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

func (c portNumberRangeCheck) Kinds() []string {
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

func (c imagePullPolicyCheck) Kinds() []string {
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

func (c mountPropagationValueCheck) Kinds() []string {
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

func (c terminationMessagePolicyValueCheck) Kinds() []string {
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

func (c volumeMountNameDuplicateCheck) Kinds() []string {
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
