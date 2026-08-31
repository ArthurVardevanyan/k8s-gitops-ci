package core

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// podDoc builds a Pod carrying the given pod-spec body above its
// containers. Every pod-spec rule is exercised against the same minimal
// Pod, so the frame is built once rather than retyped per case.
func podDoc(spec string) []byte {
	return []byte("kind: Pod\nmetadata:\n  name: test\nspec:\n" + spec +
		"  containers:\n  - name: c\n    image: nginx\n")
}

// podSpecCase is one pod-spec body and the findings it must produce.
type podSpecCase struct {
	name     string
	spec     string
	want     int
	contains string
}

func runPodSpecCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []podSpecCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := run(podDoc(tc.spec), "test.yaml")
			if len(findings) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
			}
			if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
				t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
			}
		})
	}
}

func TestPodSpecTolerationOperatorValue(t *testing.T) {
	runPodSpecCases(t, newPodSpecTolerationOperatorValueCheck().Run, []podSpecCase{
		// Lt and Gt are gated upstream and only rejected when the gate is
		// off. The gate can only widen what the cluster accepts and this
		// tool cannot see it, so the permissive branch is the one ported -
		// blocking a valid manifest here is not suppressible.
		{name: "Lt is accepted under a feature gate", spec: "  tolerations:\n  - key: k\n    operator: Lt\n    value: \"5\"\n", want: 0},
		{name: "Gt is accepted under a feature gate", spec: "  tolerations:\n  - key: k\n    operator: Gt\n    value: \"5\"\n", want: 0},
		{name: "Exists", spec: "  tolerations:\n  - key: node.kubernetes.io/not-ready\n    operator: Exists\n    effect: NoExecute\n", want: 0},
		{name: "Equal", spec: "  tolerations:\n  - key: node.kubernetes.io/memory-pressure\n    operator: Equal\n    value: \"true\"\n    effect: NoSchedule\n", want: 0},
		{name: "EmptyOperator", spec: "  tolerations:\n  - key: node.kubernetes.io/not-ready\n    effect: NoExecute\n", want: 0},
		{name: "InvalidOperator", spec: "  tolerations:\n  - key: node.kubernetes.io/disk-pressure\n    operator: InvalidOperator\n    effect: NoSchedule\n", want: 1},
		{name: "MultipleTolerations", spec: "  tolerations:\n  - key: key1\n    operator: Invalid1\n  - key: key2\n    operator: Exists\n  - key: key3\n    operator: Invalid2\n", want: 2},
	})
}

func TestPodSpecNodeSelectorInvalid(t *testing.T) {
	runPodSpecCases(t, newPodSpecNodeSelectorInvalidCheck().Run, []podSpecCase{
		{name: "Valid", spec: "  nodeSelector:\n    kubernetes.io/os: linux\n    disktype: ssd\n", want: 0},
		{name: "InvalidKey", spec: "  nodeSelector:\n    invalid key: linux\n", want: 1},
		{name: "InvalidValue", spec: "  nodeSelector:\n    kubernetes.io/os: INVALID/VALUE\n", want: 1},
		{name: "EmptyNodeSelector", spec: "", want: 0},
		{name: "AzurePrefix", spec: "  nodeSelector:\n    azure.com/region: westus\n", want: 0},
	})
}

func TestPodSpecAffinityInvalid(t *testing.T) {
	runPodSpecCases(t, newPodSpecAffinityInvalidCheck().Run, []podSpecCase{
		{name: "ValidAffinity", spec: "  affinity:\n    nodeAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n        nodeSelectorTerms:\n        - matchExpressions:\n          - key: kubernetes.io/os\n            operator: In\n            values:\n            - linux\n    podAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n      - labelSelector:\n          matchExpressions:\n          - key: app\n            operator: In\n            values:\n            - web\n        topologyKey: kubernetes.io/hostname\n", want: 0},
		{name: "InvalidNodeAffinityKey", spec: "  affinity:\n    nodeAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n        nodeSelectorTerms:\n        - matchExpressions:\n          - key: invalid key!\n            operator: In\n            values:\n            - linux\n", want: 1},
		{name: "InvalidPodAffinityLabelSelector", spec: "  affinity:\n    podAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n      - labelSelector:\n          matchLabels:\n            invalid key!: web\n        topologyKey: kubernetes.io/hostname\n", want: 1},
		{name: "InvalidPodAffinityLabelValue", spec: "  affinity:\n    podAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n      - labelSelector:\n          matchLabels:\n            app: INVALID/VALUE\n        topologyKey: kubernetes.io/hostname\n", want: 1},
		{name: "InvalidMatchExpressionsKey", spec: "  affinity:\n    podAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n      - labelSelector:\n          matchExpressions:\n          - key: invalid expression!key\n            operator: In\n            values:\n            - web\n        topologyKey: kubernetes.io/hostname\n", want: 1},
		{name: "WeightedAffinity", spec: "  affinity:\n    podAffinity:\n      preferredDuringSchedulingIgnoredDuringExecution:\n      - weight: 100\n        podAffinityTerm:\n          labelSelector:\n            matchLabels:\n              invalid key!: web\n          topologyKey: kubernetes.io/hostname\n", want: 1},
		{name: "InvalidNodeMatchFieldsKey", spec: "  affinity:\n    nodeAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n        nodeSelectorTerms:\n        - matchFields:\n          - key: invalid fields!key\n            operator: Exists\n", want: 1},
		{name: "NoAffinity", spec: "", want: 0},
		{name: "NilSelector", spec: "  affinity:\n    podAffinity:\n      requiredDuringSchedulingIgnoredDuringExecution:\n      - topologyKey: kubernetes.io/hostname\n", want: 0},
		{name: "Service", spec: "kind: Service\nmetadata:\n  name: test\nspec:\n  ports:\n  - port: 80\n", want: 0},
	})
}

