package validation

import (
	"testing"

	"sigs.k8s.io/yaml"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

func TestParallelismInvalidCheck(t *testing.T) {
	c := parallelismInvalidCheck{}

	tests := []struct {
		name        string
		kind        string
		parallelism *int32
		want        bool
	}{
		{"non-job", "Deployment", nil, false},
		{"nil parallelism", "Job", nil, false},
		{"zero parallelism", "Job", ptrInt32(0), false},
		{"positive parallelism", "Job", ptrInt32(4), false},
		{"negative parallelism", "Job", ptrInt32(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"parallelism": tt.parallelism,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestBackoffLimitInvalidCheck(t *testing.T) {
	c := backoffLimitInvalidCheck{}

	tests := []struct {
		name         string
		kind         string
		backoffLimit *int32
		want         bool
	}{
		{"non-job", "Deployment", nil, false},
		{"nil backoffLimit", "Job", nil, false},
		{"zero backoffLimit", "Job", ptrInt32(0), false},
		{"positive backoffLimit", "Job", ptrInt32(6), false},
		{"negative backoffLimit", "Job", ptrInt32(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"backoffLimit": tt.backoffLimit,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestScheduleInvalidCheck(t *testing.T) {
	c := scheduleInvalidCheck{}

	tests := []struct {
		name     string
		kind     string
		schedule string
		want     bool
	}{
		{"non-cronjob", "Job", "", false},
		{"empty schedule", "CronJob", "", false},
		{"valid schedule", "CronJob", "0 0 * * *", false},
		{"valid schedule with minute", "CronJob", "*/5 * * * *", false},
		{"valid schedule with range", "CronJob", "0 1-5 * * *", false},
		// Quarter-hourly. The values belong in the minute field: as hours,
		// 30 and 45 exceed the 0-23 range and the API server rejects the
		// schedule. The previous hand-rolled parser did not range-check
		// field values, so it accepted this invalid schedule.
		{"valid schedule with list", "CronJob", "0,15,30,45 * * * *", false},
		{"list value out of range for field", "CronJob", "0 0,15,30,45 * * *", true},
		{"invalid schedule - too few fields", "CronJob", "* *", true},
		{"invalid schedule - bad number", "CronJob", "abc 0 * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"schedule": tt.schedule,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestConcurrencyPolicyInvalidCheck(t *testing.T) {
	c := concurrencyPolicyInvalidCheck{}

	tests := []struct {
		name              string
		kind              string
		concurrencyPolicy string
		want              bool
	}{
		{"non-cronjob", "Job", "", false},
		{"empty", "CronJob", "", false},
		{"Allow", "CronJob", "Allow", false},
		{"Forbid", "CronJob", "Forbid", false},
		{"Replace", "CronJob", "Replace", false},
		{"invalid", "CronJob", "BadValue", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"concurrencyPolicy": tt.concurrencyPolicy,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestFailedJobsHistoryLimitInvalidCheck(t *testing.T) {
	c := failedJobsHistoryLimitInvalidCheck{}

	tests := []struct {
		name                   string
		kind                   string
		failedJobsHistoryLimit *int32
		want                   bool
	}{
		{"non-cronjob", "Job", nil, false},
		{"nil", "CronJob", nil, false},
		{"zero", "CronJob", ptrInt32(0), false},
		{"positive", "CronJob", ptrInt32(3), false},
		{"negative", "CronJob", ptrInt32(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"failedJobsHistoryLimit": tt.failedJobsHistoryLimit,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestSuccessfulJobsHistoryLimitInvalidCheck(t *testing.T) {
	c := successfulJobsHistoryLimitInvalidCheck{}

	tests := []struct {
		name                       string
		kind                       string
		successfulJobsHistoryLimit *int32
		want                       bool
	}{
		{"non-cronjob", "Job", nil, false},
		{"nil", "CronJob", nil, false},
		{"zero", "CronJob", ptrInt32(0), false},
		{"positive", "CronJob", ptrInt32(3), false},
		{"negative", "CronJob", ptrInt32(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"successfulJobsHistoryLimit": tt.successfulJobsHistoryLimit,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestStartingDeadlineSecondsInvalidCheck(t *testing.T) {
	c := startingDeadlineSecondsInvalidCheck{}

	tests := []struct {
		name                    string
		kind                    string
		startingDeadlineSeconds *int64
		want                    bool
	}{
		{"non-cronjob", "Job", nil, false},
		{"nil", "CronJob", nil, false},
		{"zero", "CronJob", ptrInt64(0), false},
		{"positive", "CronJob", ptrInt64(100), false},
		{"negative", "CronJob", ptrInt64(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"startingDeadlineSeconds": tt.startingDeadlineSeconds,
				},
			}
			bytes, _ := yaml.Marshal(data)
			findings := c.Run(bytes, "test.yaml")
			if tt.want && len(findings) == 0 {
				t.Error("expected findings, got none")
			}
			if !tt.want && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestCheckIDs(t *testing.T) {
	checks := []struct {
		id string
		c  runtime.Check
	}{
		{"batch/parallelism-invalid", parallelismInvalidCheck{}},
		{"batch/backoff-limit-invalid", backoffLimitInvalidCheck{}},
		{"batch/schedule-invalid", scheduleInvalidCheck{}},
		{"batch/concurrency-policy-invalid", concurrencyPolicyInvalidCheck{}},
		{"batch/failed-jobs-history-limit-invalid", failedJobsHistoryLimitInvalidCheck{}},
		{"batch/successful-jobs-history-limit-invalid", successfulJobsHistoryLimitInvalidCheck{}},
		{"batch/starting-deadline-seconds-invalid", startingDeadlineSecondsInvalidCheck{}},
	}

	for _, tc := range checks {
		t.Run(tc.id, func(t *testing.T) {
			if tc.c.ID() != tc.id {
				t.Errorf("ID mismatch: expected %q, got %q", tc.id, tc.c.ID())
			}
			if tc.c.Title() == "" {
				t.Error("Title must not be empty")
			}
			if tc.c.Category() == "" {
				t.Error("Category must not be empty")
			}
			if !tc.c.Blocking() {
				t.Error("Blocking must be true")
			}
			if !tc.c.RenderSensitive() {
				t.Error("RenderSensitive must be true")
			}
			if len(tc.c.Kinds()) == 0 {
				t.Error("Kinds must not be empty")
			}
		})
	}
}

func TestJobFindingsHaveCorrectFields(t *testing.T) {
	c := parallelismInvalidCheck{}
	data := map[string]interface{}{
		"kind": "Job",
		"spec": map[string]interface{}{
			"parallelism": -1,
		},
	}
	bytes, _ := yaml.Marshal(data)
	findings := c.Run(bytes, "test.yaml")

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.RuleID != "batch/parallelism-invalid" {
		t.Errorf("RuleID: expected 'batch/parallelism-invalid', got %q", f.RuleID)
	}
	if f.Kind != "Job" {
		t.Errorf("Kind: expected 'Job', got %q", f.Kind)
	}
	if f.Path != "spec.parallelism" {
		t.Errorf("Path: expected 'spec.parallelism', got %q", f.Path)
	}
}

func TestCronJobFindingsHaveCorrectFields(t *testing.T) {
	c := scheduleInvalidCheck{}
	data := map[string]interface{}{
		"kind": "CronJob",
		"metadata": map[string]interface{}{
			"name":      "test-cronjob",
			"namespace": "test-ns",
		},
		"spec": map[string]interface{}{
			"schedule": "invalid",
		},
	}
	bytes, _ := yaml.Marshal(data)
	findings := c.Run(bytes, "test.yaml")

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.RuleID != "batch/schedule-invalid" {
		t.Errorf("RuleID: expected 'batch/schedule-invalid', got %q", f.RuleID)
	}
	if f.Kind != "CronJob" {
		t.Errorf("Kind: expected 'CronJob', got %q", f.Kind)
	}
}

func ptrInt32(v int32) *int32 {
	return &v
}

func ptrInt64(v int64) *int64 {
	return &v
}

// TestCronScheduleAcceptsEverythingTheAPIServerAccepts covers the schedules
// the previous hand-rolled parser rejected. Each is valid to
// cron.ParseStandard, so the API server admits it; a finding here is an
// unsuppressible CI failure on a correct manifest, which is strictly worse
// than missing an invalid one.
func TestCronScheduleAcceptsEverythingTheAPIServerAccepts(t *testing.T) {
	valid := []struct{ name, schedule string }{
		{"standard 5-field", "0 0 * * *"},
		{"step values", "*/15 * * * *"},
		{"range", "0 9-17 * * *"},
		{"list", "0 0 1,15 * *"},
		{"symbolic weekday", "0 0 * * MON"},
		{"symbolic weekday range", "0 0 * * MON-FRI"},
		{"symbolic month", "0 0 1 JAN *"},
		{"descriptor daily", "@daily"},
		{"descriptor hourly", "@hourly"},
		{"descriptor weekly", "@weekly"},
		{"descriptor monthly", "@monthly"},
		{"descriptor yearly", "@yearly"},
		{"descriptor midnight", "@midnight"},
		{"every duration", "@every 1h30m"},
		{"timezone prefix", "TZ=UTC 0 0 * * *"},
		{"cron_tz prefix", "CRON_TZ=America/New_York 0 0 * * *"},
	}

	c := scheduleInvalidCheck{}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("kind: CronJob\nmetadata:\n  name: test\nspec:\n  schedule: \"" + tt.schedule + "\"\n")
			if findings := c.Run(data, "test.yaml"); len(findings) != 0 {
				t.Errorf("schedule %q is accepted by the API server but reported %d finding(s): %v",
					tt.schedule, len(findings), findings)
			}
		})
	}
}

// TestCronScheduleRejectsInvalid keeps the check from degrading into one
// that accepts anything.
func TestCronScheduleRejectsInvalid(t *testing.T) {
	invalid := []struct{ name, schedule string }{
		{"too few fields", "0 0 *"},
		{"nonsense", "not-a-schedule"},
		{"bad descriptor", "@sometimes"},
		{"minute out of range", "99 0 * * *"},
		{"bad symbolic name", "0 0 * * FUNDAY"},
	}

	c := scheduleInvalidCheck{}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("kind: CronJob\nmetadata:\n  name: test\nspec:\n  schedule: \"" + tt.schedule + "\"\n")
			if findings := c.Run(data, "test.yaml"); len(findings) != 1 {
				t.Errorf("schedule %q is rejected by the API server but produced %d finding(s)",
					tt.schedule, len(findings))
			}
		})
	}
}

// TestCronScheduleMalformedTZDoesNotPanic pins the panic-recovery half of
// the ported upstream helper. cron.ParseStandard panics on inputs like
// "TZ=0"; without recovery that would abort the entire validation run
// rather than report one bad schedule.
func TestCronScheduleMalformedTZDoesNotPanic(t *testing.T) {
	data := []byte("kind: CronJob\nmetadata:\n  name: test\nspec:\n  schedule: \"TZ=0\"\n")
	findings := scheduleInvalidCheck{}.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for a malformed TZ schedule, got %d", len(findings))
	}
}
