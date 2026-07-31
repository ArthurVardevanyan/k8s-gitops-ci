# Development

This covers build/test/lint commands, repository structure, and the
design conventions that keep this a generic, org-agnostic core. See
[`ARCHITECTURE.md`](ARCHITECTURE.md) for the runtime shape (what actually
happens when you run a pipeline) and a "Where do I find X?" table
pointing to every other doc (`CI.md`, `HOOKS.md`, `EXCEPTIONS.md`,
`TEKTON.md`, `RELEASE.md`, `SECURITY.md`, `SCHEMAS.md`).

## Table of Contents

- [Prerequisites](#prerequisites)
- [Repository Structure](#repository-structure)
- [Design Conventions](#design-conventions)
- [CLI Output & PR Comment Format](#cli-output--pr-comment-format)
- [Building](#building)
- [Testing](#testing)
  - [Negative / error-path coverage](#negative--error-path-coverage)
- [Task Targets Reference](#task-targets-reference)

## Prerequisites

- Go (see `go.mod` for the exact version)
- [`task`](https://taskfile.dev) — all build/test/lint/update workflows go
  through `task`, not raw `go` commands (see [Building](#building) for why)

Optional, used by individual `pkg/lint/*` wrappers when present on `PATH`.
Every wrapper degrades gracefully (skips or reports a clear "CLI not
found" error, never panics) when its tool is missing, so none of these are
required just to build or run the unit test suite:

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
                     per-concern sub-packages: check (registry), exempt
                     (unified exemption framework), and one package per
                     validator (namespace, psa, rbac, crb, syncopts,
                     image, namedport, podspec, nad, placeholder,
                     clusterid)
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
    Branding        Branding        // report marker/title/header
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
`pkg/validator/psa`'s `NamespaceSelectorLabelKeys`-style seams, and
`pkg/validator`'s `TektonPACDir` (defaults to `.tekton`; set to `""` to
disable the built-in exemption that lets Tekton Pipelines-as-code-managed
`PipelineRun` manifests there skip the `sync-options`/`namespace` checks,
since PAC's own controller manages their lifecycle rather than Argo CD).
The contract is always **"empty/nil is a true no-op"** — never assume a
default value that changes behavior when unset.

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
either way.

### Generic check-enablement mechanism

Every gateable check/step — whether it's a `check.Register`-based
per-document validator (`namespace`, `psa`, `rbac`, ...) or a standalone
step like `golangci` or `kyverno` — has a string ID and participates in
one shared enable/disable mechanism instead of a dedicated boolean flag
per step:

- `Options.DisabledChecks []string` — turn off a step that defaults to
  **enabled** (most steps: `sync-options`, `golangci`, `avp`, ...).
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
   bucket — see [`EXCEPTIONS.md`](EXCEPTIONS.md)).
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
40-column variant. `RecordBuild`/`RecordPass`/`RecordFailure` (and their
`InSection` variants, for use from goroutines where the shared "current
section" may have been overwritten by another goroutine) feed a final
`Summary()` banner:

```text
============================================================
  RESULTS SUMMARY
============================================================
  Builds: 4 | Passes: 3 | Failures: 1
  Warnings: 2
  Errors: 1 (see details above)
  Failed sections:
    - Build+Compliance
============================================================
```

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
Linting                              1.203s  parallel
  golangci-lint                       980ms  parallel
  kubeconform                         640ms  parallel
Static Checks                         310ms  parallel
Build+Compliance                     2.010s  parallel
--------------------------------------------------------------
TOTAL (wall)                         3.523s
TOTAL (sum)                          3.523s
Parallelism                             2.1x
Concurrency                                8  (4 CPUs × 2)
--------------------------------------------------------------
```

### `--verbose` console detail without `--comment` (`pkg/pipeline/pipeline.go`)

Every phase composes a detail-bearing `Section` (the actual file/message
list behind a step's summary "N violation(s)" log line) onto
`ValidatorResult.Sections`, but that has historically only ever been
rendered into the PR comment body via `composeSections`/`postComment` —
which is skipped whenever `--comment` isn't passed (e.g. a local/CLI-only
run). `pipeline.Run` calls `printFailedSectionDetail` under `--verbose`
(independent of whether a comment is posted) to print every errored
section's full `Body` to the console too, so `--verbose` alone is enough
to see e.g. exactly which file/check produced a Resource Compliance
finding, not just the count.

### Pipeline exit code (`pipeline.Run`)

`validator.RunAll`'s returned `error` is only ever non-nil for a hard setup
failure (e.g. failing to resolve the changeset) — a run that completes but
finds blocking/error-level findings still returns a nil error, signaling
that instead via `ValidatorResult.Blocking` (set from Resource Compliance
direct findings) and the validator's own `Logger.HasFailures()` (sees any
`Error`/`ErrorInSection` call across every phase — Linting, Static Checks,
Kustomize Build, ...). `pipeline.Run`'s `validatorResultFailed` helper
checks both, and `Run` returns a non-nil error (and the CLI exits non-zero)
whenever either is true, in addition to the PR-validation error fields
(`TitleErr`/`UnsignedErr`) it already checked.

### Unified PR-comment report (`pkg/validator/unified_report.go`, `compose_sections.go`)

The PR comment is a single markdown `Report` (one `<!-- marker -->`-tagged
comment, upserted via `pkg/github`) built from a flat `[]Section`
(`Name`, `Body`, `Error bool`) — the top-level collapsible `<details>`
blocks a reader sees first. Each top-level `Section` is composed by a
`Compose*Section` function in `compose_sections.go` (`ComposeLintingSection`,
`ComposeStaticChecksSection`, `ComposeKustomizeBuildSection`,
`ComposeScaffoldValidationSection`, `ComposeResourceComplianceSection`,
...), and `pipeline.go`'s `composeSections` assembles the final ordered
list — reusing sections `phases.go` already built onto
`Result.Sections` by name (`validatorSectionOrFallback`) rather than
re-deriving/re-composing them a second time with different (often
stub) inputs.

Within a top-level `Section`, individual sub-checks nest as their own
collapsible `<details>` via the richer `ReportSection{Name, Status,
Summary, Body}` type (`Status` is a `SectionStatus`: `StatusPassed` /
`StatusInfo` / `StatusWarning` / `StatusError`, each with its own icon —
`✅`/`ℹ️`/`⚠️`/`❌` — via `SectionStatus.Icon()`). `renderSubDropdown`
renders one `ReportSection` at an arbitrary nesting depth (`summaryIndent`
adds `&nbsp;`-padding per level, since GitHub doesn't indent `<details>`
bodies); `composeParentFromChildren` renders a list of children and rolls
their statuses up into the parent `Section.Error`. `CheckOutcome{Name,
Status, Skipped, Note}` records whether an individual lint/static check
ran, was skipped, or passed, so **every** linter/static-check always
renders its own sub-dropdown (via `composeCheckChild`) — even when
everything passed — instead of silently vanishing once there's nothing to
report.

Three sections carry real, richer per-check data instead of a placeholder
count:

- **Kustomize Build** (`ComposeKustomizeBuildSection`) — the overlay set
  itself is resolved by `detectOverlaysForChanges`
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
  N overlays sharing one underlying cause don't repeat it N times), a
  `| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |` hooks table (✅ ran
  / ❌ failed / — not defined — hooks are actually executed, not just
  detected; see [`HOOKS.md`](HOOKS.md)), files needing `kustomize edit
fix`, and a `| Overlay | Target |` ghost-patch table
  (`pkg/ghostpatch.CheckApp`, which renders overlays via the krusty SDK
  directly — no runtime dependency on a `kustomize` binary being
  present).
- **Scaffold Validation** (`ComposeScaffoldValidationSection`) — real
  per-app scaffold-drift detection across three triggers (template,
  config, and overlay changes — see [`CI.md`](CI.md#scaffold-validation)),
  plus README scaffold-status table structural-consistency checking
  (`pkg/scaffold.CheckReadmeStatus`).
- **Resource Compliance** (`ComposeResourceComplianceSection`) — findings
  grouped by `CheckID` into per-check nested `<details>` (❌ when a check
  has a finding in a directly-modified file — blocking — vs ⚠️ for a
  pre-existing, non-blocking finding only), sorted alphabetically by check
  ID (this generic core has no fixed, org-defined check ordering to
  hardcode), plus an "Accepted Exceptions" audit sub-block
  (`renderAcceptedExceptions`, table `| Resource | Value | Scope |`) built
  from applied exemptions (`check.Result.Exempted` /
  `[]exempt.Applied`), labeled `(pre-existing)` when none of the
  exemptions were applied to a directly-modified resource.

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

## Task Targets Reference

See [`CI.md`](CI.md) for what `task ci`'s underlying pipeline
(`k8s-gitops-ci pipeline`/`ci`) actually checks once built — this table
is about the local dev-loop `task` targets themselves.

| Target                    | Purpose                                                                                                |
| ------------------------- | ------------------------------------------------------------------------------------------------------ |
| `mod`                     | Download and tidy Go modules                                                                           |
| `mod:check`               | Ensure `go.mod`/`go.sum` are tidy without modifying the working tree                                   |
| `mod:verify`              | Verify Go module integrity and checksums                                                               |
| `install-tools`           | Install all Go development tools to `.tool/`                                                           |
| `install-tools:go`        | Install Go tooling (golangci-lint, govulncheck, gofumpt, goimports)                                    |
| `format`                  | Auto-format Go code (goimports + gofumpt) and tidy modules                                             |
| `format:check`            | Check Go formatting without modifying files (fails if changes needed)                                  |
| `lint`                    | Run golangci-lint                                                                                      |
| `vet`                     | Run `go vet`                                                                                           |
| `vulncheck`               | Check for known vulnerabilities (only fails on fixable findings)                                       |
| `build`                   | Build `bin/k8s-gitops-ci`                                                                              |
| `test`                    | Run test suite                                                                                         |
| `test:cover`              | Run tests with coverage profile + race detector                                                        |
| `test:race`               | Alias for `test:cover` (race detector runs there)                                                      |
| `coverage:report`         | Print per-file coverage report                                                                         |
| `coverage:html`           | Generate HTML coverage report and open in browser                                                      |
| `ci`                      | Full CI pipeline — mod check, format, schemas, lint, vulncheck, test, build                            |
| `clean`                   | Remove build artifacts, caches, and temp files                                                         |
| `update`                  | Run all update tasks (deps, schemas, policies, scoped-resources)                                       |
| `update:deps`             | Upgrade all Go dependencies and tidy `go.mod`/`go.sum`                                                 |
| `update:schemas`          | Pull embedded kubeconform schemas (see `docs/SCHEMAS.md`)                                              |
| `update:policies`         | Pull embedded Kyverno policies (placeholder by default — see `docs/SCHEMAS.md`)                        |
| `update:scoped-resources` | Regenerate `resource_scope.go`/`extra_resource_scope.go` from a live cluster's `kubectl api-resources` |

Run `task --list` for the authoritative, up-to-date list.
