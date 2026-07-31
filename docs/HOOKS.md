# Hooks

Every app can carry an optional `test.sh` at its root
(`hook.FindTestScript(app)` → `<app>/test.sh`) that declares a small set
of directives. This document describes `pkg/hook`'s actual, current
behavior — including which directives are fully wired end-to-end and
which are parsed but not yet connected to anything (see the
**Current Limitations** section below before assuming a directive does
what its name suggests).

## The `test.sh` contract

`test.sh` isn't executed as a script by this tool (it's a convention
shared with other tooling in the GitOps repo) — `pkg/hook.ParseTestScript`
line-scans it for a fixed set of recognized directives. Unrecognized
content is ignored; a missing `test.sh` parses to `hook.DefaultConfig()`
(`Scaffold: true`, everything else empty/false).

| Directive                                                      | Syntax                                                            | Effect                                                                                                                                                                              |
| -------------------------------------------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `SCAFFOLD=`                                                    | `SCAFFOLD=false` (or `true`/`yes`/`1`)                            | Opts an app out of scaffold-drift validation. Defaults to `true` (enabled) when absent.                                                                                             |
| `AVP_EXCLUDE=`                                                 | `AVP_EXCLUDE="cluster1 cluster2"`                                 | Space-separated list of overlay names to exclude from AVP secret resolution — see the **Current Limitations** note below; this is parsed but not read by anything downstream today. |
| `EXEMPTIONS=(...)` or `EXEMPTIONS="..."`                       | see below                                                         | Per-app exemption selectors — see the **Current Limitations** note below; parsed and validated, but not fed into the actual exemption-evaluation engine today.                      |
| `PRE_BUILD_HOOK=` / `POST_BUILD_HOOK=` / `POST_VALIDATE_HOOK=` | any line containing the key (e.g. `PRE_BUILD_HOOK=run_my_script`) | Presence-only marker — see **Current Limitations**.                                                                                                                                 |

### `EXEMPTIONS=(...)` selector syntax

Both the multi-line bash-array form and the single-line comma-separated
form parse identically:

```sh
EXEMPTIONS=(
  check=image-checksum,file=foo.yaml
  check=sync-options,kind=ArgoCD,name=my-argocd-instance
)
```

```sh
EXEMPTIONS="check=image-checksum,file=foo.yaml"
```

Each entry is a comma-separated set of `key=value` pairs. Recognized
selector keys, matching `pkg/validator/exempt.Selector`'s fields exactly:
`check` (required — the check ID this entry exempts), `file`, `kind`,
`name`, `namespace`, `match`, `value`, `path`, and `dir`. `check=` is
mandatory; a malformed entry (no `=`, an unknown key, an empty value, or
a missing `check=`) is collected into `Config.ExemptErrors` rather than
silently dropped or applied — `pkg/hook` fails loud on a syntax error,
even though (per the limitation below) a syntactically-valid entry
doesn't currently do anything either.

`dir` selects by root-anchored path prefix (e.g. `dir=.tekton` matches
`.tekton/foo.yaml` but not `apps/foo/.tekton/x.yaml` nested elsewhere) —
this is this repo's own addition to the selector shape; see
[EXCEPTIONS.md](EXCEPTIONS.md) for the full selector reference (used by
the one exemption selector mechanism that _is_ wired today, the built-in
`.tekton/` PipelineRun exemption).

## `hook.Source` / `ResolveSource`

`ResolveSource(signal, triggerComment, prSet)` decides whether to trust
the PR branch's `test.sh` (`SourcePR`) or fall back to the target
branch's (`SourceMain`) — fail-closed to `SourceMain` in every ambiguous
case:

- No signal at all → `SourceMain`.
- `SourceLocal` → always honored as-is (a local/CLI run explicitly
  requesting the working tree's own `test.sh`).
- `SourcePR` → only honored when the triggering comment was **exactly**
  `/hook-test` **and** a PR number is set; anything else (including a PR
  signal without that exact comment) falls back to `SourceMain`. This
  means a PR's own `test.sh` changes are never trusted by default — an
  explicit `/hook-test` comment is required, so a PR can't quietly
  smuggle in a weaker `test.sh` (e.g. removing an `EXEMPTIONS` entry's
  scoping, or disabling scaffold checks) and have its own, unreviewed
  version take effect.

This is fully implemented and unit-tested (`hook_test.go`), but as of
this writing **nothing in `pkg/pipeline`/`pkg/validator` calls
`ResolveSource`** — see Current Limitations.

## Current Limitations

Read this section before assuming a `test.sh` directive changes pipeline
behavior. Every item below was confirmed by reading every call site of
the relevant type/function, not assumed:

- **Hook scripts are detected, not executed.** `Runner.RunHooks` (via
  `HasPreBuild`/`HasPostBuild`/`HasPostValidate`) is used **only** to
  render the `| App | PRE_BUILD | POST_BUILD | POST_VALIDATE |` presence
  table in the Kustomize Build report section
  (`pkg/validator/build_wiring.go`'s `buildHookTable`) — it reports
  whether a hook is _defined_, never whether it _ran and passed_.
  `Runner.preBuild`'s underlying `runScript` is a documented placeholder:
  it checks the script file exists and returns `nil` unconditionally
  ("Placeholder: actual exec omitted to avoid runtime dep in tests" — see
  the comment in `pkg/hook/hook.go`). Do not describe
  `PRE_BUILD_HOOK`/`POST_BUILD_HOOK`/`POST_VALIDATE_HOOK` as executing
  anything.
- **`EXEMPTIONS=(...)` is parsed and validated, but never evaluated.**
  `hook.Config.ExemptSelectors` is populated correctly (and syntax errors
  are collected into `ExemptErrors`), but grep for every construction
  site of `exempt.Selector{}` used for real evaluation shows exactly one:
  the hardcoded built-in Tekton-PaC exemption in
  `pkg/validator/tekton_exemptions.go`'s `builtinExemptSelectors()` (whose
  own doc comment says its result is "to be merged ahead of any
  hook-provided EXEMPTIONS selectors" — an acknowledged, not-yet-done
  wiring point). `pkg/validator/phases.go`'s
  `selectors := builtinExemptSelectors()` call is the **only** place
  `selectors` is ever assigned before reaching `exempt.Evaluate` — no
  per-app `test.sh`'s parsed `ExemptSelectors` is ever merged in. **A
  real `EXEMPTIONS=(...)` entry in a `test.sh` today has zero effect on
  validation.** Only the annotation-based exemption mode
  (`gitops-ci.k8s.io/exempt-<check-id>` on the resource itself — see
  [EXCEPTIONS.md](EXCEPTIONS.md)) and the one built-in selector actually
  work.
- **`AVP_EXCLUDE=` is parsed, but never read outside `pkg/hook`.**
  `hook.Config.AVPExclude` is populated correctly, but grep across
  `pkg/validator`/`pkg/overlay`/`cmd/k8s-gitops-ci` shows nothing reads
  it — consistent with `pkg/overlay`'s AVP build strategy itself being
  unwired today (see [CI.md](CI.md)'s Build Strategies section).
- **`hook.ResolveSource` is implemented and tested, but not called.**
  Nothing in `pkg/pipeline` or `pkg/validator` currently resolves which
  `test.sh` source to trust via this function — there is no live
  `/hook-test`-comment-driven behavior yet.

None of the above is a bug to fix in this doc's own PR — they're
accurately-documented gaps so a reader doesn't assume a directive works
just because it parses cleanly and has no visible error.
