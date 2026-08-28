package validation

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Check 1: restart-policy-value
// restartPolicy must be one of: Always, OnFailure, Never.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:3969-3972
type podSpecRestartPolicyValueCheck struct{}

func (c podSpecRestartPolicyValueCheck) ID() string {
	return "pod-spec/restart-policy-value"
}

func (c podSpecRestartPolicyValueCheck) Title() string {
	return "RestartPolicy Must Be Valid"
}

func (c podSpecRestartPolicyValueCheck) Category() string {
	return "pod-spec"
}

func (c podSpecRestartPolicyValueCheck) Blocking() bool {
	return true
}

func (c podSpecRestartPolicyValueCheck) RenderSensitive() bool {
	return true
}

func (c podSpecRestartPolicyValueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecRestartPolicyValueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validPolicies := map[corev1.RestartPolicy]bool{
		corev1.RestartPolicyAlways:    true,
		corev1.RestartPolicyOnFailure: true,
		corev1.RestartPolicyNever:     true,
	}

	policy := info.PodSpec.RestartPolicy
	if policy == "" || validPolicies[policy] {
		return nil
	}

	return []runtime.Finding{{
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
	}}
}

// Check 2: host-network
// If hostNetwork is true, the pod uses the host network namespace.
// Reports findings when hostNetwork is enabled (blocking check).
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:3980-3990
type podSpecHostNetworkCheck struct{}

func (c podSpecHostNetworkCheck) ID() string {
	return "pod-spec/host-network"
}

func (c podSpecHostNetworkCheck) Title() string {
	return "HostNetwork Must Not Be Enabled"
}

func (c podSpecHostNetworkCheck) Category() string {
	return "pod-spec"
}

func (c podSpecHostNetworkCheck) Blocking() bool {
	return true
}

func (c podSpecHostNetworkCheck) RenderSensitive() bool {
	return true
}

func (c podSpecHostNetworkCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecHostNetworkCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	if !info.PodSpec.HostNetwork {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:      field.NewPath("spec").Child("hostNetwork").String(),
			Message:   "hostNetwork is set to true: pod uses the host network namespace",
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     "true",
		},
	}}
}

// Check 3: host-pid
// hostPID must be a boolean. Reports findings when hostPID is enabled.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:3980-3990
type podSpecHostPIDCheck struct{}

func (c podSpecHostPIDCheck) ID() string {
	return "pod-spec/host-pid"
}

func (c podSpecHostPIDCheck) Title() string {
	return "HostPID Must Not Be Enabled"
}

func (c podSpecHostPIDCheck) Category() string {
	return "pod-spec"
}

func (c podSpecHostPIDCheck) Blocking() bool {
	return true
}

func (c podSpecHostPIDCheck) RenderSensitive() bool {
	return true
}

func (c podSpecHostPIDCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecHostPIDCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	if !info.PodSpec.HostPID {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:      field.NewPath("spec").Child("hostPID").String(),
			Message:   "hostPID is set to true: pod shares the host process namespace",
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     "true",
		},
	}}
}

// Check 4: host-ipc
// hostIPC must be a boolean. Reports findings when hostIPC is enabled.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:3980-3990
type podSpecHostIPCCheck struct{}

func (c podSpecHostIPCCheck) ID() string {
	return "pod-spec/host-ipc"
}

func (c podSpecHostIPCCheck) Title() string {
	return "HostIPC Must Not Be Enabled"
}

func (c podSpecHostIPCCheck) Category() string {
	return "pod-spec"
}

func (c podSpecHostIPCCheck) Blocking() bool {
	return true
}

func (c podSpecHostIPCCheck) RenderSensitive() bool {
	return true
}

func (c podSpecHostIPCCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecHostIPCCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	if !info.PodSpec.HostIPC {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:      field.NewPath("spec").Child("hostIPC").String(),
			Message:   "hostIPC is set to true: pod shares the host IPC namespace",
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     "true",
		},
	}}
}

// Check 5: dns-policy-value
// dnsPolicy must be one of: ClusterFirst, Default, None, ClusterFirstWithHostNet.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:4000-4010
type podSpecDNSPolicyValueCheck struct{}

func (c podSpecDNSPolicyValueCheck) ID() string {
	return "pod-spec/dns-policy-value"
}

func (c podSpecDNSPolicyValueCheck) Title() string {
	return "DNSPolicy Must Be Valid"
}

func (c podSpecDNSPolicyValueCheck) Category() string {
	return "pod-spec"
}

