package version

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	out := String()
	if !strings.Contains(out, "gitops-ci version") {
		t.Errorf("unexpected version string: %s", out)
	}
	if !strings.Contains(out, "commit") {
		t.Errorf("expected String() to contain commit metadata, got: %s", out)
	}
}

func TestShort(t *testing.T) {
	out := Short()
	if !strings.Contains(out, "gitops-ci version") {
		t.Errorf("unexpected version string: %s", out)
	}
	if strings.Contains(out, "commit") || strings.Contains(out, "built") {
		t.Errorf("expected Short() to omit commit/built metadata, got: %s", out)
	}
}
