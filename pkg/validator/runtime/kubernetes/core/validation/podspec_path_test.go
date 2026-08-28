package validation

import (
	"fmt"
	"strings"
	"testing"
)

// A finding's Path tells the reader where in *their* manifest the problem
// is. Pod-spec checks receive an already-extracted PodSpec, so the workload
// kind is invisible at the point the finding is built; hard-coding "spec.*"
// there produced a path that is only correct for a bare Pod. Every
// controller nests its pod spec under spec.template.spec, and a CronJob
// under spec.jobTemplate.spec.template.spec, so those findings pointed at a
// field that does not exist in the document.
//
// Nothing caught this because no test asserted a Path on anything but a
// Pod. These do, for each nesting depth.

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
