package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- container-name tests ---

func TestContainerName_Check_ValidName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: my-container
    image: nginx
`)
	check := containerNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid container name, got %d", len(findings))
	}
}

func TestContainerName_Check_UppercaseName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: MyContainer
    image: nginx
`)
	check := containerNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for uppercase container name, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "container/container-name" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Container != "MyContainer" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

func TestContainerName_Check_EmptyName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: ""
    image: nginx
`)
	check := containerNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty container name (skipped), got %d", len(findings))
	}
}

func TestContainerName_Check_InvalidChars(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: my_container!
    image: nginx
`)
	check := containerNameCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid container name, got %d", len(findings))
	}
}

// --- duplicate-container-names tests ---

func TestDuplicateContainerNames_Check_Duplicate(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: shared
    image: nginx
  - name: shared
    image: redis
`)
	check := duplicateContainerNamesCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate container names, got %d", len(findings))
	}
	if findings[0].RuleID != "container/duplicate-container-names" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestDuplicateContainerNames_Check_InitContainerDuplicate(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  initContainers:
  - name: shared
    image: busybox
  containers:
  - name: shared
    image: nginx
`)
	check := duplicateContainerNamesCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate init/container names, got %d", len(findings))
	}
}

func TestDuplicateContainerNames_Check_UniqueNames(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: web
    image: nginx
  - name: sidecar
    image: busybox
  initContainers:
  - name: init
    image: busybox
`)
	check := duplicateContainerNamesCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unique container names, got %d", len(findings))
	}
}

// --- port-name-format tests ---

func TestPortNameFormat_Check_InvalidName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: 123http
      containerPort: 80
`)
	check := portNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid port name, got %d", len(findings))
	}
}

func TestPortNameFormat_Check_ValidName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 80
`)
	check := portNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid port name, got %d", len(findings))
	}
}

func TestPortNameFormat_Check_TooLongName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmno
      containerPort: 80
`)
	check := portNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for port name too long, got %d", len(findings))
	}
}

func TestPortNameFormat_Check_EmptyName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - containerPort: 80
`)
	check := portNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty port name (skipped), got %d", len(findings))
	}
}

// --- duplicate-port-names tests ---

func TestDuplicatePortNames_Check_Duplicate(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 80
    - name: http
      containerPort: 8080
`)
	check := duplicatePortNamesCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate port names, got %d", len(findings))
	}
}

func TestDuplicatePortNames_Check_UniqueNames(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 80
    - name: https
      containerPort: 443
`)
	check := duplicatePortNamesCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unique port names, got %d", len(findings))
	}
}

// --- port-number-range tests ---

func TestPortNumberRange_Check_InvalidPort(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 70000
`)
	check := portNumberRangeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid port number, got %d", len(findings))
	}
}

func TestPortNumberRange_Check_ZeroPort(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 0
`)
	check := portNumberRangeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for port number 0, got %d", len(findings))
	}
}

func TestPortNumberRange_Check_ValidPorts(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 80
    - name: https
      containerPort: 65535
`)
	check := portNumberRangeCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid port numbers, got %d", len(findings))
	}
}

// --- duplicate-port-numbers tests ---

func TestDuplicatePortNumbers_Check_DuplicateSameProto(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http1
      containerPort: 80
    - name: http2
      containerPort: 80
`)
	check := duplicatePortNumbersCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate port numbers, got %d", len(findings))
	}
}

func TestDuplicatePortNumbers_Check_DifferentProtocols(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http-tcp
      containerPort: 80
      protocol: TCP
    - name: http-udp
      containerPort: 80
      protocol: UDP
`)
	check := duplicatePortNumbersCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for same port with different protocols, got %d", len(findings))
	}
}

// --- port-name-unique tests ---

func TestPortNameUnique_Check_NoNames(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - containerPort: 80
    - containerPort: 8080
`)
	check := portNameUniqueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no ports are named, got %d", len(findings))
	}
}

func TestPortNameUnique_Check_SomeUnnamed(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 80
    - containerPort: 8080
`)
	check := portNameUniqueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when some ports are unnamed, got %d", len(findings))
	}
}

func TestPortNameUnique_Check_AllNamed(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    ports:
    - name: http
      containerPort: 80
    - name: https
      containerPort: 443
`)
	check := portNameUniqueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when all ports are named, got %d", len(findings))
	}
}

// --- image-pull-policy tests ---

func TestImagePullPolicy_Check_ValidAlways(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    imagePullPolicy: Always
`)
	check := imagePullPolicyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Always policy, got %d", len(findings))
	}
}

func TestImagePullPolicy_Check_InvalidPolicy(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    imagePullPolicy: InvalidPolicy
`)
	check := imagePullPolicyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid imagePullPolicy, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "container/image-pull-policy" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestImagePullPolicy_Check_EmptyPolicy(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := imagePullPolicyCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty imagePullPolicy, got %d", len(findings))
	}
}

// --- mount-propagation-value tests ---

func TestMountPropagationValue_Check_ValidNone(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    volumeMounts:
    - name: vol
      mountPath: /data
      mountPropagation: "None"
  volumes:
  - name: vol
    emptyDir: {}
`)
	check := mountPropagationValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for None propagation, got %d", len(findings))
	}
}

func TestMountPropagationValue_Check_InvalidValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    volumeMounts:
    - name: vol
      mountPath: /data
      mountPropagation: "InvalidMode"
  volumes:
  - name: vol
    emptyDir: {}
