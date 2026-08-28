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
	check := newConfigMapDataSizeExceededCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "core/configmap-data-size-exceeded" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestConfigMapDataSizeExceeded_Check_UnderLimit(t *testing.T) {
	// Printable filler, not make([]byte, n). A zeroed slice is NUL bytes,
	// which are not valid in YAML, so the document failed to parse and the
	// check returned nil - the test passed without ever reaching the size
	// comparison it exists to exercise.
	smallVal := strings.Repeat("a", maxConfigMapSize-100)
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: ` + smallVal)
	check := newConfigMapDataSizeExceededCheck()
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
	check := newConfigMapDataSizeExceededCheck()
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty data, got %d", len(findings))
	}
}

func TestConfigMap_Check_Interface(t *testing.T) {
	checks := []runtime.Check{
		newConfigMapDataSizeExceededCheck(),
	}
	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if runtime.CategoryOf(c.ID()) != "core" {
			t.Errorf("check %T has wrong category: %s", c, runtime.CategoryOf(c.ID()))
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