func TestPodSpecTopologySpreadInvalid(t *testing.T) {
	runPodSpecCases(t, newPodSpecTopologySpreadInvalidCheck().Run, []podSpecCase{
		{name: "Valid", spec: "  topologySpreadConstraints:\n  - maxSkew: 1\n    topologyKey: kubernetes.io/hostname\n    whenUnsatisfiable: DoNotSchedule\n    labelSelector:\n      matchLabels:\n        app: web\n", want: 0},
		{name: "InvalidLabelKey", spec: "  topologySpreadConstraints:\n  - maxSkew: 1\n    topologyKey: kubernetes.io/hostname\n    whenUnsatisfiable: DoNotSchedule\n    labelSelector:\n      matchLabels:\n        invalid key!: web\n", want: 1},
		{name: "InvalidLabelValue", spec: "  topologySpreadConstraints:\n  - maxSkew: 1\n    topologyKey: kubernetes.io/hostname\n    whenUnsatisfiable: DoNotSchedule\n    labelSelector:\n      matchLabels:\n        app: INVALID/VALUE\n", want: 1},
		{name: "InvalidMatchExpressionsKey", spec: "  topologySpreadConstraints:\n  - maxSkew: 1\n    topologyKey: kubernetes.io/hostname\n    whenUnsatisfiable: DoNotSchedule\n    labelSelector:\n      matchExpressions:\n      - key: invalid expr!key\n        operator: In\n        values:\n        - web\n", want: 1},
		{name: "NilSelector", spec: "  topologySpreadConstraints:\n  - maxSkew: 1\n    topologyKey: kubernetes.io/hostname\n", want: 0},
		{name: "MultipleConstraints", spec: "  topologySpreadConstraints:\n  - maxSkew: 1\n    topologyKey: kubernetes.io/hostname\n    whenUnsatisfiable: DoNotSchedule\n    labelSelector:\n      matchLabels:\n        app: valid\n  - maxSkew: 2\n    topologyKey: topology.kubernetes.io/zone\n    whenUnsatisfiable: ScheduleAnyway\n    labelSelector:\n      matchLabels:\n        invalid key!: bad\n", want: 1},
	})
}

func TestPodSpecServiceAccountNameInvalid(t *testing.T) {
	runPodSpecCases(t, newPodSpecServiceAccountNameInvalidCheck().Run, []podSpecCase{
		{name: "Valid", spec: "  serviceAccountName: my-sa\n", want: 0},
		{name: "Empty", spec: "", want: 0},
		{name: "Invalid", spec: "  serviceAccountName: invalid SA!\n", want: 1},
	})
}

func TestPodSpecActiveDeadlineSecondsNegative(t *testing.T) {
	runPodSpecCases(t, newPodSpecActiveDeadlineSecondsNegativeCheck().Run, []podSpecCase{
		// The field is an *int64 and upstream bounds it at MaxInt32, so the
		// upper bound is reachable from a manifest.
		{name: "above MaxInt32", spec: "  activeDeadlineSeconds: 2147483648\n", want: 1, contains: "between 1 and 2147483647"},
		{name: "exactly MaxInt32", spec: "  activeDeadlineSeconds: 2147483647\n", want: 0},
		{name: "Valid", spec: "  activeDeadlineSeconds: 60\n", want: 0},
		{name: "Zero", spec: "  activeDeadlineSeconds: 0\n", want: 1},
		{name: "Negative", spec: "  activeDeadlineSeconds: -1\n", want: 1},
		{name: "Empty", spec: "", want: 0},
		{name: "LargeValue", spec: "  activeDeadlineSeconds: 86400\n", want: 0},
	})
}

