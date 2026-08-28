package validation

import (
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"

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
func (c parallelismInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c parallelismInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string         `json:"kind"`
		Spec jobSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	if job.Spec.Parallelism == nil || *job.Spec.Parallelism >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("parallelism").String(),
			Message: fmt.Sprintf("parallelism: must be >= 0, got %d", *job.Spec.Parallelism),
			Kind:    job.Kind,
			Extra:   map[string]string{"parallelism": strconv.Itoa(int(*job.Spec.Parallelism))},
		},
	}}
}

// completionsInvalidCheck validates that completions must be >= parallelism.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type completionsInvalidCheck struct{}

func (c completionsInvalidCheck) ID() string            { return "batch/completions-invalid" }
func (c completionsInvalidCheck) Title() string         { return "Completions Must Be >= Parallelism" }
func (c completionsInvalidCheck) Category() string      { return "batch" }
func (c completionsInvalidCheck) Blocking() bool        { return true }
func (c completionsInvalidCheck) RenderSensitive() bool { return true }
func (c completionsInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c completionsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string         `json:"kind"`
		Spec jobSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	parallelism := job.Spec.GetParallelism()
	if parallelism == 0 {
		parallelism = 1
	}
	completions := job.Spec.Completions
	if completions == nil || *completions >= int32(parallelism) {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("completions").String(),
			Message: fmt.Sprintf("completions must be >= parallelism (got %d)", parallelism),
			Kind:    job.Kind,
			Extra:   map[string]string{"completions": strconv.Itoa(int(*completions)), "parallelism": strconv.Itoa(parallelism)},
		},
	}}
}

// activeDeadlineSecondsInvalidCheck validates that activeDeadlineSeconds must be >= 1.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type activeDeadlineSecondsInvalidCheck struct{}

func (c activeDeadlineSecondsInvalidCheck) ID() string {
	return "batch/active-deadline-seconds-invalid"
}

func (c activeDeadlineSecondsInvalidCheck) Title() string {
	return "ActiveDeadlineSeconds Must Be >= 1"
}
func (c activeDeadlineSecondsInvalidCheck) Category() string      { return "batch" }
func (c activeDeadlineSecondsInvalidCheck) Blocking() bool        { return true }
func (c activeDeadlineSecondsInvalidCheck) RenderSensitive() bool { return true }
func (c activeDeadlineSecondsInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c activeDeadlineSecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string `json:"kind"`
		Spec jobSpecWrapper
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	ads := job.Spec.ActiveDeadlineSeconds
	if ads == nil || *ads >= 1 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("activeDeadlineSeconds").String(),
			Message: fmt.Sprintf("activeDeadlineSeconds: must be >= 1, got %d", *ads),
			Kind:    job.Kind,
			Extra:   map[string]string{"activeDeadlineSeconds": strconv.Itoa(int(*ads))},
		},
	}}
}

// backoffLimitInvalidCheck validates that backoffLimit must be >= 0.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type backoffLimitInvalidCheck struct{}

func (c backoffLimitInvalidCheck) ID() string            { return "batch/backoff-limit-invalid" }
func (c backoffLimitInvalidCheck) Title() string         { return "BackoffLimit Must Be >= 0" }
func (c backoffLimitInvalidCheck) Category() string      { return "batch" }
func (c backoffLimitInvalidCheck) Blocking() bool        { return true }
func (c backoffLimitInvalidCheck) RenderSensitive() bool { return true }
func (c backoffLimitInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c backoffLimitInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string `json:"kind"`
		Spec jobSpecWrapper
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	// No backoffLimit means default (6), which is >= 0
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("backoffLimit").String(),
			Message: fmt.Sprintf("backoffLimit: must be >= 0, got %d", *job.Spec.BackoffLimit),
			Kind:    job.Kind,
			Extra:   map[string]string{"backoffLimit": strconv.Itoa(int(*job.Spec.BackoffLimit))},
		},
	}}
}

// ttlAfterFinishedInvalidCheck validates ttlAfterFinished.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type ttlAfterFinishedInvalidCheck struct{}

func (c ttlAfterFinishedInvalidCheck) ID() string { return "batch/ttl-after-finished-invalid" }

