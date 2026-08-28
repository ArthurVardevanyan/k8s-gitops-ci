package validation

import "testing"

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
	check := volumeMountNameUndefinedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing volume mount, got %d", len(findings))
	}
	if findings[0].RuleID != "container/volume-mount-name-undefined" {
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
	check := volumeMountNameUndefinedCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid volume mount, got %d", len(findings))
	}
}
