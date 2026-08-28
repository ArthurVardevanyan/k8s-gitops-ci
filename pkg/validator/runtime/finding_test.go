package runtime

import (
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// stubCheck is a minimal runtime.Check used to exercise the adapter.
type stubCheck struct {
	kinds []string
}

func (s stubCheck) ID() string            { return "stub/check" }
func (s stubCheck) Title() string         { return "Stub Check" }
func (s stubCheck) Category() string      { return "stub" }
func (s stubCheck) Blocking() bool        { return true }
func (s stubCheck) RenderSensitive() bool { return true }
func (s stubCheck) Kinds() []string       { return s.kinds }
func (s stubCheck) Run([]byte, string) []Finding {
	return nil
}

// TestAdapterImplementsDocSkipper guards a regression that silently disabled
// kind filtering for every runtime check: the adapter previously exposed a
// `DocSkipper() []string` method, which satisfies no interface at all, so
// check.DocSkipper was never detected and every check was handed every
// document. The interface is satisfied structurally, so only a compile-time
// assertion catches this.
func TestAdapterImplementsDocSkipper(t *testing.T) {
	c := CheckToRegistered(stubCheck{kinds: []string{"Pod"}})
	if _, ok := c.(check.DocSkipper); !ok {
		t.Fatal("adapter must implement check.DocSkipper, otherwise Kinds() filtering is silently ignored")
	}
}

func TestAdapterSkipDoc(t *testing.T) {
	tests := []struct {
		name  string
		kinds []string
		kind  string
		want  bool
	}{
		{name: "declared kind is not skipped", kinds: []string{"Pod", "Deployment"}, kind: "Pod", want: false},
		{name: "other declared kind is not skipped", kinds: []string{"Pod", "Deployment"}, kind: "Deployment", want: false},
		{name: "undeclared kind is skipped", kinds: []string{"Pod"}, kind: "ConfigMap", want: true},
		{name: "empty kinds applies to everything", kinds: nil, kind: "ConfigMap", want: false},
		{name: "kind matching is exact", kinds: []string{"Pod"}, kind: "pod", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CheckToRegistered(stubCheck{kinds: tt.kinds})
			skipper, ok := c.(check.DocSkipper)
			if !ok {
				t.Fatal("adapter does not implement check.DocSkipper")
			}
			if got := skipper.SkipDoc(tt.kind); got != tt.want {
				t.Errorf("SkipDoc(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

// TestAdapterIsNonExemptable pins the contract documented on Finding: runtime
// findings describe manifests the API server itself rejects, so no exemption
// annotation or EXEMPTIONS=(...) selector may suppress them.
func TestAdapterIsNonExemptable(t *testing.T) {
	c := CheckToRegistered(stubCheck{})
	ne, ok := c.(check.NonExemptable)
	if !ok {
		t.Fatal("adapter must implement check.NonExemptable")
	}
	if !ne.NonExemptable() {
		t.Error("runtime checks must report NonExemptable() == true")
	}
}
