package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestPodSpecRestartPolicyValue_Check_Always(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  restartPolicy: Always
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Always restart policy, got %d", len(findings))
	}
}

func TestPodSpecRestartPolicyValue_Check_OnFailure(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  restartPolicy: OnFailure
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for OnFailure restart policy, got %d", len(findings))
	}
}

func TestPodSpecRestartPolicyValue_Check_Never(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  restartPolicy: Never
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Never restart policy, got %d", len(findings))
	}
}

func TestPodSpecRestartPolicyValue_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty restartPolicy, got %d", len(findings))
	}
}

func TestPodSpecRestartPolicyValue_Check_InvalidValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  restartPolicy: InvalidPolicy
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid restartPolicy, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/restart-policy-value" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Kind != "Pod" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
}

func TestPodSpecRestartPolicyValue_Check_Deployment(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      restartPolicy: OnFailure
      containers:
      - name: c
        image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Deployment with valid restartPolicy, got %d", len(findings))
	}
}

func TestPodSpecDNSPolicyValue_Check_ClusterFirst(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: ClusterFirst
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for ClusterFirst dnsPolicy, got %d", len(findings))
	}
}

func TestPodSpecDNSPolicyValue_Check_None(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: None
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for None dnsPolicy, got %d", len(findings))
	}
}

func TestPodSpecDNSPolicyValue_Check_Default(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: Default
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Default dnsPolicy, got %d", len(findings))
	}
}

func TestPodSpecDNSPolicyValue_Check_ClusterFirstWithHostNet(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: ClusterFirstWithHostNet
  hostNetwork: true
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for ClusterFirstWithHostNet, got %d", len(findings))
	}
}

func TestPodSpecDNSPolicyValue_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty dnsPolicy, got %d", len(findings))
	}
}

func TestPodSpecDNSPolicyValue_Check_InvalidValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: InvalidPolicy
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid dnsPolicy, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/dns-policy-value" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "InvalidPolicy" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestPodSpecTolerationOperatorValue_Check_Exists(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
  tolerations:
  - key: node.kubernetes.io/not-ready
    operator: Exists
    effect: NoExecute
`)
	check := newPodSpecTolerationOperatorValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Exists operator, got %d", len(findings))
	}
}

func TestPodSpecTolerationOperatorValue_Check_Equal(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
  tolerations:
  - key: node.kubernetes.io/memory-pressure
    operator: Equal
    value: "true"
    effect: NoSchedule
`)
	check := newPodSpecTolerationOperatorValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Equal operator, got %d", len(findings))
	}
}

func TestPodSpecTolerationOperatorValue_Check_EmptyOperator(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
  tolerations:
  - key: node.kubernetes.io/not-ready
    effect: NoExecute
`)
	check := newPodSpecTolerationOperatorValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty operator, got %d", len(findings))
	}
}

func TestPodSpecTolerationOperatorValue_Check_InvalidOperator(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
  tolerations:
  - key: node.kubernetes.io/disk-pressure
    operator: InvalidOperator
    effect: NoSchedule
`)
	check := newPodSpecTolerationOperatorValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid operator, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/toleration-operator-value" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "InvalidOperator" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestPodSpecTolerationOperatorValue_Check_MultipleTolerations(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
  tolerations:
  - key: key1
    operator: Invalid1
  - key: key2
    operator: Exists
  - key: key3
    operator: Invalid2
