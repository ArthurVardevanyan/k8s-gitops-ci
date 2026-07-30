# Embedded Resources: Schemas & Policies

`pkg/lint/kubeconform` and `pkg/lint/kyverno` both need external data
(JSON schemas, Kyverno policy manifests) that's too large and too
slow-changing to fetch at runtime on every invocation. Both packages
embed that data into the compiled binary via `//go:embed`, populated by a
`scripts/pull-*.sh` helper. This doc explains that mechanism and,
specifically, how an org supplies schemas/policies for its own
CRDs/policies that have no generic public source.

## The pattern

```text
scripts/pull-schemas.sh    ──writes──> pkg/lint/kubeconform/schemas/schemas.tar.gz
scripts/pull-policies.sh   ──writes──> pkg/lint/kyverno/policies/policies.tar.gz
```

Both archives are committed to the repo and embedded via `//go:embed` in
their package's `embed.go`. **This is a compile-time mechanism.** There
is no runtime schema/policy-loading seam, no directory the binary scans
at startup, and no plan to add one — this is intentional, and consistent
with the other compile-time-only injection pattern in this repo (see
`docs/DEVELOPMENT.md`'s "core data + org-injectable override" section):
if you need different embedded data, you regenerate the archive and
rebuild the binary.

## Kubeconform schemas

`scripts/pull-schemas.sh` clones a public
[`kubernetes-json-schema`](https://github.com/yannh/kubernetes-json-schema)-style
repository (default source: `SCHEMA_REPO`, see the script) and packages
its `custom-standalone-strict`, `master-local`, and
`master-standalone-strict` directories into `schemas.tar.gz`.

**`SCHEMA_REPO` is a plain overridable environment variable**, not a
hardcoded literal:

```sh
SCHEMA_REPO=https://github.com/<your-org>/kubernetes-json-schema \
  task update:schemas
task build
```

### Org-specific CRDs with no public schema

If your org has internal CRDs that will never appear in any public
schema catalog, the supported path is: maintain your own fork/mirror of
the schema repository that includes **both** the public upstream set
_and_ your org's CRD schemas (in the same directory layout
`kubeconform`/this repo's `SchemaLocations()` expects), point
`SCHEMA_REPO` at it, and rebuild. There is intentionally no separate
"extra schema directory" runtime seam for this — it would duplicate the
one mechanism that already exists (swap the source, regenerate the
archive, recompile) for no real benefit, and would introduce a second,
inconsistent way to inject the same kind of data.

## Kyverno policies

Kyverno validation is **off by default** (`"kyverno"` is the one entry in
`pkg/validator/phases.go`'s `defaultOffSteps` — see `docs/DEVELOPMENT.md`'s
generic check-enablement section). This is different from kubeconform:
there's no generic, public Kyverno policy bundle that makes sense as a
default for an arbitrary org, so `scripts/pull-policies.sh` currently
just writes a placeholder archive (a `kyverno-policies/README.md`, no
real policy YAML) rather than pulling anything real.

To use Kyverno validation:

1. Point `scripts/pull-policies.sh` (or replace it entirely) at your own
   policy source and rebuild, so `policies.tar.gz` contains real policy
   manifests under a `base/` directory.
2. Opt in with `--enable-checks kyverno` (there is no dedicated
   `--enable-kyverno` flag — see `docs/DEVELOPMENT.md`'s generic
   check-enablement section).
3. `kyverno.NamespaceSelectorLabelKeys` (namespace-label keys to strip
   `namespaceSelector` gates for, since offline `kyverno apply` has no
   namespace labels available) is available today and defaults to empty
   (no stripping) — set it from your own configuration layer if needed.

> **Planned, not yet implemented:** `kyverno.ExcludedRules`
> (policy/rule combinations to drop from results) and
> `kyverno.IncludeComponents` (kustomize component paths layered on top of
> the policy base) are designed but not yet built — see the Kyverno
> section of the ongoing parity-remediation plan. Once landed, both will
> follow the same empty/no-op-by-default contract as
> `NamespaceSelectorLabelKeys`, and will be **runtime-checked exported
> variables**, not part of the embedded archive — the archive controls
> _what policies exist_, these vars control _how they're applied and
> filtered_ once loaded. This note will be removed once they ship.
