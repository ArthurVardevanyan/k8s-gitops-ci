package validation

import (
	"fmt"
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestConfigMapDataSizeExceeded_Check(t *testing.T) {
	pad := strings.Repeat("a", maxConfigMapSize+1)
	data := []byte(fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: %s`, pad))
	check := configMapDataSizeExceededCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/configmap-data-size-exceeded" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestConfigMapDataSizeExceeded_Check_UnderLimit(t *testing.T) {
	smallVal := string(make([]byte, maxConfigMapSize-100))
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: ` + smallVal)
	check := configMapDataSizeExceededCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestConfigMapDataSizeExceeded_Check_Empty(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`)
	check := configMapDataSizeExceededCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty data, got %d", len(findings))
	}
}

func TestConfigMapNameInvalid_Check(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
data:
  key: value
`)
	check := configMapNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/configmap-name-invalid" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Path != "metadata.name" {
		t.Errorf("unexpected path: %s", findings[0].Path)
	}
}

func TestConfigMapNameInvalid_Check_WithName(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: valid-name
data:
  key: value
`)
	check := configMapNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestConfigMapNameInvalid_Check_NonConfigMap(t *testing.T) {
	data := []byte(`kind: Pod`)
	check := configMapNameInvalidCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Pod, got %d", len(findings))
	}
}

func TestConfigMap_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		configMapDataSizeExceededCheck{},
		configMapNameInvalidCheck{},
	}
	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if c.Category() != "core" {
			t.Errorf("check %T has wrong category: %s", c, c.Category())
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.Kinds()) == 0 {
			t.Errorf("check %T should declare Kinds", c)
		}
	}
}