func (c ttlAfterFinishedInvalidCheck) Title() string         { return "TTLAfterFinished Must Be Non-Negative" }
func (c ttlAfterFinishedInvalidCheck) Category() string      { return "batch" }
func (c ttlAfterFinishedInvalidCheck) Blocking() bool        { return true }
func (c ttlAfterFinishedInvalidCheck) RenderSensitive() bool { return true }
func (c ttlAfterFinishedInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c ttlAfterFinishedInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string `json:"kind"`
		Spec jobSpecWrapper
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	// ttlAfterFinished nil is valid (no TTL)
	// ttlAfterFinished >= 0 is valid (seconds)
	if job.Spec.TTLAfterFinished == nil || *job.Spec.TTLAfterFinished >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("ttlAfterFinished").String(),
			Message: fmt.Sprintf("ttlAfterFinished: must be >= 0, got %d", *job.Spec.TTLAfterFinished),
			Kind:    job.Kind,
			Extra:   map[string]string{"ttlAfterFinished": strconv.Itoa(int(*job.Spec.TTLAfterFinished))},
		},
	}}
}

// completionsModeInvalidCheck validates completionsMode.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type completionsModeInvalidCheck struct{}

func (c completionsModeInvalidCheck) ID() string { return "batch/completions-mode-invalid" }

func (c completionsModeInvalidCheck) Title() string         { return "CompletionsMode Must Be Valid" }
func (c completionsModeInvalidCheck) Category() string      { return "batch" }
func (c completionsModeInvalidCheck) Blocking() bool        { return true }
func (c completionsModeInvalidCheck) RenderSensitive() bool { return true }
func (c completionsModeInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c completionsModeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string `json:"kind"`
		Spec jobSpecWrapper
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	mode := job.Spec.CompletionsMode
	if mode == "Indexed" || mode == "NonIndexed" || mode == "" {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("completionsMode").String(),
			Message: fmt.Sprintf("completionsMode: Unsupported value: %q", string(mode)),
			Kind:    job.Kind,
			Extra:   map[string]string{"completionsMode": string(mode)},
		},
	}}
}

// indexFormatInvalidCheck validates Indexed completions job label requirements.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type indexFormatInvalidCheck struct{}

func (c indexFormatInvalidCheck) ID() string            { return "batch/index-format-invalid" }
func (c indexFormatInvalidCheck) Title() string         { return "Job Index Label Required For Indexed Jobs" }
func (c indexFormatInvalidCheck) Category() string      { return "batch" }
func (c indexFormatInvalidCheck) Blocking() bool        { return true }
func (c indexFormatInvalidCheck) RenderSensitive() bool { return true }
func (c indexFormatInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c indexFormatInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string         `json:"kind"`
		Spec jobSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	if job.Spec.CompletionsMode != "Indexed" {
		return nil
	}
	jobIdxLabel := "batch.kubernetes.io/job-index"
	labelsMap := job.Spec.TemplateLabels
	if labelsMap == nil {
		labelsMap = make(map[string]string)
	}
	if _, ok := labelsMap[jobIdxLabel]; ok {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("template").Child("metadata").Child("labels").String(),
			Message: "template labels must include " + jobIdxLabel + " for Indexed jobs",
			Kind:    job.Kind,
		},
	}}
}

// maxIndexFailureInvalidCheck validates maxFailedIndexes for Indexed jobs.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type maxIndexFailureInvalidCheck struct{}

func (c maxIndexFailureInvalidCheck) ID() string { return "batch/max-index-failure-invalid" }

func (c maxIndexFailureInvalidCheck) Title() string         { return "MaxFailedIndexes Must Be >= 0" }
func (c maxIndexFailureInvalidCheck) Category() string      { return "batch" }
func (c maxIndexFailureInvalidCheck) Blocking() bool        { return true }
func (c maxIndexFailureInvalidCheck) RenderSensitive() bool { return true }
func (c maxIndexFailureInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c maxIndexFailureInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string `json:"kind"`
		Spec jobSpecWrapper
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	if job.Spec.CompletionsMode != "Indexed" {
		return nil
	}
	if job.Spec.MaxFailedIndexes == nil || *job.Spec.MaxFailedIndexes >= 0 {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("maxFailedIndexes").String(),
			Message: fmt.Sprintf("maxFailedIndexes: must be >= 0, got %d", *job.Spec.MaxFailedIndexes),
			Kind:    job.Kind,
			Extra:   map[string]string{"maxFailedIndexes": strconv.Itoa(int(*job.Spec.MaxFailedIndexes))},
		},
	}}
}

