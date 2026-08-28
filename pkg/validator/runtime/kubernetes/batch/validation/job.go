package validation

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// parallelismInvalidCheck validates that parallelism must be >= 0.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type parallelismInvalidCheck struct{}

func (c parallelismInvalidCheck) ID() string            { return "batch/parallelism-invalid" }
func (c parallelismInvalidCheck) Title() string         { return "Parallelism Must Be >= 0" }
func (c parallelismInvalidCheck) Category() string      { return "batch" }
func (c parallelismInvalidCheck) Blocking() bool        { return true }
func (c parallelismInvalidCheck) RenderSensitive() bool { return true }
func (c parallelismInvalidCheck) Kinds() []string       { return []string{"Job"} }

func (c parallelismInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "Job", "parallelism", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.Parallelism)
	})
}

// backoffLimitInvalidCheck validates that backoffLimit must be >= 0.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type backoffLimitInvalidCheck struct{}

func (c backoffLimitInvalidCheck) ID() string            { return "batch/backoff-limit-invalid" }
func (c backoffLimitInvalidCheck) Title() string         { return "BackoffLimit Must Be >= 0" }
func (c backoffLimitInvalidCheck) Category() string      { return "batch" }
func (c backoffLimitInvalidCheck) Blocking() bool        { return true }
func (c backoffLimitInvalidCheck) RenderSensitive() bool { return true }
func (c backoffLimitInvalidCheck) Kinds() []string       { return []string{"Job"} }

func (c backoffLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	// No backoffLimit means default (6), which is >= 0
	return nonNegativeIntFindings(c, data, "Job", "backoffLimit", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.BackoffLimit)
	})
}

// Register registers all Job validation checks with the check registry.
func Register() {
	checks := []runtime.Check{
		parallelismInvalidCheck{},
		backoffLimitInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
