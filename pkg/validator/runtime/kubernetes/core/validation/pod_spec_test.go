package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- restart-policy-value ---

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
	check := podSpecRestartPolicyValueCheck{}
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
	check := podSpecRestartPolicyValueCheck{}
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
	check := podSpecRestartPolicyValueCheck{}
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
	check := podSpecRestartPolicyValueCheck{}
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
	check := podSpecRestartPolicyValueCheck{}
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
	check := podSpecRestartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Deployment with valid restartPolicy, got %d", len(findings))
	}
}

// --- host-network ---

func TestPodSpecHostNetwork_Check_Disabled(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostNetwork: false
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostNetworkCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for disabled hostNetwork, got %d", len(findings))
	}
}

func TestPodSpecHostNetwork_Check_DisabledDefault(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostNetworkCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unset hostNetwork, got %d", len(findings))
	}
}

func TestPodSpecHostNetwork_Check_Enabled(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostNetwork: true
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostNetworkCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for enabled hostNetwork, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/host-network" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Value != "true" {
		t.Errorf("unexpected value: %s", findings[0].Value)
	}
}

func TestPodSpecHostNetwork_Check_Deployment(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      hostNetwork: true
      containers:
      - name: c
        image: nginx
`)
	check := podSpecHostNetworkCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for Deployment with hostNetwork, got %d", len(findings))
	}
	if findings[0].Kind != "Deployment" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

// --- host-pid ---

func TestPodSpecHostPID_Check_Disabled(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostPID: false
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostPIDCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for disabled hostPID, got %d", len(findings))
	}
}

func TestPodSpecHostPID_Check_Enabled(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostPID: true
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostPIDCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for enabled hostPID, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/host-pid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Message != "hostPID is set to true: pod shares the host process namespace" {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestPodSpecHostPID_Check_StatefulSet(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  template:
    spec:
      hostPID: true
      containers:
      - name: c
        image: nginx
`)
	check := podSpecHostPIDCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for StatefulSet with hostPID, got %d", len(findings))
	}
	if findings[0].Kind != "StatefulSet" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

// --- host-ipc ---

func TestPodSpecHostIPC_Check_Disabled(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostIPC: false
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostIPCCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for disabled hostIPC, got %d", len(findings))
	}
}

func TestPodSpecHostIPC_Check_Enabled(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostIPC: true
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostIPCCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for enabled hostIPC, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/host-ipc" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Message != "hostIPC is set to true: pod shares the host IPC namespace" {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

// --- dns-policy-value ---

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
	check := podSpecDNSPolicyValueCheck{}
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
	check := podSpecDNSPolicyValueCheck{}
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
	check := podSpecDNSPolicyValueCheck{}
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
	check := podSpecDNSPolicyValueCheck{}
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
	check := podSpecDNSPolicyValueCheck{}
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
	check := podSpecDNSPolicyValueCheck{}
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

// --- dns-config-invalid ---

func TestPodSpecDNSConfigInvalid_Check_DNSPolicyNoneWithConfig(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: None
  dnsConfig:
    nameservers:
    - 8.8.8.8
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDNSConfigInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for dnsPolicy None with dnsConfig, got %d", len(findings))
	}
}

func TestPodSpecDNSConfigInvalid_Check_DNSPolicyNoneEmptyConfig(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: None
  dnsConfig: {}
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDNSConfigInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for dnsPolicy None with empty dnsConfig, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/dns-config-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Message != "dnsConfig is required when dnsPolicy is None" {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestPodSpecDNSConfigInvalid_Check_DNSPolicyNoneNoConfig(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: None
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDNSConfigInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for dnsPolicy None without dnsConfig, got %d", len(findings))
	}
}

func TestPodSpecDNSConfigInvalid_Check_DNSPolicyClusterFirst(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: ClusterFirst
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDNSConfigInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for dnsPolicy ClusterFirst, got %d", len(findings))
	}
}

func TestPodSpecDNSConfigInvalid_Check_DNSPolicyNoneSearches(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: None
  dnsConfig:
    searches:
    - my.svc.cluster.local
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDNSConfigInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for dnsPolicy None with dnsConfig searches, got %d", len(findings))
	}
}

func TestPodSpecDNSConfigInvalid_Check_DNSPolicyNoneOptions(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  dnsPolicy: None
  dnsConfig:
    options:
    - name: ndots
      value: "5"
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDNSConfigInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for dnsPolicy None with dnsConfig options, got %d", len(findings))
	}
}

// --- toleration-operator-value ---

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
	check := podSpecTolerationOperatorValueCheck{}
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
	check := podSpecTolerationOperatorValueCheck{}
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
	check := podSpecTolerationOperatorValueCheck{}
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
	check := podSpecTolerationOperatorValueCheck{}
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
	check := podSpecTolerationOperatorValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for two invalid operators, got %d", len(findings))
	}
}

// --- affinity-node-selector-invalid ---

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
	check := podSpecNodeSelectorInvalidCheck{}
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
	check := podSpecNodeSelectorInvalidCheck{}
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
	check := podSpecNodeSelectorInvalidCheck{}
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
	check := podSpecNodeSelectorInvalidCheck{}
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
	check := podSpecNodeSelectorInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for azure.com/region key, got %d", len(findings))
	}
}