func (c podSpecDNSPolicyValueCheck) Blocking() bool {
	return true
}

func (c podSpecDNSPolicyValueCheck) RenderSensitive() bool {
	return true
}

func (c podSpecDNSPolicyValueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecDNSPolicyValueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validPolicies := map[corev1.DNSPolicy]bool{
		corev1.DNSClusterFirstWithHostNet: true,
		corev1.DNSClusterFirst:            true,
		corev1.DNSDefault:                 true,
		corev1.DNSNone:                    true,
	}

	policy := info.PodSpec.DNSPolicy
	if policy == "" || validPolicies[policy] {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:      field.NewPath("spec").Child("dnsPolicy").String(),
			Message:   fmt.Sprintf("dnsPolicy: Unsupported value: %q", string(policy)),
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     string(policy),
		},
	}}
}

// Check 6: dns-config-invalid
// If dnsPolicy is None, dnsConfig must be specified.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:4000-4010
type podSpecDNSConfigInvalidCheck struct{}

func (c podSpecDNSConfigInvalidCheck) ID() string {
	return "pod-spec/dns-config-invalid"
}

func (c podSpecDNSConfigInvalidCheck) Title() string {
	return "DNSConfig Required When DNSPolicy Is None"
}

func (c podSpecDNSConfigInvalidCheck) Category() string {
	return "pod-spec"
}

func (c podSpecDNSConfigInvalidCheck) Blocking() bool {
	return true
}

func (c podSpecDNSConfigInvalidCheck) RenderSensitive() bool {
	return true
}

func (c podSpecDNSConfigInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecDNSConfigInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	if info.PodSpec.DNSPolicy != corev1.DNSNone {
		return nil
	}

	if info.PodSpec.DNSConfig != nil && len(info.PodSpec.DNSConfig.Nameservers)+
		len(info.PodSpec.DNSConfig.Searches)+
		len(info.PodSpec.DNSConfig.Options) > 0 {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:      field.NewPath("spec").Child("dnsConfig").String(),
			Message:   "dnsConfig is required when dnsPolicy is None",
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
		},
	}}
}

// Check 7: toleration-operator-value
// toleration.operator must be one of: Exists, Equal.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:4500-4510
type podSpecTolerationOperatorValueCheck struct{}

func (c podSpecTolerationOperatorValueCheck) ID() string {
	return "pod-spec/toleration-operator-value"
}

func (c podSpecTolerationOperatorValueCheck) Title() string {
	return "Toleration Operator Must Be Valid"
}

func (c podSpecTolerationOperatorValueCheck) Category() string {
	return "pod-spec"
}

func (c podSpecTolerationOperatorValueCheck) Blocking() bool {
	return true
}

func (c podSpecTolerationOperatorValueCheck) RenderSensitive() bool {
	return true
}

func (c podSpecTolerationOperatorValueCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecTolerationOperatorValueCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	validOperators := map[corev1.TolerationOperator]bool{
		corev1.TolerationOpExists: true,
		corev1.TolerationOpEqual:  true,
	}

	var findings []runtime.Finding
	for i, tol := range info.PodSpec.Tolerations {
		op := tol.Operator
		if op == "" || validOperators[op] {
			continue
		}
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      field.NewPath("spec").Child("tolerations").Index(i).Child("operator").String(),
				Message:   fmt.Sprintf("toleration[%d]: operator: Unsupported value: %q: supported values: 'Exists', 'Equal'", i, string(op)),
				Kind:      info.Kind,
				Name:      info.Name,
				Namespace: info.Namespace,
				Value:     string(op),
			},
		})
	}

	return findings
}

// Check 8: affinity-node-selector-invalid
type podSpecNodeSelectorInvalidCheck struct{}

func (c podSpecNodeSelectorInvalidCheck) ID() string {
	return "pod-spec/affinity-node-selector-invalid"
}

func (c podSpecNodeSelectorInvalidCheck) Title() string         { return "NodeSelector Values Must Be Valid" }
func (c podSpecNodeSelectorInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecNodeSelectorInvalidCheck) Blocking() bool        { return true }
func (c podSpecNodeSelectorInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecNodeSelectorInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecNodeSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	var findings []runtime.Finding
	for key, value := range info.PodSpec.NodeSelector {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("nodeSelector").Key(key).String(),
					Message: fmt.Sprintf("nodeSelector[%q]: invalid key: %s", key, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: key,
				},
			})
			continue
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("nodeSelector").Key(key).String(),
					Message: fmt.Sprintf("nodeSelector[%q]: invalid value: %s", key, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: value,
				},
			})
		}
	}
	return findings
}

