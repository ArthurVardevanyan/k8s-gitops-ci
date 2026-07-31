# Exceptions

`pkg/validator/exempt` is the unified exemption framework every
registered check's findings pass through. There are two modes, both wired
into evaluation today — see [Which one do I use?](#which-one-do-i-use)
for the tradeoffs between them.

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

Every check registered via `check.Register` becomes exemptable
automatically, under its own check ID (`check.Register` calls
`exempt.RegisterExemptable(c.ID())` unconditionally) — so `namespace`,
`psa-labels`, `rbac-readonly`, `rbac-wildcards`, `crb`, `sync-options`,
`named-ports`, `podspec-defaults`, and `placeholder` are all exemptable
by their own ID with no extra wiring needed. Three IDs get special
treatment instead of using their owning check's own ID:

| ID                 | Used by                                                                                                                                                                    | Exemptable?                                                                                                                                                                                                                                                                                                                                                |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `image-checksum`   | `pkg/validator/image`                                                                                                                                                      | Yes                                                                                                                                                                                                                                                                                                                                                        |
| `cluster-name`     | `pkg/validator/clusterid` (a cluster-identity sub-finding)                                                                                                                 | Yes                                                                                                                                                                                                                                                                                                                                                        |
| `project-ref`      | `pkg/validator/clusterid` (a project-identity sub-finding)                                                                                                                 | Yes                                                                                                                                                                                                                                                                                                                                                        |
| `cluster-identity` | `pkg/validator/clusterid` (the fallback bucket for structural findings that don't set a more specific ID — e.g. a hypothetical future infraID-mismatch/invalid-JSON check) | **No — deliberately non-exemptable.** `exempt.Exemptable` hardcodes this ID to always return `false`, and `RegisterExemptable` refuses to register it even if called. This is intentional: a structural finding here means the data itself is malformed/untrustworthy, which isn't the kind of thing a selector or annotation should be able to wave away. |

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

## How exceptions surface in the PR comment

`ComposeResourceComplianceSection` (`pkg/validator/compose_sections.go`)
renders an **"Accepted Exceptions"** audit sub-block from
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

| Field       | Matches                                                                                                                                                                                           |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Check`     | The check ID this selector applies to (**required**).                                                                                                                                             |
| `File`      | Finding's file, by exact basename or `/`-suffix path match (not a raw substring — `File: "app"` won't match `myapp-config.yaml`).                                                                 |
| `Kind`      | Finding's `Kind`, exact match.                                                                                                                                                                    |
| `Name`      | Finding's `Name`, exact match.                                                                                                                                                                    |
| `Namespace` | Finding's `Namespace`, exact match.                                                                                                                                                               |
| `Value`     | Finding's `Value`/`Token` (see [Value vs. Token](#value-vs-token)), exact match.                                                                                                                  |
| `Match`     | Finding's `Value`/`Token`, substring match.                                                                                                                                                       |
| `Path`      | Finding's `Path`, aligned as a suffix; `[]` in either side wildcards that index, `[N]` pins an exact index (e.g. `containers[1].image` matches only index 1).                                     |
| `Dir`       | Finding's `File`, root-anchored directory-prefix match (`Dir: ".tekton"` matches `.tekton/x.yaml`, not `apps/foo/.tekton/x.yaml`) — this repo's own addition beyond the reference selector shape. |
