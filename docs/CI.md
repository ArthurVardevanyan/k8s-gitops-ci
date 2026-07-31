# CI Pipeline

This document describes what `k8s-gitops-ci pipeline`/`ci` (and the
standalone subcommands) actually do today. See
[ARCHITECTURE.md](ARCHITECTURE.md) for the one-paragraph overview and
package map; this is the detailed, step-by-step reference.

## Pipeline flow

`pkg/validator/phases.go`'s `RunAll` runs two or three phases, in order:

```mermaid
flowchart TD
    A[Resolve Changeset] --> B["Linting (parallel)"]
    B --> C["Static Checks (parallel)"]
    C --> D{LintOnly?}
    D -- yes --> H[Report]
    D -- no --> E["Build Overlays + Resource Compliance"]
    E --> H[Report]
```

- **Linting** and **Static Checks** each fan every one of their steps out
  across goroutines (bounded by `Workers(opts)` — see
  [Concurrency](#concurrency) below) rather than running sequentially;
  every step still gets recorded (pass/fail/skip) in the report even when
  it found nothing wrong, so a linter never silently disappears from the
  output.
- **Build Overlays + Resource Compliance** only runs unless
  `--lint-only` is passed. It resolves the affected overlay set
  (`detectOverlaysForChanges` — see [ARCHITECTURE.md](ARCHITECTURE.md)),
  runs the doc-scoped and overlay-scoped checks (see
  [Registered Checks](#registered-checks) below) concurrently across a
  bounded worker pool, and classifies every finding as blocking
  (direct) or warning-only (external) — see
  [Direct vs. external findings](#direct-vs-external-findings).

## Modes

| Mode                  | Command                                                            | Changeset source                                                                                                                                                                                                                                                                                                                                                                                                |
| --------------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Full pipeline         | `k8s-gitops-ci pipeline --url <repo> --pr <n>` (alias: `ci`)       | PR's changed files via the GitHub API                                                                                                                                                                                                                                                                                                                                                                           |
| Local PR check        | `k8s-gitops-ci pipeline --revision <sha> --target-branch <branch>` | `git diff <target>...<revision>`                                                                                                                                                                                                                                                                                                                                                                                |
| Working-tree scan     | `k8s-gitops-ci test-all [dirs...]`                                 | every file under the given directories (not a diff — the full tree under each path)                                                                                                                                                                                                                                                                                                                             |
| Uncommitted-diff scan | `k8s-gitops-ci scan-all`                                           | **`git diff` + `git diff --cached`** in the current working tree — despite the name, this does **not** scan every file in the repository; it only sees uncommitted changes, since it passes no `--dirs`/`--url`/`--pr`/`--revision` and `resolveChangeset` falls back to a working-tree diff when none of those are set. To validate the _entire_ repository regardless of git state, use `test-all .` instead. |
| Ad-hoc overlay build  | `k8s-gitops-ci build-yaml --app <app> --cluster <cluster>`         | Same fallback as `scan-all` (uncommitted working-tree diff) — see the known limitation immediately below.                                                                                                                                                                                                                                                                                                       |

`--lint-only` (pipeline mode only) skips the Build Overlays + Resource
Compliance phase entirely — useful for a fast Linting/Static-Checks-only
pass.

> **Known limitation — `--app`/`--cluster` are currently unwired.** > `Options.Apps`/`Options.Clusters` are threaded all the way from the
> `pipeline`/`build-yaml` CLI flags down to `validator.Options`, but as of
> this writing **nothing in `pkg/validator` reads either field** to scope
> the changeset or the overlay set — `resolveChangeset` only ever looks
> at `Dirs`/`RepoURL`/`PR`/`BaseRef`/`IncludeDeletions`. (`pkg/scaffold.Run`
> takes its own, unrelated per-app `RunOptions.App` - see
> [Scaffold Validation](#scaffold-validation) below - and is unaffected by
> this limitation; it's driven entirely by the resolved changeset, not
> `Options.Apps`/`Clusters`.) Practically: `k8s-gitops-ci build-yaml --app
foo --cluster bar` today behaves identically to `k8s-gitops-ci scan-all`
> (an uncommitted-working-tree-diff scan across the whole repo) — the
> `--app`/`--cluster` values are silently accepted and then have no
> effect. Verify against the current code before relying on app/cluster
> scoping actually narrowing a run.

## Build Strategies

`pkg/validator/phases.go`'s Build + Compliance phase picks a
`pkg/overlay.Strategy` per app (`overlay.DetectStrategy`, wired via
`resolveAppBuildStrategies` in `pkg/validator/avp_wiring.go`) before
rendering any of that app's overlays:

- A `base/kustomization.yaml` → Kustomize, via the native Kustomize SDK
  (`sigs.k8s.io/kustomize/api/krusty`) — no runtime dependency on a
  `kustomize` binary. A `Chart.yaml` alongside it is still built via
  Kustomize (consumed through its own `helmCharts` inflator).
- No `kustomization.yaml` but a `base/Chart.yaml` → Helm, via the native
  chart loader + rendering engine (`helm.sh/helm/v3/pkg/...`) — no
  runtime dependency on a `helm` binary either, mirroring what
  `helm template` produces.
- Either way, if `DetectStrategy` finds an AVP indicator anywhere under
  the app (a direct `argocd-vault-plugin` reference, an
  `avp.kubernetes.io` annotation, or a `<path:...>`/`<vault:...>`/
  `<aws:...>`/`<gcp:...>` placeholder — see
  `overlay.AppHasAVPIndicators`), that overlay's rendered output is
  additionally piped through `argocd-vault-plugin generate -`, resolving
  those placeholders the way ArgoCD's real AVP plugin does at sync time
  — unless the overlay's basename is listed in that app's `test.sh`
  `AVP_EXCLUDE=` (`hook.Config.AVPExclude`, now actually read - see
  [HOOKS.md](HOOKS.md)).

The **`avp`** step ID (default **on**, same generic enable/disable
mechanism as every other step - see `Options.DisabledChecks`'s doc
comment) gates this entirely: `--disable-checks avp` forces every app's
Strategy back to plain Kustomize/Helm regardless of any AVP indicator
found, for an operator running without the `argocd-vault-plugin` binary
or a configured secret backend.

Every real caller that needs a fully-rendered overlay - this phase's
build-error detection (`pkg/validator/hook_wiring.go`'s
`buildOverlayWithHooks`) - goes through this strategy-aware path
(`overlay.RenderWithStrategy`). Two callers deliberately still render
Kustomize-only, unconditionally, via `overlay.RenderKustomize` directly:
`pkg/validator/kubeconform_overlay.go`'s schema validation over rendered
overlays (runs during the earlier Linting phase, before any app's
`test.sh`/strategy is resolved) and `pkg/ghostpatch/ghostpatch.go`'s
patch-vs-base drift detection (structural, unaffected by secret
placeholders either way). Don't be misled by the `placeholder` check's
AVP-pattern recognition (`<path:...>` etc. are real regexes in
`pkg/validator/placeholder`) into assuming _that_ check resolves
anything - it only _detects_ unresolved AVP tokens in
already-rendered/committed YAML, independent of the build-time
resolution described above.

## Scaffold Validation

Apps that opt into `scafctl`-based scaffolding (a `.scafctl/configs/<app>.yaml`
config exists - unrelated apps are skipped entirely, not treated as an
error) are re-validated against their scaffold template/config whenever a
change could affect their generated content
(`pkg/validator/scaffold_wiring.go`'s `runScaffoldValidation`, called from
the Build + Compliance phase). Three independent triggers, each skipping
any app an earlier one already tested, cover every way a change can
require this:

1. **Template changes** (`configdiff.DetectTemplateChanges`) - a shared
   template changed, so every overlay of every app using it is
   re-checked (a full test).
2. **Config changes** (`configdiff.DetectAffectedApps`) - either specific
   clusters (an override changed) or a full test (a change that fans out
   cluster-independently, e.g. a changeGroup reassignment - see
   `provider.Providers.ChangeGroups`).
3. **An app's own overlay files changed**, not already covered above -
   only the overlays the PR actually touched, using the same trigger
   classification `overlay.GetOverlaysToTest` uses for the build phase
   itself.

For each app, `pkg/scaffold.Run` regenerates its overlays via scafctl once
(bounded by a 2-minute timeout) and diffs the result against every
overlay actually being checked, **bounded-parallel** (up to
`runtime.NumCPU()*2` overlays at once - the per-app fan-out above is
similarly bounded-parallel across apps). An overlay is skipped rather
than failed when it's disabled - either explicitly
(`scaffoldDisabled: [...]` in the app's own scafctl config - see
`scaffold.IsOverlayDisabled`) or via change-group 0
(`scaffold.IsChangeGroupDisabled`) - or has no on-disk directory at all
(a cluster not yet rolled out, or removed by this PR;
`scaffold.Summary.SkippedClusters`, aggregated per app by
`runScaffoldValidation` and flattened by `flattenSkippedClusters` into the
Scaffold Validation section's "Missing Clusters" bullet). Any real content
mismatch or scafctl execution failure is always treated as blocking - a
simpler, more conservative policy than trying to distinguish "pre-existing
drift this PR didn't cause" (which would need re-running scaffold against
the merge-base template/config, a real but substantially riskier technique
this implementation deliberately doesn't attempt). Missing clusters
themselves are **not** blocking, unlike drift/exec failures - a skip is an
expected, informational "here's what wasn't checked and why", never a
finding (see `scaffold.Run`'s own doc comment).

Separately, every app whose overlays or `.scafctl` template/config
changed is also checked for whether it has drift **coverage** at all:
`findUnprotectedApps` (`pkg/validator/scaffold_wiring.go`) flags any app
that has a scaffold template (so drift detection is actually available
for it) but has opted out via `SCAFFOLD=false` in its `test.sh` (see
[HOOKS.md](HOOKS.md)) - these apps are silently skipped by every trigger
above (`scaffold.HasScaffoldEnabled` gates all three), so a real drift
there would otherwise go completely unreported. This renders as its own
"Scaffold Drift Protection" report section, always present,
non-blocking (a coverage gap warning, not a drift finding).

Separately, `scaffold.CheckReadmeStatus` is a cheap, structural,
per-PR check of the README's `<!-- scaffold-status -->` table: does it
list exactly the (app, overlay) pairs that exist on disk today, with no
missing or stale rows? It does **not** recompute actual drift (that
would mean scaffolding every app in the repo on every PR, not just the
ones it touched) - that's `scaffold.UpdateReadmeStatus`'s job instead,
a full-repo-scan regeneration meant to be run deliberately (the
`update-scaffold-status` CLI command), not on every PR. Unlike the
three drift triggers above, this check is gated behind the
**`scaffold-readme`** step ID, default **off** - see the standalone
steps list below for why. It runs as its own **"scaffold table"**
sub-check in the **Static Checks** section (not folded into Scaffold
Validation's drift summary), so a failure automatically gets an
actionable `k8s-gitops-ci update-scaffold-status` fix-command hint the
same way `config-sort`/`prettier`/`markdownlint` do (`hintByCheck` in
`comments.go`).

## Registered checks

Every check below is registered via `check.Register` in
`pkg/validator/register_checks.go` and, by that registration alone,
automatically exemptable via its own check ID (see
[EXCEPTIONS.md](EXCEPTIONS.md)) unless noted otherwise.

| ID                 | Package                     | Scope   | What it checks                                                                                                                                                                                                                                                                     |
| ------------------ | --------------------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `namespace`        | `pkg/validator/namespace`   | Doc     | Namespace-scoped resources declare `metadata.namespace`; cluster-scoped resources don't (except build-time-only Kustomize control objects)                                                                                                                                         |
| `psa-labels`       | `pkg/validator/psa`         | Doc     | Pod Security Admission namespace labels                                                                                                                                                                                                                                            |
| `rbac-readonly`    | `pkg/validator/rbac`        | Doc     | A `ClusterRole` carrying an aggregate-to-view/cluster-reader label only grants read-only verbs (with a narrow, exact-match allowlist for a handful of known exceptions)                                                                                                            |
| `rbac-wildcards`   | `pkg/validator/rbac`        | Doc     | No `"*"` in `verbs`/`resources`/`apiGroups` on `Role`/`ClusterRole`                                                                                                                                                                                                                |
| `crb`              | `pkg/validator/crb`         | Doc     | `ClusterRoleBinding` subject namespace sanity                                                                                                                                                                                                                                      |
| `sync-options`     | `pkg/validator/syncopts`    | Doc     | Non-builtin API-group resources carry the ArgoCD `SkipDryRunOnMissingResource=true` sync-options annotation (builtin/core and, with `--assume-openshift`, OpenShift-only API groups are exempt)                                                                                    |
| `image-checksum`   | `pkg/validator/image`       | Doc     | Every OCI image reference is pinned to a `sha256:` digest, not just a tag                                                                                                                                                                                                          |
| `named-ports`      | `pkg/validator/namedport`   | Doc     | Container/Service ports are named, not numeric, everywhere they're referenced                                                                                                                                                                                                      |
| `podspec-defaults` | `pkg/validator/podspec`     | Doc     | Required pod-level fields (`enableServiceLinks`, `restartPolicy`, ...) and container `securityContext`/`resources.requests`/`resources.limits` are all set                                                                                                                         |
| `placeholder`      | `pkg/validator/placeholder` | Doc     | No unresolved `<PLACEHOLDER>`-style tokens, AVP secret-reference tokens, or sentinel words (`CHANGEME`, `FIXME`, `XXX`, ...) left in committed YAML                                                                                                                                |
| `cluster-identity` | `pkg/validator/clusterid`   | Overlay | No copy/paste of another cluster's identity (cluster name, project ref) into this overlay — see `exempt.IDClusterName`/`IDProjectRef` (exemptable) vs. `exempt.IDClusterIdentity` (a deliberately non-exemptable structural bucket for findings that don't set a more specific ID) |

Four standalone (non-registry) steps participate in the same
enable/disable ID mechanism (see `docs/DEVELOPMENT.md`'s
[Generic check-enablement mechanism](DEVELOPMENT.md#generic-check-enablement-mechanism)):

- **`golangci`** — Go linting via `pkg/lint/golangci`, default **on**.
- **`avp`** — per-app AVP strategy auto-detection (see
  [Build Strategies](#build-strategies) above), default **on**.
- **`scaffold-readme`** — the README scaffold-status table structural
  check (see [Scaffold Validation](#scaffold-validation) above), default
  **off**. Like `kyverno` below, this generic core can't know whether a
  given repo's table actually matches the one-row-per-app-per-overlay
  shape the check expects, so it's opt-in
  (`--enable-checks scaffold-readme`) until an org confirms
  compatibility - the other three scaffold-drift triggers
  (template/config/overlay changes) are unaffected and always run.
- **`kyverno`** — policy validation via `pkg/lint/kyverno`, default
  **off** (an org must opt in and supply its own policies — see
  [SCHEMAS.md](SCHEMAS.md)). Once enabled (`--enable-checks kyverno`),
  every successfully-built overlay from this phase's build loop (see
  [Build Strategies](#build-strategies) above) is batched into one
  `kyverno apply` invocation
  (`pkg/validator/kyverno_wiring.go`'s `runKyvernoValidation`) against the
  prepared policy bundle; results render as a non-blocking "Kyverno
  Policies" advisory section (`kyverno.FormatComment`'s own doc comment:
  findings never contribute to `res.Blocking`). A missing `kyverno`/
  `kustomize` CLI, unpreparable policies, or a write failure all degrade to
  an empty section rather than failing the run - Kyverno support is
  opt-in and best-effort once enabled, not a hard CI dependency.

## NetworkAttachmentDefinition (NAD) validation

`pkg/validator/nad` validates every successfully-rendered overlay's
`NetworkAttachmentDefinition` resources (`runNADValidation` in
`pkg/validator/nad_wiring.go`, over the same batch of rendered overlay
output the `kyverno` step consumes — see
[Build Strategies](#build-strategies)). Unlike the checks in the table
above, it is **not** part of the `check.Register` framework: it's always
on (not gateable via `DisabledChecks`/`EnabledChecks`) and its findings
are **not** exemptable via `EXEMPTIONS=(...)` or the
`gitops-ci.k8s.io/exempt-<check-id>` annotation (see
[EXCEPTIONS.md](EXCEPTIONS.md)). It renders as its own always-present
"NetworkAttachmentDefinition Validation" report section, blocking on any
finding.

It has two tiers:

- **Structural** (always on): the resource is a
  `NetworkAttachmentDefinition` and its `spec.config` field is present
  and non-empty. CNI-neutral — no assumption about which CNI the config
  targets.
- **OVN-Kubernetes-aware** (opt-in via `--assume-openshift`, i.e.
  `Options.AssumeOpenShift` — the same flag that exempts OpenShift-only
  API groups from `sync-options` above, since an OpenShift/OKD cluster's
  default CNI is OVN-Kubernetes): parses `spec.config` as an OVN netconf
  and applies OVN's semantic rules (topology/role/subnet/transport
  constraints, ported from `ovn-kubernetes/util.ValidateNetConf` — see
  `pkg/validator/nad`'s package doc comment for what's intentionally
  omitted: runtime-only checks that depend on live cluster state).

`validate-nad` also exposes this directly as a CLI subcommand, bypassing
the full pipeline, for validating a directory or explicit file list
(`k8s-gitops-ci validate-nad [--assume-openshift] --dir <path>` or
`... <file.yaml> ...`).

## Linting phase (all steps run concurrently)

- **markdownlint**, **prettier**, **golangci** (Go files only),
  **kubeconform** (schema-validates changed YAML plus every affected
  overlay's _rendered_ output, not just the raw source) — each wraps its
  underlying CLI/library and degrades gracefully (skips, doesn't fail)
  when the tool isn't installed.
- **shellcheck** — wraps the raw `shellcheck` CLI over changed `.sh`/
  `.bash` files (or any file whose shebang matches), **plus** extracts
  and lints:

  - Bash steps embedded in Tekton `Task` manifests
    (`spec.steps[].script`, `pkg/lint/shellcheck/tekton.go`).
  - Bash embedded in workload container/initContainer `command` fields
    (`[bash|sh, -c, <script>]` form) and ConfigMap `.sh`/`.bash` data
    keys (`pkg/lint/shellcheck/embedded.go`), across
    `Pod`/`Job`/`CronJob`/`Deployment`/`DaemonSet`/`StatefulSet`/
    `ReplicaSet` (`CronJob`'s pod spec is nested three levels deeper than
    every other kind, handled the same way `namedport`/`podspec` handle
    it). Non-bash scripts (python, plain `sh`, no shebang) are silently
    skipped — shellcheck only understands bash/sh/dash/ksh.

  Extracted-script findings are classified **direct/blocking** (the
  script's source YAML file was itself changed in this diff) vs.
  **external/warning-only** (the file was only pulled into scope because
  the overlay it lives in was affected by an unrelated base/component
  change elsewhere), reusing the exact same file-set logic
  (`externalOverlayYAMLFiles`, a directory walk over every overlay
  `detectOverlaysForChanges` resolves, excluding files already in the
  diff) as the identical direct/indirect split `finalizeCompliance`
  applies to Resource Compliance findings. Raw `.sh` file findings are
  always direct — they're literally files in the diff.

## Static Checks phase (all steps run concurrently)

- **large-file** — flags files over a size threshold
  (`pkg/largefile.DefaultMaxSize`).
- **YAML-syntax** — parse-level YAML validity (`pkg/lint/yamlsyntax`),
  independent of and cheaper than schema validation.
- **config-sort** — repo config files are alphabetically sorted
  (`pkg/config.CheckSortOrder`).
- **startingCSV** — an OLM `ClusterServiceVersion`'s folder name matches
  its `startingCSV` reference (`pkg/csv`).
- **scaffold table** — the README's `<!-- scaffold-status -->` table
  structural check (`scaffold.CheckReadmeStatus`; see
  [Scaffold Validation](#scaffold-validation) above). Gated behind the
  **`scaffold-readme`** step ID, default **off**.

## Direct vs. external findings

Every Resource Compliance finding (and, as of the shellcheck extraction
work above, every extracted-script finding too) is classified by
`finalizeCompliance`/its shellcheck-step equivalent using one rule: was
the finding's source file itself part of this changeset's diff? If yes,
it's **direct** (❌, blocking — the author touched this file). If the
file wasn't changed but was pulled into scope only because a shared
base/component the affected overlay depends on changed elsewhere, it's
**external** (⚠️, warning-only, non-blocking) — an issue you didn't
introduce and shouldn't be blocked by fixing right now.

## Concurrency

`Workers(opts)` returns `opts.Concurrency` if set (`--concurrency`), else
`runtime.NumCPU() * 2`. This bounds:

- The Linting/Static-Checks goroutine fan-out (one goroutine per step,
  not pooled — there are only ~9 steps total, so no pool is needed).
- The per-overlay worker pool in the Build + Compliance phase (capped at
  `min(Workers(opts), len(overlays))`, so a small changeset never
  over-allocates goroutines for a handful of overlays).

`pkg/validator/timing.go`'s `TimingCollector` records both a
parallelism-efficiency ratio and the resolved concurrency in the final
timing table — see `docs/DEVELOPMENT.md`'s
[Timing table](DEVELOPMENT.md#timing-table-pkgvalidatortimingg) section
for the exact rendering. (This repo has no per-document content-dedup/
hashing layer to describe here — every changed/affected file is checked
independently, once.)

## Report structure

See `docs/DEVELOPMENT.md`'s
[Unified PR-comment report](DEVELOPMENT.md#unified-pr-comment-report-pkgvalidatorunified_reportgo-compose_sectionsgo)
section for the full rendering model (sections, sub-check dropdowns,
status icons). In short: one PR comment, each top-level section a
collapsible `<details>` block - PR Checks, Linting, Static Checks,
Kustomize Build, Scaffold Validation, Scaffold Drift Protection,
Resource Compliance, NetworkAttachmentDefinition Validation, CI Notes,
plus Kyverno Policies when the opt-in `kyverno` step is enabled (see
[Registered checks](#registered-checks) below). Resource Compliance
additionally groups findings by check ID with an "Accepted Exceptions"
audit sub-block.

Each check-ID group under Resource Compliance renders using that check's
registered `check.TableSpec` (`pkg/validator/register_tables.go`) when
one exists: its own descriptive title/preamble and columns (e.g.
`image-checksum` shows Kind/Name/Image/File, not just a flat File/
Message dump), with findings that are the same underlying issue fanned
out across multiple overlays/build locations collapsed into one row
listing every affected file (`dedupFindingsForTable` in
`compose_sections.go`) instead of repeating an otherwise-identical row
per location. A check id with no registered `TableSpec` still falls back
to the original generic two-column File/Message table.
