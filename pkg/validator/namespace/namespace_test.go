package namespace

import (
	"strings"
	"testing"
)

func TestValidateBytes_MissingNamespace(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
`)
	errs := ValidateBytes(data, "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
	want := "test.yaml: Deployment/my-deploy: namespace-scoped resource missing metadata.namespace"
	if errs[0].String() != want {
		t.Errorf("got %q want %q", errs[0].String(), want)
	}
}

func TestValidateBytes_WithNamespace(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: Service
metadata:
  name: svc
  namespace: ns
`)
	errs := ValidateBytes(data, "test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateBytes_ClusterScopedResource(t *testing.T) {
	data := []byte(`kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: cr
`)
	errs := ValidateBytes(data, "test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for cluster scoped: %v", errs)
	}
}

func TestValidateBytes_KustomizationResource(t *testing.T) {
	data := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - base
`)
	errs := ValidateBytes(data, "kustomization.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for Kustomization: %v", errs)
	}
}

func TestValidateBytes_UnknownResource(t *testing.T) {
	data := []byte(`kind: Widget
apiVersion: custom.example.com/v1
metadata:
  name: w
`)
	errs := ValidateBytes(data, "test.yaml")
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "unknown resource") {
		t.Fatalf("expected unknown resource error: %v", errs)
	}
}

func TestValidateFile_Nonexistent(t *testing.T) {
	if got := ValidateFile("/tmp/does-not-exist-ns.lkj"); got != nil {
		t.Errorf("expected nil: %v", got)
	}
}

func TestDeduplicate(t *testing.T) {
	errs := []ValidationError{
		{Kind: "Deployment", Name: "d", Message: "missing ns"},
		{Kind: "Deployment", Name: "d", Message: "missing ns"},
	}
	ded := Deduplicate(errs)
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}