`)
	check := newPodSpecTolerationOperatorValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for two invalid operators, got %d", len(findings))
	}
}

func TestPodSpecNodeSelectorInvalid_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  nodeSelector:
    kubernetes.io/os: linux
    disktype: ssd
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecNodeSelectorInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid nodeSelector, got %d", len(findings))
	}
}

func TestPodSpecNodeSelectorInvalid_Check_InvalidKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  nodeSelector:
    invalid key: linux
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecNodeSelectorInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid nodeSelector key, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/affinity-node-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "invalid key" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestPodSpecNodeSelectorInvalid_Check_InvalidValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  nodeSelector:
    kubernetes.io/os: INVALID/VALUE
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecNodeSelectorInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid nodeSelector value, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/affinity-node-selector-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecNodeSelectorInvalid_Check_EmptyNodeSelector(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecNodeSelectorInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty nodeSelector, got %d", len(findings))
	}
}

func TestPodSpecNodeSelectorInvalid_Check_AzurePrefix(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  nodeSelector:
    azure.com/region: westus
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecNodeSelectorInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for azure.com/region key, got %d", len(findings))
	}
}

func TestPodSpecAffinityInvalid_Check_ValidAffinity(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/os
            operator: In
            values:
            - linux
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app
            operator: In
            values:
            - web
        topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid affinity, got %d: %v", len(findings), findings)
	}
}

func TestPodSpecAffinityInvalid_Check_InvalidNodeAffinityKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: invalid key!
            operator: In
            values:
            - linux
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid nodeAffinity key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAffinityInvalid_Check_InvalidPodAffinityLabelSelector(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            invalid key!: web
        topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid podAffinity label key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAffinityInvalid_Check_InvalidPodAffinityLabelValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: INVALID/VALUE
        topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid podAffinity label value, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAffinityInvalid_Check_InvalidMatchExpressionsKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: invalid expression!key
            operator: In
            values:
            - web
        topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid matchExpressions key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAffinityInvalid_Check_WeightedAffinity(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              invalid key!: web
          topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid weighted podAffinity, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAffinityInvalid_Check_InvalidNodeMatchFieldsKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchFields:
          - key: invalid fields!key
            operator: Exists
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid node matchFields key, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAffinityInvalid_Check_NoAffinity(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for no affinity, got %d", len(findings))
	}
}

func TestPodSpecAffinityInvalid_Check_NilSelector(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil labelSelector, got %d", len(findings))
	}
}

func TestPodSpecTopologySpreadInvalid_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        app: web
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecTopologySpreadInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid topologySpreadConstraints, got %d", len(findings))
	}
}

func TestPodSpecTopologySpreadInvalid_Check_InvalidLabelKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        invalid key!: web
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecTopologySpreadInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid topologySpread label key, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/topology-spread-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecTopologySpreadInvalid_Check_InvalidLabelValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        app: INVALID/VALUE
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecTopologySpreadInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid topologySpread label value, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/topology-spread-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecTopologySpreadInvalid_Check_InvalidMatchExpressionsKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchExpressions:
      - key: invalid expr!key
        operator: In
        values:
        - web
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecTopologySpreadInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid topologySpread matchExpressions key, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/topology-spread-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecTopologySpreadInvalid_Check_NilSelector(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecTopologySpreadInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil labelSelector, got %d", len(findings))
	}
}

func TestPodSpecTopologySpreadInvalid_Check_MultipleConstraints(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        app: valid
  - maxSkew: 2
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: ScheduleAnyway
    labelSelector:
      matchLabels:
        invalid key!: bad
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecTopologySpreadInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for second constraint with invalid key, got %d", len(findings))
	}
}

func TestPodSpecServiceAccountName_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  serviceAccountName: my-sa
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecServiceAccountNameInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid serviceAccountName, got %d", len(findings))
	}
}

func TestPodSpecServiceAccountName_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecServiceAccountNameInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty serviceAccountName, got %d", len(findings))
	}
}

func TestPodSpecServiceAccountName_Check_Invalid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  serviceAccountName: invalid SA!
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecServiceAccountNameInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid serviceAccountName, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/service-account-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecActiveDeadlineSeconds_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  activeDeadlineSeconds: 60
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecActiveDeadlineSecondsNegativeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid activeDeadlineSeconds, got %d", len(findings))
	}
}

func TestPodSpecActiveDeadlineSeconds_Check_Zero(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  activeDeadlineSeconds: 0
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecActiveDeadlineSecondsNegativeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for activeDeadlineSeconds=0, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/active-deadline-seconds-negative" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecActiveDeadlineSeconds_Check_Negative(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  activeDeadlineSeconds: -1
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecActiveDeadlineSecondsNegativeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for activeDeadlineSeconds=-1, got %d", len(findings))
	}
	if findings[0].Value != "-1" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestPodSpecActiveDeadlineSeconds_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecActiveDeadlineSecondsNegativeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty activeDeadlineSeconds, got %d", len(findings))
	}
}

func TestPodSpecActiveDeadlineSeconds_Check_LargeValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  activeDeadlineSeconds: 86400
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecActiveDeadlineSecondsNegativeCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for large activeDeadlineSeconds, got %d", len(findings))
	}
}

func TestPodSpecReadinessGate_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  readinessGates:
  - conditionType: my-condition
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecReadinessGateInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid readinessGate, got %d", len(findings))
	}
}

func TestPodSpecReadinessGate_Check_EmptyConditionType(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  readinessGates:
  - conditionType: ""
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecReadinessGateInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty conditionType, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/readiness-gate-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Message != "readinessGates[0]: conditionType must not be empty" {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestPodSpecReadinessGate_Check_InvalidConditionType(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  readinessGates:
  - conditionType: invalid condition!type
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecReadinessGateInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid conditionType, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/readiness-gate-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecReadinessGate_Check_MultipleGates(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  readinessGates:
  - conditionType: valid-condition
  - conditionType: invalid!gate
  - conditionType: another-valid
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecReadinessGateInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid second gate, got %d", len(findings))
	}
}

func TestPodSpecReadinessGate_Check_EmptyReadinessGates(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  readinessGates: []
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecReadinessGateInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty readinessGates, got %d", len(findings))
	}
}

func TestPodSpecReadinessGate_Check_NoReadinessGates(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := newPodSpecReadinessGateInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for no readinessGates, got %d", len(findings))
	}
}

func TestPodSpecRestartPolicyValue_Check_CronJob(t *testing.T) {
	data := []byte(`apiVersion: batch/v1
kind: CronJob
metadata:
  name: test
spec:
  schedule: "*/1 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
          - name: c
            image: nginx
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for CronJob with valid restartPolicy, got %d", len(findings))
	}
}

func TestPodSpecRestartPolicyValue_Check_Service(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for Service, got %v", findings)
	}
}

func TestPodSpecDNSPolicyValue_Check_ConfigMap(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for ConfigMap, got %v", findings)
	}
}

func TestPodSpecAffinityInvalid_Check_Service(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	check := newPodSpecAffinityInvalidCheck()
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for Service, got %v", findings)
	}
}

func TestPodSpecRestartPolicyValue_Check_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	check := newPodSpecRestartPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

func TestPodSpecDNSPolicyValue_Check_InvalidYAML(t *testing.T) {
	data := []byte(`---`)
	check := newPodSpecDNSPolicyValueCheck()
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

func TestPodSpecChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		newPodSpecRestartPolicyValueCheck(),
		newPodSpecDNSPolicyValueCheck(),
		newPodSpecTolerationOperatorValueCheck(),
		newPodSpecNodeSelectorInvalidCheck(),
		newPodSpecAffinityInvalidCheck(),
		newPodSpecTopologySpreadInvalidCheck(),
		newPodSpecServiceAccountNameInvalidCheck(),
		newPodSpecActiveDeadlineSecondsNegativeCheck(),
		newPodSpecReadinessGateInvalidCheck(),
	}

	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if runtime.CategoryOf(c.ID()) == "" {
			t.Errorf("check %T has empty Category", c)
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.Kinds()) == 0 {
			t.Errorf("check %T should declare Kinds", c)
		}
	}
}
