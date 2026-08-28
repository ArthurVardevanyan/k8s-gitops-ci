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
		Functions:   []string{"validateScheduleFormat"},
		Digest:      "sha256:aa239363d1159d12d4f403084a1cfa1c58e5b737414d05e9f597411716bf4061",
		ValidatedAt: validatedAt,
		Note: "Ports the ParseCronScheduleWithPanicRecovery -> field.Invalid branch. Deliberate " +
			"divergence: the parse is a self-contained 5-field cron structural parser rather than the " +
			"upstream robfig/cron parser, so it accepts a superset (it does not range-check field " +
			"values or resolve names like MON/JAN); it is therefore never stricter than the API " +
			"server. An empty schedule is skipped rather than reported Required (that branch lives " +
			"in validateCronJobSpec and is covered by the schema's `required`), and the TZ/CRON_TZ " +
			"branches are not ported.",
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
