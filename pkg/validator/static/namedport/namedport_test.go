package namedport

import (
	"os"
	"strings"
	"testing"
)

func readTestdata(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// --- inline regression tests (kept from before this PR) -------------------

func TestValidateReader_DeploymentUnnamedPort(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - name: c
        ports:
        - containerPort: 8080
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

func TestValidateReader_CronJobUnnamedPort(t *testing.T) {
	// Regression: CronJob nests its pod spec three levels deeper than
	// every other workload kind. Previously the hardcoded
	// "spec.template.spec" path never resolved for CronJob, so it was
	// silently never validated at all (zero findings, not an error).
	data := `kind: CronJob
metadata:
  name: cj
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: c
            ports:
            - containerPort: 8080
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for an unnamed CronJob container port, got: %v", errs)
	}
	if !strings.Contains(errs[0].Path, "jobTemplate") {
		t.Errorf("expected the reported path to reflect CronJob's nested pod spec, got: %q", errs[0].Path)
	}
}

func TestValidateReader_ContainerNoPortsNotFlagged(t *testing.T) {
	// Regression: a container with no ports list at all is normal, not a
	// violation. Only unnamed *existing* ports should be flagged.
	data := `kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - name: c
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a container with no ports, got: %v", errs)
	}
}

func TestValidateReader_DeploymentNumericProbe(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - name: c
        ports:
        - name: http
          containerPort: 8080
        livenessProbe:
          httpGet:
            port: 8080
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

func TestValidateReader_ServiceNumericTargetPort(t *testing.T) {
	data := `kind: Service
metadata:
  name: svc
spec:
  ports:
  - name: http
    port: 80
    targetPort: 8080
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

func TestValidateReader_ServiceTargetPort_CarriesValueAndAnnotations(t *testing.T) {
	data := `kind: Service
metadata:
  name: svc
  annotations:
    gitops-ci.k8s.io/exempt-named-ports: "8080"
spec:
  ports:
  - name: http
    port: 80
    targetPort: 8080
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
	if errs[0].Value != "8080" {
		t.Errorf("Value = %q, want %q", errs[0].Value, "8080")
	}
	if errs[0].Annotations["gitops-ci.k8s.io/exempt-named-ports"] != "8080" {
		t.Errorf("Annotations not carried through: %v", errs[0].Annotations)
	}
}

func TestValidateReader_IngressNumericPort(t *testing.T) {
	data := `kind: Ingress
metadata:
  name: ing
spec:
  rules:
  - host: x
    http:
      paths:
      - path: /
        backend:
          service:
            name: svc
            port:
              number: 80
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

// --- testdata-fixture-driven tests (this PR) -------------------------------

func TestValidateFile_GoodFixtures(t *testing.T) {
	for _, f := range []string{
		"testdata/good-deployment.yaml",
		"testdata/good-service.yaml",
		"testdata/invalid/good-ingress.yaml",
	} {
		t.Run(f, func(t *testing.T) {
			errs := ValidateFile(f)
			if len(errs) != 0 {
				t.Errorf("expected no findings for %s, got: %v", f, errs)
			}
		})
	}
}

func TestValidateFile_BadDeploymentNumericProbe(t *testing.T) {
	// Both httpGet and tcpSocket numeric ports in one container: 2 findings
	// (closes the previously-untested numeric-tcpSocket path).
	errs := ValidateFile("testdata/invalid/bad-deployment-numeric-probe.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_BadDeploymentUnnamedContainerPort(t *testing.T) {
	errs := ValidateFile("testdata/bad-deployment-unnamed-containerport.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding (missing name only), got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_BadServiceNumericTargetPort(t *testing.T) {
	// Isolates "numeric targetPort" from "missing name" - exactly 1 finding.
	errs := ValidateFile("testdata/invalid/bad-service-numeric-targetport.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_BadServiceUnnamedPort(t *testing.T) {
	// Isolates "missing name" from "numeric targetPort" - exactly 1 finding.
	errs := ValidateFile("testdata/invalid/bad-service-unnamed-port.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_BadIngressNumber(t *testing.T) {
	errs := ValidateFile("testdata/invalid/bad-ingress-number.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Path, "rules[].http.paths[") {
		t.Errorf("expected rules[].http.paths[] path, got %q", errs[0].Path)
	}
}

func TestValidateFile_BadIngressDefaultBackendNumber(t *testing.T) {
	errs := ValidateFile("testdata/invalid/bad-ingress-defaultbackend-number.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %v", len(errs), errs)
	}
	if errs[0].Path != "spec.defaultBackend.service.port.number" {
		t.Errorf("unexpected path: %q", errs[0].Path)
	}
}

func TestValidateFile_MultiDoc(t *testing.T) {
	// Findings from every document in the multi-doc stream.
	errs := ValidateFile("testdata/multi-doc.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 findings (one per doc), got %d: %v", len(errs), errs)
	}
}

func TestValidateBytes_NonTarget(t *testing.T) {
	data, err := readTestdata("testdata/invalid/non-target.yaml")
	if err != nil {
		t.Fatal(err)
	}
	errs := ValidateBytes(data, "non-target.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no findings for a ConfigMap, got: %v", errs)
	}
}

func TestValidateFile_InvalidFixturesYieldNoFindings(t *testing.T) {
	// False-positive-guard cases: each of these must yield exactly zero
	// findings even though they superficially resemble a violation.
	for _, f := range []string{
		"testdata/invalid/container-no-ports.yaml",
		"testdata/invalid/exec-probe.yaml",
		"testdata/invalid/externalname-service.yaml",
		"testdata/invalid/grpc-probe.yaml",
	} {
		t.Run(f, func(t *testing.T) {
			errs := ValidateFile(f)
			if len(errs) != 0 {
				t.Errorf("expected no findings for %s, got: %v", f, errs)
			}
		})
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	errs := ValidateFile("testdata/does-not-exist.yaml")
	if errs != nil {
		t.Errorf("expected nil for a nonexistent file, got: %v", errs)
	}
}

func TestValidateReader_Sanity(t *testing.T) {
	data, err := readTestdata("testdata/bad-deployment-unnamed-containerport.yaml")
	if err != nil {
		t.Fatal(err)
	}
	byBytes := ValidateBytes(data, "x.yaml")
	byReader := ValidateReader(strings.NewReader(string(data)), "x.yaml")
	if len(byBytes) != len(byReader) {
		t.Fatalf("ValidateBytes and ValidateReader disagree: %d vs %d", len(byBytes), len(byReader))
	}
}

// --- TrimSpace / whitespace-only name regressions --------------------------

func TestValidateReader_WhitespaceOnlyPortNameFlagged(t *testing.T) {
	data := `kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - name: c
        ports:
        - name: " "
          containerPort: 8080
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected a whitespace-only port name to be flagged as missing, got: %v", errs)
	}
}

func TestValidateReader_WhitespaceOnlyServicePortNameFlagged(t *testing.T) {
	data := `kind: Service
metadata:
  name: svc
spec:
  ports:
  - name: "  "
    port: 80
    targetPort: http
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected a whitespace-only service port name to be flagged as missing, got: %v", errs)
	}
}

// --- isNumericPort ----------------------------------------------------------

func TestIsNumericPort(t *testing.T) {
	data := `kind: Service
metadata:
  name: svc
spec:
  ports:
  - name: http
    port: 80
    targetPort: "  8080  "
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected a whitespace-padded numeric targetPort to still be flagged, got: %v", errs)
	}
}

