package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

type duplicateContainerNamesCheck struct{ runtime.Meta }

func newDuplicateContainerNamesCheck() duplicateContainerNamesCheck {
	return duplicateContainerNamesCheck{runtime.Meta{
		RuleID:    "container/duplicate-container-names",
		RuleTitle: "Duplicate Container Names Not Allowed",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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
				Finding: check.Finding{
					Path:      field.NewPath(info.PodSpecPath).Child("containers").Key(name).String(),
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

type duplicatePortNamesCheck struct{ runtime.Meta }

func newDuplicatePortNamesCheck() duplicatePortNamesCheck {
	return duplicatePortNamesCheck{runtime.Meta{
		RuleID:    "container/duplicate-port-names",
		RuleTitle: "Duplicate Port Names Not Allowed",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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

type portNumberRangeCheck struct{ runtime.Meta }

func newPortNumberRangeCheck() portNumberRangeCheck {
	return portNumberRangeCheck{runtime.Meta{
		RuleID:    "container/port-number-range",
		RuleTitle: "Port Number Must Be In Range 1-65535",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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

type imagePullPolicyCheck struct{ runtime.Meta }

func newImagePullPolicyCheck() imagePullPolicyCheck {
	return imagePullPolicyCheck{runtime.Meta{
		RuleID:    "container/image-pull-policy",
		RuleTitle: "ImagePullPolicy Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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

type mountPropagationValueCheck struct{ runtime.Meta }

func newMountPropagationValueCheck() mountPropagationValueCheck {
	return mountPropagationValueCheck{runtime.Meta{
		RuleID:    "container/mount-propagation-value",
		RuleTitle: "MountPropagation Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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

type terminationMessagePolicyValueCheck struct{ runtime.Meta }

func newTerminationMessagePolicyValueCheck() terminationMessagePolicyValueCheck {
	return terminationMessagePolicyValueCheck{runtime.Meta{
		RuleID:    "container/termination-message-policy-value",
		RuleTitle: "TerminationMessagePolicy Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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

type volumeMountNameUndefinedCheck struct{ runtime.Meta }

func newVolumeMountNameUndefinedCheck() volumeMountNameUndefinedCheck {
	return volumeMountNameUndefinedCheck{runtime.Meta{
		RuleID:    "container/volume-mount-name-undefined",
		RuleTitle: "VolumeMount Name Must Match a Volume Definition",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

func (c volumeMountNameUndefinedCheck) Run(data []byte, source string) []runtime.Finding {
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
