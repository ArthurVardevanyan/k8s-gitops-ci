package core

import (
	"slices"
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// containerCase is one full Pod spec body and the findings it must
// produce. The container rules differ in which part of the spec they need
// - init containers, ports, volumes - so a case supplies the whole spec
// rather than a fragment slotted into a fixed frame.
//
// wantPaths, where set, asserts the exact field path of every finding in
// order. Counting findings alone cannot catch a rule that reports the
// right number of problems against the wrong locations, and two findings
// that agree on every field are indistinguishable to the report's
// deduplication - so a mis-indexed pair silently becomes one row.
type containerCase struct {
	name      string
	spec      string
	want      int
	contains  string
	wantPaths []string
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
			if tc.wantPaths != nil {
				got := make([]string, len(findings))
				for i, f := range findings {
					got[i] = f.Path
				}
				if !slices.Equal(got, tc.wantPaths) {
					t.Errorf("paths = %v, want %v", got, tc.wantPaths)
				}
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
		// Two entries sharing a number and protocol. Deriving the index by
		// searching for a matching port returned 0 for both, making the
		// findings byte-identical so the report's deduplication kept one.
		{
			name:      "TwoInvalidPortsSameNumberGetDistinctIndices",
			spec:      "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: a\n      containerPort: 70000\n    - name: b\n      containerPort: 70000\n",
			want:      2,
			wantPaths: []string{"spec.containers[c].ports[0].containerPort", "spec.containers[c].ports[1].containerPort"},
		},
		{
			name:      "InvalidPortAfterValidOneIsIndexedAtItsOwnPosition",
			spec:      "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - name: ok\n      containerPort: 80\n    - name: bad\n      containerPort: 0\n",
			want:      1,
			wantPaths: []string{"spec.containers[c].ports[1].containerPort"},
		},
	})
}

func TestHostPortRange(t *testing.T) {
	runContainerCases(t, newHostPortRangeCheck().Run, []containerCase{
		{name: "TooLarge", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      hostPort: 70000\n", want: 1, contains: "invalid hostPort 70000"},
		{name: "Negative", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      hostPort: -1\n", want: 1},
		// Upstream guards on HostPort != 0; an unset hostPort is not port zero.
		{name: "UnsetIsNotReported", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n", want: 0},
		{name: "ExplicitZeroIsNotReported", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      hostPort: 0\n", want: 0},
		{name: "Valid", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      hostPort: 8080\n", want: 0},
		{
			name:      "IndexedAtItsOwnPosition",
			spec:      "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      hostPort: 8080\n    - containerPort: 81\n      hostPort: 70000\n",
			want:      1,
			wantPaths: []string{"spec.containers[c].ports[1].hostPort"},
		},
	})
}

func TestPortProtocolInvalid(t *testing.T) {
	runContainerCases(t, newPortProtocolInvalidCheck().Run, []containerCase{
		{name: "Unsupported", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      protocol: HTTP\n", want: 1, contains: "must be TCP, UDP or SCTP"},
		{name: "LowercaseIsNotAccepted", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      protocol: tcp\n", want: 1},
		// ContainerPort.Protocol carries `+default="TCP"`, so an omitted
		// protocol is defaulted before upstream's Required branch runs.
		{name: "OmittedIsDefaultedNotReported", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n", want: 0},
		{name: "ExplicitEmptyIsDefaultedNotReported", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      protocol: \"\"\n", want: 0},
		{name: "TCP", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      protocol: TCP\n", want: 0},
		{name: "UDP", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 53\n      protocol: UDP\n", want: 0},
		{name: "SCTP", spec: "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      protocol: SCTP\n", want: 0},
		{
			name:      "IndexedAtItsOwnPosition",
			spec:      "  containers:\n  - name: c\n    image: nginx\n    ports:\n    - containerPort: 80\n      protocol: TCP\n    - containerPort: 81\n      protocol: HTTP\n",
			want:      1,
			wantPaths: []string{"spec.containers[c].ports[1].protocol"},
		},
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

// TestVolumeMountNameUndefinedAcceptsClaimTemplates is the regression test
// for the false positive this rule produced against real StatefulSets.
//
// Mounting a volumeClaimTemplate by name is how the kind is normally used,
// and the API server accepts it because it synthesizes a volume per claim
// template before validating the pod template. Reporting it here was a
// blocking, non-exemptable finding on a valid manifest.
func TestVolumeMountNameUndefinedAcceptsClaimTemplates(t *testing.T) {
	doc := func(claims string) []byte {
		return []byte("apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: prometheus\n" +
			"spec:\n  serviceName: p\n  selector:\n    matchLabels:\n      app: p\n" +
			"  template:\n    metadata:\n      labels:\n        app: p\n    spec:\n" +
			"      containers:\n        - name: prometheus\n          image: prom\n" +
			"          volumeMounts:\n            - name: data\n              mountPath: /data\n" +
			"            - name: config\n              mountPath: /etc\n" +
			"      volumes:\n        - name: config\n          emptyDir: {}\n" + claims)
	}
	withClaim := "  volumeClaimTemplates:\n    - metadata:\n        name: data\n      spec:\n" +
		"        accessModes: [\"ReadWriteOnce\"]\n        resources:\n          requests:\n            storage: 1Gi\n"

	c := newVolumeMountNameUndefinedCheck()

	if got := c.Run(doc(withClaim), "test.yaml"); len(got) != 0 {
		t.Errorf("mounting a volumeClaimTemplate reported %d finding(s): %v", len(got), got)
	}

	// The rule must still fire when the name really is undefined - dropping
	// it entirely would also make the manifest above pass.
	got := c.Run(doc(""), "test.yaml")
	if len(got) != 1 {
		t.Fatalf("an undefined mount reported %d finding(s), want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Message, `"data"`) {
		t.Errorf("finding does not name the undefined mount: %s", got[0].Message)
	}
}

// TestContainerRulesCoverEphemeralContainers pins that the shared traversal
// reaches all three container lists.
//
// Upstream runs validateContainerCommon over ephemeral containers too and
// requires their names to be unique against both other lists. Walking only
// spec.containers and spec.initContainers silently exempted anything
// declared in spec.ephemeralContainers, which for a non-exemptable family
// is the wrong direction to be wrong in.
func TestContainerRulesCoverEphemeralContainers(t *testing.T) {
	const doc = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\nspec:\n" +
		"  containers:\n    - name: app\n      image: nginx\n" +
		"  initContainers:\n    - name: setup\n      image: busybox\n" +
		"  ephemeralContainers:\n    - name: app\n      image: debug\n" +
		"      imagePullPolicy: Bogus\n"

	t.Run("container rules apply", func(t *testing.T) {
		got := newImagePullPolicyCheck().Run([]byte(doc), "test.yaml")
		if len(got) != 1 {
			t.Fatalf("got %d finding(s), want 1: %v", len(got), got)
		}
		if !strings.Contains(got[0].Path, "ephemeralContainers") {
			t.Errorf("finding path %q does not point at spec.ephemeralContainers", got[0].Path)
		}
	})

	t.Run("names collide across lists", func(t *testing.T) {
		got := newDuplicateContainerNamesCheck().Run([]byte(doc), "test.yaml")
		if len(got) != 1 {
			t.Fatalf("an ephemeral container reusing a container name reported %d finding(s), want 1: %v", len(got), got)
		}
	})
}
