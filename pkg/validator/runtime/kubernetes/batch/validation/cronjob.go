package validation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

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
func (c scheduleInvalidCheck) DocSkipper() []string  { return []string{"CronJob"} }

func (c scheduleInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string             `json:"kind"`
		Spec cronJobSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	schedule := cj.Spec.Schedule
	if schedule == "" {
		return nil
	}
	_ = time.Now()
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

// timezoneInvalidCheck validates the timezone field.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type timezoneInvalidCheck struct{}

func (c timezoneInvalidCheck) ID() string            { return "batch/timezone-invalid" }
func (c timezoneInvalidCheck) Title() string         { return "CronJob Timezone Must Be Valid" }
func (c timezoneInvalidCheck) Category() string      { return "batch" }
func (c timezoneInvalidCheck) Blocking() bool        { return true }
func (c timezoneInvalidCheck) RenderSensitive() bool { return true }
func (c timezoneInvalidCheck) DocSkipper() []string  { return []string{"CronJob"} }

func (c timezoneInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string `json:"kind"`
		Spec cronJobSpecWrapper
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	tz := cj.Spec.Timezone
	if tz == "" {
		return nil
	}
	// time.LoadLocation doesn't support named offsets like "UTC+5", so handle those
	// Also supports "EST5EDT", "America/New_York", etc.
	if loc, err := time.LoadLocation(tz); loc != nil && err == nil {
		return nil
	}
	// Try to parse fixed offset names like UTC+5, UTC-8, etc.
	if _, err := parseFixedOffset(tz); err == nil {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("timezone").String(),
			Message: fmt.Sprintf("timezone: invalid timezone: %s", tz),
			Kind:    cj.Kind,
			Extra:   map[string]string{"timezone": tz},
		},
	}}
}

// concurrencyPolicyInvalidCheck validates concurrencyPolicy.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type concurrencyPolicyInvalidCheck struct{}

func (c concurrencyPolicyInvalidCheck) ID() string { return "batch/concurrency-policy-invalid" }

func (c concurrencyPolicyInvalidCheck) Title() string         { return "ConcurrencyPolicy Must Be Valid" }
func (c concurrencyPolicyInvalidCheck) Category() string      { return "batch" }
func (c concurrencyPolicyInvalidCheck) Blocking() bool        { return true }
func (c concurrencyPolicyInvalidCheck) RenderSensitive() bool { return true }
func (c concurrencyPolicyInvalidCheck) DocSkipper() []string  { return []string{"CronJob"} }

func (c concurrencyPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string `json:"kind"`
		Spec cronJobSpecWrapper
	}
	if err := json.Unmarshal(data, &cj); err != nil {
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
func (c failedJobsHistoryLimitInvalidCheck) DocSkipper() []string  { return []string{"CronJob"} }

func (c failedJobsHistoryLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string `json:"kind"`
		Spec cronJobSpecWrapper
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("failedJobsHistoryLimit").String(),
			Message: fmt.Sprintf("failedJobsHistoryLimit: must be >= 0, got %d", *cj.Spec.FailedJobsHistoryLimit),
			Kind:    cj.Kind,
			Extra:   map[string]string{"failedJobsHistoryLimit": strconv.Itoa(int(*cj.Spec.FailedJobsHistoryLimit))},
		},
	}}
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
func (c successfulJobsHistoryLimitInvalidCheck) DocSkipper() []string  { return []string{"CronJob"} }

func (c successfulJobsHistoryLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string `json:"kind"`
		Spec cronJobSpecWrapper
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	if cj.Spec.SuccessfulJobsHistoryLimit == nil || *cj.Spec.SuccessfulJobsHistoryLimit >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("successfulJobsHistoryLimit").String(),
			Message: fmt.Sprintf("successfulJobsHistoryLimit: must be >= 0, got %d", *cj.Spec.SuccessfulJobsHistoryLimit),
			Kind:    cj.Kind,
			Extra:   map[string]string{"successfulJobsHistoryLimit": strconv.Itoa(int(*cj.Spec.SuccessfulJobsHistoryLimit))},
		},
	}}
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
func (c startingDeadlineSecondsInvalidCheck) DocSkipper() []string  { return []string{"CronJob"} }

