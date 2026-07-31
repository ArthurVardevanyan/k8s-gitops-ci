# Hooks

Every app can carry an optional `test.sh` at its root
(`hook.FindTestScript(app)` → `<app>/test.sh`) that declares a small set
of directives. This document describes `pkg/hook`'s actual, current
behavior, and exactly how each directive is wired into the pipeline (see
**Current Limitations** for anything not yet connected end-to-end).

## The `test.sh` contract

`test.sh` is a convention shared with other tooling in the GitOps repo,
so this tool never runs it top-to-bottom as a script — `pkg/hook.ParseTestScript`
line-scans it for a fixed set of recognized directives, and
`pkg/hook.RunPreBuildHook`/`RunPostBuildHook`/`RunPostValidateHook`
(see **Hook execution** below) only ever `source` it to invoke one named
function per hook. Unrecognized content is ignored; a missing `test.sh`
parses to `hook.DefaultConfig()` (`Scaffold: true`, everything else
empty/false).

| Directive                                                      | Syntax                                                                  | Effect                                                                                                                                                                                    |
| -------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `SCAFFOLD=`                                                    | `SCAFFOLD=false` (or `true`/`yes`/`1`)                                  | Opts an app out of scaffold-drift validation. Defaults to `true` (enabled) when absent.                                                                                                   |
| `AVP_EXCLUDE=`                                                 | `AVP_EXCLUDE="cluster1 cluster2"`                                       | Space-separated list of overlay names to exclude from AVP secret resolution — see [CI.md](CI.md)'s Build Strategies section.                                                              |
| `EXEMPTIONS=(...)` or `EXEMPTIONS="..."`                       | see below                                                               | Per-app exemption selectors, merged into every check run during the Build + Compliance phase (see **`EXEMPTIONS=(...)` wiring** below).                                                   |
| `PRE_BUILD_HOOK=` / `POST_BUILD_HOOK=` / `POST_VALIDATE_HOOK=` | `PRE_BUILD_HOOK=<cmd>` (optionally `export`-prefixed, like `SCAFFOLD=`) | Names a shell function (defined elsewhere in the same `test.sh`) or external command to invoke around the build — see **Hook execution** below. `<cmd>` empty/absent means "not defined". |

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
`name`, `namespace`, `match`, `value`, `path`. (`dir` is also a valid
`exempt.Selector` field — see [EXCEPTIONS.md](EXCEPTIONS.md) — but isn't
a recognized `EXEMPTIONS=(...)` key today.) `check=` is mandatory; a
malformed entry (no `=`, an unknown key, an empty value, or a missing
`check=`) is collected into `Config.ExemptErrors` rather than silently
dropped or applied.

### `EXEMPTIONS=(...)` wiring

Every app whose test.sh is resolved during the Build + Compliance phase
(see `resolveAppHookConfigs` in `pkg/validator/hook_wiring.go`) has its
parsed `ExemptSelectors` bridged into `pkg/validator/exempt.Selector` and
merged with the built-in selectors (e.g. the `.tekton/` PipelineRun
default — see `tekton_exemptions.go`) before either the doc-check or
overlay-check engines run. A real `EXEMPTIONS=(...)` entry therefore
suppresses matching findings for **any exemptable check** (currently
`image-checksum`, `cluster-name`, `project-ref` — see
`pkg/validator/exempt.Exemptable`) across the whole run, the same as the
annotation-based exemption mode
(`gitops-ci.k8s.io/exempt-<check-id>` — see [EXCEPTIONS.md](EXCEPTIONS.md)).

Selectors are merged flatly across every app in the run, not scoped to
only that app's own files - matching how the built-in selectors already
behave. A malformed `EXEMPTIONS` token parses to **zero** selectors for
that entry (fail-closed: it exempts nothing) and is additionally
surfaced as a blocking failure (`log.ErrorInSection("Hooks", ...)`, folded
into the "Kustomize Build" report section) so the author notices and
fixes the syntax error instead of unknowingly under-exempting.

## Hook execution

