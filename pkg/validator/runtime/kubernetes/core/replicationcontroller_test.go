package core

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// rc builds a ReplicationController with the given spec body.
func rc(spec string) []byte {
	return []byte("apiVersion: v1\nkind: ReplicationController\nmetadata:\n  name: test\nspec:\n" + spec)
}

// templateWithLabels is a minimal pod template carrying labels, which is what
// SetDefaults_ReplicationController copies into an empty selector.
const templateWithLabels = "  template:\n    metadata:\n      labels:\n        app: test\n" +
	"    spec:\n      containers:\n        - name: c\n          image: nginx\n"

// templateWithoutLabels is the same template with nothing to default from.
const templateWithoutLabels = "  template:\n    metadata: {}\n" +
	"    spec:\n      containers:\n        - name: c\n          image: nginx\n"

type rcCase struct {
	name     string
	spec     string
	want     int
	contains string
}

func TestReplicationControllerReplicasInvalid(t *testing.T) {
	c := newReplicationControllerReplicasInvalidCheck()
	cases := []rcCase{
		{name: "positive", spec: "  replicas: 3\n" + templateWithLabels, want: 0},
		{name: "zero is a valid scale-to-nothing", spec: "  replicas: 0\n" + templateWithLabels, want: 0},
		// Defaulting supplies 1, so an unrendered manifest omitting the
		// field is not an error even though upstream reports Required on a
		// nil replicas.
		{name: "omitted", spec: templateWithLabels, want: 0},
		{name: "negative", spec: "  replicas: -1\n" + templateWithLabels, want: 1, contains: "must be >= 0"},
	}
	runRCCases(t, c.Run, cases)
}

func TestReplicationControllerMinReadySecondsInvalid(t *testing.T) {
	c := newReplicationControllerMinReadySecondsInvalidCheck()
	runRCCases(t, c.Run, []rcCase{
		{name: "positive", spec: "  minReadySeconds: 10\n" + templateWithLabels, want: 0},
		{name: "zero", spec: "  minReadySeconds: 0\n" + templateWithLabels, want: 0},
		{name: "omitted", spec: templateWithLabels, want: 0},
		{name: "negative", spec: "  minReadySeconds: -1\n" + templateWithLabels, want: 1, contains: "must be >= 0"},
	})
}

// TestReplicationControllerSelectorInvalid covers the defaulting that makes
// an omitted selector valid.
//
// SetDefaults_ReplicationController copies the pod template's labels into an
// empty selector before validation, so almost every real
// ReplicationController omits spec.selector. A rule reading the field as
// written reports all of them, on a check that cannot be exempted.
func TestReplicationControllerSelectorInvalid(t *testing.T) {
	c := newReplicationControllerSelectorInvalidCheck()
	runRCCases(t, c.Run, []rcCase{
		{
			name: "explicit selector",
			spec: "  selector:\n    app: test\n" + templateWithLabels,
			want: 0,
		},
		{
			name: "omitted selector is defaulted from the template labels",
			spec: templateWithLabels,
			want: 0,
		},
		{
			name: "empty selector is defaulted from the template labels",
			spec: "  selector: {}\n" + templateWithLabels,
			want: 0,
		},
		{
			// Nothing to default from, so upstream's selector is still
			// empty when ValidateNonEmptySelector runs.
			name:     "no selector and no template labels",
			spec:     templateWithoutLabels,
			want:     1,
			contains: "Required value",
		},
	})
}

// TestReplicationControllerChecksIgnoreOtherKinds pins the kind guard. A
// ReplicaSet has the same fields and would otherwise be reported twice, once
// by its own apps rule and once by this one.
func TestReplicationControllerChecksIgnoreOtherKinds(t *testing.T) {
	doc := []byte("apiVersion: apps/v1\nkind: ReplicaSet\nmetadata:\n  name: test\nspec:\n  replicas: -1\n")
	for _, run := range []func([]byte, string) []runtime.Finding{
		newReplicationControllerReplicasInvalidCheck().Run,
		newReplicationControllerSelectorInvalidCheck().Run,
		newReplicationControllerMinReadySecondsInvalidCheck().Run,
	} {
		if got := run(doc, "test.yaml"); len(got) != 0 {
			t.Errorf("reported %d finding(s) on a ReplicaSet", len(got))
		}
	}
}

func runRCCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []rcCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := run(rc(tc.spec), "test.yaml")
			if len(findings) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
			}
			if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
				t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
			}
		})
	}
}