`)
	check := mountPropagationValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid mountPropagation, got %d", len(findings))
	}
}

func TestMountPropagationValue_Check_NilPropagation(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    volumeMounts:
    - name: vol
      mountPath: /data
  volumes:
  - name: vol
    emptyDir: {}
`)
	check := mountPropagationValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil mountPropagation, got %d", len(findings))
	}
}

// --- restart-policy-value tests ---

func TestRestartPolicyValue_Check_ValidAlways(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  restartPolicy: Always
  containers:
  - name: c
    image: nginx
`)
	check := restartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Always restart policy, got %d", len(findings))
	}
}

func TestRestartPolicyValue_Check_InvalidValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  restartPolicy: InvalidPolicy
  containers:
  - name: c
    image: nginx
`)
	check := restartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid restartPolicy, got %d", len(findings))
	}
}

func TestRestartPolicyValue_Check_EmptyValue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := restartPolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty restartPolicy, got %d", len(findings))
	}
}

// --- termination-message-path tests ---

func TestTerminationMessagePath_Check_ValidPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    terminationMessagePath: /dev/termination-log
`)
	check := terminationMessagePathCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid termination message path, got %d", len(findings))
	}
}

func TestTerminationMessagePath_Check_InvalidPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    terminationMessagePath: relative/path
`)
	check := terminationMessagePathCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for relative termination message path, got %d", len(findings))
	}
}

func TestTerminationMessagePath_Check_EmptyPath(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := terminationMessagePathCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty termination message path, got %d", len(findings))
	}
}

// --- termination-message-policy-value tests ---

func TestTerminationMessagePolicyValue_Check_ValidReadFile(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    terminationMessagePolicy: File
`)
	check := terminationMessagePolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for File policy, got %d", len(findings))
	}
}

func TestTerminationMessagePolicyValue_Check_InvalidPolicy(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    terminationMessagePolicy: InvalidPolicy
`)
	check := terminationMessagePolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for invalid termination message policy, got %d", len(findings))
	}
}

func TestTerminationMessagePolicyValue_Check_EmptyPolicy(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := terminationMessagePolicyValueCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty termination message policy, got %d", len(findings))
	}
}

// --- env-name-duplicate tests ---

func TestEnvNameDuplicate_Check_DuplicateNames(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    env:
    - name: MY_VAR
      value: "a"
    - name: MY_VAR
      value: "b"
`)
	check := envNameDuplicateCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for duplicate env names, got %d", len(findings))
	}
}

func TestEnvNameDuplicate_Check_UniqueNames(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    env:
    - name: VAR1
      value: "a"
    - name: VAR2
      value: "b"
`)
	check := envNameDuplicateCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for unique env names, got %d", len(findings))
	}
}

// --- env-name-format tests ---

func TestEnvNameFormat_Check_ValidName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    env:
    - name: MY_VAR_123
      value: "a"
`)
	check := envNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid env name, got %d", len(findings))
	}
}

func TestEnvNameFormat_Check_StartsWithDigit(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    env:
    - name: 1INVALID
      value: "a"
`)
	check := envNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for env name starting with digit, got %d", len(findings))
	}
}

func TestEnvNameFormat_Check_EmptyName(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    env:
    - name: ""
      value: "a"
`)
	check := envNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for empty env name, got %d", len(findings))
	}
}

func TestEnvNameFormat_Check_InvalidChars(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    env:
    - name: MY-VAR
      value: "a"
`)
	check := envNameFormatCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for env name with dash, got %d", len(findings))
	}
}

// --- volume-mount-name-duplicate tests ---

func TestVolumeMountNameDuplicate_Check_MissingVolume(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    volumeMounts:
    - name: nonexistent
      mountPath: /data
  volumes:
  - name: other
    emptyDir: {}
`)
	check := volumeMountNameDuplicateCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing volume mount, got %d", len(findings))
	}
	if findings[0].RuleID != "container/volume-mount-name-duplicate" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestVolumeMountNameDuplicate_Check_ValidMount(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    volumeMounts:
    - name: myvol
      mountPath: /data
  volumes:
  - name: myvol
    emptyDir: {}
`)
	check := volumeMountNameDuplicateCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid volume mount, got %d", len(findings))
	}
}

// --- Interface conformance ---

func TestContainerChecksImplementCheckInterface(t *testing.T) {
	var _ runtime.Check = containerNameCheck{}
	var _ runtime.Check = duplicateContainerNamesCheck{}
	var _ runtime.Check = portNameFormatCheck{}
	var _ runtime.Check = duplicatePortNamesCheck{}
	var _ runtime.Check = portNumberRangeCheck{}
	var _ runtime.Check = duplicatePortNumbersCheck{}
	var _ runtime.Check = portNameUniqueCheck{}
	var _ runtime.Check = imagePullPolicyCheck{}
	var _ runtime.Check = mountPropagationValueCheck{}
	var _ runtime.Check = restartPolicyValueCheck{}
	var _ runtime.Check = terminationMessagePathCheck{}
	var _ runtime.Check = terminationMessagePolicyValueCheck{}
	var _ runtime.Check = envNameDuplicateCheck{}
	var _ runtime.Check = envNameFormatCheck{}
	var _ runtime.Check = volumeMountNameDuplicateCheck{}
}

// --- Register conformance ---

// Note: TestRegister in security_context_test.go verifies that Register() can
// be called without panicking (including all new container checks).
