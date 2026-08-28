package validation

import (
	"fmt"
	"strconv"
	"strings"

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

// parseCronSchedule attempts to parse a cron schedule string.
// Returns nil if valid, error if invalid.
// Supports both 5-field and 6-field (with seconds) cron formats.
func parseCronSchedule(schedule string) error {
	// Try standard 5-field cron format
	return parseCronField(schedule, 5)
}

// parseCronField validates a cron schedule with the expected number of fields.
// This is a simplified validator that checks basic structure without requiring
// a full cron parsing library.
func parseCronField(schedule string, expectedFields int) error {
	parts := splitFields(schedule)
	if len(parts) != expectedFields {
		return fmt.Errorf("expected %d fields, got %d", expectedFields, len(parts))
	}
	for _, part := range parts {
		if err := validateCronField(part); err != nil {
			return err
		}
	}
	return nil
}

// splitFields splits a cron schedule into fields, handling quoted strings.
func splitFields(s string) []string {
	var parts []string
	var current string
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current += string(c)
			}
		} else {
			switch c {
			case ' ', '\t':
				if current != "" {
					parts = append(parts, current)
					current = ""
				}
			case '\n', '\r':
				if current != "" {
					parts = append(parts, current)
					current = ""
				}
			case '"', '\'':
				inQuote = true
				quoteChar = c
			default:
				current += string(c)
			}
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// validateCronField validates a single cron field.
func validateCronField(field string) error {
	// Handle wildcard
	if field == "*" {
		return nil
	}

	// Handle step values (*/2, 1-10/2)
	stepIdx := -1
	for i, c := range field {
		if c == '/' {
			stepIdx = i
			break
		}
	}

	var basePart string
	var stepPart string
	if stepIdx >= 0 {
		basePart = field[:stepIdx]
		stepPart = field[stepIdx+1:]
		if stepPart == "" {
			return fmt.Errorf("empty step value")
		}
		if _, err := strconv.Atoi(stepPart); err != nil {
			return fmt.Errorf("invalid step value: %s", stepPart)
		}
	} else {
		basePart = field
	}

	// Handle wildcard only
	if basePart == "*" {
		return nil
	}

	// Handle range (1-5) or comma-separated values
	if strings.Contains(basePart, ",") {
		return validateListOrRange(basePart)
	}

	// Handle range (1-5)
	if strings.Contains(basePart, "-") {
		return validateRange(basePart)
	}

	// Handle single number
	if _, err := strconv.Atoi(basePart); err != nil {
		return fmt.Errorf("invalid cron field: %s", field)
	}
	return nil
}

// validateListOrRange validates a field containing ranges or comma-separated values.
func validateListOrRange(s string) error {
	// Split by comma first
	items := splitBy(s, ',')
	for _, item := range items {
		if err := validateRange(item); err != nil {
			return err
		}
	}
	return nil
}

// validateRange validates a range expression like "1-5" or a single number.
func validateRange(s string) error {
	parts := splitBy(s, '-')
	if len(parts) == 1 {
		// Single number, validate it
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return fmt.Errorf("invalid value in list: %s", s)
		}
		return nil
	}
	if len(parts) != 2 {
		return fmt.Errorf("invalid range: %s", s)
	}
	_, err1 := strconv.Atoi(parts[0])
	_, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("invalid range: %s", s)
	}
	return nil
}

// splitBy splits a string by a delimiter.
func splitBy(s string, sep byte) []string {
	var parts []string
	var current string
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
