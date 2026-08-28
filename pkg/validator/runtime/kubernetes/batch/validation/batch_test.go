package validation

import (
	"encoding/json"
	"testing"

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
			bytes, _ := json.Marshal(data)
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

func TestCompletionsInvalidCheck(t *testing.T) {
	c := completionsInvalidCheck{}

	tests := []struct {
		name        string
		kind        string
		parallelism *int32
		completions *int32
		want        bool
	}{
		{"non-job", "Deployment", nil, nil, false},
		{"nil parallelism, nil completions", "Job", nil, nil, false},
		{"parallelism 1, nil completions", "Job", ptrInt32(1), nil, false},
		{"parallelism 4, completions 4", "Job", ptrInt32(4), ptrInt32(4), false},
		{"parallelism 4, completions 8", "Job", ptrInt32(4), ptrInt32(8), false},
		{"parallelism 4, completions 3", "Job", ptrInt32(4), ptrInt32(3), true},
		{"default parallelism 1, completions 0", "Job", nil, ptrInt32(0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"parallelism": tt.parallelism,
					"completions": tt.completions,
				},
			}
			bytes, _ := json.Marshal(data)
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

func TestActiveDeadlineSecondsInvalidCheck(t *testing.T) {
	c := activeDeadlineSecondsInvalidCheck{}

	tests := []struct {
		name string
		kind string
		ads  *int64
		want bool
	}{
		{"non-job", "Deployment", nil, false},
		{"nil ads", "Job", nil, false},
		{"ads = 1", "Job", ptrInt64(1), false},
		{"ads = 3600", "Job", ptrInt64(3600), false},
		{"ads = 0", "Job", ptrInt64(0), true},
		{"ads = -1", "Job", ptrInt64(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"activeDeadlineSeconds": tt.ads,
				},
			}
			bytes, _ := json.Marshal(data)
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
			bytes, _ := json.Marshal(data)
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

func TestTTLAfterFinishedInvalidCheck(t *testing.T) {
	c := ttlAfterFinishedInvalidCheck{}

	tests := []struct {
		name             string
		kind             string
		ttlAfterFinished *int32
		want             bool
	}{
		{"non-job", "Deployment", nil, false},
		{"nil ttlAfterFinished", "Job", nil, false},
		{"zero ttlAfterFinished", "Job", ptrInt32(0), false},
		{"positive ttlAfterFinished", "Job", ptrInt32(300), false},
		{"negative ttlAfterFinished", "Job", ptrInt32(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"ttlAfterFinished": tt.ttlAfterFinished,
				},
			}
			bytes, _ := json.Marshal(data)
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

func TestCompletionsModeInvalidCheck(t *testing.T) {
	c := completionsModeInvalidCheck{}

	tests := []struct {
		name            string
		kind            string
		completionsMode string
		want            bool
	}{
		{"non-job", "Deployment", "", false},
		{"empty completionsMode", "Job", "", false},
		{"NonIndexed", "Job", "NonIndexed", false},
		{"Indexed", "Job", "Indexed", false},
		{"invalid value", "Job", "BadValue", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"completionsMode": tt.completionsMode,
				},
			}
			bytes, _ := json.Marshal(data)
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

func TestIndexFormatInvalidCheck(t *testing.T) {
	c := indexFormatInvalidCheck{}

	tests := []struct {
		name            string
		kind            string
		completionsMode string
		labels          map[string]string
		want            bool
	}{
		{"non-job", "Deployment", "", nil, false},
		{"non-indexed", "Job", "NonIndexed", nil, false},
		{"indexed with label", "Job", "Indexed", map[string]string{"batch.kubernetes.io/job-index": "0"}, false},
		{"indexed without label", "Job", "Indexed", nil, true},
		{"indexed with wrong label", "Job", "Indexed", map[string]string{"some-other": "label"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"completionsMode": tt.completionsMode,
					"labels":          tt.labels,
				},
			}
			bytes, _ := json.Marshal(data)
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

func TestMaxIndexFailureInvalidCheck(t *testing.T) {
	c := maxIndexFailureInvalidCheck{}

	tests := []struct {
		name             string
		kind             string
		completionsMode  string
		maxFailedIndexes *int32
		want             bool
	}{
		{"non-job", "Deployment", "", nil, false},
		{"non-indexed", "Job", "NonIndexed", nil, false},
		{"nil maxFailedIndexes", "Job", "Indexed", nil, false},
		{"zero maxFailedIndexes", "Job", "Indexed", ptrInt32(0), false},
		{"positive maxFailedIndexes", "Job", "Indexed", ptrInt32(5), false},
		{"negative maxFailedIndexes", "Job", "Indexed", ptrInt32(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"completionsMode":  tt.completionsMode,
					"maxFailedIndexes": tt.maxFailedIndexes,
				},
			}
			bytes, _ := json.Marshal(data)
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

func TestPodStartupPolicyInvalidCheck(t *testing.T) {
	c := podStartupPolicyInvalidCheck{}

	tests := []struct {
		name             string
		kind             string
		podStartupPolicy string
		want             bool
	}{
		{"non-job", "Deployment", "", false},
		{"empty", "Job", "", false},
		{"RequiredStartupPolicy", "Job", "RequiredStartupPolicy", false},
		{"DelayedStartupPolicy", "Job", "DelayedStartupPolicy", false},
		{"invalid", "Job", "BadValue", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"podStartupPolicy": tt.podStartupPolicy,
				},
			}
			bytes, _ := json.Marshal(data)
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

func TestSelectorInvalidCheck(t *testing.T) {
	c := selectorInvalidCheck{}

	tests := []struct {
		name     string
		kind     string
		selector string
		want     bool
	}{
		{"non-job", "Deployment", "", false},
		{"empty selector", "Job", "", false},
		{"valid selector", "Job", "app=myapp", false},
		{"valid selector with eq", "Job", "app in (foo,bar)", false},
		{"invalid selector", "Job", "[invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"selector": tt.selector,
				},
			}
			bytes, _ := json.Marshal(data)
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
		{"valid schedule with list", "CronJob", "0 0,15,30,45 * * *", false},
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
			bytes, _ := json.Marshal(data)
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

func TestTimezoneInvalidCheck(t *testing.T) {
	c := timezoneInvalidCheck{}

	tests := []struct {
		name     string
		kind     string
		timezone string
		want     bool
	}{
		{"non-cronjob", "Job", "", false},
		{"empty timezone", "CronJob", "", false},
		{"valid timezone", "CronJob", "America/New_York", false},
		{"valid timezone UTC", "CronJob", "UTC", false},
		{"valid timezone UTC+5", "CronJob", "UTC+5", false},
		{"invalid timezone", "CronJob", "Invalid/Timezone", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"kind": tt.kind,
				"spec": map[string]interface{}{
					"timezone": tt.timezone,
				},
			}
			bytes, _ := json.Marshal(data)
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
			bytes, _ := json.Marshal(data)
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
			bytes, _ := json.Marshal(data)
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
			bytes, _ := json.Marshal(data)
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
			bytes, _ := json.Marshal(data)
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
		{"batch/completions-invalid", completionsInvalidCheck{}},
		{"batch/active-deadline-seconds-invalid", activeDeadlineSecondsInvalidCheck{}},
		{"batch/backoff-limit-invalid", backoffLimitInvalidCheck{}},
		{"batch/ttl-after-finished-invalid", ttlAfterFinishedInvalidCheck{}},
		{"batch/completions-mode-invalid", completionsModeInvalidCheck{}},
		{"batch/index-format-invalid", indexFormatInvalidCheck{}},
		{"batch/max-index-failure-invalid", maxIndexFailureInvalidCheck{}},
		{"batch/pod-startup-policy-invalid", podStartupPolicyInvalidCheck{}},
		{"batch/selector-invalid", selectorInvalidCheck{}},
		{"batch/schedule-invalid", scheduleInvalidCheck{}},
		{"batch/timezone-invalid", timezoneInvalidCheck{}},
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
			if len(tc.c.DocSkipper()) == 0 {
				t.Error("DocSkipper must not be empty")
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
	bytes, _ := json.Marshal(data)
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
	bytes, _ := json.Marshal(data)
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