func TestIsNumericPort_NonScalarNotNumeric(t *testing.T) {
	// A mapping/sequence node where a scalar is expected must not be
	// mistaken for numeric (Kind guard regression).
	data := `kind: Service
metadata:
  name: svc
spec:
  ports:
  - name: http
    port: 80
    targetPort:
      foo: bar
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no findings when targetPort is a mapping node, got: %v", errs)
	}
}

// --- Deduplicate / DeduplicatedError.String --------------------------------

func TestDeduplicate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", Issue: "missing name"},
		{File: "b.yaml", Kind: "Deployment", Name: "d", Issue: "missing name"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}

func TestDeduplicate_Empty(t *testing.T) {
	ded := Deduplicate(nil, 0)
	if len(ded) != 0 {
		t.Errorf("expected no dedup entries for empty input, got: %v", ded)
	}
}

func TestDeduplicate_DifferentIssueKeptSeparate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", Issue: "missing name"},
		{File: "a.yaml", Kind: "Deployment", Name: "d", Issue: "numeric port"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 2 {
		t.Fatalf("expected 2 separate dedup entries for different issues, got: %v", ded)
	}
}

func TestDeduplicate_DifferentContainerKeptSeparate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", Container: "c1", Issue: "missing name"},
		{File: "a.yaml", Kind: "Deployment", Name: "d", Container: "c2", Issue: "missing name"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 2 {
		t.Fatalf("expected 2 separate dedup entries for different containers, got: %v", ded)
	}
}

func TestDeduplicate_DifferentPathKeptSeparate(t *testing.T) {
	// Regression for change #1: the dedup key now includes Path, so two
	// findings that differ only by Path must not silently collapse into
	// one bucket.
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Service", Name: "svc", Issue: "missing name", Path: "spec.ports[0]"},
		{File: "a.yaml", Kind: "Service", Name: "svc", Issue: "missing name", Path: "spec.ports[1]"},
	}
	ded := Deduplicate(errs, 0)
	if len(ded) != 2 {
		t.Fatalf("expected 2 separate dedup entries for different paths, got: %v", ded)
	}
}

func TestDeduplicate_MaxFilesCapping(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", Kind: "Deployment", Name: "d", Issue: "missing name"},
		{File: "b.yaml", Kind: "Deployment", Name: "d", Issue: "missing name"},
		{File: "c.yaml", Kind: "Deployment", Name: "d", Issue: "missing name"},
	}
	ded := Deduplicate(errs, 2)
	if len(ded) != 1 {
		t.Fatalf("expected 1 dedup entry, got: %v", ded)
	}
	if ded[0].Count != 3 {
		t.Errorf("expected Count to track all 3 occurrences, got %d", ded[0].Count)
	}
	if len(ded[0].Files) != 2 {
		t.Errorf("expected Files capped at maxFiles=2, got %d: %v", len(ded[0].Files), ded[0].Files)
	}
}

func TestValidationError_String(t *testing.T) {
	e := ValidationError{
		File: "x.yaml", Kind: "Deployment", Name: "d", Container: "c",
		Path: "spec.template.spec.containers[0].ports[0]", Issue: "missing name",
	}
	s := e.String()
	if !strings.Contains(s, "c") || !strings.Contains(s, "missing name") || !strings.Contains(s, "spec.template.spec.containers[0].ports[0]") {
		t.Errorf("String() dropped a field: %q", s)
	}
}

func TestDeduplicatedError_String(t *testing.T) {
	// Regression for change #1: String() must render Container and Path,
	// not just Kind/Name/Issue/Count.
	d := DeduplicatedError{
		Kind: "Deployment", Name: "d", Container: "c",
		Path: "spec.template.spec.containers[0].ports[0]", Issue: "missing name",
		Files: []string{"x.yaml"}, Count: 2,
	}
	s := d.String()
	if !strings.Contains(s, "c") {
		t.Errorf("String() dropped Container: %q", s)
	}
	if !strings.Contains(s, "spec.template.spec.containers[0].ports[0]") {
		t.Errorf("String() dropped Path: %q", s)
	}
}

func TestDeduplicatedError_String_NoContainerNoPath(t *testing.T) {
	d := DeduplicatedError{Kind: "Service", Name: "svc", Issue: "missing name", Count: 1}
	s := d.String()
	if strings.Contains(s, "container") {
		t.Errorf("String() should not render a container clause when Container is empty: %q", s)
	}
	if strings.Contains(s, " at ") {
		t.Errorf("String() should not render a location clause when Path is empty: %q", s)
	}
}
