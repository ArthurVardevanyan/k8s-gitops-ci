package runtime

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// AllContainers builds the full container list (regular + init) with path context.
func AllContainers(info *PodSpecInfo) []ContainerWithPath {
	out := make([]ContainerWithPath, 0, len(info.Containers)+len(info.InitContainers))
	for i := range info.Containers {
		out = append(out, ContainerWithPath{
			Container: info.Containers[i],
			Path:      field.NewPath(info.ContainersPath).Key(info.Containers[i].Name),
			IsInit:    false,
		})
	}
	for i := range info.InitContainers {
		out = append(out, ContainerWithPath{
			Container: info.InitContainers[i],
			Path:      field.NewPath(info.InitContainersPath).Key(info.InitContainers[i].Name),
			IsInit:    true,
		})
	}
	return out
}

func validateDuplicateContainerNames(info *PodSpecInfo) []Finding {
	var findings []Finding

	containers := AllContainers(info)

	names := make(map[string]int)
	for _, c := range containers {
		if c.Container.Name == "" {
			continue
		}
		names[c.Container.Name]++
	}

	for name, count := range names {
		if count > 1 {
			findings = append(findings, Finding{
				RuleID:    "container/duplicate-container-names",
				RuleTitle: "Duplicate Container Names",
				Category:  "container",
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("containers").Key(name).String(),
					Message:   fmt.Sprintf("duplicate container names: %q appears %d times", name, count),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

func validateMissingContainerName(info *PodSpecInfo) []Finding {
	var findings []Finding

	containers := AllContainers(info)

	for _, c := range containers {
		if c.Container.Name == "" {
			findings = append(findings, Finding{
				RuleID:    "container/missing-container-name",
				RuleTitle: "Container Name Required",
				Category:  "container",
				Finding: check.Finding{
					Path:      c.Path.String(),
					Message:   "container name is required",
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

func validateImagePullPolicyAlwaysForLatest(info *PodSpecInfo) []Finding {
	var findings []Finding

	containers := AllContainers(info)

	for _, c := range containers {
		image := c.Container.Image
		if image == "" {
			continue
		}

		usesLatest := false
		tag := ""

		// Handle image references with registry (e.g., registry.io/repo:tag)
		parts := strings.Split(image, "/")
		lastPart := parts[len(parts)-1]

		if strings.Contains(lastPart, ":") {
			tag = strings.Split(lastPart, ":")[1]
			// Handle digest references (sha256:...)
			if strings.HasPrefix(tag, "sha256:") {
				continue
			}
			usesLatest = tag == "latest"
		} else {
			// No tag specified — implicitly latest
			usesLatest = true
		}

		if !usesLatest {
			continue
		}

		policy := c.Container.ImagePullPolicy
		if policy == corev1.PullAlways {
			continue
		}

		findings = append(findings, Finding{
			RuleID:    "container/image-pull-policy-always-for-latest",
			RuleTitle: "ImagePullPolicy Should Be Always For Latest Tag",
			Category:  "container",
			Finding: check.Finding{
				Path:      c.Path.Child("image").String(),
				Message:   fmt.Sprintf("container %q: image %q uses 'latest' tag or no tag — ImagePullPolicy should be 'Always'", c.Container.Name, image),
				Kind:      info.Kind,
				Name:      info.Name,
				Namespace: info.Namespace,
				Container: c.Container.Name,
				Value:     image,
			},
		})
	}

	return findings
}

func validateMissingProbe(
	info *PodSpecInfo,
	probeField string,
	probeFunc func(corev1.Container) bool,
	ruleID string,
	ruleTitle string,
	message string,
) []Finding {
	var findings []Finding

	containers := AllContainers(info)

	for _, c := range containers {
		if probeFunc(c.Container) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:    ruleID,
			RuleTitle: ruleTitle,
			Category:  "container",
			Finding: check.Finding{
				Path:      c.Path.Child(probeField).String(),
				Message:   fmt.Sprintf("container %q: "+message, c.Container.Name),
				Kind:      info.Kind,
				Name:      info.Name,
				Namespace: info.Namespace,
				Container: c.Container.Name,
			},
		})
	}

	return findings
}

func validateMissingLivenessProbe(info *PodSpecInfo) []Finding {
	return validateMissingProbe(info, "livenessProbe", func(c corev1.Container) bool {
		return c.LivenessProbe != nil
	}, "container/missing-liveness-probe", "Missing Liveness Probe", "no liveness probe configured")
}

func validateMissingReadinessProbe(info *PodSpecInfo) []Finding {
	return validateMissingProbe(info, "readinessProbe", func(c corev1.Container) bool {
		return c.ReadinessProbe != nil
	}, "container/missing-readiness-probe", "Missing Readiness Probe", "no readiness probe configured")
}

func validateHostPortConflict(info *PodSpecInfo) []Finding {
	var findings []Finding

	containers := AllContainers(info)

	type hostPortKey struct {
		port     int32
		protocol corev1.Protocol
	}

	hostPortMap := make(map[hostPortKey][]string)

	for _, c := range containers {
		for _, port := range c.Container.Ports {
			if port.HostPort == 0 {
				continue
			}
			key := hostPortKey{port: port.HostPort, protocol: port.Protocol}
			hostPortMap[key] = append(hostPortMap[key], c.Container.Name)
		}
	}

	for key, containers := range hostPortMap {
		if len(containers) > 1 {
			findings = append(findings, Finding{
				RuleID:    "container/hostport-conflict",
				RuleTitle: "HostPort Conflict",
				Category:  "container",
				Finding: check.Finding{
					Path:      field.NewPath("spec").Child("containers").Child("ports").String(),
					Message:   fmt.Sprintf("hostPort %d/%s is used by multiple containers: %s", key.port, key.protocol, strings.Join(containers, ", ")),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
				},
			})
		}
	}

	return findings
}

// ValidateContainers runs all container-level validation rules.
func ValidateContainers(info *PodSpecInfo) []Finding {
	findings := make([]Finding, 0, 6)
	findings = append(findings, validateDuplicateContainerNames(info)...)
	findings = append(findings, validateMissingContainerName(info)...)
	findings = append(findings, validateImagePullPolicyAlwaysForLatest(info)...)
	findings = append(findings, validateMissingLivenessProbe(info)...)
	findings = append(findings, validateMissingReadinessProbe(info)...)
	findings = append(findings, validateHostPortConflict(info)...)
	return findings
}