// Check 9: pod-affinity-invalid
type podSpecAffinityInvalidCheck struct{}

func (c podSpecAffinityInvalidCheck) ID() string { return "pod-spec/pod-affinity-invalid" }

func (c podSpecAffinityInvalidCheck) Title() string         { return "Pod Affinity Labels Must Be Valid" }
func (c podSpecAffinityInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecAffinityInvalidCheck) Blocking() bool        { return true }
func (c podSpecAffinityInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecAffinityInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecAffinityInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	var findings []runtime.Finding
	if info.PodSpec.Affinity != nil {
		na := info.PodSpec.Affinity.NodeAffinity
		if na != nil && na.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for i, term := range na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				for j, me := range term.MatchExpressions {
					if errs := validation.IsQualifiedName(me.Key); len(errs) > 0 {
						findings = append(findings, runtime.Finding{
							RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
							Finding: check.Finding{
								Path:    field.NewPath("spec").Child("affinity").Child("nodeAffinity").Child("requiredDuringSchedulingIgnoredDuringExecution").Child("nodeSelectorTerms").Index(i).Child("matchExpressions").Index(j).Child("key").String(),
								Message: fmt.Sprintf("nodeAffinity matchExpressions[%d]: invalid key: %s", j, strings.Join(errs, ", ")),
								Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: me.Key,
							},
						})
					}
				}
				for j, mf := range term.MatchFields {
					if errs := validation.IsQualifiedName(mf.Key); len(errs) > 0 {
						findings = append(findings, runtime.Finding{
							RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
							Finding: check.Finding{
								Path:    field.NewPath("spec").Child("affinity").Child("nodeAffinity").Child("requiredDuringSchedulingIgnoredDuringExecution").Child("nodeSelectorTerms").Index(i).Child("matchFields").Index(j).Child("key").String(),
								Message: fmt.Sprintf("nodeAffinity matchFields[%d]: invalid key: %s", j, strings.Join(errs, ", ")),
								Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: mf.Key,
							},
						})
					}
				}
			}
		}
		pa := info.PodSpec.Affinity.PodAffinity
		if pa != nil {
			checkPodAffinityTerms(pa.RequiredDuringSchedulingIgnoredDuringExecution, "requiredDuringSchedulingIgnoredDuringExecution", info, &findings)
			checkWeightedPodAffinityTerms(pa.PreferredDuringSchedulingIgnoredDuringExecution, "preferredDuringSchedulingIgnoredDuringExecution", info, &findings)
		}
		paa := info.PodSpec.Affinity.PodAntiAffinity
		if paa != nil {
			checkPodAffinityTerms(paa.RequiredDuringSchedulingIgnoredDuringExecution, "requiredDuringSchedulingIgnoredDuringExecution", info, &findings)
			checkWeightedPodAffinityTerms(paa.PreferredDuringSchedulingIgnoredDuringExecution, "preferredDuringSchedulingIgnoredDuringExecution", info, &findings)
		}
	}
	return findings
}

func checkPodAffinityTerms(terms []corev1.PodAffinityTerm, pathPrefix string, info *runtime.PodSpecInfo, findings *[]runtime.Finding) {
	for i, term := range terms {
		if term.LabelSelector == nil {
			continue
		}
		psPath := field.NewPath("spec").Child("affinity").Child("podAffinity").Child(pathPrefix).Index(i).Child("labelSelector")
		for j, expr := range term.LabelSelector.MatchExpressions {
			if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
				*findings = append(*findings, runtime.Finding{
					RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid", Category: "pod-spec",
					Finding: check.Finding{
						Path:    psPath.Child("matchExpressions").Index(j).Child("key").String(),
						Message: fmt.Sprintf("podAffinity[%d]: invalid key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: expr.Key,
					},
				})
			}
		}
		for j, sel := range term.LabelSelector.MatchLabels {
			if errs := validation.IsQualifiedName(j); len(errs) > 0 {
				*findings = append(*findings, runtime.Finding{
					RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid", Category: "pod-spec",
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("podAffinity[%d]: invalid label key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: j,
					},
				})
			}
			if errs := validation.IsValidLabelValue(sel); len(errs) > 0 {
				*findings = append(*findings, runtime.Finding{
					RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid", Category: "pod-spec",
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("podAffinity[%d]: invalid label value for %q: %s", i, j, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sel,
					},
				})
			}
		}
	}
}