func TestPodSpecReadinessGateInvalid(t *testing.T) {
	runPodSpecCases(t, newPodSpecReadinessGateInvalidCheck().Run, []podSpecCase{
		{name: "Valid", spec: "  readinessGates:\n  - conditionType: my-condition\n", want: 0},
		{name: "EmptyConditionType", spec: "  readinessGates:\n  - conditionType: \"\"\n", want: 1},
		{name: "InvalidConditionType", spec: "  readinessGates:\n  - conditionType: invalid condition!type\n", want: 1},
		{name: "MultipleGates", spec: "  readinessGates:\n  - conditionType: valid-condition\n  - conditionType: invalid!gate\n  - conditionType: another-valid\n", want: 1},
		{name: "EmptyReadinessGates", spec: "  readinessGates: []\n", want: 0},
		{name: "NoReadinessGates", spec: "", want: 0},
	})
}

// podSpecEnumDoc builds a workload document carrying a single pod-spec
// field, at the nesting depth the given kind uses. The enum checks all
// exercise the same shape, so the fixture is built rather than repeated.
func podSpecEnumDoc(kind, field, value string) []byte {
	setting := ""
	if value != "" {
		setting = field + ": " + value + "\n"
	}
	switch kind {
	case "Pod":
		return []byte("kind: Pod\nmetadata:\n  name: test\nspec:\n  " + setting +
			"  containers:\n  - name: c\n    image: nginx\n")
	case "CronJob":
		return []byte("kind: CronJob\nmetadata:\n  name: test\nspec:\n  jobTemplate:\n    spec:\n" +
			"      template:\n        spec:\n          " + setting +
			"          containers:\n          - name: c\n            image: nginx\n")
	default:
		return []byte("kind: " + kind + "\nmetadata:\n  name: test\nspec:\n  template:\n    spec:\n      " +
			setting + "      containers:\n      - name: c\n        image: nginx\n")
	}
}

// The restartPolicy and dnsPolicy rules are the same shape: an enum with a
// NotSupported branch, where an empty value is deliberately skipped because
// defaulting fills it before the API server validates.
func TestPodSpecEnumFieldValues(t *testing.T) {
	tests := []struct {
		name    string
		check   runtime.Check
		ruleID  string
		kind    string
		field   string
		value   string
		wantNum int
	}{
		{"restartPolicy Always", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "Pod", "restartPolicy", "Always", 0},
		{"restartPolicy OnFailure", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "Pod", "restartPolicy", "OnFailure", 0},
		{"restartPolicy Never", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "Pod", "restartPolicy", "Never", 0},
		{"restartPolicy absent", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "Pod", "restartPolicy", "", 0},
		{"restartPolicy invalid", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "Pod", "restartPolicy", "InvalidPolicy", 1},
		{"restartPolicy invalid in Deployment", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "Deployment", "restartPolicy", "InvalidPolicy", 1},
		{"restartPolicy invalid in CronJob", newPodSpecRestartPolicyValueCheck(), "pod-spec/restart-policy-value", "CronJob", "restartPolicy", "InvalidPolicy", 1},

		{"dnsPolicy ClusterFirst", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Pod", "dnsPolicy", "ClusterFirst", 0},
		{"dnsPolicy None", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Pod", "dnsPolicy", "None", 0},
		{"dnsPolicy Default", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Pod", "dnsPolicy", "Default", 0},
		{"dnsPolicy ClusterFirstWithHostNet", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Pod", "dnsPolicy", "ClusterFirstWithHostNet", 0},
		{"dnsPolicy absent", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Pod", "dnsPolicy", "", 0},
		{"dnsPolicy invalid", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Pod", "dnsPolicy", "InvalidPolicy", 1},
		{"dnsPolicy invalid in Deployment", newPodSpecDNSPolicyValueCheck(), "pod-spec/dns-policy-value", "Deployment", "dnsPolicy", "InvalidPolicy", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.check.Run(podSpecEnumDoc(tt.kind, tt.field, tt.value), "test.yaml")
			if len(got) != tt.wantNum {
				t.Fatalf("expected %d finding(s), got %d: %v", tt.wantNum, len(got), got)
			}
			if tt.wantNum == 0 {
				return
			}
			if got[0].RuleID != tt.ruleID {
				t.Errorf("unexpected rule ID: %s", got[0].RuleID)
			}
			if got[0].Kind != tt.kind || got[0].Name != "test" {
				t.Errorf("unexpected kind/name: %s/%s", got[0].Kind, got[0].Name)
			}
		})
	}
}
