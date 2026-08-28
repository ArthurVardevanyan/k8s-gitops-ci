package validation

import (
	"fmt"

	"github.com/robfig/cron/v3"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// scheduleInvalidCheck validates the cron schedule field.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type scheduleInvalidCheck struct{ runtime.Meta }

func newScheduleInvalidCheck() scheduleInvalidCheck {
	return scheduleInvalidCheck{runtime.Meta{
		RuleID:    "batch/schedule-invalid",
		RuleTitle: "CronJob Schedule Must Be Valid",
		AppliesTo: []string{"CronJob"},
	}}
}

func (c scheduleInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string             `json:"kind"`
		Spec cronJobSpecWrapper `json:"spec"`
	}
	if err := yaml.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	schedule := cj.Spec.Schedule
	if schedule == "" {
		return nil
	}
	if err := parseCronSchedule(schedule); err != nil {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("schedule").String(),
				Message: fmt.Sprintf("schedule: invalid cron schedule: %s", err.Error()),
				Kind:    cj.Kind,
				Extra:   map[string]string{"schedule": schedule},
			},
		}}
	}
	return nil
}

// concurrencyPolicyInvalidCheck validates concurrencyPolicy.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type concurrencyPolicyInvalidCheck struct{ runtime.Meta }

func newConcurrencyPolicyInvalidCheck() concurrencyPolicyInvalidCheck {
	return concurrencyPolicyInvalidCheck{runtime.Meta{
		RuleID:    "batch/concurrency-policy-invalid",
		RuleTitle: "ConcurrencyPolicy Must Be Valid",
		AppliesTo: []string{"CronJob"},
	}}
}

func (c concurrencyPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string `json:"kind"`
		Spec cronJobSpecWrapper
	}
	if err := yaml.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	policy := cj.Spec.ConcurrencyPolicy
	if policy == "Allow" || policy == "Forbid" || policy == "Replace" || policy == "" {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("concurrencyPolicy").String(),
			Message: fmt.Sprintf("concurrencyPolicy: Unsupported value: %q", string(policy)),
			Kind:    cj.Kind,
			Extra:   map[string]string{"concurrencyPolicy": string(policy)},
		},
	}}
}

// failedJobsHistoryLimitInvalidCheck validates failedJobsHistoryLimit.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type failedJobsHistoryLimitInvalidCheck struct{ runtime.Meta }

func newFailedJobsHistoryLimitInvalidCheck() failedJobsHistoryLimitInvalidCheck {
	return failedJobsHistoryLimitInvalidCheck{runtime.Meta{
		RuleID:    "batch/failed-jobs-history-limit-invalid",
		RuleTitle: "FailedJobsHistoryLimit Must Be >= 0",
		AppliesTo: []string{"CronJob"},
	}}
}

func (c failedJobsHistoryLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "CronJob", "failedJobsHistoryLimit", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.FailedJobsHistoryLimit)
	})
}

// successfulJobsHistoryLimitInvalidCheck validates successfulJobsHistoryLimit.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type successfulJobsHistoryLimitInvalidCheck struct{ runtime.Meta }

func newSuccessfulJobsHistoryLimitInvalidCheck() successfulJobsHistoryLimitInvalidCheck {
	return successfulJobsHistoryLimitInvalidCheck{runtime.Meta{
		RuleID:    "batch/successful-jobs-history-limit-invalid",
		RuleTitle: "SuccessfulJobsHistoryLimit Must Be >= 0",
		AppliesTo: []string{"CronJob"},
	}}
}

func (c successfulJobsHistoryLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "CronJob", "successfulJobsHistoryLimit", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.SuccessfulJobsHistoryLimit)
	})
}

// startingDeadlineSecondsInvalidCheck validates startingDeadlineSeconds.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type startingDeadlineSecondsInvalidCheck struct{ runtime.Meta }

func newStartingDeadlineSecondsInvalidCheck() startingDeadlineSecondsInvalidCheck {
	return startingDeadlineSecondsInvalidCheck{runtime.Meta{
		RuleID:    "batch/starting-deadline-seconds-invalid",
		RuleTitle: "StartingDeadlineSeconds Must Be >= 0",
		AppliesTo: []string{"CronJob"},
	}}
}

func (c startingDeadlineSecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "CronJob", "startingDeadlineSeconds", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int64Value(spec.StartingDeadlineSeconds)
	})
}

// cronJobSpecWrapper holds batch/v1.CronJobSpec fields we need to validate.
// The numeric "must be >= 0" fields live in nonNegativeSpecWrapper.
type cronJobSpecWrapper struct {
	Schedule          string `json:"schedule"`
	ConcurrencyPolicy string `json:"concurrencyPolicy"`
}

// parseCronSchedule parses a cron schedule exactly as the API server does.
//
// Upstream's validateScheduleFormat calls
// k8s.io/kubernetes/pkg/util/parsers.ParseCronScheduleWithPanicRecovery,
// which is cron.ParseStandard from github.com/robfig/cron/v3 at the
// revision Kubernetes pins, wrapped in a panic recovery. Calling the same
// parser is what makes this a port rather than an approximation: a
// hand-written one rejects the symbolic names (MON-FRI, JAN), the
// @daily/@hourly descriptors and the TZ=/CRON_TZ= prefix that
// ParseStandard accepts, and a non-exemptable check cannot afford to
// reject a schedule the cluster admits.
//
// The recovery is ported behavior, not caution: ParseStandard panics on
// some malformed input (upstream cites "TZ=0"), which would otherwise
// abort the whole run instead of reporting one bad schedule.
func parseCronSchedule(schedule string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid schedule: %v", r)
		}
	}()
	_, err = cron.ParseStandard(schedule)
	return err
}
