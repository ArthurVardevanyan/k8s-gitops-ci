package validation

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// nonNegativeSpecWrapper holds every batch spec field validated by the shared
// "must be >= 0" checks. Only the field selected by the calling check is read,
// so the same wrapper serves both Job and CronJob.
type nonNegativeSpecWrapper struct {
	Parallelism                *int32 `json:"parallelism"`
	BackoffLimit               *int32 `json:"backoffLimit"`
	FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit"`
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit"`
	StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds"`
}

// nonNegativeIntFindings reports a finding when the named spec field of an
// object of the given kind is set to a negative value. The batch checks for
// parallelism, backoffLimit, failedJobsHistoryLimit,
// successfulJobsHistoryLimit and startingDeadlineSeconds all share this shape
// and differ only in kind, field name and value accessor.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
func nonNegativeIntFindings(
	c runtime.Check,
	data []byte,
	kind string,
	fieldName string,
	value func(nonNegativeSpecWrapper) (int64, bool),
) []runtime.Finding {
	var obj struct {
		Kind string                 `json:"kind"`
		Spec nonNegativeSpecWrapper `json:"spec"`
	}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil
	}
	if obj.Kind != kind {
		return nil
	}
	val, ok := value(obj.Spec)
	if !ok || val >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child(fieldName).String(),
			Message: fmt.Sprintf("%s: must be >= 0, got %d", fieldName, val),
			Kind:    obj.Kind,
			Extra:   map[string]string{fieldName: strconv.FormatInt(val, 10)},
		},
	}}
}

// int32Value adapts an optional int32 spec field to the accessor signature
// expected by nonNegativeIntFindings.
func int32Value(v *int32) (int64, bool) {
	if v == nil {
		return 0, false
	}
	return int64(*v), true
}

// int64Value adapts an optional int64 spec field to the accessor signature
// expected by nonNegativeIntFindings.
func int64Value(v *int64) (int64, bool) {
	if v == nil {
		return 0, false
	}
	return *v, true
}