// podStartupPolicyInvalidCheck validates podStartupPolicy.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type podStartupPolicyInvalidCheck struct{}

func (c podStartupPolicyInvalidCheck) ID() string { return "batch/pod-startup-policy-invalid" }

func (c podStartupPolicyInvalidCheck) Title() string         { return "PodStartupPolicy Must Be Valid" }
func (c podStartupPolicyInvalidCheck) Category() string      { return "batch" }
func (c podStartupPolicyInvalidCheck) Blocking() bool        { return true }
func (c podStartupPolicyInvalidCheck) RenderSensitive() bool { return true }
func (c podStartupPolicyInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c podStartupPolicyInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string `json:"kind"`
		Spec jobSpecWrapper
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	policy := job.Spec.PodStartupPolicy
	if policy == "RequiredStartupPolicy" || policy == "DelayedStartupPolicy" || policy == "" {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("podStartupPolicy").String(),
			Message: fmt.Sprintf("podStartupPolicy: Unsupported value: %q", string(policy)),
			Kind:    job.Kind,
			Extra:   map[string]string{"podStartupPolicy": string(policy)},
		},
	}}
}

// selectorInvalidCheck validates the job status selector.
// Source: k8s.io/kubernetes/pkg/apis/batch/validation/validation.go
type selectorInvalidCheck struct{}

func (c selectorInvalidCheck) ID() string            { return "batch/selector-invalid" }
func (c selectorInvalidCheck) Title() string         { return "Job Status Selector Must Be Valid" }
func (c selectorInvalidCheck) Category() string      { return "batch" }
func (c selectorInvalidCheck) Blocking() bool        { return true }
func (c selectorInvalidCheck) RenderSensitive() bool { return true }
func (c selectorInvalidCheck) DocSkipper() []string  { return []string{"Job"} }

func (c selectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var job struct {
		Kind string         `json:"kind"`
		Spec jobSpecWrapper `json:"spec"`
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return nil
	}
	if job.Kind != "Job" {
		return nil
	}
	selector := job.Spec.Selector
	if selector == "" {
		return nil
	}
	if _, err := labels.Parse(selector); err != nil {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    field.NewPath("spec").Child("selector").String(),
				Message: fmt.Sprintf("invalid label selector: %s", err.Error()),
				Kind:    job.Kind,
				Extra:   map[string]string{"selector": selector},
			},
		}}
	}
	return nil
}

// jobSpecWrapper holds batch/v1.JobSpec fields we need to validate.
type jobSpecWrapper struct {
	Parallelism           *int32            `json:"parallelism"`
	Completions           *int32            `json:"completions"`
	ActiveDeadlineSeconds *int64            `json:"activeDeadlineSeconds"`
	BackoffLimit          *int32            `json:"backoffLimit"`
	TTLAfterFinished      *int32            `json:"ttlAfterFinished"`
	CompletionsMode       string            `json:"completionsMode"`
	MaxFailedIndexes      *int32            `json:"maxFailedIndexes"`
	PodStartupPolicy      string            `json:"podStartupPolicy"`
	Selector              string            `json:"selector"`
	TemplateLabels        map[string]string `json:"labels"`
}

func (w *jobSpecWrapper) GetParallelism() int {
	if w.Parallelism == nil {
		return 1
	}
	return int(*w.Parallelism)
}

// Register registers all Job validation checks with the check registry.
func Register() {
	checks := []runtime.Check{
		parallelismInvalidCheck{},
		completionsInvalidCheck{},
		activeDeadlineSecondsInvalidCheck{},
		backoffLimitInvalidCheck{},
		ttlAfterFinishedInvalidCheck{},
		completionsModeInvalidCheck{},
		indexFormatInvalidCheck{},
		maxIndexFailureInvalidCheck{},
		podStartupPolicyInvalidCheck{},
		selectorInvalidCheck{},
	}

	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
