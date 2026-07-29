package namedport

import (
	"strings"
	"testing"
)

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
