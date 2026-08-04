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

func TestValidateReader_CarriesResourceAnnotationsAndValue(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: bad
  annotations:
    gitops-ci.k8s.io/exempt-podspec-defaults: "enableServiceLinks, restartPolicy, schedulerName, dnsPolicy, automountServiceAccountToken"
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
	var podLevel *ValidationError
	for i := range errs {
		if errs[i].Container == "" {
			podLevel = &errs[i]
			break
		}
	}
	if podLevel == nil {
		t.Fatalf("expected a pod-level (Container == \"\") finding, got %v", errs)
	}
	wantValue := "enableServiceLinks, restartPolicy, schedulerName, dnsPolicy, automountServiceAccountToken"
	if podLevel.Value() != wantValue {
		t.Errorf("Value() = %q, want %q", podLevel.Value(), wantValue)
	}
	if podLevel.Annotations["gitops-ci.k8s.io/exempt-podspec-defaults"] != wantValue {
		t.Errorf("Annotations not carried through: %v", podLevel.Annotations)
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

func TestFormatComment_Empty(t *testing.T) {
	s := FormatComment(nil)
	if s != "" {
		t.Errorf("expected empty string for no findings, got: %q", s)
	}
}

func TestFormatComment_PathColumn(t *testing.T) {
	// Regression for change #2: FormatComment must render a Path column
	// so the exact fix location isn't lost in the PR comment.
	err := ValidationError{
		Kind: "Deployment", Name: "bad", Container: "c",
		MissingFields: []string{"resources.limits"},
		Path:          "spec.template.spec.containers[]",
	}
	s := FormatComment([]ValidationError{err})
	if !strings.Contains(s, "| Resource | Path | Missing Fields |") {
		t.Errorf("expected a Path column header, got: %q", s)
	}
	if !strings.Contains(s, "`spec.template.spec.containers[]`") {
		t.Errorf("expected the Path value to be rendered, got: %q", s)
	}
}

func TestFormatDeduplicatedComment_Empty(t *testing.T) {
	s := FormatDeduplicatedComment(nil)
	if s != "" {
		t.Errorf("expected empty string for no findings, got: %q", s)
	}
}

func TestFormatDeduplicatedComment_SingleFileShowsFilename(t *testing.T) {
	d := DeduplicatedError{
		Kind: "Deployment", Name: "d", MissingFields: []string{"runAsNonRoot"},
		Path:  "spec.template.spec.containers[].securityContext",
		Files: []string{"a.yaml"}, Count: 1,
	}
	s := FormatDeduplicatedComment([]DeduplicatedError{d})
	if !strings.Contains(s, "| Deployment/d | spec.template.spec.containers[].securityContext | runAsNonRoot | a.yaml |") {
		t.Errorf("expected the single filename in the Overlays column, got: %q", s)
	}
}

func TestFormatDeduplicatedComment_MultiFileShowsCount(t *testing.T) {
	d := DeduplicatedError{
		Kind: "Deployment", Name: "d", MissingFields: []string{"runAsNonRoot"},
		Path:  "spec.template.spec.containers[].securityContext",
		Files: []string{"a.yaml", "b.yaml"}, Count: 2,
	}
	s := FormatDeduplicatedComment([]DeduplicatedError{d})
	if !strings.Contains(s, "| Deployment/d | spec.template.spec.containers[].securityContext | runAsNonRoot | 2 |") {
		t.Errorf("expected the overlay count in the Overlays column, got: %q", s)
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

func TestDeduplicate_MaxFilesCapping(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", MissingFields: []string{"x"}, Path: "p"},
		{File: "b.yaml", Kind: "Deployment", Name: "d", MissingFields: []string{"x"}, Path: "p"},
		{File: "c.yaml", Kind: "Deployment", Name: "d", MissingFields: []string{"x"}, Path: "p"},
	}
	ded := Deduplicate(errs, 2)
	if len(ded) != 1 || ded[0].Count != 3 || len(ded[0].Files) != 2 {
		t.Errorf("unexpected dedup: %+v", ded)
	}
}

func TestDeduplicate_DifferentContainerKeptSeparate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", Container: "c1", MissingFields: []string{"x"}, Path: "p"},
		{File: "a.yaml", Kind: "Deployment", Name: "d", Container: "c2", MissingFields: []string{"x"}, Path: "p"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 2 {
		t.Errorf("expected 2 separate dedup entries for different containers, got: %v", ded)
	}
}

func TestDeduplicate_DifferentPathKeptSeparate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", MissingFields: []string{"x"}, Path: "p1"},
		{File: "a.yaml", Kind: "Deployment", Name: "d", MissingFields: []string{"x"}, Path: "p2"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 2 {
		t.Errorf("expected 2 separate dedup entries for different paths, got: %v", ded)
	}
}

// --- testdata-fixture-driven tests ------------------------------------------

func TestValidateFile_GoodFixtures(t *testing.T) {
	for _, f := range []string{"testdata/good-deployment.yaml", "testdata/good-pod.yaml"} {
		t.Run(f, func(t *testing.T) {
			errs := ValidateFile(f)
			if len(errs) != 0 {
				t.Errorf("expected no findings for %s, got: %v", f, errs)
			}
		})
	}
}

func TestValidateFile_BadDeployment(t *testing.T) {
	// The fixture has no pod-level fields, no securityContext, and no
	// resources at all - 3 distinct findings.
	errs := ValidateFile("testdata/bad-deployment.yaml")
	if len(errs) != 3 {
		t.Fatalf("expected 3 findings (pod fields + container securityContext + resources), got %d: %v", len(errs), errs)
	}
	var sawPod, sawSC, sawRes bool
	for _, e := range errs {
		switch {
		case e.Container == "":
			sawPod = true
			for _, f := range RequiredPodFields {
				found := false
				for _, mf := range e.MissingFields {
					if mf == f {
						found = true
					}
				}
				if !found {
					t.Errorf("expected pod-level finding to list missing field %q, got: %v", f, e.MissingFields)
				}
			}
		case len(e.MissingFields) > 0 && strings.HasPrefix(e.MissingFields[0], "resources."):
			sawRes = true
			if len(e.MissingFields) != 2 || e.MissingFields[0] != "resources.requests" || e.MissingFields[1] != "resources.limits" {
				t.Errorf("expected both resources.requests and resources.limits missing, got: %v", e.MissingFields)
			}
		default:
			sawSC = true
			for _, f := range RequiredSecurityContextFields {
				found := false
				for _, mf := range e.MissingFields {
					if mf == f {
						found = true
					}
				}
				if !found {
					t.Errorf("expected container finding to list missing securityContext field %q, got: %v", f, e.MissingFields)
				}
			}
		}
	}
	if !sawPod || !sawSC || !sawRes {
		t.Errorf("expected pod-level, securityContext, and resources findings, got: %v", errs)
	}
}

func TestValidateFile_BadCronJob(t *testing.T) {
	errs := ValidateFile("testdata/bad-cronjob.yaml")
	if len(errs) == 0 {
		t.Fatal("expected findings for a noncompliant CronJob")
	}
	for _, e := range errs {
		if !strings.Contains(e.Path, "jobTemplate") {
			t.Errorf("expected path to reflect CronJob's nested pod spec, got: %q", e.Path)
		}
	}
}

func TestValidateFile_PartialStatefulSet(t *testing.T) {
	// Regression for change #1: resources.requests/limits are now checked
	// independently. This fixture has requests set but not limits, and a
	// securityContext missing exactly seccompProfile.
	errs := ValidateFile("testdata/partial-statefulset.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected exactly 2 findings, got %d: %v", len(errs), errs)
	}
	var sawResources, sawSC bool
	for _, e := range errs {
		switch {
		case len(e.MissingFields) == 1 && e.MissingFields[0] == "resources.limits":
			sawResources = true
		case len(e.MissingFields) == 1 && e.MissingFields[0] == "seccompProfile":
			sawSC = true
		default:
			t.Errorf("unexpected finding: %+v", e)
		}
	}
	if !sawResources {
		t.Errorf("expected a finding with exactly [resources.limits] missing, got: %v", errs)
	}
	if !sawSC {
		t.Errorf("expected a finding with exactly [seccompProfile] missing, got: %v", errs)
	}
}

func TestValidateFile_MissingInitContainerSecurityContext(t *testing.T) {
	// The initContainer has neither securityContext nor resources set -
	// closes the previously entirely-untested initContainers code path.
	errs := ValidateFile("testdata/missing-init-sc.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected exactly 2 findings (initContainer missing securityContext, and missing resources), got %d: %v", len(errs), errs)
	}
	for _, e := range errs {
		if !strings.Contains(e.Path, ".initContainers[") {
			t.Errorf("expected path to reference initContainers[], got: %q", e.Path)
		}
	}
}

func TestValidateFile_NonWorkload(t *testing.T) {
	errs := ValidateFile("testdata/non-workload.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for Service+ConfigMap, got: %v", errs)
	}
}

func TestValidateFile_MultiDoc(t *testing.T) {
	errs := ValidateFile("testdata/multi-doc.yaml")
	if len(errs) == 0 {
		t.Fatal("expected findings from both documents in the multi-doc stream")
	}
	kinds := map[string]bool{}
	for _, e := range errs {
		kinds[e.Kind] = true
	}
	if !kinds["Deployment"] || !kinds["Pod"] {
		t.Errorf("expected findings from both Deployment and Pod docs, got: %v", errs)
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	errs := ValidateFile("testdata/does-not-exist.yaml")
	if errs != nil {
		t.Errorf("expected nil for a nonexistent file, got: %v", errs)
	}
}