`PRE_BUILD_HOOK=`/`POST_BUILD_HOOK=`/`POST_VALIDATE_HOOK=` name a shell
function (or external command) that `pkg/hook`'s `RunPreBuildHook`/
`RunPostBuildHook`/`RunPostValidateHook` (`pkg/hook/exec.go`) actually
invoke, via `bash -c 'source <test.sh>; "$CMD" <args>'`, during the
Build+Compliance phase (`buildOverlayWithHooks`/`runAppPostValidateHooks`
in `pkg/validator/hook_wiring.go`):

- **`PRE_BUILD_HOOK`** runs once per overlay, before that overlay is
  built, with `$1`=the overlay's absolute path and `$2`=where its
  rendered YAML will be written. A failing `PRE_BUILD_HOOK` skips the
  build for that overlay entirely.
- **`POST_BUILD_HOOK`** runs once per successfully-built overlay, with
  `$1`=the rendered YAML's absolute path, `$2`=the overlay's basename
  (e.g. `prod`), `$3`=the overlay's absolute directory path.
- **`POST_VALIDATE_HOOK`** runs once per app, after every one of its
  overlays has been built, with `$1`=the app's build directory
  (containing every overlay's rendered YAML) and `$2`=the app name.

Every invocation runs with a per-app environment (`APP`, `ROOT_PATH`,
`APP_TMP_DIR`, `ERROR_LOG`, `PIPELINE=yes`) and a timeout (60s for
`PRE_BUILD_HOOK`/`POST_BUILD_HOOK`, 120s for `POST_VALIDATE_HOOK`). A
hook fails the build either by exiting non-zero, or by writing to
`"${ERROR_LOG}"` (even on exit 0) — the latter lets a hook report
multiple problems from one invocation. A hook failure is reported as a
blocking build error (`kustomize build <path>: pre-build hook: ...` /
`post-build hook: ...` / `post-validate hook: ...`, grouped into the
"Kustomize Build" report section the same way a `kustomize build`
failure is) and every attempted hook's outcome (✅ ran / ❌ failed / —
not defined) is rendered in that section's `| App | PRE_BUILD |
POST_BUILD | POST_VALIDATE |` table.

Rendered YAML is only actually written to disk for an app when it has a
`POST_BUILD_HOOK` or `POST_VALIDATE_HOOK` defined (see `needsBuildDir` in
`pkg/validator/hook_wiring.go`) - the common no-hooks-defined case never
touches the filesystem for this.

## `hook.Source` / `ResolveSource`

`ResolveSource(signal, triggerComment, prSet)` decides whether to trust
the PR branch's `test.sh` (`SourcePR`) or fall back to the target
branch's (`SourceMain`) — fail-closed to `SourceMain` in every ambiguous
case:

- No signal at all → `SourceMain`.
- `SourceLocal` → always honored as-is (a local/CLI run explicitly
  requesting the working tree's own `test.sh`, e.g. via
  `--hook-source local`).
- `SourcePR` → only honored when the triggering comment was **exactly**
  `/hook-test` **and** a PR number is set; anything else (including a PR
  signal without that exact comment) falls back to `SourceMain`. This
  means a PR's own `test.sh` changes are never trusted by default — an
  explicit `/hook-test` comment is required, so a PR can't quietly
  smuggle in a weaker `test.sh` (e.g. removing an `EXEMPTIONS` entry's
  scoping, or disabling scaffold checks) and have its own, unreviewed
  version take effect.

The CLI's `--hook-source`/`--trigger-comment` flags
(`pipeline.Options.HookSource`/`TriggerComment`) flow through to
`validator.Options`, which calls `ResolveSource` once per run
(`resolveHookSource` in `pkg/validator/hook_wiring.go`) before resolving
any app's `test.sh` - every app in the same run shares one resolved
`hook.Source`.

## Current Limitations

None currently known - every directive in the table above is parsed
_and_ connected to real behavior. See [CI.md](CI.md)'s Build Strategies
section for exactly which overlay-rendering call sites `AVP_EXCLUDE=`
does (and, deliberately, doesn't) affect.
