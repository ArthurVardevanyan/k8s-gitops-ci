package validation

import (
	"fmt"
	"strings"
	"testing"
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
