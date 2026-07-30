package podspec

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
      enableServiceLinks: false
      restartPolicy: Always
      schedulerName: default
      dnsPolicy: ClusterFirst
      automountServiceAccountToken: false
      containers:
      - name: c
        image: x
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          privileged: false
          runAsNonRoot: true
          capabilities:
            drop:
            - ALL
          seccompProfile:
            type: RuntimeDefault
        resources:
          requests:
            cpu: 10m
          limits:
            cpu: 100m
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateReader_BadDeployment(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: bad
spec:
  template:
    spec:
      containers:
      - name: c
        image: x
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) == 0 {
		t.Fatal("expected errors")
	}
}

func TestValidateReader_BadCronJob(t *testing.T) {
	// Regression: CronJob nests its pod spec three levels deeper than
	// every other workload kind (spec.jobTemplate.spec.template.spec, not
	// spec.template.spec). Previously the hardcoded path never resolved
	// for CronJob, so it was silently never validated at all.
	data := `kind: CronJob
metadata:
  name: bad-cj
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: c
            image: x
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) == 0 {
		t.Fatal("expected errors for a CronJob missing required pod/security fields")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Path, "jobTemplate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected at least one error path to reflect CronJob's nested pod spec, got: %v", errs)
	}
}

func TestValidateReader_GoodCronJob(t *testing.T) {
	data := `kind: CronJob
metadata:
  name: good-cj
spec:
  jobTemplate:
    spec:
      template:
        spec:
          enableServiceLinks: false
          restartPolicy: Never
          schedulerName: default
          dnsPolicy: ClusterFirst
          automountServiceAccountToken: false
          containers:
          - name: c
            image: x
            securityContext:
              allowPrivilegeEscalation: false
              readOnlyRootFilesystem: true
              privileged: false
              runAsNonRoot: true
              capabilities:
                drop:
                - ALL
              seccompProfile:
                type: RuntimeDefault
            resources:
              requests:
                cpu: 10m
              limits:
                cpu: 100m
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for a compliant CronJob: %v", errs)
	}
}

func TestFormatComment(t *testing.T) {
	err := ValidationError{Kind: "Deployment", Name: "bad", MissingFields: []string{"automountServiceAccountToken"}}
	s := FormatComment([]ValidationError{err})
	if !strings.Contains(s, Marker) {
		t.Errorf("expected marker: %q", s)
	}
}

func TestDeduplicate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", Container: "c", MissingFields: []string{"runAsNonRoot"}, Path: "spec.template.spec.containers[].securityContext"},
		{File: "b.yaml", Kind: "Deployment", Name: "d", Container: "c", MissingFields: []string{"runAsNonRoot"}, Path: "spec.template.spec.containers[].securityContext"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}
