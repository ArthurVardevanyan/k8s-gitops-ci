package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// privilegedAllowPrivEscCheck detects privileged=true AND allowPrivilegeEscalation=false,
// which Kubernetes rejects at admission time.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type privilegedAllowPrivEscCheck struct{}

func (c privilegedAllowPrivEscCheck) ID() string {
	return "security-context/privileged-allow-priv-esc"
}

func (c privilegedAllowPrivEscCheck) Title() string {
	return "Privileged Container Must Allow Privilege Escalation"
}

func (c privilegedAllowPrivEscCheck) Category() string {
	return "security-context"
}

func (c privilegedAllowPrivEscCheck) Blocking() bool {
	return true
}

func (c privilegedAllowPrivEscCheck) RenderSensitive() bool {
	return true
}

func (c privilegedAllowPrivEscCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c privilegedAllowPrivEscCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		if ctr.Container.SecurityContext == nil {
			continue
		}

		sc := ctr.Container.SecurityContext
		scPath := ctr.Path.Child("securityContext")

		if sc.Privileged != nil && *sc.Privileged {
			if sc.AllowPrivilegeEscalation == nil || !*sc.AllowPrivilegeEscalation {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      scPath.String(),
						Message:   fmt.Sprintf("container %q: cannot set allowPrivilegeEscalation to false and privileged to true", ctr.Container.Name),
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

// allowPrivEscCapSysAdminCheck detects allowPrivilegeEscalation=false AND
// capabilities.Add CAP_SYS_ADMIN, which Kubernetes rejects at admission time.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type allowPrivEscCapSysAdminCheck struct{}

func (c allowPrivEscCapSysAdminCheck) ID() string {
	return "security-context/allow-priv-esc-cap-sys-admin"
}

func (c allowPrivEscCapSysAdminCheck) Title() string {
	return "CAP_SYS_ADMIN Requires allowPrivilegeEscalation"
}

func (c allowPrivEscCapSysAdminCheck) Category() string {
	return "security-context"
}

func (c allowPrivEscCapSysAdminCheck) Blocking() bool {
	return true
}

func (c allowPrivEscCapSysAdminCheck) RenderSensitive() bool {
	return true
}

func (c allowPrivEscCapSysAdminCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c allowPrivEscCapSysAdminCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		if ctr.Container.SecurityContext == nil {
			continue
		}

		sc := ctr.Container.SecurityContext
		scPath := ctr.Path.Child("securityContext")

		if sc.AllowPrivilegeEscalation != nil && !*sc.AllowPrivilegeEscalation {
			hasCapSysAdmin := false
			for _, cap := range sc.Capabilities.Add {
				if cap == corev1.Capability("CAP_SYS_ADMIN") {
					hasCapSysAdmin = true
					break
				}
			}
			if hasCapSysAdmin {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      scPath.String(),
						Message:   fmt.Sprintf("container %q: cannot set allowPrivilegeEscalation to false and capabilities.Add CAP_SYS_ADMIN", ctr.Container.Name),
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

// runAsNonRootUserZeroCheck detects runAsNonRoot=true AND runAsUser=0 at the
// container level, which Kubernetes rejects at admission time.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type runAsNonRootUserZeroCheck struct{}

func (c runAsNonRootUserZeroCheck) ID() string {
	return "security-context/run-as-non-root-user-zero"
}

func (c runAsNonRootUserZeroCheck) Title() string {
	return "runAsNonRoot Must Not Specify runAsUser 0"
}

func (c runAsNonRootUserZeroCheck) Category() string {
	return "security-context"
}

func (c runAsNonRootUserZeroCheck) Blocking() bool {
	return true
}

func (c runAsNonRootUserZeroCheck) RenderSensitive() bool {
	return true
}

func (c runAsNonRootUserZeroCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c runAsNonRootUserZeroCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		if ctr.Container.SecurityContext == nil {
			continue
		}

		sc := ctr.Container.SecurityContext
		scPath := ctr.Path.Child("securityContext")

		if sc.RunAsNonRoot != nil && *sc.RunAsNonRoot {
			if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      scPath.String(),
						Message:   fmt.Sprintf("container %q: cannot set runAsNonRoot to true and runAsUser to 0", ctr.Container.Name),
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

// runAsNonRootUserZeroPodLevelCheck detects pod-level runAsNonRoot=true with
// no explicit runAsUser at container level but pod-level runAsUser=0,
// which Kubernetes rejects at admission time.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type runAsNonRootUserZeroPodLevelCheck struct{}

func (c runAsNonRootUserZeroPodLevelCheck) ID() string {
	return "security-context/run-as-non-root-user-zero-pod-level"
}

func (c runAsNonRootUserZeroPodLevelCheck) Title() string {
	return "Pod-Level runAsNonRoot Must Not Allow runAsUser 0"
}

func (c runAsNonRootUserZeroPodLevelCheck) Category() string {
	return "security-context"
}

func (c runAsNonRootUserZeroPodLevelCheck) Blocking() bool {
	return true
}

func (c runAsNonRootUserZeroPodLevelCheck) RenderSensitive() bool {
	return true
}

func (c runAsNonRootUserZeroPodLevelCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c runAsNonRootUserZeroPodLevelCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	podSC := info.PodSecurityContext

	if podSC == nil {
		return findings
	}

	if podSC.RunAsNonRoot != nil && *podSC.RunAsNonRoot {
		if podSC.RunAsUser != nil && *podSC.RunAsUser == 0 {
			containers := runtime.AllContainers(info)
			for _, ctr := range containers {
				ctrSCPath := ctr.Path.Child("securityContext")
				if ctr.Container.SecurityContext != nil && ctr.Container.SecurityContext.RunAsUser != nil {
					continue
				}
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      ctrSCPath.String(),
						Message:   fmt.Sprintf("container %q: pod-level runAsNonRoot=true and runAsUser=0 — container inherits root user", ctr.Container.Name),
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

// invalidPrivilegeEscalationFieldCheck detects containers where
// allowPrivilegeEscalation is not explicitly set, which Kubernetes may reject
// when strict security feature gates are enabled.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type invalidPrivilegeEscalationFieldCheck struct{}

func (c invalidPrivilegeEscalationFieldCheck) ID() string {
	return "security-context/invalid-privilege-escalation-field"
}

func (c invalidPrivilegeEscalationFieldCheck) Title() string {
	return "allowPrivilegeEscalation Must Be Explicitly Set"
}

func (c invalidPrivilegeEscalationFieldCheck) Category() string {
	return "security-context"
}

func (c invalidPrivilegeEscalationFieldCheck) Blocking() bool {
	return true
}

func (c invalidPrivilegeEscalationFieldCheck) RenderSensitive() bool {
	return true
}

func (c invalidPrivilegeEscalationFieldCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c invalidPrivilegeEscalationFieldCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		if ctr.Container.SecurityContext == nil {
			continue
		}

		sc := ctr.Container.SecurityContext
		scPath := ctr.Path.Child("securityContext")

		if sc.AllowPrivilegeEscalation == nil {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      scPath.String(),
					Message:   fmt.Sprintf("container %q: allowPrivilegeEscalation must be explicitly set", ctr.Container.Name),
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

// capabilitiesDropAllAddSysAdminCheck detects when capabilities.Drop includes
// ALL but capabilities.Add is not empty, which Kubernetes rejects at admission.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type capabilitiesDropAllAddSysAdminCheck struct{}

func (c capabilitiesDropAllAddSysAdminCheck) ID() string {
	return "security-context/capabilities-drop-all-add-sys-admin"
}

func (c capabilitiesDropAllAddSysAdminCheck) Title() string {
	return "Cannot Add Capabilities When ALL Are Dropped"
}

func (c capabilitiesDropAllAddSysAdminCheck) Category() string {
	return "security-context"
}

func (c capabilitiesDropAllAddSysAdminCheck) Blocking() bool {
	return true
}

func (c capabilitiesDropAllAddSysAdminCheck) RenderSensitive() bool {
	return true
}

func (c capabilitiesDropAllAddSysAdminCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c capabilitiesDropAllAddSysAdminCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		if ctr.Container.SecurityContext == nil {
			continue
		}

		sc := ctr.Container.SecurityContext
		scPath := ctr.Path.Child("securityContext")

		dropsAll := false
		for _, cap := range sc.Capabilities.Drop {
			if cap == corev1.Capability("ALL") {
				dropsAll = true
				break
			}
		}

		if dropsAll && len(sc.Capabilities.Add) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      scPath.String(),
					Message:   fmt.Sprintf("container %q: cannot add capabilities when all capabilities are dropped", ctr.Container.Name),
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

// ValidateSecurityContext runs all security context validation rules against a
// parsed PodSpecInfo. It is called by kubernetes/register.go init() to register
// all checks and is the public entry point for running these validations.
func ValidateSecurityContext(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	checks := []struct {
		name string
		fn   func(*runtime.PodSpecInfo) []runtime.Finding
	}{
		{"privileged-allow-priv-esc", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			containers := runtime.AllContainers(info)
			for _, ctr := range containers {
				if ctr.Container.SecurityContext == nil {
					continue
				}
				sc := ctr.Container.SecurityContext
				scPath := ctr.Path.Child("securityContext")
				if sc.Privileged != nil && *sc.Privileged {
					if sc.AllowPrivilegeEscalation == nil || !*sc.AllowPrivilegeEscalation {
						findings = append(findings, runtime.Finding{
							RuleID:    "security-context/privileged-allow-priv-esc",
							RuleTitle: "Privileged Container Must Allow Privilege Escalation",
							Category:  "security-context",
							Finding: check.Finding{
								Path:      scPath.String(),
								Message:   fmt.Sprintf("container %q: cannot set allowPrivilegeEscalation to false and privileged to true", ctr.Container.Name),
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
		}},
		{"allow-priv-esc-cap-sys-admin", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			containers := runtime.AllContainers(info)
			for _, ctr := range containers {
				if ctr.Container.SecurityContext == nil {
					continue
				}
				sc := ctr.Container.SecurityContext
				scPath := ctr.Path.Child("securityContext")
				if sc.AllowPrivilegeEscalation != nil && !*sc.AllowPrivilegeEscalation {
					hasCapSysAdmin := false
					for _, cap := range sc.Capabilities.Add {
						if cap == corev1.Capability("CAP_SYS_ADMIN") {
							hasCapSysAdmin = true
							break
						}
					}
					if hasCapSysAdmin {
						findings = append(findings, runtime.Finding{
							RuleID:    "security-context/allow-priv-esc-cap-sys-admin",
							RuleTitle: "CAP_SYS_ADMIN Requires allowPrivilegeEscalation",
							Category:  "security-context",
							Finding: check.Finding{
								Path:      scPath.String(),
								Message:   fmt.Sprintf("container %q: cannot set allowPrivilegeEscalation to false and capabilities.Add CAP_SYS_ADMIN", ctr.Container.Name),
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
		}},
		{"run-as-non-root-user-zero", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			containers := runtime.AllContainers(info)
			for _, ctr := range containers {
				if ctr.Container.SecurityContext == nil {
					continue
				}
				sc := ctr.Container.SecurityContext
				scPath := ctr.Path.Child("securityContext")
				if sc.RunAsNonRoot != nil && *sc.RunAsNonRoot {
					if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
						findings = append(findings, runtime.Finding{
							RuleID:    "security-context/run-as-non-root-user-zero",
							RuleTitle: "runAsNonRoot Must Not Specify runAsUser 0",
							Category:  "security-context",
							Finding: check.Finding{
								Path:      scPath.String(),
								Message:   fmt.Sprintf("container %q: cannot set runAsNonRoot to true and runAsUser to 0", ctr.Container.Name),
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
		}},
		{"run-as-non-root-user-zero-pod-level", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			podSC := info.PodSecurityContext

			if podSC == nil {
				return findings
			}

			if podSC.RunAsNonRoot != nil && *podSC.RunAsNonRoot {
				if podSC.RunAsUser != nil && *podSC.RunAsUser == 0 {
					containers := runtime.AllContainers(info)
					for _, ctr := range containers {
						ctrSCPath := ctr.Path.Child("securityContext")
						if ctr.Container.SecurityContext != nil && ctr.Container.SecurityContext.RunAsUser != nil {
							continue
						}
						findings = append(findings, runtime.Finding{
							RuleID:    "security-context/run-as-non-root-user-zero-pod-level",
							RuleTitle: "Pod-Level runAsNonRoot Must Not Allow runAsUser 0",
							Category:  "security-context",
							Finding: check.Finding{
								Path:      ctrSCPath.String(),
								Message:   fmt.Sprintf("container %q: pod-level runAsNonRoot=true and runAsUser=0 — container inherits root user", ctr.Container.Name),
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
		}},
		{"invalid-privilege-escalation-field", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			containers := runtime.AllContainers(info)
			for _, ctr := range containers {
				if ctr.Container.SecurityContext == nil {
					continue
				}
				sc := ctr.Container.SecurityContext
				scPath := ctr.Path.Child("securityContext")
				if sc.AllowPrivilegeEscalation == nil {
					findings = append(findings, runtime.Finding{
						RuleID:    "security-context/invalid-privilege-escalation-field",
						RuleTitle: "allowPrivilegeEscalation Must Be Explicitly Set",
						Category:  "security-context",
						Finding: check.Finding{
							Path:      scPath.String(),
							Message:   fmt.Sprintf("container %q: allowPrivilegeEscalation must be explicitly set", ctr.Container.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
							Container: ctr.Container.Name,
						},
					})
				}
			}
			return findings
		}},
		{"capabilities-drop-all-add-sys-admin", func(info *runtime.PodSpecInfo) []runtime.Finding {
			var findings []runtime.Finding
			containers := runtime.AllContainers(info)
			for _, ctr := range containers {
				if ctr.Container.SecurityContext == nil {
					continue
				}
				sc := ctr.Container.SecurityContext
				scPath := ctr.Path.Child("securityContext")
				dropsAll := false
				for _, cap := range sc.Capabilities.Drop {
					if cap == corev1.Capability("ALL") {
						dropsAll = true
						break
					}
				}
				if dropsAll && len(sc.Capabilities.Add) > 0 {
					findings = append(findings, runtime.Finding{
						RuleID:    "security-context/capabilities-drop-all-add-sys-admin",
						RuleTitle: "Cannot Add Capabilities When ALL Are Dropped",
						Category:  "security-context",
						Finding: check.Finding{
							Path:      scPath.String(),
							Message:   fmt.Sprintf("container %q: cannot add capabilities when all capabilities are dropped", ctr.Container.Name),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
							Container: ctr.Container.Name,
						},
					})
				}
			}
			return findings
		}},
	}

	var findings []runtime.Finding
	for _, c := range checks {
		findings = append(findings, c.fn(info)...)
	}

	return findings
}

// Register registers all security context, container, and volume validation checks
// with the check registry.
func Register() {
	checks := []runtime.Check{
		privilegedAllowPrivEscCheck{},
		allowPrivEscCapSysAdminCheck{},
		runAsNonRootUserZeroCheck{},
		runAsNonRootUserZeroPodLevelCheck{},
		invalidPrivilegeEscalationFieldCheck{},
		capabilitiesDropAllAddSysAdminCheck{},
		containerNameCheck{},
		duplicateContainerNamesCheck{},
		portNameFormatCheck{},
		duplicatePortNamesCheck{},
		portNumberRangeCheck{},
		duplicatePortNumbersCheck{},
		portNameUniqueCheck{},
		imagePullPolicyCheck{},
		mountPropagationValueCheck{},
		restartPolicyValueCheck{},
		terminationMessagePathCheck{},
		terminationMessagePolicyValueCheck{},
		envNameDuplicateCheck{},
		envNameFormatCheck{},
		volumeMountNameDuplicateCheck{},
		volumeNameCheck{},
		duplicateVolumeNamesCheck{},
		volumeNameUniqueCheck{},
		emptydirSizeLimitCheck{},
		pvcVolumeCheck{},
		secretVolumeCheck{},
		configmapVolumeCheck{},
		downwardAPIVolumeCheck{},
		projectedVolumeCheck{},
		volumeTypeUndefinedCheck{},
		hostPathVolumeCheck{},
		nfsVolumeCheck{},
		csiVolumeCheck{},
		cinderVolumeCheck{},
		gcePDVolumeCheck{},
		azureDiskVolumeCheck{},
		azureFileVolumeCheck{},
		glusterfsVolumeCheck{},
		iscsiVolumeCheck{},
		rbdVolumeCheck{},
		resourceQuantityFormatCheck{},
		resourceLimitsMissingCheck{},
		resourceRequestsGreaterThanLimitsCheck{},
		hugepagesInRequestsCheck{},
		resourceQuantityZeroCheck{},
		resourceQuantityNegativeCheck{},
		podSpecRestartPolicyValueCheck{},
		podSpecHostNetworkCheck{},
		podSpecHostPIDCheck{},
		podSpecHostIPCCheck{},
		podSpecDNSPolicyValueCheck{},
		podSpecDNSConfigInvalidCheck{},
		podSpecTolerationOperatorValueCheck{},
		podSpecNodeSelectorInvalidCheck{},
		podSpecAffinityInvalidCheck{},
		podSpecAntiAffinityInvalidCheck{},
		podSpecTopologySpreadInvalidCheck{},
		podSpecSchedulerNameInvalidCheck{},
		podSpecServiceAccountNameInvalidCheck{},
		podSpecAutomountSATokenValueCheck{},
		podSpecActiveDeadlineSecondsNegativeCheck{},
		podSpecSubdomainInvalidCheck{},
		podSpecHostnameInvalidCheck{},
		podSpecDomainNameInvalidCheck{},
		podSpecReadinessGateInvalidCheck{},
		podSpecHostPortsOverlapCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