func checkWeightedPodAffinityTerms(terms []corev1.WeightedPodAffinityTerm, pathPrefix string, info *runtime.PodSpecInfo, findings *[]runtime.Finding) {
	for i, term := range terms {
		if term.PodAffinityTerm.LabelSelector == nil {
			continue
		}
		psPath := field.NewPath("spec").Child("affinity").Child("podAffinity").Child(pathPrefix).Index(i).Child("labelSelector")
		for j, expr := range term.PodAffinityTerm.LabelSelector.MatchExpressions {
			if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
				*findings = append(*findings, runtime.Finding{
					RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid", Category: "pod-spec",
					Finding: check.Finding{
						Path:    psPath.Child("matchExpressions").Index(j).Child("key").String(),
						Message: fmt.Sprintf("weightedPodAffinity[%d]: invalid key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: expr.Key,
					},
				})
			}
		}
		for j, sel := range term.PodAffinityTerm.LabelSelector.MatchLabels {
			if errs := validation.IsQualifiedName(j); len(errs) > 0 {
				*findings = append(*findings, runtime.Finding{
					RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid", Category: "pod-spec",
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("weightedPodAffinity[%d]: invalid label key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: j,
					},
				})
			}
			if errs := validation.IsValidLabelValue(sel); len(errs) > 0 {
				*findings = append(*findings, runtime.Finding{
					RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid", Category: "pod-spec",
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("weightedPodAffinity[%d]: invalid label value for %q: %s", i, j, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sel,
					},
				})
			}
		}
	}
}

// Check 10: pod-anti-affinity-invalid
type podSpecAntiAffinityInvalidCheck struct{}

func (c podSpecAntiAffinityInvalidCheck) ID() string { return "pod-spec/pod-anti-affinity-invalid" }
func (c podSpecAntiAffinityInvalidCheck) Title() string {
	return "Pod Anti-Affinity Labels Must Be Valid"
}
func (c podSpecAntiAffinityInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecAntiAffinityInvalidCheck) Blocking() bool        { return true }
func (c podSpecAntiAffinityInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecAntiAffinityInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecAntiAffinityInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return podSpecAffinityInvalidCheck{}.Run(data, source)
}

// Check 11: topology-spread-invalid
type podSpecTopologySpreadInvalidCheck struct{}

func (c podSpecTopologySpreadInvalidCheck) ID() string { return "pod-spec/topology-spread-invalid" }
func (c podSpecTopologySpreadInvalidCheck) Title() string {
	return "TopologySpreadConstraints Labels Must Be Valid"
}
func (c podSpecTopologySpreadInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecTopologySpreadInvalidCheck) Blocking() bool        { return true }
func (c podSpecTopologySpreadInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecTopologySpreadInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecTopologySpreadInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	var findings []runtime.Finding
	for i, tc := range info.PodSpec.TopologySpreadConstraints {
		if tc.LabelSelector == nil {
			continue
		}
		psPath := field.NewPath("spec").Child("topologySpreadConstraints").Index(i).Child("labelSelector")
		for j, expr := range tc.LabelSelector.MatchExpressions {
			if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
				findings = append(findings, runtime.Finding{
					RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
					Finding: check.Finding{
						Path:    psPath.Child("matchExpressions").Index(j).Child("key").String(),
						Message: fmt.Sprintf("topologySpreadConstraints[%d]: invalid key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: expr.Key,
					},
				})
			}
		}
		for j, sel := range tc.LabelSelector.MatchLabels {
			if errs := validation.IsQualifiedName(j); len(errs) > 0 {
				findings = append(findings, runtime.Finding{
					RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("topologySpreadConstraints[%d]: invalid label key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: j,
					},
				})
			}
			if errs := validation.IsValidLabelValue(sel); len(errs) > 0 {
				findings = append(findings, runtime.Finding{
					RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("topologySpreadConstraints[%d]: invalid label value for %q: %s", i, j, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sel,
					},
				})
			}
		}
	}
	return findings
}

// Check 12: scheduler-name-invalid
type podSpecSchedulerNameInvalidCheck struct{}

func (c podSpecSchedulerNameInvalidCheck) ID() string { return "pod-spec/scheduler-name-invalid" }

