package validation

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// batchValidationPath is pkg/apis/batch/validation/validation.go in
// kubernetes/kubernetes, which holds every rule ported by this package.
const batchValidationPath = "pkg/apis/batch/validation/validation.go"

// validatedAt is the kubernetes/kubernetes tag every digest below was taken
// at. It matches the tag derived from go.mod that
// `task verify:upstream-refs` pins to.
const validatedAt = "v1.37.0"

// upstreamRefs cites the exact upstream Kubernetes function each check in this
// package ports. See pkg/validator/runtime/upstream.go for why a file-only
// citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	// --- Job ---------------------------------------------------------------
	"batch/parallelism-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateJobSpec"},
		Digest:      "sha256:d8620c5fe4869a483b10904705c22a32910bc93de74f65d567ffe957fdd26c63",
		ValidatedAt: validatedAt,
		Note: "Ports the ValidateNonnegativeField(*spec.Parallelism, ...) branch. The indexed-completion " +
			"upper-bound branches on the same field are not ported: they are cross-field rules against " +
			"spec.completionMode and spec.completions.",
	},
	"batch/backoff-limit-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateJobSpec"},
		Digest:      "sha256:d8620c5fe4869a483b10904705c22a32910bc93de74f65d567ffe957fdd26c63",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(*spec.BackoffLimit, ...) branch. An unset backoffLimit is skipped, matching the upstream nil guard (defaulting supplies 6).",
	},

	// --- CronJob -----------------------------------------------------------
	"batch/schedule-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateCronJobSpec", "validateScheduleFormat"},
		Digest:      "sha256:2e26cd4c64b4c3c8b0c5adbcc20bcfccbcd0929f5d214385f097f3fddb9cbb0c",
		ValidatedAt: validatedAt,
		Note: "Ports the ParseCronScheduleWithPanicRecovery -> field.Invalid branch, calling the " +
			"same parser upstream does: cron.ParseStandard from github.com/robfig/cron/v3 at the " +
			"v3.0.1 revision Kubernetes pins, wrapped in the same panic recovery (ParseStandard " +
			"panics on inputs such as \"TZ=0\"). Also ports validateCronJobSpec's " +
			"len(spec.Schedule) == 0 -> field.Required branch, but only when the key is present " +
			"and empty. An omitted schedule is left to the schema's `required` array to avoid " +
			"double-reporting; that array does not cover an explicitly-empty schedule, since it " +
			"asserts the key exists and the schema puts no minLength on the value. The " +
			"TZ-vs-timeZone conflict branches are not ported, since they depend on the " +
			"CronJobTimeZone feature gate.",
	},
	"batch/concurrency-policy-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateConcurrencyPolicy"},
		Digest:      "sha256:95938fa31bac68bf125531e52958324beb1995d268d7819cabb942177b783369",
		ValidatedAt: validatedAt,
		Note:        "Ports the default -> field.NotSupported branch. Deliberate divergence: the empty case, which upstream reports Required, is skipped because defaulting supplies Allow.",
	},
	"batch/failed-jobs-history-limit-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateCronJobSpec"},
		Digest:      "sha256:77970eb95ee7d7eeddce0a77759ac613a0c49315630b7f4864b748ac691c74cc",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(*spec.FailedJobsHistoryLimit, ...) branch; zero is valid, matching upstream.",
	},
	"batch/successful-jobs-history-limit-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateCronJobSpec"},
		Digest:      "sha256:77970eb95ee7d7eeddce0a77759ac613a0c49315630b7f4864b748ac691c74cc",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(*spec.SuccessfulJobsHistoryLimit, ...) branch; zero is valid, matching upstream.",
	},
	"batch/starting-deadline-seconds-invalid": {
		Path:        batchValidationPath,
		Functions:   []string{"validateCronJobSpec"},
		Digest:      "sha256:77970eb95ee7d7eeddce0a77759ac613a0c49315630b7f4864b748ac691c74cc",
		ValidatedAt: validatedAt,
		Note:        "Ports the ValidateNonnegativeField(*spec.StartingDeadlineSeconds, ...) branch.",
	},
}
