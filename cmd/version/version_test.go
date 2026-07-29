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
}
