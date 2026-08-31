package validation

import (
	"fmt"
	"strings"
	"testing"
)

// A finding's Path tells the reader where in their manifest the problem is.
// Pod-spec checks receive an already-extracted PodSpec, so the workload kind
// is invisible where the finding is built, and a hard-coded "spec.*" is
// correct only for a bare Pod: controllers nest under spec.template.spec and
// CronJob under spec.jobTemplate.spec.template.spec. These assert the path
// at each nesting depth.

func duplicateVolumeDoc(kind string) []byte {
	vols := `      volumes:
      - name: dup
        emptyDir: {}
      - name: dup
        emptyDir: {}`
	switch kind {
	case "Pod":
		return []byte(`apiVersion: v1
kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: app
    image: nginx@sha256:0000000000000000000000000000000000000000000000000000000000000000
  volumes:
  - name: dup
    emptyDir: {}
  - name: dup
    emptyDir: {}
`)
	case "CronJob":
		return []byte(`apiVersion: batch/v1
kind: CronJob
metadata:
  name: test
spec:
  schedule: "* * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: app
            image: nginx@sha256:0000000000000000000000000000000000000000000000000000000000000000
          volumes:
          - name: dup
            emptyDir: {}
          - name: dup
            emptyDir: {}
`)
	default:
		return []byte(fmt.Sprintf(`apiVersion: apps/v1
kind: %s
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx@sha256:0000000000000000000000000000000000000000000000000000000000000000
%s
`, kind, vols))
	}
}

func TestVolumeFindingPathMatchesWorkloadNesting(t *testing.T) {
	tests := []struct {
		kind     string
		wantPath string
	}{
		{"Pod", "spec.volumes"},
		{"Deployment", "spec.template.spec.volumes"},
		{"StatefulSet", "spec.template.spec.volumes"},
		{"DaemonSet", "spec.template.spec.volumes"},
		{"Job", "spec.template.spec.volumes"},
		{"CronJob", "spec.jobTemplate.spec.template.spec.volumes"},
	}

	c := newDuplicateVolumeNamesCheck()
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			findings := c.Run(duplicateVolumeDoc(tt.kind), "test.yaml")
			if len(findings) != 1 {
				t.Fatalf("expected 1 duplicate-volume finding, got %d: %v", len(findings), findings)
			}
			if got := findings[0].Path; !strings.HasPrefix(got, tt.wantPath) {
				t.Errorf("finding Path = %q, want prefix %q; the path must point at a field that "+
					"actually exists in a %s manifest", got, tt.wantPath, tt.kind)
			}
		})
	}
}

// TestPodSpecFindingPathMatchesWorkloadNesting covers the same rule for a
// non-volume pod-spec field, so the fix is not narrowly tied to volumes.
func TestPodSpecFindingPathMatchesWorkloadNesting(t *testing.T) {
	deployment := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      restartPolicy: Sometimes
      containers:
      - name: app
        image: nginx@sha256:0000000000000000000000000000000000000000000000000000000000000000
`)

	findings := newPodSpecRestartPolicyValueCheck().Run(deployment, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 restartPolicy finding, got %d: %v", len(findings), findings)
	}
	if got, want := findings[0].Path, "spec.template.spec.restartPolicy"; got != want {
		t.Errorf("finding Path = %q, want %q", got, want)
	}
}

// Affinity findings must name the term they came from. The four affinity
// lists live at different paths, and a weighted term nests its selector
// under podAffinityTerm, so a single hard-coded base sends the reader to a
// field that is not in their manifest.
func TestPodAffinityFindingPathsMatchTheTerm(t *testing.T) {
	doc := func(section, body string) []byte {
		return []byte("kind: Pod\nmetadata:\n  name: test\nspec:\n  affinity:\n    " +
			section + ":\n" + body +
			"  containers:\n  - name: c\n    image: nginx\n")
	}
	required := "      requiredDuringSchedulingIgnoredDuringExecution:\n" +
		"      - topologyKey: kubernetes.io/hostname\n" +
		"        labelSelector:\n          matchLabels:\n            \"bad key\": v\n"
	preferred := "      preferredDuringSchedulingIgnoredDuringExecution:\n" +
		"      - weight: 1\n        podAffinityTerm:\n" +
		"          topologyKey: kubernetes.io/hostname\n" +
		"          labelSelector:\n            matchLabels:\n              \"bad key\": v\n"

	tests := []struct {
		name, section, body, wantPath string
	}{
		{
			"podAffinity required", "podAffinity", required,
			"spec.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].labelSelector",
		},
		{
			"podAntiAffinity required", "podAntiAffinity", required,
			"spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].labelSelector",
		},
		{
			"podAffinity preferred", "podAffinity", preferred,
			"spec.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector",
		},
		{
			"podAntiAffinity preferred", "podAntiAffinity", preferred,
			"spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newPodSpecAffinityInvalidCheck().Run(doc(tt.section, tt.body), "test.yaml")
			if len(got) == 0 {
				t.Fatalf("expected a finding for an invalid label key")
			}
			if !strings.HasPrefix(got[0].Path, tt.wantPath) {
				t.Errorf("finding Path = %q, want prefix %q", got[0].Path, tt.wantPath)
			}
		})
	}
}
