package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

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

// Kinds without a pod spec, and unparseable input, must yield nothing
// rather than an error or a spurious finding.
func TestPodSpecEnumChecksIgnoreIrrelevantInput(t *testing.T) {
	for _, c := range []runtime.Check{newPodSpecRestartPolicyValueCheck(), newPodSpecDNSPolicyValueCheck()} {
		for _, data := range [][]byte{
			[]byte("kind: Service\nmetadata:\n  name: test\n"),
			[]byte("kind: ConfigMap\nmetadata:\n  name: test\n"),
			[]byte("not valid yaml {{"),
		} {
			if got := c.Run(data, "test.yaml"); len(got) != 0 {
				t.Errorf("check %s reported %d finding(s) for %q", c.ID(), len(got), data)
			}
		}
	}
}