// --- pod-affinity-invalid ---

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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
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
	check := podSpecAffinityInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil labelSelector, got %d", len(findings))
	}
}

// --- pod-anti-affinity-invalid ---

func TestPodSpecAntiAffinityInvalid_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: web
        topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := podSpecAntiAffinityInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid pod anti-affinity, got %d", len(findings))
	}
}

func TestPodSpecAntiAffinityInvalid_Check_InvalidKey(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            invalid key!: web
        topologyKey: kubernetes.io/hostname
  containers:
  - name: c
    image: nginx
`)
	check := podSpecAntiAffinityInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid anti-affinity key, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/pod-affinity-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestPodSpecAntiAffinityInvalid_Check_StatefulSet(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
spec:
  template:
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: web
            topologyKey: kubernetes.io/hostname
      containers:
      - name: c
        image: nginx
`)
	check := podSpecAntiAffinityInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for StatefulSet valid anti-affinity, got %d", len(findings))
	}
}

// --- topology-spread-invalid ---

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
	check := podSpecTopologySpreadInvalidCheck{}
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
	check := podSpecTopologySpreadInvalidCheck{}
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
	check := podSpecTopologySpreadInvalidCheck{}
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
	check := podSpecTopologySpreadInvalidCheck{}
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
	check := podSpecTopologySpreadInvalidCheck{}
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
	check := podSpecTopologySpreadInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for second constraint with invalid key, got %d", len(findings))
	}
}

// --- scheduler-name-invalid ---

func TestPodSpecSchedulerName_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  schedulerName: my-scheduler
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSchedulerNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid schedulerName, got %d", len(findings))
	}
}

func TestPodSpecSchedulerName_Check_DefaultScheduler(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  schedulerName: default-scheduler
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSchedulerNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for default-scheduler, got %d", len(findings))
	}
}

func TestPodSpecSchedulerName_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSchedulerNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty schedulerName, got %d", len(findings))
	}
}

func TestPodSpecSchedulerName_Check_Invalid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  schedulerName: invalid scheduler!
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSchedulerNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid schedulerName, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/scheduler-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

// --- service-account-name-invalid ---

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
	check := podSpecServiceAccountNameInvalidCheck{}
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
	check := podSpecServiceAccountNameInvalidCheck{}
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
	check := podSpecServiceAccountNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid serviceAccountName, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/service-account-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

// --- automount-sa-token-value (no-op, always valid) ---

func TestPodSpecAutomountSATokenValue_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  automountServiceAccountToken: true
  containers:
  - name: c
    image: nginx
`)
	check := podSpecAutomountSATokenValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings (no-op check), got %d", len(findings))
	}
}

func TestPodSpecAutomountSATokenValue_Check_False(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  automountServiceAccountToken: false
  containers:
  - name: c
    image: nginx
`)
	check := podSpecAutomountSATokenValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings (no-op check), got %d", len(findings))
	}
}

func TestPodSpecAutomountSATokenValue_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := podSpecAutomountSATokenValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings (no-op check), got %d", len(findings))
	}
}

// --- active-deadline-seconds-negative ---

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
	check := podSpecActiveDeadlineSecondsNegativeCheck{}
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
	check := podSpecActiveDeadlineSecondsNegativeCheck{}
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
	check := podSpecActiveDeadlineSecondsNegativeCheck{}
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
	check := podSpecActiveDeadlineSecondsNegativeCheck{}
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
	check := podSpecActiveDeadlineSecondsNegativeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for large activeDeadlineSeconds, got %d", len(findings))
	}
}

// --- subdomain-invalid ---

func TestPodSpecSubdomain_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  subdomain: my-subdomain
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSubdomainInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid subdomain, got %d", len(findings))
	}
}

func TestPodSpecSubdomain_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSubdomainInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty subdomain, got %d", len(findings))
	}
}

func TestPodSpecSubdomain_Check_Invalid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  subdomain: INVALID/SUBDOMAIN
  containers:
  - name: c
    image: nginx
`)
	check := podSpecSubdomainInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid subdomain, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/subdomain-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

// --- set-hostname-invalid ---

func TestPodSpecHostname_Check_Valid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostname: my-host
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostnameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid hostname, got %d", len(findings))
	}
}

func TestPodSpecHostname_Check_Empty(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostnameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty hostname, got %d", len(findings))
	}
}

func TestPodSpecHostname_Check_Invalid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostname: INVALID/HOST!
  containers:
  - name: c
    image: nginx
`)
	check := podSpecHostnameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid hostname, got %d", len(findings))
	}
	if findings[0].RuleID != "pod-spec/set-hostname-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

// --- set-domain-name-invalid (no-op, field doesn't exist) ---

func TestPodSpecDomainNameInvalid_Check_AlwaysValid(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := podSpecDomainNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings (no-op check), got %d", len(findings))
	}
}

