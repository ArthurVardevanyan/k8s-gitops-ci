package validation

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// Check 1: restart-policy-value
// restartPolicy must be one of: Always, OnFailure, Never.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type podSpecRestartPolicyValueCheck struct{ runtime.Meta }

func newPodSpecRestartPolicyValueCheck() podSpecRestartPolicyValueCheck {
	return podSpecRestartPolicyValueCheck{runtime.Meta{
		RuleID:    "pod-spec/restart-policy-value",
		RuleTitle: "RestartPolicy Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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
		Finding: check.Finding{
			Path:      field.NewPath(info.PodSpecPath).Child("restartPolicy").String(),
			Message:   fmt.Sprintf("restartPolicy: Unsupported value: %q", string(policy)),
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     string(policy),
		},
	}}
}

// Check 5: dns-policy-value
// dnsPolicy must be one of: ClusterFirst, Default, None, ClusterFirstWithHostNet.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type podSpecDNSPolicyValueCheck struct{ runtime.Meta }

func newPodSpecDNSPolicyValueCheck() podSpecDNSPolicyValueCheck {
	return podSpecDNSPolicyValueCheck{runtime.Meta{
		RuleID:    "pod-spec/dns-policy-value",
		RuleTitle: "DNSPolicy Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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
		Finding: check.Finding{
			Path:      field.NewPath(info.PodSpecPath).Child("dnsPolicy").String(),
			Message:   fmt.Sprintf("dnsPolicy: Unsupported value: %q", string(policy)),
			Kind:      info.Kind,
			Name:      info.Name,
			Namespace: info.Namespace,
			Value:     string(policy),
		},
	}}
}

// Check 7: toleration-operator-value
// toleration.operator must be one of: Exists, Equal.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type podSpecTolerationOperatorValueCheck struct{ runtime.Meta }

