package validation

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// containerCase is one full Pod spec body and the findings it must
// produce. The container rules differ in which part of the spec they need
// - init containers, ports, volumes - so a case supplies the whole spec
// rather than a fragment slotted into a fixed frame.
type containerCase struct {
	name     string
	spec     string
	want     int
	contains string
}

func runContainerCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []containerCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := []byte("kind: Pod\nmetadata:\n  name: test\nspec:\n" + tc.spec)
			findings := run(doc, "test.yaml")
			if len(findings) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
			}
			if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
				t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
			}
		})
	}
}

func TestDuplicateContainerNames(t *testing.T) {
	runContainerCases(t, newDuplicateContainerNamesCheck().Run, []containerCase{
		{name: "Duplicate", spec: "  containers:\n  - name: shared\n    image: nginx\n  - name: shared\n    image: redis\n", want: 1},
		{name: "InitContainerDuplicate", spec: "  initContainers:\n  - name: shared\n    image: busybox\n  containers:\n  - name: shared\n    image: nginx\n", want: 1},
		{name: "UniqueNames", spec: "  containers:\n  - name: web\n    image: nginx\n  - name: sidecar\n    image: busybox\n  initContainers:\n  - name: init\n    image: busybox\n", want: 0},
	})
}

func TestDuplicatePortNames(t *testing.T) {
	runContainerCases(t, newDuplicatePortNamesCheck().Run, []containerCase{
		{name: "Duplicate", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: http\n      containerPort: 80\n    - name: http\n      containerPort: 8080\n", want: 1},
		{name: "UniqueNames", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: http\n      containerPort: 80\n    - name: https\n      containerPort: 443\n", want: 0},
	})
}

func TestPortNumberRange(t *testing.T) {
	runContainerCases(t, newPortNumberRangeCheck().Run, []containerCase{
		{name: "InvalidPort", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: http\n      containerPort: 70000\n", want: 1},
		{name: "ZeroPort", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: http\n      containerPort: 0\n", want: 1},
		{name: "ValidPorts", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: http\n      containerPort: 80\n    - name: https\n      containerPort: 65535\n", want: 0},
	})
}

func TestImagePullPolicy(t *testing.T) {
	runContainerCases(t, newImagePullPolicyCheck().Run, []containerCase{
		{name: "ValidAlways", spec: "  containers:\n  - name: c\n    image: nginx\n    imagePullPolicy: Always\n", want: 0},
		{name: "InvalidPolicy", spec: "  containers:\n  - name: c\n    image: nginx\n    imagePullPolicy: InvalidPolicy\n", want: 1},
		{name: "EmptyPolicy", spec: "  containers:\n  - name: c\n    image: nginx\n", want: 0},
	})
}

func TestMountPropagationValue(t *testing.T) {
	runContainerCases(t, newMountPropagationValueCheck().Run, []containerCase{
		{name: "ValidNone", spec: "  containers:\n  - name: c\n    image: nginx\n    volumeMounts:\n    - name: vol\n      mountPath: /data\n      mountPropagation: \"None\"\n  volumes:\n  - name: vol\n    emptyDir: {}\n", want: 0},
		{name: "InvalidValue", spec: "  containers:\n  - name: c\n    image: nginx\n    volumeMounts:\n    - name: vol\n      mountPath: /data\n      mountPropagation: \"InvalidMode\"\n  volumes:\n  - name: vol\n    emptyDir: {}\n", want: 1},
		{name: "NilPropagation", spec: "  containers:\n  - name: c\n    image: nginx\n    volumeMounts:\n    - name: vol\n      mountPath: /data\n  volumes:\n  - name: vol\n    emptyDir: {}\n", want: 0},
	})
}

func TestTerminationMessagePolicyValue(t *testing.T) {
	runContainerCases(t, newTerminationMessagePolicyValueCheck().Run, []containerCase{
		{name: "ValidReadFile", spec: "  containers:\n  - name: c\n    image: nginx\n    terminationMessagePolicy: File\n", want: 0},
		{name: "InvalidPolicy", spec: "  containers:\n  - name: c\n    image: nginx\n    terminationMessagePolicy: InvalidPolicy\n", want: 1},
		{name: "EmptyPolicy", spec: "  containers:\n  - name: c\n    image: nginx\n", want: 0},
	})
}

func TestVolumeMountNameUndefined(t *testing.T) {
	runContainerCases(t, newVolumeMountNameUndefinedCheck().Run, []containerCase{
		{name: "MissingVolume", spec: "  containers:\n  - name: c\n    image: nginx\n    volumeMounts:\n    - name: nonexistent\n      mountPath: /data\n  volumes:\n  - name: other\n    emptyDir: {}\n", want: 1},
		{name: "ValidMount", spec: "  containers:\n  - name: c\n    image: nginx\n    volumeMounts:\n    - name: myvol\n      mountPath: /data\n  volumes:\n  - name: myvol\n    emptyDir: {}\n", want: 0},
	})
}