// --- readiness-gate-invalid ---

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
	check := podSpecReadinessGateInvalidCheck{}
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
	check := podSpecReadinessGateInvalidCheck{}
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
	check := podSpecReadinessGateInvalidCheck{}
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
	check := podSpecReadinessGateInvalidCheck{}
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
	check := podSpecReadinessGateInvalidCheck{}
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
	check := podSpecReadinessGateInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for no readinessGates, got %d", len(findings))
	}
}

// --- host-ports-overlap ---

func TestPodSpecHostPortsOverlap_Check_NoHostNetwork(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c1
    image: nginx
    ports:
    - containerPort: 80
  - name: c2
    image: nginx
    ports:
    - containerPort: 80
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when hostNetwork=false, got %d", len(findings))
	}
}

func TestPodSpecHostPortsOverlap_Check_NoHostPorts(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostNetwork: true
  containers:
  - name: c1
    image: nginx
    ports:
    - containerPort: 80
  - name: c2
    image: nginx
    ports:
    - containerPort: 443
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no hostPorts, got %d", len(findings))
	}
}

func TestPodSpecHostPortsOverlap_Check_SameHostPortDifferentProto(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostNetwork: true
  containers:
  - name: c1
    image: nginx
    ports:
    - containerPort: 80
      hostPort: 8080
      protocol: TCP
  - name: c2
    image: nginx
    ports:
    - containerPort: 443
      hostPort: 8080
      protocol: UDP
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for same hostPort with different protocols, got %d", len(findings))
	}
}

func TestPodSpecHostPortsOverlap_Check_SameHostPortSameProto(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostNetwork: true
  containers:
  - name: c1
    image: nginx
    ports:
    - containerPort: 80
      hostPort: 8080
      protocol: TCP
  - name: c2
    image: nginx
    ports:
    - containerPort: 443
      hostPort: 8080
      protocol: TCP
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for overlapping hostPort, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "pod-spec/host-ports-overlap" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Container != "c2" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

func TestPodSpecHostPortsOverlap_Check_Deployment(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      hostNetwork: true
      containers:
      - name: c1
        image: nginx
        ports:
        - containerPort: 80
          hostPort: 8080
          protocol: TCP
      - name: c2
        image: nginx
        ports:
        - containerPort: 443
          hostPort: 8080
          protocol: TCP
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for Deployment hostPort overlap, got %d", len(findings))
	}
	if findings[0].Kind != "Deployment" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPodSpecHostPortsOverlap_Check_InitContainer(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  hostNetwork: true
  initContainers:
  - name: init
    image: nginx
    ports:
    - containerPort: 80
      hostPort: 8080
      protocol: TCP
  containers:
  - name: c1
    image: nginx
    ports:
    - containerPort: 80
      hostPort: 8080
      protocol: TCP
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for initContainer hostPort overlap, got %d", len(findings))
	}
	if findings[0].Container != "init" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

// --- CronJob tests ---

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
	check := podSpecRestartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for CronJob with valid restartPolicy, got %d", len(findings))
	}
}

func TestPodSpecHostNetwork_Check_CronJob(t *testing.T) {
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
          hostNetwork: true
          containers:
          - name: c
            image: nginx
`)
	check := podSpecHostNetworkCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for CronJob with hostNetwork, got %d", len(findings))
	}
	if findings[0].Kind != "CronJob" {
		t.Errorf("unexpected kind: %s", findings[0].Kind)
	}
}

func TestPodSpecAntiAffinityInvalid_Check_CronJob(t *testing.T) {
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
          affinity:
            podAntiAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
              - labelSelector:
                  matchLabels:
                    app: web
                topologyKey: kubernetes.io/hostname
          containers:
          - name: c
            image: nginx
`)
	check := podSpecAntiAffinityInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for CronJob valid anti-affinity, got %d", len(findings))
	}
}

// --- Non-workload kinds ---

func TestPodSpecRestartPolicyValue_Check_Service(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	check := podSpecRestartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for Service, got %v", findings)
	}
}

func TestPodSpecHostNetwork_Check_Service(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	check := podSpecHostNetworkCheck{}
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
	check := podSpecDNSPolicyValueCheck{}
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
	check := podSpecAffinityInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for Service, got %v", findings)
	}
}

func TestPodSpecHostPortsOverlap_Check_Service(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	check := podSpecHostPortsOverlapCheck{}
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for Service, got %v", findings)
	}
}

// --- Invalid YAML ---

func TestPodSpecRestartPolicyValue_Check_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	check := podSpecRestartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

func TestPodSpecHostNetwork_Check_InvalidYAML(t *testing.T) {
	data := []byte(`{{invalid yaml`)
	check := podSpecHostNetworkCheck{}
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

func TestPodSpecDNSPolicyValue_Check_InvalidYAML(t *testing.T) {
	data := []byte(`---`)
	check := podSpecDNSPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

// --- Interface conformance ---

func TestPodSpecChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
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
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if c.Category() == "" {
			t.Errorf("check %T has empty Category", c)
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}

// --- Register conformance ---

// Note: TestRegister in security_context_test.go verifies that Register() can
// be called without panicking (including all pod-spec checks).