func (c podSpecSchedulerNameInvalidCheck) Title() string         { return "SchedulerName Must Be Valid" }
func (c podSpecSchedulerNameInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecSchedulerNameInvalidCheck) Blocking() bool        { return true }
func (c podSpecSchedulerNameInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecSchedulerNameInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecSchedulerNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	sn := info.PodSpec.SchedulerName
	if sn == "" || sn == "default-scheduler" {
		return nil
	}
	if errs := validation.IsDNS1123Subdomain(sn); len(errs) > 0 {
		return []runtime.Finding{{
			RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("schedulerName").String(),
				Message: fmt.Sprintf("schedulerName: invalid value: %s", strings.Join(errs, ", ")),
				Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sn,
			},
		}}
	}
	return nil
}

// Check 13: service-account-name-invalid
type podSpecServiceAccountNameInvalidCheck struct{}

func (c podSpecServiceAccountNameInvalidCheck) ID() string {
	return "pod-spec/service-account-name-invalid"
}

func (c podSpecServiceAccountNameInvalidCheck) Title() string {
	return "ServiceAccountName Must Be Valid"
}
func (c podSpecServiceAccountNameInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecServiceAccountNameInvalidCheck) Blocking() bool        { return true }
func (c podSpecServiceAccountNameInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecServiceAccountNameInvalidCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecServiceAccountNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	sa := info.PodSpec.ServiceAccountName
	if sa == "" {
		return nil
	}
	if errs := validation.IsDNS1123Subdomain(sa); len(errs) > 0 {
		return []runtime.Finding{{
			RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("serviceAccountName").String(),
				Message: fmt.Sprintf("serviceAccountName: invalid value: %s", strings.Join(errs, ", ")),
				Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sa,
			},
		}}
	}
	return nil
}

// Check 14: automount-sa-token-value
type podSpecAutomountSATokenValueCheck struct{}

func (c podSpecAutomountSATokenValueCheck) ID() string { return "pod-spec/automount-sa-token-value" }

func (c podSpecAutomountSATokenValueCheck) Title() string {
	return "AutomountServiceAccountToken Must Be Boolean"
}
func (c podSpecAutomountSATokenValueCheck) Category() string      { return "pod-spec" }
func (c podSpecAutomountSATokenValueCheck) Blocking() bool        { return true }
func (c podSpecAutomountSATokenValueCheck) RenderSensitive() bool { return true }
func (c podSpecAutomountSATokenValueCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecAutomountSATokenValueCheck) Run(data []byte, source string) []runtime.Finding {
	return nil
}

// Check 15: active-deadline-seconds-negative
type podSpecActiveDeadlineSecondsNegativeCheck struct{}

func (c podSpecActiveDeadlineSecondsNegativeCheck) ID() string {
	return "pod-spec/active-deadline-seconds-negative"
}

func (c podSpecActiveDeadlineSecondsNegativeCheck) Title() string {
	return "ActiveDeadlineSeconds Must Be >= 1"
}
func (c podSpecActiveDeadlineSecondsNegativeCheck) Category() string      { return "pod-spec" }
func (c podSpecActiveDeadlineSecondsNegativeCheck) Blocking() bool        { return true }
func (c podSpecActiveDeadlineSecondsNegativeCheck) RenderSensitive() bool { return true }
func (c podSpecActiveDeadlineSecondsNegativeCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c podSpecActiveDeadlineSecondsNegativeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	ads := info.PodSpec.ActiveDeadlineSeconds
	if ads == nil || *ads >= 1 {
		return nil
	}
	return []runtime.Finding{{
		RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("activeDeadlineSeconds").String(),
			Message: fmt.Sprintf("activeDeadlineSeconds: must be >= 1, got %d", *ads),
			Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: fmt.Sprintf("%d", *ads),
		},
	}}
}

// Check 16: subdomain-invalid
type podSpecSubdomainInvalidCheck struct{}

func (c podSpecSubdomainInvalidCheck) ID() string            { return "pod-spec/subdomain-invalid" }
func (c podSpecSubdomainInvalidCheck) Title() string         { return "Subdomain Must Be Valid" }
func (c podSpecSubdomainInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecSubdomainInvalidCheck) Blocking() bool        { return true }
func (c podSpecSubdomainInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecSubdomainInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecSubdomainInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	sd := info.PodSpec.Subdomain
	if sd == "" {
		return nil
	}
	if errs := validation.IsDNS1123Subdomain(sd); len(errs) > 0 {
		return []runtime.Finding{{
			RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("subdomain").String(),
				Message: fmt.Sprintf("subdomain: invalid value: %s", strings.Join(errs, ", ")),
				Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sd,
			},
		}}
	}
	return nil
}

// Check 17: set-hostname-invalid
type podSpecHostnameInvalidCheck struct{}

