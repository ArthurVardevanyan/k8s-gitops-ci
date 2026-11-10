package livenessprobe

import (
	"strings"
	"testing"
)

func TestValidateReader_GoodDeployment(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: good
spec:
  template:
    spec:
      containers:
      - name: c
        image: nginx
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateReader_MissingLivenessProbe(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: bad
spec:
  template:
    spec:
      containers:
      - name: c
        image: nginx
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Kind != "Deployment" || errs[0].Name != "bad" || errs[0].Container != "c" {
		t.Errorf("unexpected finding: %+v", errs[0])
	}
	if !strings.Contains(errs[0].Path, "livenessProbe") {
		t.Errorf("expected path to contain 'livenessProbe', got: %q", errs[0].Path)
	}
}

func TestValidateReader_MultipleContainers(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: multi
spec:
  template:
    spec:
      containers:
      - name: c1
        image: nginx
      - name: c2
        image: redis
        livenessProbe:
          tcpSocket:
            port: 6379
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (only c1 missing probe), got %d: %v", len(errs), errs)
	}
	if errs[0].Container != "c1" {
		t.Errorf("expected finding for container c1, got: %q", errs[0].Container)
	}
}

func TestValidateReader_MissingInitContainerProbe(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: deploy
spec:
  template:
    spec:
      initContainers:
      - name: init
        image: busybox
      containers:
      - name: app
        image: nginx
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (only init missing probe), got %d: %v", len(errs), errs)
	}
	if errs[0].Container != "init" {
		t.Errorf("expected finding for init container, got: %q", errs[0].Container)
	}
	if !strings.Contains(errs[0].Path, "initContainers") {
		t.Errorf("expected path to reference initContainers, got: %q", errs[0].Path)
	}
	if strings.Contains(errs[0].Path, ".containers") {
		t.Errorf("expected path not to reference .containers, got: %q", errs[0].Path)
	}
}

func TestValidateReader_Pod(t *testing.T) {
	data := `kind: Pod
metadata:
  name: pod-missing
spec:
  containers:
  - name: app
    image: busybox
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for Pod, got %d: %v", len(errs), errs)
	}
	if errs[0].Kind != "Pod" {
		t.Errorf("expected Kind=Pod, got: %q", errs[0].Kind)
	}
}

func TestValidateReader_StatefulSet(t *testing.T) {
	data := `kind: StatefulSet
metadata:
  name: ss
spec:
  template:
    spec:
      containers:
      - name: web
        image: nginx
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for StatefulSet, got %d: %v", len(errs), errs)
	}
}

func TestValidateReader_DaemonSet(t *testing.T) {
	data := `kind: DaemonSet
metadata:
  name: ds
spec:
  template:
    spec:
      containers:
      - name: agent
        image: fluentd
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for DaemonSet, got %d: %v", len(errs), errs)
	}
}

func TestValidateReader_Job(t *testing.T) {
	data := `kind: Job
metadata:
  name: job
spec:
  template:
    spec:
      containers:
      - name: worker
        image: python
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for Job, got %d: %v", len(errs), errs)
	}
}

func TestValidateReader_CronJob(t *testing.T) {
	data := `kind: CronJob
metadata:
  name: cj
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: batch
            image: python
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for CronJob, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Path, "jobTemplate") {
		t.Errorf("expected path to include 'jobTemplate', got: %q", errs[0].Path)
	}
}

func TestValidateReader_ReplicaSet(t *testing.T) {
	data := `kind: ReplicaSet
metadata:
  name: rs
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for ReplicaSet, got %d: %v", len(errs), errs)
	}
}

func TestValidateReader_NonWorkload(t *testing.T) {
	data := `apiVersion: v1
kind: Service
metadata:
  name: svc
spec:
  ports:
  - port: 80
`
	errs := ValidateReader(strings.NewReader(data), "test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for Service, got: %v", errs)
	}
}

func TestValidateFile_GoodFixtures(t *testing.T) {
	for _, f := range []string{"testdata/good.yaml"} {
		t.Run(f, func(t *testing.T) {
			errs := ValidateFile(f)
			if len(errs) != 0 {
				t.Errorf("expected no findings for %s, got: %v", f, errs)
			}
		})
	}
}

func TestValidateFile_BadDeployment(t *testing.T) {
	errs := ValidateFile("testdata/invalid/missing-liveness-probe.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Kind != "Deployment" || errs[0].Container != "app" {
		t.Errorf("unexpected finding: %+v", errs[0])
	}
}
