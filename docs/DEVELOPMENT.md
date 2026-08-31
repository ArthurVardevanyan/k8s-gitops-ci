# Development

This covers build/test/lint commands, repository structure, and the
design conventions that keep this a generic, org-agnostic core. See
[`ARCHITECTURE.md`](ARCHITECTURE.md) for the runtime shape (what actually
happens when you run a pipeline) and a "Where do I find X?" table
pointing to every other doc (`CI.md`, `HOOKS.md`, `EXEMPTIONS.md`,
`TEKTON.md`, `RELEASE.md`, `SECURITY.md`, `SCHEMAS.md`).

## Table of Contents

- [Prerequisites](#prerequisites)
- [Repository Structure](#repository-structure)
- [Design Conventions](#design-conventions)
  - [Options-struct pattern](#options-struct-pattern)
  - [`provider.Providers` — the runtime injection seam](#providerproviders--the-runtime-injection-seam)
  - [Exported override vars — the compile-time injection seam](#exported-override-vars--the-compile-time-injection-seam)
  - [The "core data + org-injectable override" pattern](#the-core-data--org-injectable-override-pattern)
  - [Generic check-enablement mechanism](#generic-check-enablement-mechanism)
  - [Adding a new validator](#adding-a-new-validator)
- [CLI Output & PR Comment Format](#cli-output--pr-comment-format)
  - [Logger banner and section headers (`pkg/logger`)](#logger-banner-and-section-headers-pkglogger)
  - [Timing table (`pkg/validator/timing.go`)](#timing-table-pkgvalidatortiminggo)
  - [Failed-section console detail without `--comment` (`pkg/pipeline/pipeline.go`)](#failed-section-console-detail-without---comment-pkgpipelinepipelinego)
  - [Pipeline exit code (`pipeline.Run`, `Result.Failed`)](#pipeline-exit-code-pipelinerun-resultfailed)
  - [Unified PR-comment report (`pkg/validator/unified_report.go`, `compose_sections.go`)](#unified-pr-comment-report-pkgvalidatorunified_reportgo-compose_sectionsgo)
- [Building](#building)
- [Testing](#testing)
  - [Negative / error-path coverage](#negative--error-path-coverage)
  - [End-to-end / regression replay](#end-to-end--regression-replay)
- [Task Targets Reference](#task-targets-reference)

## Prerequisites

- Go (see `go.mod` for the exact version)
- [`task`](https://taskfile.dev) — all build/test/lint/update workflows go
  through `task`, not raw `go` commands (see [Building](#building) for why)

Optional, used by individual `pkg/lint/*` wrappers when present on `PATH`.
Every wrapper itself returns a typed "CLI not found" error rather than
panicking when its tool is missing, so none of these are required just to
build or run the unit test suite (missing-CLI tests either skip
themselves or assert that typed error directly) — but note that's a
different question from what a _caller_ of that wrapper does with the
error: the Linting phase (`pkg/validator/phases.go`) treats a missing
`markdownlint`/`prettier`/`shellcheck`/`golangci-lint` as a **hard
failure** in an actual pipeline run, not a graceful skip (see
[`CI.md`](CI.md#kustomize-fix)) — these tools just aren't required to
build or exercise this repo's own test suite:

- `kustomize` — real `kustomize edit fix --vars` wrapping (`pkg/kustomize`)
- `kubeconform` — schema validation (`pkg/lint/kubeconform`) — the schema
  data itself is embedded at build time; see [`SCHEMAS.md`](SCHEMAS.md)
- `kyverno` — policy validation (`pkg/lint/kyverno`), off by default (see
  [`SCHEMAS.md`](SCHEMAS.md))
- `shellcheck`, `prettier`, `markdownlint-cli2`, `golangci-lint` — the
  linters wrapped by their matching `pkg/lint/*` package

## Repository Structure

```text
cmd/
  k8s-gitops-ci/    CLI entrypoint (subcommand dispatch, flag parsing)
  version/          build-time version metadata (ldflags-injected)
pkg/
  validator/        orchestration (RunAll, phases, report composition) +
                       `static/` sub-package containing all 9
                       check.Register-driven validator engines
                       (namespace, psa, rbac, crb, syncopts, image,
                       namedport, podspec, placeholder), their 12 check
                       adapters, and the `RegisterAll` registration layer;
                       `check/` (the registry engine) and `exempt/`
                       (unified exemption framework) sit at top level;
                       `clusterid/` and `nad/` remain top-level —
                       clusterid is the engine the adapter wraps, nad is a
                       separate always-on validator over rendered overlay
                       output (not check.Register-driven; see
                       docs/CI.md#networkattachmentdefinition-nad-validation)
                       Shared types across wiring and phase logic are
                       centralized in `types.go`; phases are orchestrated
                       in `phases.go` (~1100 lines); wiring logic lives
                       in `build_wiring.go`, `hook_wiring.go`,
                       `target_wiring.go`, `scaffold_wiring.go`,
                       `kyverno_wiring.go`, `avp_wiring.go`,
                       `nad_wiring.go`, `nonapp_wiring.go`, and
                       `dispatch.go` (see
                       docs/ARCHITECTURE.md#future-simplifications).
  validator/runtime/ shared types (Finding, Check interface) and the
                     dual-pass wiring (ScopeDoc + IsRenderSensitive)
                     used by the runtime validation checks; the checks
                     themselves live under runtime/kubernetes/* (one
                     package per API group, each with a validation/
                     subpackage containing admission-enforced
                     structural rules)
  lint/              CLI-tool wrappers, one package per tool (golangci,
                     kubeconform, kyverno, markdownlint, prettier,
                     shellcheck, yamlsyntax)
  pipeline/          top-level pipeline orchestration (PR checks + calls
                     into pkg/validator)
  overlay/           kustomize/Helm overlay discovery and build, including
                     GetOverlaysToTest (which overlays a change implies)
                     and FilterOverlaysByRefs (narrows a base/component
                     change down to just the overlay(s) that reference it)
  kustomize/          kustomization.yaml `edit fix` checking, plus
                     ResolveRefs (recursive resources/components/bases
                     reference-chain resolution used by
                     overlay.FilterOverlaysByRefs)
  ghostpatch/        detects kustomize patches that target nothing
  scaffold/          scafctl CLI wrapper for scaffold-drift checking
  configdiff/        detects which apps/clusters a repo-config change affects
  changeset/         changed-file resolution (git diff / gh PR files)
  github/, git/      thin wrappers around the gh/git CLIs
  cluster/           cluster/project-identity indexing
  provider/          org-injectable interfaces (see Design Conventions)
  csv/, largefile/, config/, convention/, logger/, hook/  smaller,
                     focused leaf packages
scripts/             shell helpers that regenerate embedded/generated
                     resources (see below)
```

## Design Conventions

This is a **generic core**: no org names, domains, cluster names,
cloud-provider, or vendored-tool specifics belong anywhere under `pkg/`.
Anything that varies by org is injected through one of the seams below —
never hardcoded and never special-cased in `pkg/`.

### Options-struct pattern

Every non-trivial function accepts a single `Options`/`RunOptions` struct
rather than a long parameter list (e.g. `validator.Options`,
`pipeline.Options`, `overlay.BuildOptions`). This keeps call sites stable
as new fields get added and makes it obvious at the call site which
behavior is being configured.

### `provider.Providers` — the runtime injection seam

`pkg/provider` defines small interfaces for behavior that legitimately
varies by org — report branding, which foreign PR-comment markers to
prune, secret-backend auth-error hints, and cluster/project identity
metadata:

```go
type Providers struct {
    Branding        Branding        // report marker/title/header/binary name/org version
    CommentPolicy   CommentPolicy   // foreign comment markers to prune
    Secrets         SecretBackend   // auth-error hint text
    ClusterMetadata ClusterMetadata // project/cluster identity + change groups
}
```

Every field is nilable, and every accessor method on `Providers` falls
back to a sensible generic default when its field is nil (see
`pkg/provider/provider.go`). An org wires a real implementation from its
own `main()`/`Configure()` equivalent and passes it through
`pipeline.Options.Providers`/`validator.Options.Providers` — the core
never imports anything org-specific to do this.

### Exported override vars — the compile-time injection seam

Some behavior can't reasonably be expressed as a runtime interface (it's
data, not logic, or it needs to be baked into the binary). For this, a
package exports a plain variable with a generic (usually empty/no-op)
default that an org sets once during its own initialization, or that gets
regenerated by a `scripts/*.sh` helper and committed by the org layer.
Examples: `pkg/lint/kyverno`'s `ExcludedRules`/`IncludeComponents`,
`pkg/validator/psa`'s `NamespaceSelectorLabelKeys`-style seams,
`pkg/validator`'s `TektonPACDir` (defaults to `.tekton`; set to `""` to
disable the built-in exemption that lets Tekton Pipelines-as-code-managed
`PipelineRun` manifests there skip the `sync-options`/`namespace` checks,
since PAC's own controller manages their lifecycle rather than Argo CD),
`pkg/scaffold`'s `Binary`/`ConfigSource` (retarget the scaffolding
CLI/config-source name), `DriftMode`/`ScaffoldArgs`/`CreatedFileMarkers`
(switch from the default `DiffDirs` generate-and-diff strategy to
`DryRunParse` for a scaffolding CLI whose dry-run mode reports the files
it would create rather than supporting a directory-diff contract), and
`ExcludedClusters` (permanently skip named overlays from scaffold-drift
validation, independent of the per-app
`IsOverlayDisabled`/`IsChangeGroupDisabled` config-driven opt-outs),
`pkg/scaffold`'s `OverlayConfigDisabled` (opt a per-overlay entry out of
scaffold-drift validation via a `disabled: true` flag in its scaffold
config's override section - the generic default reads the widely-used
`overlayDefinitions.overrides.<cluster>.disabled` shape; an org whose
tool lays that section out differently overrides just this lookup, which
keeps a downstream consumer from hard-failing on a PR that hand-edits a
by-design-disabled overlay),
`pkg/validator`'s `ExtraNonAppDirs` (extra top-level repository
directories - e.g. a vendored example or internal-tooling directory
whose layout coincidentally matches an app's `base`/`overlays` shape -
that `detectAppRoots` must never treat as an app root), and
`pkg/github`'s `TitleSuggestion` (an optional, always-non-blocking
PR-title convention check - e.g. an org's ticket-reference suffix -
layered on top of the required Conventional Commits prefix; see
`PRTitleSuggestion` and `ComposePRChecksSection`'s rendering of it as a
warning, never a failure), `pkg/validator`'s `DefaultAssumeOpenShift`
(sets `Options.AssumeOpenShift`'s effective default for every `RunAll`
call that doesn't set it explicitly, for an org whose fleet is
exclusively OpenShift/OKD) and `DefaultEnabledChecks` (an org-wide
default for `Options.EnabledChecks`, used whenever a given run's own
`EnabledChecks` is empty - see `resolveEnabledChecks` in `phases.go`).
The contract is always **"empty/nil is a true no-op"** — never assume a
default value that changes behavior when unset.

A closely related variant swaps out a whole function rather than a data
value: `kubeconform.ExtractSchemas` and `kyverno.PreparePolicies` are
themselves plain package vars (defaulting to the `embedschemas`
build-tag-gated archive extraction - see
[`docs/SCHEMAS.md`](SCHEMAS.md)), so an org/consumer layer can replace
either with a function pulling schemas/policies from an entirely
different source (e.g. an OCI artifact fetched at process startup)
without needing that build tag at all.

### The "core data + org-injectable override" pattern

Worked example: `pkg/validator/namespace/resource_scope.go` (generated
by `scripts/pull-scoped-resources.sh` from `kubectl api-resources`
against a live cluster; core, generic Kinds only) plus
`pkg/validator/namespace/extra_resource_scope.go` (an empty, exported
`ExtraResourceScope` map that an org populates with its own CRDs). The
lookup checks the override map first, falls back to the core map, and
never mutates or deletes from the core map. This is a **compile-time**
pattern — the org edits/regenerates the override file and rebuilds; there
is no runtime file-loading involved. Follow this same shape (core map +
empty exported override map, override checked first) any time a new
package needs org-extensible, purely-data lookup tables.

The `namespace` check itself enforces both directions of this scope map:
namespace-scoped resources (`false`) must declare `metadata.namespace`,
and cluster-scoped resources (`true`) must **not** declare it — except
for build-time-only objects (currently just Kustomize's own
`Kustomization`/`Component` control objects, listed in
`clusterScopeNamespaceExempt` in `pkg/validator/namespace/namespace.go`)
that are never applied to a cluster and so aren't meaningfully "scoped"
either way. Kinds that are never submitted to a Kubernetes API server at
all — local installer config artifacts like the OpenShift agent-based
installer's `AgentConfig`/`InstallConfig` (typically declared with a
bare, groupless `apiVersion`) — are matched by Kind and skipped by the
check entirely via `installerOnlyKinds`, mirroring the equivalent list in
`pkg/validator/syncopts`.

### Generic check-enablement mechanism

Every gateable check/step — whether it's a `check.Register`-based
per-document validator (`namespace`, `psa`, `rbac`, ...) or a standalone
step like `golangci` or `kyverno` — has a string ID and participates in
one shared enable/disable mechanism instead of a dedicated boolean flag
per step:

- `Options.DisabledChecks []string` — turn off a step that defaults to
  **enabled** (most steps: `sync-options`, `markdownlint`, `prettier`,
  `shellcheck`, `golangci`, `kubeconform`, `avp`, `kustomize-fix`, ...). For
  `markdownlint`/`prettier`/`shellcheck`/`golangci`/`kustomize-fix`,
  `DisabledChecks` doesn't mean "org-specific opt-out" so much as "no
  `<tool>` binary available" - see [`CI.md`](CI.md#kustomize-fix) for why
  a missing CLI is a hard failure rather than a graceful skip for these
  five steps. `kubeconform` is different: it has no external CLI
  dependency (schema validation runs in-process against the embedded/
  extracted schema set - see [`SCHEMAS.md`](SCHEMAS.md)), so
  `--disable-checks kubeconform` is a genuine wholesale opt-out - e.g. a
  `--lint-only` invocation over a repo whose changeset can include
  non-Kubernetes root config files (`Taskfile.yml`, `.golangci.yml`, ...)
  that would otherwise trip kubeconform's "missing 'kind' key" error. For
  finer-grained, per-file exemptions instead of disabling the whole step,
  see `check=kubeconform` selectors in
  [`EXEMPTIONS.md`](EXEMPTIONS.md#exemptable-check-ids).
- `Options.EnabledChecks []string` — turn on a step that defaults to
  **disabled**: `kyverno` (see [`SCHEMAS.md`](SCHEMAS.md) for why) and
  `scaffold-readme` (see [`CI.md`](CI.md#scaffold-validation) for why).
- `pkg/validator/phases.go`'s `defaultOffSteps` map is the single place
  that lists which IDs default off; `stepEnabled(id, disabled, enabled)`
  is the shared decision function every gateable step calls.

Add a new ID to `defaultOffSteps` only when a feature has no sane generic
default an arbitrary org could run out of the box — this is not a
general-purpose feature-flag system, just a way to make a small number of
inherently-org-dependent features opt-in without inventing a new flag each
time.

### Adding a new validator

1. Define your check type implementing `check.Check` plus one of
   `DocCheck`/`OverlayCheck`/`FileCheck`/`RepoCheck` (see
   `pkg/validator/check/check.go`) in its own `pkg/validator/<name>`
   package.
2. Call `check.Register(yourCheck{})` from an `init()` or from
   `pkg/validator/register_checks.go`'s adapter wiring — registering
   auto-marks the check's ID as exemptable (see
   `check.Register`/`exempt.RegisterExemptable`) unless it's explicitly
   guarded against exemption (the only current example is
   `exempt.IDClusterIdentity`, a deliberately non-exemptable structural
   bucket — see [`EXEMPTIONS.md`](EXEMPTIONS.md)).
3. Add `testdata/` fixtures under your package (`testdata/` for
   fixtures expected to pass or produce specific findings,
   `testdata/invalid/` for deliberately-malformed inputs) — this repo's
   existing packages are inconsistent about having these; new validators
   should have them from the start.
4. Write table-driven tests (`[]struct{ name string; ... }` +
   `t.Run(tt.name, ...)`) matching the style already used across
   `pkg/validator/*`.

## CLI Output & PR Comment Format

### Logger banner and section headers (`pkg/logger`)

`Logger` writes timestamped `[HH:MM:SS] [LEVEL] message` lines to stdout
(and optionally a log file). `Header(title)` prints a boxed section banner
(a `====...====` divider, then an indented `title` line, then another
divider, 60 columns wide) and records `title`
as the current section for error attribution; `SubHeader` prints a lighter
40-column variant. `Error`/`ErrorInSection` (the latter for use from
goroutines where the shared "current section" may have been overwritten by
another goroutine) track errors and failed sections, feeding a final
`Summary(totalSections, warnedSections, failedSections int)` banner - the
three counts (typically `len(validator.Result.Sections)`,
`validator.Result.WarnedSectionCount()`, and
`validator.Result.FailedSectionCount()`) render a leading "Sections: N
passed, X warned, Y failed" line (passed = total - warned - failed);
pkg/logger can't import pkg/validator itself (validator already imports
logger), so callers pass plain ints rather than a `ReportSection` slice,
and passing `0, 0, 0` (e.g. callers with no `validator.Result`) omits the
line entirely. `FailedSectionCount`/`HasErrorSection` count only
`StatusError` sections and `WarnedSectionCount` only `StatusWarning` ones -
a `StatusWarning` section is surfaced in the "warned" tally and the PR
comment but is never a hard failure (`StatusInfo` counts as neither):

```text
============================================================
  RESULTS SUMMARY
============================================================
  Sections: 6 passed, 1 warned, 2 failed
  Warnings: 2
  Errors: 1 (see details above)
  Failed sections:
    - Post-Build Validation
============================================================
```

`Info`/`Warn`/`Error`/`Debug` are for single structured log lines — a
multi-line message passed to one of these still gets the `[time] [LEVEL]`
prefix on every resulting line (see `write`'s per-line split), so a
multi-finding tool summary doesn't degrade into "first line tagged, rest
bare" output. For an already-formatted, potentially multi-line block meant
to be printed verbatim instead (e.g. `Summary()` itself, or a rendered
section body) use `Raw(msg)`, which prints with no prefix at all, matching
the no-prefix convention `Header`/`SubHeader` already use for banner lines.

`Logger.Scope()` returns a `ScopedLogger` that buffers a goroutine's log
lines and flushes them atomically (`Flush()`) once the goroutine finishes,
so concurrent phases (linting, static checks, per-overlay builds) don't
interleave their output line-by-line — verbose mode (`--verbose`) streams
immediately instead, for debugging hangs.

### Timing table (`pkg/validator/timing.go`)

`TimingCollector.Record(name, duration, parallel bool)` records a top-level
phase; `RecordStep(phase, name, duration)` records a sub-step nested under
a phase (e.g. one linter, or one overlay's build). `SetConcurrency(cpus,
concurrency)` records the worker-pool size used for the run. `Summary`
renders a fixed-width table, phases flush-left and sub-steps indented and
sorted longest-first, with a parallelism-efficiency line
(`sum(phase durations) / wall-clock duration`, e.g. `3.2x` means phases
that together took 3.2x the wall-clock time ran in parallel) and a
concurrency footer:

```text
--------------------------------------------------------------
Step                              Duration  Mode
--------------------------------------------------------------
Large File Check                          0s  seq
YAML Syntax                               0s  seq
Linting                              1.203s  parallel
  golangci-lint                       980ms  parallel
  kubeconform                         640ms  parallel
Static Checks                         310ms  parallel
Build YAML                           1.510s  parallel
  app1/overlays/prod                  1.510s  parallel
Post-Build Validation                 500ms  parallel
--------------------------------------------------------------
TOTAL (wall)                         3.523s
TOTAL (sum)                          4.523s
Parallelism                             1.3x
Concurrency                                8  (4 CPUs × 2)
--------------------------------------------------------------
```

Setup (in `pkg/pipeline`, when running the full `pipeline` command rather
than `test`/`build-yaml`) additionally records `clone`,
`schemas`, and `policies` sub-steps - schemas are always prefetched once,
up front (a cheap, pure embedded-archive extraction reused by the
Linting phase's kubeconform step instead of extracting lazily on every
run - see `validator.Options.SchemaDir`); policies are only prefetched
when the opt-in `kyverno` step is actually enabled (preparing them shells
out to `kustomize build`, not worth paying for on every run - see
`validator.Options.PolicyPath`).

The `Build YAML` phase also prints an always-on, one-line summary
(`build: N overlay(s) rendered (<duration>)`, `pkg/validator/phases.go`)
matching the other phases' own `... passed (<duration>)` lines - it
previously had no inline console duration at all, only the table row
above. The per-overlay `Build YAML` sub-step row, however, times an
entire overlay's build (overlay checks + PRE_BUILD_HOOK + render +
POST_BUILD_HOOK) as one opaque unit, and `POST_VALIDATE_HOOK` isn't
timed anywhere in this table. When diagnosing a slow Build phase, run
with `--verbose` to get a finer breakdown as Debug lines (also always
written to the log file regardless of verbosity - see `Logger.write`):

```text
[12:00:01] [DEBUG] overlay app1/overlays/prod: render=120ms pre-hook=0s post-hook=1.2s
[12:00:03] [DEBUG] post-validate hook app1: 3.4s
```

`overlay <path>: render=... pre-hook=... post-hook=...` is emitted by
`buildOverlayWithHooks` (`pkg/validator/hook_wiring.go`) for every
overlay build; `render` includes the `argocd-vault-plugin` subprocess
for AVP strategies (piped through internally by
`overlay.RenderWithStrategy`), so it isn't split out further.
`post-validate hook <app>: ...` is emitted by `runAppPostValidateHooks`
for each app with a `POST_VALIDATE_HOOK` - the hook most likely to shell
out to a slow external call (e.g. a cloud API lookup per referenced
image), and the one previously invisible in any timing output.

Between the per-overlay build loop and the phase's own `tc.Record("Build
YAML", ...)`, `runBuildAndPostBuild` also runs a handful of serial,
non-overlay steps - scaffold validation, the `kustomize edit fix` check,
resolving this PR's added-files set, and ghost-patch detection (see
[Ghost Patch Detection](CI.md#ghost-patch-detection)) - that previously had
no timing at all, so a slow one of these was invisible in both the console
and the timing table; the entire per-overlay build loop could finish in
seconds while the phase's total wall time was minutes, with nothing
pointing at why. Each is now both a `tc.RecordStep("Build YAML", ...)` row
(`scaffold-validation`, `kustomize-fix`, `added-files`, `ghost-patch`) and a
`--verbose` Debug line, e.g.:

```text
[12:00:05] [DEBUG] scaffold-validation: 12ms
[12:00:05] [DEBUG] kustomize-fix: 8ms
[12:00:06] [DEBUG] added-files: 340ms
[12:00:06] [DEBUG] ghost-patch: 6ms
```

This instrumentation is what surfaced (and let us fix) `ghost-patch`
previously taking minutes on apps with hundreds of overlays - see
[Ghost Patch Detection](CI.md#ghost-patch-detection) for why it's now scoped
to only the overlays this run rendered.

### Failed-section console detail without `--comment` (`pkg/pipeline/pipeline.go`)

Every phase composes a detail-bearing `ReportSection` (the actual
file/message list behind a step's summary "N violation(s)" log line) onto
`ValidatorResult.Sections`, but that has historically only ever been
rendered into the PR comment body via `composeSections`/`postComment` —
which is skipped whenever `--comment` isn't passed (e.g. a local/CLI-only
run). `pipeline.Run` unconditionally calls `printFailedSectionDetail`
(independent of `--verbose` and of whether a comment is posted) to print
every `StatusError` section's full `Body` to the console too
(`StatusWarning`/`StatusInfo` sections are worth a look in the PR comment
but aren't printed here — this mirrors `FailedSectionCount`/
`HasErrorSection`'s same StatusError-only distinction), so a failed run
always shows e.g. exactly which file/check produced a Resource Compliance
finding, not just the count — without requiring `--verbose` or
`--comment`. A clean run has no `StatusError` sections, so
`printFailedSectionDetail` prints nothing extra in that case.

`ReportSection.Body` is always GitHub-flavored markdown built for the PR
comment's `<details>`/`<summary>` dropdown renderer (literal HTML tags,
`&nbsp;` indentation, `**bold**`), which would show up as raw markup on a
plain terminal. `printFailedSectionDetail` runs each errored section's
`Body` through `SanitizeSectionBodyForConsole`
(`pkg/pipeline/console_format.go`) first — converting `<summary>X</summary>`
to a plain `X:` label and stripping the rest — and prints the result via
`Logger.Raw` rather than `Info`, since it's a pre-formatted block, not a
single structured log line.

### Pipeline exit code (`pipeline.Run`, `Result.Failed`)

`validator.RunAll`'s returned `error` is only ever non-nil for a hard setup
failure (e.g. failing to resolve the changeset) — a run that completes but
finds blocking/error-level findings still returns a nil error, signaling
that instead via `Result.Failed()` (`pkg/validator/types.go`): `Blocking`
(set from Resource Compliance direct findings, or a blocking ghost patch)
OR the run's own `Logger.HasFailures()` (sees any `Error`/`ErrorInSection`
call across every phase — Linting, Static Checks, Kustomize Build, ...).
`pipeline.Run`'s `validatorResultFailed` helper (and `test`'s own exit
check in `cmd/k8s-gitops-ci/main.go`) both delegate to `Result.Failed()`
rather than keeping their own copies of this OR, so they can't drift apart
the way they once did: `Result.Failed()` was added specifically because
Kustomize Fix findings rendered as a `StatusError` ("❌") sub-dropdown in
the report (`composeKustomizeFixChild`) but `runBuildAndPostBuild` never
called `log.ErrorInSection` for them — unlike every sibling check in that
same section (Overlay Build errors, blocking Ghost Patches) — so neither
`Blocking` nor `Logger.HasFailures()` ever saw them, and both `pipeline`
and `test` reported success despite a real ❌ in the printed report.
`Run` returns a non-nil error (and the CLI exits non-zero) whenever
`Result.Failed()` is true, in addition to the PR-validation error fields
(`TitleErr`/`UnsignedErr`) it already checked. `test --quiet` is deliberately
exempt from all of this — it's a pure reporting tool (prints only failing
sections, never returns a failing exit code).

### Unified PR-comment report (`pkg/validator/unified_report.go`, `compose_sections.go`)

The PR comment is a single markdown `Report` (one `<!-- marker -->`-tagged
comment, upserted via `pkg/github`) built from a flat `[]ReportSection` —
the top-level collapsible `<details>` blocks a reader sees first.
`ReportSection{Name, Status, Summary, Body}` is the **one** section type
end to end, used for both top-level "Expand: `<Name>`" sections
(`Report.Sections`/`Result.Sections`) and every nested sub-check dropdown
underneath one - there used to be a separate, bool-only `Section{Name,
Body, Error}` type for the top level, which meant a section could only
ever render ✅ or ❌ and had no way to represent "worth a look, but not
blocking" (see the "Regression: worst-case status must roll up to the
parent" note below for exactly what that caused). `Status` is a
`SectionStatus` (`pkg/validator/unified_report.go`): `StatusPassed` /
`StatusInfo` / `StatusWarning` / `StatusError`, ordered least-to-most-severe
and each with its own icon —
`✅`/`ℹ️`/`⚠️`/`❌` — via `SectionStatus.Icon()`; `Report.Render()` uses a
section's own `Status.Icon()` directly for its `<summary>` line, falling
back to `Summary` when `Body` is empty (the "passed, nothing more to
say" case).

Each top-level `ReportSection` is composed by a `Compose*Section` function
in `compose_sections.go` (`ComposeLintingSection`,
`ComposeStaticChecksSection`, `ComposeKustomizeBuildSection`,
`ComposeScaffoldValidationSection`, `ComposeResourceComplianceSection`,
...), and `pipeline.go`'s `composeSections` assembles the final ordered
list — reusing sections `phases.go` already built onto
`Result.Sections` by name (`validatorSectionOrFallback`) rather than
re-deriving/re-composing them a second time with different (often
stub) inputs. The one exception is CI Notes
(`ComposeCINotesSection`), whose body `composeSections` builds directly:
always the tool version (`version.String()`), plus an "Org version:" bullet
from `opts.Providers.OrgVersion()` when a `Branding` provider supplies one
(generic builds omit it).

`renderSubDropdown` renders one `ReportSection` as a nested `<details>` at
an arbitrary depth (`summaryIndent` adds `&nbsp;`-padding per level, since
GitHub doesn't indent `<details>` bodies); `composeParentFromChildren`
renders a list of children this way and rolls **the most severe status
among them** up into the parent's own `Status` (`if c.Status > status {
status = c.Status }` — safe because the four statuses are declared in
increasing-severity `iota` order), so a parent section's icon always
reflects the worst thing inside it: one `StatusWarning` child among
otherwise-`StatusPassed` siblings rolls the parent up to ⚠️, never a
misleadingly-plain ✅ (which would hide it) or an overstated ❌.
`CheckOutcome{Name, Status, Skipped, Note}` records whether an individual
lint/static check ran, was skipped, or passed, so **every**
linter/static-check always renders its own sub-dropdown (via
`composeCheckChild`) — even when everything passed — instead of silently
vanishing once there's nothing to report.

> **Regression: worst-case status must roll up to the parent.** Before
> the `Section`/`ReportSection` unification, `ComposeKustomizeBuildSection`
> and `ComposeScaffoldValidationSection` built their sub-checks as raw
> markdown bullets/dropdowns instead of `ReportSection` children, so (a) a
> non-blocking-only Ghost Patches/Pre-Existing-Scaffold-Drift finding could
> render with no icon at all on its own dropdown line (a bare
> `RenderSubDropdown(title, body)` call, which by design carries no
> status icon — that's still the right helper for a one-off nested detail
> block that isn't itself a `ReportSection`, e.g. a fix-hint sub-block, but
> the wrong one for a sub-check that needs its own pass/fail signal), and
> (b) even once every sub-check got its own icon, the _parent_ section
> still only had a bare `Error bool` and so could only ever show ✅ or ❌ —
> a warning-only child still rolled the parent all the way up to ❌ (before
> the icon fix) or hid it entirely behind a plain ✅ (after, since the
> child's own ⚠️ never affected `hasError`). Both `ComposeKustomizeBuildSection`
> and `ComposeScaffoldValidationSection` are now built the same way
> `ComposeLintingSection`/`ComposeStaticChecksSection`/`ComposePRChecksSection`
> already were: a list of `ReportSection` children (`composeOverlayBuildChild`,
> `composeHooksChild`, `composeKustomizeFixChild`, `composeGhostPatchesChild`;
> `composeScaffoldDriftChild`, `composeScaffoldExecChild`,
> `composePreExistingDriftChild`, `composeClusterCoverageChild`) fed through
> `composeParentFromChildren`, so every sub-check gets a single icon-bearing
> `<details>` line and the parent's icon is structurally guaranteed to
> reflect the worst one, rather than relying on every call site remembering
> to keep a separate bullet and an `Error`/`Warning` bool in sync by hand.

Three sections carry real, richer per-check data instead of a placeholder
count:

- **Kustomize Build** (`ComposeKustomizeBuildSection`) — always renders
  four children: Overlay Build, Hooks, Kustomize Fix, Ghost Patches. The
  overlay set itself is resolved by `detectOverlaysForChanges`
  (`pkg/validator/build_wiring.go`), which is app-aware rather than a bare
  "any path segment literally named `overlays/`" match: it finds each
  changed app root (`detectAppRoots`), asks `overlay.GetOverlaysToTest`
  whether the change is cluster-specific or a base/component change (which
  could affect every overlay of that app), and for the latter narrows that
  down via `overlay.FilterOverlaysByRefs` (which parses each overlay's
  kustomization `resources`/`components`/`bases` reference chain via
  `kustomize.ResolveRefs` to see whether it actually depends on the changed
  directory) - so a change to a shared base file, with no `overlays/`
  segment anywhere in its path, still resolves to the right overlay(s)
  instead of silently producing zero. Build errors are then grouped by root
  cause (`groupBuildErrors`/`formatBuildErrors`, so
  N overlays sharing one underlying cause don't repeat it N times) into the
  Overlay Build child's `StatusError` body; Hooks renders a
  `| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |` table (✅ ran /
  ❌ failed / — not defined — hooks are actually executed, not just
  detected; see [`HOOKS.md`](HOOKS.md)) with `StatusError` when any hook
  actually failed (`anyHookFailed`); Kustomize Fix (`kustomize.CheckFix`,
  which shells out to the real `kustomize edit fix --vars` - see
  `pkg/kustomize`'s package doc comment for why, and its `StatusError` on
  a `CheckFix` failure itself, e.g. a missing `kustomize` binary, which
  this repo treats as a hard failure rather than the graceful skip every
  `pkg/lint/*` wrapper's own missing-CLI handling uses) lists files
  needing a fix, plus a `k8s-gitops-ci kustomize-fix -dir <dir>` fix
  command per affected directory (real and working, unlike the
  never-actually-reachable `hintByCheck["kustomize fix"]` entry in
  `comments.go` - nothing produces a `LintFinding` with that check name);
  and Ghost Patches renders a
  `| Overlay | Target |` table (`pkg/ghostpatch.CheckApp`, which renders
  overlays via the krusty SDK directly — no runtime dependency on a
  `kustomize` binary being present) with `StatusError` only when at least
  one detected ghost is blocking, `StatusWarning` otherwise (see
  [`CI.md`](CI.md#ghost-patch-detection)).
- **Scaffold Validation** (`ComposeScaffoldValidationSection`) — always
  renders four children: Scaffold Drift, Scaffold Exec (both
  `StatusError` on failure), Pre-Existing Scaffold Drift, and Cluster
  Coverage. Real per-app scaffold-drift detection across three triggers
  (template, config, and overlay changes — see
  [`CI.md`](CI.md#scaffold-validation)). A mismatch the PR doesn't itself
  touch is checked against the merge-base template/config
  (`computeBaselineMismatches`) and rendered as a separate,
  `StatusWarning` "Pre-Existing Scaffold Drift" child when it drifts
  there too; skipped/not-yet-rolled-out clusters render as a
  `StatusInfo` "Cluster Coverage" child (deliberately quieter than
  `StatusWarning` — per `scaffold.Run`'s own doc comment, a skip is
  informational, never a finding). (The README scaffold-status table's
  own structural check lives in Static Checks as "scaffold table", not
  here — see below.)
- **Resource Compliance** (`ComposeResourceComplianceSection`) — findings
  grouped by `CheckID` into per-check nested `<details>` (❌ when a check
  has a finding on a directly-modified resource — blocking — vs ⚠️ for a
  pre-existing, non-blocking finding only), ordered by the fixed
  `complianceCheckOrder` (`register_tables.go`, image-checksum first) with
  any check not in that list appended alphabetically
  (`orderedComplianceIDs`), plus an "Accepted Exemptions" audit sub-block
  (`renderAcceptedExemptions`, table `| Resource | Value | Scope |`) built
  from applied exemptions (`check.Result.Exempted` /
  `[]exempt.Applied`), labeled `(pre-existing)` when none of the
  exemptions were applied to a directly-modified resource. The section's
  own top-level `Status` is `StatusError` when any blocking finding
  exists, `StatusWarning` for warning-only findings, and `StatusInfo` when
  only exemptions are present (no findings at all) — an audit trail worth
  a glance, but not an active warning. Each check-ID group's table
  (`renderComplianceSub`) uses that check's registered `check.TableSpec`
  (`register_tables.go`'s `checkTableSpecs`) when one exists — its own
  descriptive title/preamble and columns, e.g. `image-checksum` renders
  Kind/Name/Image rather than a flat two-column dump — falling back to a
  generic `| File | Message |` table for any check id without one.
  Findings that are the same underlying resource/issue fanned out across
  many rendered overlays (identical Kind/Name/Message etc., differing only
  in `File` - see `engine.go`'s per-unique-document fan-out) are deduped by
  resource identity (`dedupComplianceRows`) into a single row carrying an
  **Overlays** column (a count of the distinct overlays it spans, or the
  single built-file label when it appears in just one); blocking
  sub-sections additionally gain a **Source File(s)** column. The header's
  `(N finding(s))` count reflects those deduped **unique** issues, not the
  raw per-overlay fan-out. A check with a `TableSpec` but no resource key
  (e.g. `placeholder`) instead keeps the file-list dedup
  (`dedupFindingsForTable`, one row whose File cell lists every distinct
  location).

## Building

```sh
task build
```

Builds `bin/k8s-gitops-ci` with version metadata injected via `-ldflags`
(see `Taskfile.yml`'s `LDFLAGS` var and `cmd/version`). **Do not** run
`go build ./cmd/k8s-gitops-ci` directly for anything other than a quick
compile check — you'll get a binary with no version info, and you'll miss
whatever `task build`'s dependencies do (currently just `mod`; this may
grow to include embedded-resource generation, so prefer `task build` even
when it looks equivalent to the raw command today).

This repo's own release process (versioning, published artifacts) and CI
infrastructure (the Tekton pipeline that actually runs `task ci` and
these release steps) are covered in [`RELEASE.md`](RELEASE.md) and
[`TEKTON.md`](TEKTON.md) — distinct from this section, which is about
building the binary locally.

## Testing

```sh
task test          # go test ./...
task test:cover     # + coverage profile, race detector
task coverage:report
task coverage:html
```

Every package's tests live alongside its source
(`pkg/foo/foo_test.go`), and prefer table-driven tests over one-off
`Test*` functions per case, matching the existing style.

### Test performance: caching, parallelism, and shared fixtures

`task test`/`task test:cover` deliberately avoid several things that
silently defeat Go's own test-result cache or serialize otherwise-
parallelizable work — worth understanding before adding a `GOFLAGS`/
`GOTESTFLAGS` default or a new expensive per-test fixture:

- **No forced `-count=1` or `-shuffle=on` default.** Per `go help test`,
  a test run is only cache-eligible when every flag on the command line
  comes from a small "cacheable" set (`-run`, `-short`, `-timeout`,
  `-v`, `-parallel`, `-cpu`, `-list`, `-coverprofile`, `-outputdir`,
  ...) — `-shuffle` is _not_ in that set, so passing it at all disables
  caching unconditionally, regardless of `-count`. Both were previously
  forced on by default in `Taskfile.yml`, meaning `go test ./...` never
  hit its own cache even on a fully unchanged tree. `GOFLAGS`/
  `GOTESTFLAGS` still default to empty/`-mod=mod` and remain overridable
  per-invocation (e.g. `task test GOTESTFLAGS=-shuffle=on` to check for
  order-dependent tests on demand) — just not on by default.
- **The embedded archives `go:embed`ed by `pkg/lint/kubeconform/schemas`
  and `pkg/lint/kyverno/policies` must be byte-reproducible.**
  `scripts/pull-schemas.sh`/`pull-policies.sh` build them with
  `tar --sort=name --mtime=... --owner=0 --group=0 --numeric-owner`
  piped through `gzip -n`, stripping every source of run-to-run
  non-determinism (directory-walk order, checkout/generation-time
  mtimes, local uid/gid, gzip's own header timestamp). Regenerating
  either archive without these flags gives it different bytes on every
  run even when its logical content is unchanged - which invalidates
  Go's build/test cache for every package that embeds it (`pkg/pipeline`,
  `pkg/lint/kyverno`, `pkg/validator`, `cmd/k8s-gitops-ci`, ...) on
  every single CI invocation. See [SCHEMAS.md](SCHEMAS.md) for the
  matching `.ref`-marker short-circuit that skips regenerating them at
  all when the pin/content hasn't changed.
- **`go test`'s cache keys file inputs on mtime+size, never content** —
  a fact with no bearing on local dev (you're not re-cloning between
  runs), but critical for CI: the `.tekton/k8s-gitops-ci.yaml` build
  step's own source checkout is persisted on the `go-cache` PVC and
  reused via `git fetch` + `git reset --hard` + `git clean -fd` instead
  of a fresh `git init` every run, specifically so unchanged files keep
  their prior mtime and `go test` can hit its cache on a rerun that
  touches unrelated files. See [TEKTON.md](TEKTON.md#caching) for the
  full mechanism and `go doc -src cmd/go/internal/test.hashOpen` for the
  underlying Go behavior (also refuses to cache a file read within 2
  seconds of its mtime at all, via `modTimeCutoff`/`errFileTooNew`).
- **`pkg/validator` shares one kubeconform schema extraction across its
  whole test binary, via `TestMain` (`pkg/validator/main_test.go`).**
  `RunAll` calls `kubeconform.ExtractSchemas()` whenever it reaches the
  kubeconform phase without a pre-set `Options.SchemaDir` - and this
  package alone has dozens of tests that call `RunAll`. Left to extract
  independently, each call unpacks the embedded archive (108MB across
  ~3100 files) into its own fresh temp dir; real CI timing showed 19
  separate extractions costing 227s combined, by far the largest single
  cost in the package's test suite. `TestMain` extracts once via the
  exported `kubeconform.ExtractSchemas` override seam (the same seam
  `kubeconform.TestExtractSchemas_IsOverridable` guards) and points every
  call at that one shared, read-only directory for the rest of the run.
  This doesn't reduce coverage of the real extraction path itself -
  that's exercised directly, independently of this package, by
  `pkg/lint/kubeconform`'s own `TestExtractSchemas_ContainsExpectedSubdirs`
  and `TestExtractSchemas_IsOverridable`.
- **`t.Parallel()` is used throughout `pkg/validator`, with two hard
  exclusions.** Most of the package's ~265 tests call `t.Parallel()`.
  Excluded, and must stay excluded for any new test added to this
  package:
  1. Anything using `t.Chdir`/`os.Chdir` — process-global, and `t.Chdir`
     itself panics inside a parallel test.
  2. Anything that calls `RunAll`, or otherwise writes a package-level
     var — `RunAll` itself mutates `syncopts.AssumeOpenShift` and
     `ClusterIndexProvider` on every call (see `validator.go`,
     `register_checks.go`), and several tests directly assign other
     unexported package globals (e.g. `hookBuildRoot` in
     `hook_wiring_test.go`, `DefaultEnabledChecks` in `phases_test.go`).
     None of this is enforced by the compiler or `go vet` — `-race`
     will only catch it when two such tests happen to run concurrently,
     so **grep for `RunAll(` and for direct assignment
     (`someVar = ...`) to any package-level var before adding
     `t.Parallel()` to a new test in this package**, don't assume a test
     is safe just because its own body looks side-effect-free.

### Negative / error-path coverage

`testdata/invalid/` (see [Adding a new
validator](#adding-a-new-validator) above) is this repo's convention for
deliberately-malformed/false-positive-guard fixtures — inputs that
superficially resemble a violation but must yield zero findings. Every
`pkg/validator/*` package that has one was backfilled with real
`testdata/`/`testdata/invalid/` fixtures during a parity pass; current
coverage (`task coverage:report`, `go test ./... -cover`) for that pass's
packages:

| Package                 | Coverage | Notes                                                                                                                                                                                                                                    |
| ----------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `validator/namedport`   | 87.8%    | —                                                                                                                                                                                                                                        |
| `validator/podspec`     | 87.7%    | —                                                                                                                                                                                                                                        |
| `validator/image`       | 81.8%    | —                                                                                                                                                                                                                                        |
| `validator/syncopts`    | 90.8%    | —                                                                                                                                                                                                                                        |
| `validator/placeholder` | 97.1%    | —                                                                                                                                                                                                                                        |
| `validator/rbac`        | 87.2%    | No logic bug found here — pure test backfill.                                                                                                                                                                                            |
| `lint/shellcheck`       | 70.1%    | Lower because `RunTekton`/`RunEmbedded`'s end-to-end paths skip cleanly (not fail) when no `shellcheck` binary is on `PATH` — they run fully in the real CI image (see [TEKTON.md](TEKTON.md)), just not in every local dev environment. |

Every package above also now has `TestValidateFile_*`/
`TestValidateFile_MissingFile` coverage over the `ValidateFile`
code path specifically (previously only `ValidateReader`/`ValidateBytes`
were exercised in most of them). Don't re-derive these percentages from
memory in a future doc update — re-run `task coverage:report`/
`go test ./... -cover` and paste the fresh numbers; they will drift as
these packages keep changing.

### End-to-end / regression replay

Beyond the unit suite, `task test:homelab-prs` runs the freshly-built
binary against the last _N_ **merged** PRs of a real GitOps repository
(default [`ArthurVardevanyan/HomeLab`](https://github.com/ArthurVardevanyan/HomeLab),
the corpus this engine was designed around) and prints a Markdown
pass/fail report. It shells out to
[`scripts/test-homelab-prs.sh`](../scripts/test-homelab-prs.sh),
invoking `k8s-gitops-ci pipeline --url <repo> --pr <n>` per PR — the same
`pipeline` entrypoint production and any downstream consumer use, so a
real-world break surfaces as a real failure. `GH_TOKEN` is scrubbed for
the binary (it can never post a comment) and `avp` is disabled by default
(no secret backend for a public repo).

```sh
task test:homelab-prs                       # last 15 HomeLab PRs
task test:homelab-prs COUNT=5               # fewer, faster
task test:homelab-prs OUTPUT=report.md      # write a Markdown report
task test:homelab-prs REPOS=some/other-repo # a different corpus
```

**This is a smoke/acceptance gate, not a deterministic regression lock.**
Understand its limits before relying on it:

1. **Non-deterministic.** It reads live GitHub state; HomeLab's merged-PR
   history can (and does) move under you, so a rerun isn't guaranteed to
   test the same inputs. It needs network access and a working `gh` auth.
2. **Not a behavior lock.** It only asserts each PR's overall pass/fail —
   there is no golden baseline, so a _silent_ change in a check's output
   or findings (one that doesn't flip pass↔fail) will slip through
   undetected.
3. **Edge coverage is incidental.** It exercises only whatever a real PR
   happened to touch; rare validator edges no PR has hit are simply never
   covered.
4. **Intentional changes can fail old PRs.** A newer, stricter check may
   legitimately fail a PR that merged cleanly under the old rules — a red
   result here is a prompt to _eyeball the diff_, not automatically a
   regression.
5. **Blind to false negatives.** Every input in the corpus is a _merged_
   PR — i.e. expected to **pass**. So the replay can flag a PR that
   _newly fails_ (a false **positive** — the tool became too strict), but
   it is structurally incapable of catching a false **negative**: a check
   that silently stops firing (returns "pass" when it should fail) keeps
   every replay green forever. This is the more dangerous class, and it is
   **not** this tool's job — see the coverage split below.

#### Coverage: who catches what

| Failure class                   | Live replay (all-pass corpus)    | Unit tests (`testdata/invalid/`, `*_wiring_test.go`, `phases_test.go`)                         | Golden e2e (deferred) |
| ------------------------------- | -------------------------------- | ---------------------------------------------------------------------------------------------- | --------------------- |
| **False positive** (too strict) | Partial — a formerly-green PR ❌ | ✅ (`testdata/` valid inputs assert zero findings)                                             | ✅                    |
| **False negative** (too lax)    | ❌ **blind**                     | ✅ per-check (`wantErrs: 1` / `expected exactly 1 finding`) + wiring (`RunAll` `res.Sections`) | ✅ end-to-end         |

False negatives are the **unit suite's** responsibility: each validator's
`testdata/invalid/` fixtures assert findings _fire_, and
`phases_test.go`/`*_wiring_test.go` assert each check is actually reached
during `RunAll`. The replay is explicitly a **false-positive smoke
gate**, not a false-negative net — treat it as complementary to, never a
substitute for, the unit tests.

The one residual gap even the unit + wiring tests don't close is fully
black-box: they call `RunAll` in-process, so they prove a check _fires_
but not that it fires **through the built binary end to end**. Only a
golden run over deliberately-broken fixtures (see "Future work" below)
would catch a check that's correct in isolation yet dropped from the
compiled pipeline.

Its intended use is a **re-pin / release acceptance gate** (does the
current engine still pass real-world PRs?) and ad-hoc smoke testing —
not a substitute for unit tests.

#### In CI

The replay also runs automatically in this repo's own meta-CI, but
**non-blocking**. The `build` task in
[`.tekton/k8s-gitops-ci.yaml`](../.tekton/k8s-gitops-ci.yaml) runs
`task test:homelab-prs` (small `COUNT`) after `task ci` on pull-request
events and feeds the result — plus the blocking `task ci` verdict — into
a single self-CI status comment (`<!-- ci-self-report -->`) via the
`k8s-gitops-ci ci-report` subcommand. Only `task ci` blocks merge; a
replay failure surfaces as a ⚠️ section in that comment and never fails
the build (for all the reasons above — it reads live state and can't see
false negatives). See [TEKTON.md](TEKTON.md) for the pipeline wiring.

#### Future work: deterministic golden regression

If a silent regression (limitation #2) ever escapes into a release, the
targeted fix is a small **deterministic golden fixture** for that
specific case rather than reaching for the live replay. The design that
was scoped for this (and can be revived on demand):

- Vendor a tiny fixture as a **two-commit local git repo** (`base/` tree
  → first commit, `head/` tree → the change under test), snapshotted from
  a pinned upstream commit for realism.
- Drive it offline through the real pipeline via
  `pipeline --target-branch <base-sha>` (no `--url`/`--pr`), which falls
  through to a local `git diff <base>...HEAD` in `pkg/changeset` — no
  remote, no PR, no GitHub — plus optionally `test <dir>` for the
  standalone whole-repo path.
- Normalize the console output down to the stable result surface (strip
  the version banner, `[HH:MM:SS]` timestamps, `(…ms)`/`(…s)` durations,
  the per-run timing footer, and absolute temp paths; keep section
  verdicts, `[WARN]`/finding lines, and the exit code) and compare
  against a committed `*.golden`, with an `UPDATE_GOLDEN=1` regenerate
  path for reviewed intentional changes.

The **known cost** is that normalization: run-to-run non-determinism
(timestamps, durations, and goroutine-ordering of parallel-phase log
lines) has to be scrubbed carefully or the goldens flake. That tax — plus
the vendored fixture files — is why this tier was deferred in favor of the
live replay above; add it only when a concrete silent-regression escape
justifies the maintenance.

## Task Targets Reference

See [`CI.md`](CI.md) for what `task ci`'s underlying pipeline
(`k8s-gitops-ci pipeline`/`ci`) actually checks once built — this table
is about the local dev-loop `task` targets themselves.

| Target                    | Purpose                                                                                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `version:check`           | Validate the `VERSION` file (semver; never behind latest tag/`main` unless this PR doesn't touch `VERSION`; not an under-bump for the commits since the last release)            |
| `mod`                     | Download and tidy Go modules                                                                                                                                                     |
| `mod:check`               | Ensure `go.mod`/`go.sum` are tidy without modifying the working tree                                                                                                             |
| `mod:verify`              | Verify Go module integrity and checksums                                                                                                                                         |
| `install-tools`           | Install all Go development tools to `.tool/`                                                                                                                                     |
| `install-tools:go`        | Install Go tooling (golangci-lint, govulncheck, gofumpt, goimports)                                                                                                              |
| `format`                  | Auto-format Go code (goimports + gofumpt) and tidy modules                                                                                                                       |
| `format:check`            | Check Go formatting without modifying files (fails if changes needed)                                                                                                            |
| `lint`                    | Run golangci-lint                                                                                                                                                                |
| `vet`                     | Run `go vet`                                                                                                                                                                     |
| `vulncheck`               | Check for known vulnerabilities (only fails on fixable findings)                                                                                                                 |
| `build`                   | Build `bin/k8s-gitops-ci`                                                                                                                                                        |
| `test`                    | Run test suite                                                                                                                                                                   |
| `test:cover`              | Run tests with coverage profile + race detector                                                                                                                                  |
| `test:race`               | Alias for `test:cover` (race detector runs there)                                                                                                                                |
| `test:homelab-prs`        | Replay the last _N_ merged PRs of a real GitOps repo (default HomeLab) — smoke gate, see [Testing](#end-to-end--regression-replay)                                               |
| `coverage:report`         | Print per-file coverage report                                                                                                                                                   |
| `coverage:html`           | Generate HTML coverage report and open in browser                                                                                                                                |
| `ci`                      | Full CI pipeline — version check, mod check, format, schemas, upstream refs, lint, vulncheck, test, build, smoke gate                                                            |
| `clean`                   | Remove build artifacts, caches, and temp files                                                                                                                                   |
| `update`                  | Run all update tasks (deps, schemas/policies pin bump + repack, scoped-resources)                                                                                                |
| `update:deps`             | Upgrade all Go dependencies and tidy `go.mod`/`go.sum`                                                                                                                           |
| `schemas:pull`            | Repack embedded kubeconform schemas from the pinned `SCHEMA_REPO_SHA` (see `docs/SCHEMAS.md`)                                                                                    |
| `policies:pull`           | Repack embedded Kyverno policies (placeholder by default — see `docs/SCHEMAS.md`)                                                                                                |
| `update:checks`           | Regenerate the registered runtime checks snapshot (`testdata/registered_checks.txt`)                                                                                             |
| `update:schemas`          | Bump the pinned `SCHEMA_REPO_SHA` in `scripts/pull-schemas.sh` to the `SCHEMA_REPO_BRANCH` tip (see `docs/SCHEMAS.md`)                                                           |
| `update:policies`         | Bump the pinned policy ref (no-op while policies are a static placeholder — see `docs/SCHEMAS.md`)                                                                               |
| `update:scoped-resources` | Regenerate `resource_scope.go`/`extra_resource_scope.go` from a live cluster's `kubectl api-resources`                                                                           |
| `verify:upstream-refs`    | Prove every runtime validation check's cited upstream Kubernetes function still exists and is unchanged since it was validated (runs as a step in `task ci`; fetches are cached) |

Run `task --list` for the authoritative, up-to-date list.