func (c podSpecHostnameInvalidCheck) ID() string            { return "pod-spec/set-hostname-invalid" }
func (c podSpecHostnameInvalidCheck) Title() string         { return "Hostname Must Be Valid" }
func (c podSpecHostnameInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecHostnameInvalidCheck) Blocking() bool        { return true }
func (c podSpecHostnameInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecHostnameInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecHostnameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	hn := info.PodSpec.Hostname
	if hn == "" {
		return nil
	}
	if errs := validation.IsDNS1123Subdomain(hn); len(errs) > 0 {
		return []runtime.Finding{{
			RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("hostname").String(),
				Message: fmt.Sprintf("hostname: invalid value: %s", strings.Join(errs, ", ")),
				Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: hn,
			},
		}}
	}
	return nil
}

// Check 18: set-domain-name-invalid
type podSpecDomainNameInvalidCheck struct{}

func (c podSpecDomainNameInvalidCheck) ID() string            { return "pod-spec/set-domain-name-invalid" }
func (c podSpecDomainNameInvalidCheck) Title() string         { return "DomainName Must Be Valid" }
func (c podSpecDomainNameInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecDomainNameInvalidCheck) Blocking() bool        { return true }
func (c podSpecDomainNameInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecDomainNameInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecDomainNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nil
}

// Check 19: readiness-gate-invalid
type podSpecReadinessGateInvalidCheck struct{}

func (c podSpecReadinessGateInvalidCheck) ID() string { return "pod-spec/readiness-gate-invalid" }
func (c podSpecReadinessGateInvalidCheck) Title() string {
	return "ReadinessGate ConditionType Must Be Valid"
}
func (c podSpecReadinessGateInvalidCheck) Category() string      { return "pod-spec" }
func (c podSpecReadinessGateInvalidCheck) Blocking() bool        { return true }
func (c podSpecReadinessGateInvalidCheck) RenderSensitive() bool { return true }
func (c podSpecReadinessGateInvalidCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecReadinessGateInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	var findings []runtime.Finding
	for i, gate := range info.PodSpec.ReadinessGates {
		if gate.ConditionType == "" {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("readinessGates").Index(i).Child("conditionType").String(),
					Message: fmt.Sprintf("readinessGates[%d]: conditionType must not be empty", i),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace,
				},
			})
			continue
		}
		if errs := validation.IsQualifiedName(string(gate.ConditionType)); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("readinessGates").Index(i).Child("conditionType").String(),
					Message: fmt.Sprintf("readinessGates[%d]: conditionType: invalid value: %s", i, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: string(gate.ConditionType),
				},
			})
		}
	}
	return findings
}

// Check 20: host-ports-overlap
type podSpecHostPortsOverlapCheck struct{}

func (c podSpecHostPortsOverlapCheck) ID() string            { return "pod-spec/host-ports-overlap" }
func (c podSpecHostPortsOverlapCheck) Title() string         { return "HostPorts Must Not Overlap" }
func (c podSpecHostPortsOverlapCheck) Category() string      { return "pod-spec" }
func (c podSpecHostPortsOverlapCheck) Blocking() bool        { return true }
func (c podSpecHostPortsOverlapCheck) RenderSensitive() bool { return true }
func (c podSpecHostPortsOverlapCheck) DocSkipper() []string  { return runtime.HasPodSpecKinds() }

func (c podSpecHostPortsOverlapCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	if !info.PodSpec.HostNetwork {
		return nil
	}
	type portKey struct {
		hostPort int32
		protocol corev1.Protocol
	}
	usedPorts := make(map[portKey]string)
	var findings []runtime.Finding
	allContainers := runtime.AllContainers(info)
	for _, ctr := range allContainers {
		for _, port := range ctr.Container.Ports {
			if port.HostPort == 0 {
				continue
			}
			key := portKey{hostPort: port.HostPort, protocol: port.Protocol}
			if existing, exists := usedPorts[key]; exists {
				findings = append(findings, runtime.Finding{
					RuleID: c.ID(), RuleTitle: c.Title(), Category: c.Category(),
					Finding: check.Finding{
						Path:    ctr.Path.Child("ports").String(),
						Message: fmt.Sprintf("container %q: host port %d/%s overlaps with %q", ctr.Container.Name, port.HostPort, port.Protocol, existing),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Container: ctr.Container.Name,
					},
				})
			}
			usedPorts[key] = ctr.Container.Name
		}
	}
	return findings
}