func newPodSpecTolerationOperatorValueCheck() podSpecTolerationOperatorValueCheck {
	return podSpecTolerationOperatorValueCheck{runtime.Meta{
		RuleID:    "pod-spec/toleration-operator-value",
		RuleTitle: "Toleration Operator Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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
			Finding: check.Finding{
				Path:      field.NewPath(info.PodSpecPath).Child("tolerations").Index(i).Child("operator").String(),
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
type podSpecNodeSelectorInvalidCheck struct{ runtime.Meta }

func newPodSpecNodeSelectorInvalidCheck() podSpecNodeSelectorInvalidCheck {
	return podSpecNodeSelectorInvalidCheck{runtime.Meta{
		RuleID:    "pod-spec/affinity-node-selector-invalid",
		RuleTitle: "NodeSelector Values Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

func (c podSpecNodeSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	var findings []runtime.Finding
	for key, value := range info.PodSpec.NodeSelector {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath(info.PodSpecPath).Child("nodeSelector").Key(key).String(),
					Message: fmt.Sprintf("nodeSelector[%q]: invalid key: %s", key, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: key,
				},
			})
			continue
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath(info.PodSpecPath).Child("nodeSelector").Key(key).String(),
					Message: fmt.Sprintf("nodeSelector[%q]: invalid value: %s", key, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: value,
				},
			})
		}
	}
	return findings
}

// Check 9: pod-affinity-invalid
type podSpecAffinityInvalidCheck struct{ runtime.Meta }

func newPodSpecAffinityInvalidCheck() podSpecAffinityInvalidCheck {
	return podSpecAffinityInvalidCheck{runtime.Meta{
		RuleID:    "pod-spec/pod-affinity-invalid",
		RuleTitle: "Pod Affinity Labels Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

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
							RuleID: c.ID(), RuleTitle: c.Title(),
							Finding: check.Finding{
								Path:    field.NewPath(info.PodSpecPath).Child("affinity").Child("nodeAffinity").Child("requiredDuringSchedulingIgnoredDuringExecution").Child("nodeSelectorTerms").Index(i).Child("matchExpressions").Index(j).Child("key").String(),
								Message: fmt.Sprintf("nodeAffinity matchExpressions[%d]: invalid key: %s", j, strings.Join(errs, ", ")),
								Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: me.Key,
							},
						})
					}
				}
				for j, mf := range term.MatchFields {
					if errs := validation.IsQualifiedName(mf.Key); len(errs) > 0 {
						findings = append(findings, runtime.Finding{
							RuleID: c.ID(), RuleTitle: c.Title(),
							Finding: check.Finding{
								Path:    field.NewPath(info.PodSpecPath).Child("affinity").Child("nodeAffinity").Child("requiredDuringSchedulingIgnoredDuringExecution").Child("nodeSelectorTerms").Index(i).Child("matchFields").Index(j).Child("key").String(),
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
		checkPodAffinityLabelSelector(term.LabelSelector, i, "podAffinity", pathPrefix, info, findings)
	}
}

func checkWeightedPodAffinityTerms(terms []corev1.WeightedPodAffinityTerm, pathPrefix string, info *runtime.PodSpecInfo, findings *[]runtime.Finding) {
	for i, term := range terms {
		checkPodAffinityLabelSelector(term.PodAffinityTerm.LabelSelector, i, "weightedPodAffinity", pathPrefix, info, findings)
	}
}

// checkPodAffinityLabelSelector validates the label selector of the i-th
// (weighted) pod affinity term. msgPrefix distinguishes the two callers in the
// reported message ("podAffinity" vs "weightedPodAffinity").
func checkPodAffinityLabelSelector(selector *metav1.LabelSelector, i int, msgPrefix, pathPrefix string, info *runtime.PodSpecInfo, findings *[]runtime.Finding) {
	if selector == nil {
		return
	}
	psPath := field.NewPath(info.PodSpecPath).Child("affinity").Child("podAffinity").Child(pathPrefix).Index(i).Child("labelSelector")
	for j, expr := range selector.MatchExpressions {
		if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
			*findings = append(*findings, runtime.Finding{
				RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid",
				Finding: check.Finding{
					Path:    psPath.Child("matchExpressions").Index(j).Child("key").String(),
					Message: fmt.Sprintf("%s[%d]: invalid key: %s", msgPrefix, i, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: expr.Key,
				},
			})
		}
	}
	for j, sel := range selector.MatchLabels {
		if errs := validation.IsQualifiedName(j); len(errs) > 0 {
			*findings = append(*findings, runtime.Finding{
				RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid",
				Finding: check.Finding{
					Path:    psPath.Child("matchLabels").Key(j).String(),
					Message: fmt.Sprintf("%s[%d]: invalid label key: %s", msgPrefix, i, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: j,
				},
			})
		}
		if errs := validation.IsValidLabelValue(sel); len(errs) > 0 {
			*findings = append(*findings, runtime.Finding{
				RuleID: "pod-spec/pod-affinity-invalid", RuleTitle: "Pod Affinity Labels Must Be Valid",
				Finding: check.Finding{
					Path:    psPath.Child("matchLabels").Key(j).String(),
					Message: fmt.Sprintf("%s[%d]: invalid label value for %q: %s", msgPrefix, i, j, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sel,
				},
			})
		}
	}
}

// Check 11: topology-spread-invalid
type podSpecTopologySpreadInvalidCheck struct{ runtime.Meta }

func newPodSpecTopologySpreadInvalidCheck() podSpecTopologySpreadInvalidCheck {
	return podSpecTopologySpreadInvalidCheck{runtime.Meta{
		RuleID:    "pod-spec/topology-spread-invalid",
		RuleTitle: "TopologySpreadConstraints Labels Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

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
		psPath := field.NewPath(info.PodSpecPath).Child("topologySpreadConstraints").Index(i).Child("labelSelector")
		for j, expr := range tc.LabelSelector.MatchExpressions {
			if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
				findings = append(findings, runtime.Finding{
					RuleID: c.ID(), RuleTitle: c.Title(),
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
					RuleID: c.ID(), RuleTitle: c.Title(),
					Finding: check.Finding{
						Path:    psPath.Child("matchLabels").Key(j).String(),
						Message: fmt.Sprintf("topologySpreadConstraints[%d]: invalid label key: %s", i, strings.Join(errs, ", ")),
						Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: j,
					},
				})
			}
			if errs := validation.IsValidLabelValue(sel); len(errs) > 0 {
				findings = append(findings, runtime.Finding{
					RuleID: c.ID(), RuleTitle: c.Title(),
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

// Check 13: service-account-name-invalid
type podSpecServiceAccountNameInvalidCheck struct{ runtime.Meta }

func newPodSpecServiceAccountNameInvalidCheck() podSpecServiceAccountNameInvalidCheck {
	return podSpecServiceAccountNameInvalidCheck{runtime.Meta{
		RuleID:    "pod-spec/service-account-name-invalid",
		RuleTitle: "ServiceAccountName Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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
			RuleID: c.ID(), RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    field.NewPath(info.PodSpecPath).Child("serviceAccountName").String(),
				Message: fmt.Sprintf("serviceAccountName: invalid value: %s", strings.Join(errs, ", ")),
				Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: sa,
			},
		}}
	}
	return nil
}

// Check 15: active-deadline-seconds-negative
type podSpecActiveDeadlineSecondsNegativeCheck struct{ runtime.Meta }

func newPodSpecActiveDeadlineSecondsNegativeCheck() podSpecActiveDeadlineSecondsNegativeCheck {
	return podSpecActiveDeadlineSecondsNegativeCheck{runtime.Meta{
		RuleID:    "pod-spec/active-deadline-seconds-negative",
		RuleTitle: "ActiveDeadlineSeconds Must Be >= 1",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
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
		RuleID: c.ID(), RuleTitle: c.Title(),
		Finding: check.Finding{
			Path:    field.NewPath(info.PodSpecPath).Child("activeDeadlineSeconds").String(),
			Message: fmt.Sprintf("activeDeadlineSeconds: must be >= 1, got %d", *ads),
			Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: fmt.Sprintf("%d", *ads),
		},
	}}
}

// Check 19: readiness-gate-invalid
type podSpecReadinessGateInvalidCheck struct{ runtime.Meta }

func newPodSpecReadinessGateInvalidCheck() podSpecReadinessGateInvalidCheck {
	return podSpecReadinessGateInvalidCheck{runtime.Meta{
		RuleID:    "pod-spec/readiness-gate-invalid",
		RuleTitle: "ReadinessGate ConditionType Must Be Valid",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

func (c podSpecReadinessGateInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}
	var findings []runtime.Finding
	for i, gate := range info.PodSpec.ReadinessGates {
		if gate.ConditionType == "" {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath(info.PodSpecPath).Child("readinessGates").Index(i).Child("conditionType").String(),
					Message: fmt.Sprintf("readinessGates[%d]: conditionType must not be empty", i),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace,
				},
			})
			continue
		}
		if errs := validation.IsQualifiedName(string(gate.ConditionType)); len(errs) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID: c.ID(), RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath(info.PodSpecPath).Child("readinessGates").Index(i).Child("conditionType").String(),
					Message: fmt.Sprintf("readinessGates[%d]: conditionType: invalid value: %s", i, strings.Join(errs, ", ")),
					Kind:    info.Kind, Name: info.Name, Namespace: info.Namespace, Value: string(gate.ConditionType),
				},
			})
		}
	}
	return findings
}