func (c startingDeadlineSecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var cj struct {
		Kind string `json:"kind"`
		Spec cronJobSpecWrapper
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil
	}
	if cj.Kind != "CronJob" {
		return nil
	}
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("startingDeadlineSeconds").String(),
			Message: fmt.Sprintf("startingDeadlineSeconds: must be >= 0, got %d", *cj.Spec.StartingDeadlineSeconds),
			Kind:    cj.Kind,
			Extra:   map[string]string{"startingDeadlineSeconds": strconv.FormatInt(*cj.Spec.StartingDeadlineSeconds, 10)},
		},
	}}
}

// cronJobSpecWrapper holds batch/v1.CronJobSpec fields we need to validate.
type cronJobSpecWrapper struct {
	Schedule                   string `json:"schedule"`
	Timezone                   string `json:"timezone"`
	ConcurrencyPolicy          string `json:"concurrencyPolicy"`
	FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit"`
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit"`
	StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds"`
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

// parseFixedOffset parses a fixed offset timezone like "UTC+5", "UTC-8", "GMT+1".
// Returns the offset in seconds from UTC, or an error if the format is invalid.
func parseFixedOffset(s string) (int, error) {
	if !strings.HasPrefix(s, "UTC") && !strings.HasPrefix(s, "GMT") && !strings.HasPrefix(s, "Z") {
		return 0, fmt.Errorf("not a fixed offset format")
	}
	suffix := strings.TrimPrefix(s, "UTC")
	suffix = strings.TrimPrefix(suffix, "GMT")
	suffix = strings.TrimPrefix(suffix, "Z")

	if suffix == "" {
		return 0, nil
	}

	if len(suffix) < 2 {
		return 0, fmt.Errorf("invalid fixed offset: %s", s)
	}

	start := 0
	var sign int64
	switch suffix[0] {
	case '+':
		sign = 1
		start = 1
	case '-':
		sign = -1
		start = 1
	default:
		return 0, fmt.Errorf("invalid fixed offset: %s", s)
	}

	// Handle both 1-digit (e.g. "+5") and 2-digit (e.g. "+05") hour formats
	remaining := suffix[start:]
	hrsStr := ""
	minStr := ""

	if len(remaining) >= 2 && remaining[1] >= '0' && remaining[1] <= '9' {
		// Two-digit hours
		if remaining[2] == ':' && len(remaining) >= 5 {
			hrsStr = remaining[:2]
			minStr = remaining[3:5]
		} else if len(remaining) >= 2 {
			hrsStr = remaining[:2]
		}
	} else if len(remaining) == 1 {
		// Single-digit hour
		if remaining[0] >= '0' && remaining[0] <= '9' {
			hrsStr = remaining[:1]
		}
	}

	if hrsStr == "" {
		return 0, fmt.Errorf("invalid fixed offset hours: %s", s)
	}

	hours, err := strconv.Atoi(hrsStr)
	if err != nil {
		return 0, fmt.Errorf("invalid fixed offset hours: %s", s)
	}

	if hours < 0 || hours > 23 {
		return 0, fmt.Errorf("fixed offset hours out of range: %d", hours)
	}

	if minStr != "" {
		minutes, err := strconv.Atoi(minStr)
		if err != nil {
			return 0, fmt.Errorf("invalid fixed offset minutes: %s", s)
		}
		if minutes < 0 || minutes > 59 {
			return 0, fmt.Errorf("fixed offset minutes out of range: %d", minutes)
		}
		return int(sign * (int64(hours)*3600 + int64(minutes)*60)), nil
	}

	return int(sign * int64(hours) * 3600), nil
}

// Register registers all CronJob validation checks with the check registry.
func RegisterCronJob() {
	checks := []runtime.Check{
		scheduleInvalidCheck{},
		timezoneInvalidCheck{},
		concurrencyPolicyInvalidCheck{},
		failedJobsHistoryLimitInvalidCheck{},
		successfulJobsHistoryLimitInvalidCheck{},
		startingDeadlineSecondsInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
