package batch

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// parallelismInvalidCheck validates that parallelism must be >= 0.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type parallelismInvalidCheck struct{ runtime.Meta }

func newParallelismInvalidCheck() parallelismInvalidCheck {
	return parallelismInvalidCheck{runtime.Meta{
		RuleID:    "batch/parallelism-invalid",
		RuleTitle: "Parallelism Must Be >= 0",
		AppliesTo: []string{"Job"},
	}}
}

func (c parallelismInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "Job", "parallelism", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.Parallelism)
	})
}

// backoffLimitInvalidCheck validates that backoffLimit must be >= 0.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type backoffLimitInvalidCheck struct{ runtime.Meta }

func newBackoffLimitInvalidCheck() backoffLimitInvalidCheck {
	return backoffLimitInvalidCheck{runtime.Meta{
		RuleID:    "batch/backoff-limit-invalid",
		RuleTitle: "BackoffLimit Must Be >= 0",
		AppliesTo: []string{"Job"},
	}}
}

func (c backoffLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	// No backoffLimit means default (6), which is >= 0
	return nonNegativeIntFindings(c, data, "Job", "backoffLimit", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.BackoffLimit)
	})
}
