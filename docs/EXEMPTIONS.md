# Exemptions

`pkg/validator/exempt` is the unified exemption framework every
registered check's findings pass through. There are two modes, both wired
into evaluation today — see [Which one do I use?](#which-one-do-i-use)
for the tradeoffs between them.

## Table of Contents

- [The two modes](#the-two-modes)
  - [1. Annotation exemption](#1-annotation-exemption)
  - [2. `EXEMPTIONS=(...)` selector](#2-exemptions-selector)
  - [The built-in Tekton-PaC exemption](#the-built-in-tekton-pac-exemption)
- [Which one do I use?](#which-one-do-i-use)
- [Exemptable check IDs](#exemptable-check-ids)
- [Non-app `test.sh` scoping](#non-app-testsh-scoping)
- [Value vs. Token](#value-vs-token)
- [Adding exemption support to a new check](#adding-exemption-support-to-a-new-check)
- [How exemptions surface in the PR comment](#how-exemptions-surface-in-the-pr-comment)
- [Selector reference](#selector-reference)

## The two modes

### 1. Annotation exemption

A resource grants its own exemption via a `gitops-ci.k8s.io/exempt-<check-id>`
annotation whose value must **exactly** match the finding's value (or its
`Token`, when the check sets one — see [Value vs. Token](#value-vs-token)
below):

```yaml
metadata:
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: "registry.example.com/app:latest"
```

This exempts _only_ an `image-checksum` finding whose image is exactly
`registry.example.com/app:latest` on _this_ resource — a different image
on the same resource, or the same image on a different resource, still
enforces normally. `exempt.Accepts` fails closed: an empty annotation
value, or no annotations at all, never matches, even against an
empty-valued finding.

`image-checksum` also accepts a **repo-level** value with no tag or
digest, e.g. `docker.io/linuxserver/heimdall` (registry + repo only). This
exempts _every_ tag/digest of that repo, so the exemption survives a
Renovate tag bump instead of needing to be updated every time:

```yaml
metadata:
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: "docker.io/linuxserver/heimdall"
```

This matches a finding whose image is `docker.io/linuxserver/heimdall:2.8.2`,
`docker.io/linuxserver/heimdall:2.9.0`, or any other tag/digest of that
same repo — but **not** `docker.io/linuxserver/heimdall-extra:1.0` (an
unrelated repo that merely shares a name prefix); the match is anchored to
the exact `registry/repo` string, not a substring/prefix check. The exact
full-reference form (with a tag or digest) still works too — both are
accepted (see [Value vs. Token](#value-vs-token) for how this is wired via
`MatchAliases`).

`image-checksum` also supports **comma-separated** values to exempt
multiple images in a single annotation:

```yaml
metadata:
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: "cuda,nvidia/driver,toolkit.image"
```

Each entry is checked independently against the annotation value. Whitespace
is trimmed (`"cuda, nvidia/driver"` works the same as `"cuda,nvidia/driver"`).
Each entry matches against the finding's value **and** its repo-level alias,
so the same annotation can mix tagged images (`cuda`) with repo-level
references (`docker.io/nvidia/driver`).

`rbac-wildcards` also supports **comma-separated** values to exempt
wildcards in multiple fields in a single annotation:

```yaml
metadata:
  annotations:
    gitops-ci.k8s.io/exempt-rbac-wildcards: "apiGroups,resources"
```

Whitespace is trimmed (`"apiGroups, resources"` works the same as
`"apiGroups,resources"`). Each entry matches independently — only the
wildcarded fields listed in the annotation are exempted; any wildcard
field not listed still produces a finding.

`image-fqdn` (the check that requires an explicit registry host — see
[CI.md](CI.md#image-fqdn)) is **not** exemptable by either mode: an
unqualified image reference is almost always a mistake, and the one
plausible legitimate exception (e.g. an OpenShift ImageStream-triggered
bare reference) is better handled by a targeted skip in the check itself
than a manual escape hatch.

### 2. `EXEMPTIONS=(...)` selector

A `test.sh`'s `EXEMPTIONS=(...)` block (see [HOOKS.md](HOOKS.md) for the
full directive syntax) declares selector-based exemptions scoped by
`check`/`file`/`kind`/`name`/`namespace`/`match`/`value`/`path` rather
than a self-granted annotation. This is useful when you can't (or don't
want to) annotate the resource itself — e.g. exempting every resource
under a directory, or a resource you don't control the manifest of.

Every app's `test.sh` is resolved once per run (subject to
`hook.ResolveSource`'s fail-closed rules — see [HOOKS.md](HOOKS.md)), its parsed
`ExemptSelectors` bridged from `hook.ExemptSelector` into
`pkg/validator/exempt.Selector`, and merged with the built-in selectors
below before either check engine runs
(`hookExemptSelectorsAndErrors`/`resolveAppHookConfigs` in
`pkg/validator/hook_wiring.go`). Selectors are merged flatly across every
app in the run (not scoped to just that app's own files), matching how
the built-in selectors already behave. A malformed `EXEMPTIONS` token
parses to zero selectors (fail-closed — it exempts nothing) **and** is
surfaced as a blocking build error, so a syntax error is never silently
ignored.

### The built-in Tekton-PaC exemption

`pkg/validator/tekton_exemptions.go`'s `builtinExemptSelectors()` is
merged into every run's `selectors` list unconditionally (not from any
`test.sh`):

```go
{Check: "sync-options", Kind: "PipelineRun", Dir: ".tekton"}
{Check: "namespace", Kind: "PipelineRun", Dir: ".tekton"}
```

`PipelineRun` resources under a `.tekton/` directory (Tekton
Pipelines-as-Code's own convention — PaC's controller applies and prunes
them directly, rather than relying on Argo CD's sync lifecycle) are
exempt from `sync-options`/`namespace` for that reason. Set
`validator.TektonPACDir = ""` (once, at process startup) to disable this
default and have those `PipelineRun`s checked like any other resource.

## Which one do I use?

Both modes take effect today; pick based on scope:

- **Annotation exemption** — a self-granted, single-resource exemption.
  Prefer this when you control the manifest: it's visible right next to
  the field it exempts, and doesn't require touching `test.sh`.
- **`EXEMPTIONS=(...)`** — a centrally-declared exemption scoped by
  `file`/`kind`/`name`/`namespace`/`match`/`value`/`path` (or left
  wide-open on just `check=`). Use this for a resource you don't control
  the manifest of, or to exempt every match under a directory/pattern
  without annotating each resource individually.

## Exemptable check IDs

Every check registered via `check.Register` becomes exemptable via the
`EXEMPTIONS=(...)` **selector** mode automatically, under its own check ID
(`check.Register` calls `exempt.RegisterExemptable(c.ID())`
unconditionally) — so `namespace`, `psa-labels`, `rbac-readonly`,
`rbac-wildcards`, `crb`, `sync-options`, `named-ports`, `podspec-defaults`,
and `placeholder` are all selector-exemptable by their own ID with no
extra wiring needed.

**Annotation-mode support is separate and per-check** (see "Adding
exemption support to a new check" below): a check only honors
`gitops-ci.k8s.io/exempt-<id>` if its adapter in `register_checks.go`
populates `Finding.Value`/`Token` (what the annotation's value must match)
_and_ `Finding.Annotations` (the resource's own annotations, threaded
through from the underlying validator). Of the checks above,
`rbac-wildcards`, `named-ports`, and `podspec-defaults` do both today
(`Value` is the wildcarded rule field, the numeric port, and the
joined missing-fields list, respectively). `namespace`, `psa-labels`,
`rbac-readonly`, `crb`, and `sync-options` don't populate either (no
`Value`/`Token`/`Annotations` set on their findings), so annotation
mode and `value=`/`match=` selectors don't work for them — only
`kind`/`name`/`file` narrowing. `placeholder` populates `Value` (the
matched token) but not `Annotations` — its annotation form fails closed
too (no annotations to match against), but its `value=`/`match=` selectors
do work.
Four IDs get special treatment instead of using their owning check's own
ID:

| ID                 | Used by                                                                                                                                                                    | Exemptable?                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `image-checksum`   | `pkg/validator/image`                                                                                                                                                      | Yes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `cluster-name`     | `pkg/validator/clusterid` (a cluster-identity sub-finding)                                                                                                                 | Yes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `project-ref`      | `pkg/validator/clusterid` (a project-identity sub-finding)                                                                                                                 | Yes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `kubeconform`      | The kubeconform lint step (`pkg/validator/phases.go`) — a standalone linter, not a `check.Register` entry                                                                  | Yes — `check=kubeconform,file=...` in a `test.sh` `EXEMPTIONS=(...)` block skips matching files from kubeconform schema validation. The `Selector.Dir` field is reachable only from the built-in Go selectors (`pkg/validator/tekton_exemptions.go`), not from `EXEMPTIONS=` — writing `dir=...` is a blocking parse error. Whole non-manifest YAML on the raw path (no root `kind`/`apiVersion`) is now **auto-skipped** by the content gate (`kubeconform.IsManifestYAML`, see [CI.md](CI.md#kubeconform)), so a selector is only needed for a file that _is_ a manifest but whose schema check you deliberately want suppressed. See [Non-app `test.sh` scoping](#non-app-testsh-scoping) below. |
| `cluster-identity` | `pkg/validator/clusterid` (the fallback bucket for structural findings that don't set a more specific ID — e.g. a hypothetical future infraID-mismatch/invalid-JSON check) | **No — deliberately non-exemptable.** `exempt.Exemptable` hardcodes this ID to always return `false`, and `RegisterExemptable` refuses to register it even if called. This is intentional: a structural finding here means the data itself is malformed/untrustworthy, which isn't the kind of thing a selector or annotation should be able to wave away.                                                                                                                                                                                                                                                                                                                                          |
| `image-fqdn`       | `pkg/validator/image`                                                                                                                                                      | **No — deliberately non-exemptable.** See [Annotation exemption](#1-annotation-exemption) above for the rationale.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `largefile`        | The large file/binary check (`pkg/largefile`)                                                                                                                              | Yes — `check=largefile,file=...` in a `test.sh` `EXEMPTIONS=(...)` block skips matching files from the large file check. Supports `file` and `Dir` selectors.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |

`pkg/validator/nad`'s NetworkAttachmentDefinition validation (see
[CI.md](CI.md#networkattachmentdefinition-nad-validation)) is **not**
part of the `check.Register` framework at all, so it has no check ID to
exempt by either mode — a NAD hard-error finding always blocks regardless
of `EXEMPTIONS=(...)` or annotations (its non-blocking advisory warnings
never block in the first place).

## Non-app `test.sh` scoping

The `EXEMPTIONS=(...)` mechanism in [HOOKS.md](HOOKS.md) normally only
applies to kustomize apps — directories with a `base/`, `overlays/`, or
`components/` subdirectory whose changed files trigger overlay detection.
For directories that have no kustomize structure (e.g. `okd/node-config/`,
which contains agent-based installer node configuration YAML that is not a
Kubernetes resource), those files are never associated with any app and
their `test.sh` is never read by the standard app-hook resolution path.

To cover this gap, `pkg/validator/nonapp_wiring.go`'s
`resolveNonAppHookConfigs` reads a `test.sh` for each unique directory of
changed files that does **not** fall under any detected app root, before
either validation phase runs. Its `EXEMPTIONS=(...)` selectors are
resolved early (via the same `hook.Resolve` trust model — fail-closed to
`SourceMain`) and passed directly into the kubeconform lint step as
`earlySelectors`. This means:

- A `test.sh` placed in a non-app directory (e.g. `okd/test.sh`) is read
  whenever a changed file's directory — or any of that directory's
  descendants with no `test.sh` of their own — appears in the changeset.
- Resolution walks **upward** from a changed file's own directory toward
  the repository root, stopping at the **nearest ancestor that actually
  declares a `test.sh`** (closest-match-wins — the same cascading pattern
  `.gitignore`/`.editorconfig` use, not a merge across ancestors). This
  lets one `test.sh` placed at a shared parent directory (e.g.
  `okd/test.sh`) cover both files directly in that directory and files in
  a non-app subdirectory (e.g. `okd/node-config/*.yaml`) that has no
  `test.sh` of its own — no need for a `test.sh` in every leaf directory.
  If a subdirectory **does** declare its own `test.sh`, that one applies
  instead and the parent's is never consulted for those files.
- Its `EXEMPTIONS=(...)` entries support the same selector keys as any
  other `test.sh` (`file`, `kind`, `name`, `namespace`, `match`, `value`,
  `path` — see the [selector reference](#selector-reference)).
- Only `check=kubeconform` selectors have effect today — doc/overlay checks
  run in the Build YAML/Post-Build Validation phases and consume their own
  app-level `EXEMPTIONS=(...)` separately.

**Example** — one `test.sh` at `okd/` covering both root-level files and a
non-app subdirectory:

```sh
export EXEMPTIONS=(
  "check=kubeconform,file=install-config.yaml"
  "check=kubeconform,file=node-config/gpu-1.yaml"
  "check=kubeconform,file=node-config/worker-1.yaml"
)
```

## Value vs. Token

A `Scalar`'s exemption-matching value is its `Token` when set, falling
back to `Value` when `Token` is empty (`Scalar.annotationValue()`).
`Token` exists for checks whose human-readable `Value` isn't a stable
match target (e.g. it might contain a per-render timestamp or a
formatted message) — such a check sets a separate, stable `Token` for
matching while still reporting the friendly `Value` in the report table.
`image-checksum`'s value (the image reference) already doubles as a
stable token, so it doesn't need to set one separately.

## Adding exemption support to a new check

If you're adding a new `check.Check`:

1. **Nothing extra to do for the common case.** Call `check.Register`
   as usual — registration alone makes the check's own ID exemptable.
2. **Set `check.Finding.Value`** to whatever a human/annotation should
   match against (and `Token`, only if `Value` isn't itself a stable
   match target — see [Value vs. Token](#value-vs-token)).
3. **Only use a dedicated `exempt.ID*` constant instead of the check's
   own ID** if a single check produces multiple logically-distinct
   finding types that should be exempted independently (like
   `clusterid`'s `cluster-name`/`project-ref`/`cluster-identity` split) —
   otherwise, just use the check's own `ID()`.

## How exemptions surface in the PR comment

`ComposeResourceComplianceSection` (`pkg/validator/compose_sections.go`)
renders an **"Accepted Exemptions"** audit sub-block from
`check.Result.Exempted` (`[]exempt.Applied`) whenever any exemption was
applied, listing `| Resource | Value | Scope |` per exemption.

> **Known limitation:** the block's label and each row's "Scope" column
> are meant to distinguish an exemption applied to a directly-modified
> resource (`Direct: true`, rendered "directly modified") from a
> pre-existing one (rendered "pre-existing") — but `exempt.Applied.Direct`
> is **never actually set to `true`** anywhere in the current code (the
> only two `Applied{...}` construction sites, both in
> `exempt.Evaluate`, omit the field, leaving it at its `false` zero
> value). In practice, today, the block always renders with the
> `(pre-existing)` label and every row shows `pre-existing`, even for an
> exemption that was applied to a resource this PR directly touches.
> This mirrors `finalizeCompliance`'s blocking/warning split
> conceptually, but that wiring (setting `Direct` from the same
> changed-file-set check `finalizeCompliance` already does) hasn't been
> connected yet.

## Selector reference

`exempt.Selector` fields, all optional except `Check`:

| Field       | Matches                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Check`     | The check ID this selector applies to (**required**).                                                                                                                                                                                                                                                                                                                                                                                                      |
| `File`      | Finding's file, by exact basename match (`file=foo.yaml` matches any `…/foo.yaml`) or `/`-prefix suffix match (`file=bar/foo.yaml` matches `…/bar/foo.yaml` but NOT the root-relative `bar/foo.yaml` itself — a leading `/` separator must appear before the value). A full repo-relative path like `okd/node-config/foo.yaml` never self-matches via suffix; use a partial suffix such as `node-config/foo.yaml` or the bare basename `foo.yaml` instead. |
| `Kind`      | Finding's `Kind`, exact match.                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `Name`      | Finding's `Name`, exact match.                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `Namespace` | Finding's `Namespace`, exact match.                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `Value`     | Finding's `Value`/`Token` (see [Value vs. Token](#value-vs-token)), exact match.                                                                                                                                                                                                                                                                                                                                                                           |
| `Match`     | Finding's `Value`/`Token`, substring match.                                                                                                                                                                                                                                                                                                                                                                                                                |
| `Path`      | Finding's `Path`, aligned as a suffix; `[]` in either side wildcards that index, `[N]` pins an exact index (e.g. `containers[1].image` matches only index 1).                                                                                                                                                                                                                                                                                              |
| `Dir`       | Finding's `File`, root-anchored directory-prefix match (`Dir: ".tekton"` matches `.tekton/x.yaml`, not `apps/foo/.tekton/x.yaml`) — this repo's own addition beyond the reference selector shape.                                                                                                                                                                                                                                                          |
