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
type scheduleInvalidCheck struct{}

func (c scheduleInvalidCheck) ID() string            { return "batch/schedule-invalid" }
func (c scheduleInvalidCheck) Title() string         { return "CronJob Schedule Must Be Valid" }
func (c scheduleInvalidCheck) Category() string      { return "batch" }
func (c scheduleInvalidCheck) Blocking() bool        { return true }
func (c scheduleInvalidCheck) RenderSensitive() bool { return true }
func (c scheduleInvalidCheck) Kinds() []string       { return []string{"CronJob"} }

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
			Category:  c.Category(),
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
type concurrencyPolicyInvalidCheck struct{}

func (c concurrencyPolicyInvalidCheck) ID() string { return "batch/concurrency-policy-invalid" }

func (c concurrencyPolicyInvalidCheck) Title() string         { return "ConcurrencyPolicy Must Be Valid" }
func (c concurrencyPolicyInvalidCheck) Category() string      { return "batch" }
func (c concurrencyPolicyInvalidCheck) Blocking() bool        { return true }
func (c concurrencyPolicyInvalidCheck) RenderSensitive() bool { return true }
func (c concurrencyPolicyInvalidCheck) Kinds() []string       { return []string{"CronJob"} }

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
		Category:  c.Category(),
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
type failedJobsHistoryLimitInvalidCheck struct{}

func (c failedJobsHistoryLimitInvalidCheck) ID() string {
	return "batch/failed-jobs-history-limit-invalid"
}

func (c failedJobsHistoryLimitInvalidCheck) Title() string {
	return "FailedJobsHistoryLimit Must Be >= 0"
}
func (c failedJobsHistoryLimitInvalidCheck) Category() string      { return "batch" }
func (c failedJobsHistoryLimitInvalidCheck) Blocking() bool        { return true }
func (c failedJobsHistoryLimitInvalidCheck) RenderSensitive() bool { return true }
func (c failedJobsHistoryLimitInvalidCheck) Kinds() []string       { return []string{"CronJob"} }

func (c failedJobsHistoryLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "CronJob", "failedJobsHistoryLimit", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.FailedJobsHistoryLimit)
	})
}

// successfulJobsHistoryLimitInvalidCheck validates successfulJobsHistoryLimit.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type successfulJobsHistoryLimitInvalidCheck struct{}

func (c successfulJobsHistoryLimitInvalidCheck) ID() string {
	return "batch/successful-jobs-history-limit-invalid"
}

func (c successfulJobsHistoryLimitInvalidCheck) Title() string {
	return "SuccessfulJobsHistoryLimit Must Be >= 0"
}
func (c successfulJobsHistoryLimitInvalidCheck) Category() string      { return "batch" }
func (c successfulJobsHistoryLimitInvalidCheck) Blocking() bool        { return true }
func (c successfulJobsHistoryLimitInvalidCheck) RenderSensitive() bool { return true }
func (c successfulJobsHistoryLimitInvalidCheck) Kinds() []string       { return []string{"CronJob"} }

func (c successfulJobsHistoryLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nonNegativeIntFindings(c, data, "CronJob", "successfulJobsHistoryLimit", func(spec nonNegativeSpecWrapper) (int64, bool) {
		return int32Value(spec.SuccessfulJobsHistoryLimit)
	})
}

// startingDeadlineSecondsInvalidCheck validates startingDeadlineSeconds.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type startingDeadlineSecondsInvalidCheck struct{}

func (c startingDeadlineSecondsInvalidCheck) ID() string {
	return "batch/starting-deadline-seconds-invalid"
}

func (c startingDeadlineSecondsInvalidCheck) Title() string {
	return "StartingDeadlineSeconds Must Be >= 0"
}
func (c startingDeadlineSecondsInvalidCheck) Category() string      { return "batch" }
func (c startingDeadlineSecondsInvalidCheck) Blocking() bool        { return true }
func (c startingDeadlineSecondsInvalidCheck) RenderSensitive() bool { return true }
func (c startingDeadlineSecondsInvalidCheck) Kinds() []string       { return []string{"CronJob"} }

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
// which is cron.ParseStandard from github.com/robfig/cron/v3 wrapped in a
// panic recovery. This calls the same parser at the same version
// Kubernetes pins, so the rule is a genuine 1:1 port rather than an
// approximation of one.
//
// It previously used a hand-rolled 5-field structural parser. That parser
// was documented as accepting a superset of what the API server accepts -
// "never stricter" - but it was not: it resolved every field with
// strconv.Atoi, so the symbolic names cron.ParseStandard supports (MON-SUN,
// JAN-DEC) were rejected, as were the @daily/@hourly descriptors and the
// TZ=/CRON_TZ= prefix. Those are valid schedules the cluster accepts, and
// this check is non-exemptable, so each was an unsuppressible failure on a
// correct manifest.
//
// The panic recovery is part of the ported behavior, not defensive
// programming: cron.ParseStandard panics on some malformed input (upstream
// cites "TZ=0"), and a panic here would take down the whole run instead of
// reporting one invalid schedule.
func parseCronSchedule(schedule string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid schedule: %v", r)
		}
	}()
	_, err = cron.ParseStandard(schedule)
	return err
}
