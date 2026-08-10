# Embedded Resources: Schemas & Policies

`pkg/lint/kubeconform` and `pkg/lint/kyverno` both need external data
(JSON schemas, Kyverno policy manifests) that's too large and too
slow-changing to fetch at runtime on every invocation. Both packages can
embed that data into the compiled binary via `//go:embed`, populated by a
`scripts/pull-*.sh` helper. This doc explains that mechanism and,
specifically, how an org supplies schemas/policies for its own
CRDs/policies that have no generic public source.

## Table of Contents

- [The pattern](#the-pattern)
- [Kubeconform schemas](#kubeconform-schemas)
  - [Org-specific CRDs with no public schema](#org-specific-crds-with-no-public-schema)
- [Kyverno policies](#kyverno-policies)

## The pattern

```text
scripts/pull-schemas.sh    ──writes──> pkg/lint/kubeconform/schemas/schemas.tar.gz
scripts/pull-policies.sh   ──writes──> pkg/lint/kyverno/policies/policies.tar.gz
```

Both archives are gitignored, generated, build-time artifacts (`task
schemas:pull`/`policies:pull` regenerate them, and `task build`/
`task test`/`task lint` depend on those tasks so they're always present
before anything that needs them runs) and embedded via `//go:embed`, but
**only when built with the `embedschemas` build tag**
(`go build -tags embedschemas`). This repo's own binary is always built
this way: `task build`/`task test` pass `-tags embedschemas` directly,
and `.goreleaser.yaml`'s `builds[].flags` passes the same tag (with its
`before.hooks` running `task schemas:pull`/`task policies:pull`
first so the archives exist on disk) for every published GitHub Release
binary - so the standalone CLI works out of the box with no extra
configuration, matching a local `task build`. Without the tag - the
default for a bare `go build ./cmd/k8s-gitops-ci`, and how downstream Go
module consumers import these packages as a library - no archive is
compiled in at all, keeping the package importable without needing the
(large, gitignored) archive to be present.
`Extract()`/`EnsureArchive()` return `ErrNoEmbeddedArchive` in that case,
and callers (`kubeconform.ExtractSchemas`, `kyverno.PreparePolicies`,
`pkg/pipeline`'s Setup phase, `pkg/validator/phases.go`) fall back
gracefully rather than failing the run - kubeconform/Kyverno then rely on
an explicitly-supplied `Options.SchemaDir`/`Options.PolicyPath` instead.
A binary missing the tag doesn't fail loudly: kubeconform still
validates built-in Kubernetes kinds fine (via its own default schema
location) and only errors with "could not find schema for X" for kinds
that need the embedded archive (CRDs, and `CustomResourceDefinition`
itself) - if you see that error against a binary you expected to have
schemas embedded, verify it was actually built with `-tags embedschemas`
(e.g. compare its size against one you know has the tag; the archive
adds roughly its own compressed size to the binary).

`kubeconform.ExtractSchemas` and `kyverno.PreparePolicies` are themselves
plain overridable package vars (the same exported-override-var pattern
`docs/DEVELOPMENT.md` documents elsewhere), defaulting to the
archive-extracting implementation above. An org/consumer layer that wants
a different schema/policy source entirely (e.g. pulled from an OCI
artifact at process startup instead of `//go:embed`) can replace either
var with its own function from its own `main()`/`Configure()` equivalent,
without needing the `embedschemas` build tag at all -
`kyverno.PreparePoliciesFrom(dir)` exposes the standard render/strip
behavior against an already-extracted directory for exactly this case,
so a custom source only has to provide the raw directory, not
reimplement policy rendering.

### Both archives must be byte-reproducible

Both `pull-schemas.sh` and `pull-policies.sh` build their archive with:

```sh
tar --sort=name --mtime="UTC 1970-01-01" --owner=0 --group=0 --numeric-owner \
  -cf - -C "$TMP_DIR" "<top-level-dir>" | gzip -n >"$TMP_DIR/archive.tar.gz"
```

rather than a plain `tar -czf`. Without `--sort`/`--mtime`/`--owner`/
`--group`/`--numeric-owner`, `tar` picks up non-determinism from the
filesystem itself (directory-walk order, and the mtime/uid/gid of files
that were just freshly checked out or generated in a temp dir); without
`gzip -n`, gzip embeds its own header timestamp. The result: two
back-to-back runs over logically-identical content produce **different
bytes** every time.

That matters beyond reproducibility for its own sake: both archives are
`//go:embed`ed (see [The pattern](#the-pattern) above), so a
byte-different archive is a content change from Go's build/test cache's
point of view, even when nothing an operator would call "changed"
actually did. A non-deterministic archive silently invalidates the
build/test cache for every package that embeds it — `pkg/pipeline`,
`pkg/lint/kyverno`, `pkg/validator`, `cmd/k8s-gitops-ci` — on every
single CI run. If you add a third `pull-*.sh` script or otherwise
regenerate either archive, reuse this same `tar`/`gzip` invocation
rather than a plain `tar -czf`.

Those flags (`--sort`/`--mtime`/`--owner`/`--group`/`--numeric-owner`)
are **GNU tar** features. macOS's default `tar` is bsdtar, which rejects
`--sort=name`, so both scripts resolve GNU tar explicitly — `gtar` if
present (homebrew's `gnu-tar`), otherwise a `tar` that reports itself as
GNU — and **hard-fail** with install guidance if only bsdtar is found,
rather than silently producing a non-reproducible archive with the wrong
tar. On macOS: `brew install gnu-tar` (provides `gtar`).

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
  task schemas:pull
task build
```

### Pinning (`SCHEMA_REPO_SHA`/`SCHEMA_REPO_BRANCH`)

By default `scripts/pull-schemas.sh` doesn't float to `SCHEMA_REPO`'s
branch tip on every run - it fetches and checks out an explicit,
tracked commit, **`SCHEMA_REPO_SHA`** (pinned in the script itself, next
to a `# renovate: datasource=git-refs depName=...` marker comment), so
`task schemas:pull`/`task build` produce the exact same
`schemas.tar.gz` every time for a given commit of this repo, instead of
silently picking up whatever the upstream schema repo happens to
contain at build time. `task update:schemas` (and Renovate, which tracks
`SCHEMA_REPO_BRANCH`'s tip via `renovate.json`'s `customManagers`)
bumps `SCHEMA_REPO_SHA` when that branch advances - the pin bump and the
archive repack are two separate steps: `update:schemas` rewrites the
pinned SHA in `scripts/pull-schemas.sh`, and the next `schemas:pull`/
`build` repacks `schemas.tar.gz` from it. Upgrading the schema set is
then an explicit, reviewable diff instead of untracked drift.

In pinned mode the script also short-circuits when it's already up to
date: it writes a sibling marker (`schemas.tar.gz.ref`, gitignored)
recording the `SCHEMA_REPO_SHA` the current archive was built from, and
skips the network `git fetch` + repack entirely when that marker already
matches. Since `schemas:pull` is a `deps:` of `test`/`build`/`lint`/
`vulncheck` and runs on every CI invocation, this avoids a per-run
upstream fetch whenever the pin hasn't moved. Floating mode (empty
`SCHEMA_REPO_SHA`) always re-fetches, since a branch tip can advance
without the marker changing.

`SCHEMA_REPO_BRANCH` (default `main`) is both the ref `update:schemas`
resolves to bump the pin, and the ref `pull-schemas.sh` fetches when
`SCHEMA_REPO_SHA` is explicitly set to empty - the "floating" fallback,
for an org overriding `SCHEMA_REPO` to a different fork that doesn't
share the default pin's commit history and doesn't have a known-good SHA
to pin to yet:

```sh
SCHEMA_REPO=https://github.com/<your-org>/kubernetes-json-schema \
SCHEMA_REPO_BRANCH=main \
SCHEMA_REPO_SHA= \
  task schemas:pull
```

Once you have a commit you're happy with, pin it by setting
`SCHEMA_REPO_SHA` back to that commit's SHA (either as an env var
override, or by editing the default in `scripts/pull-schemas.sh` if
you've forked this repo too).

### Org-specific CRDs with no public schema

If your org has internal CRDs that will never appear in any public
schema catalog, the supported path is: maintain your own fork/mirror of
the schema repository that includes **both** the public upstream set
_and_ your org's CRD schemas (in the same directory layout
`kubeconform`/this repo's `SchemaLocations()` expects), point
`SCHEMA_REPO` at it, and rebuild with `-tags embedschemas`. Prefer this
over overriding `kubeconform.ExtractSchemas` for this specific case: it
keeps the public and org-specific schema sets as one archive behind one
`SchemaLocations()` lookup, rather than needing a second directory /
override function kept in sync alongside it.

## Kyverno policies

Kyverno validation is **off by default** (`"kyverno"` is the one entry in
`pkg/validator/phases.go`'s `defaultOffSteps` — see `docs/DEVELOPMENT.md`'s
generic check-enablement section). This is different from kubeconform:
there's no generic, public Kyverno policy bundle that makes sense as a
default for an arbitrary org, so `scripts/pull-policies.sh` currently
just writes a placeholder archive (a `kyverno-policies/README.md`, no
real policy YAML) rather than pulling anything real.

Like `pull-schemas.sh` (see [Pinning](#pinning-schema_repo_shaschema_repo_branch)
above), `pull-policies.sh` short-circuits via a sibling marker
(`policies.tar.gz.ref`, gitignored) instead of always regenerating: it
records a `PLACEHOLDER_REF` constant for the currently-generated
placeholder content, and skips rebuilding the archive when the marker
already matches. Since there's no upstream ref to pin against yet (the
content is a static placeholder, not fetched from anywhere), bump
`PLACEHOLDER_REF` in the script itself whenever you change what it
generates - e.g. once you replace the placeholder with a real policy
source pulled from somewhere, at which point this should gain a real
upstream SHA pin the same way `pull-schemas.sh` has one.

To use Kyverno validation:

1. Point `scripts/pull-policies.sh` (or replace it entirely) at your own
   policy source and rebuild, so `policies.tar.gz` contains real policy
   manifests under a `kyverno-policies/base/` directory.
2. Opt in with `--enable-checks kyverno` (there is no dedicated
   `--enable-kyverno` flag — see `docs/DEVELOPMENT.md`'s generic
   check-enablement section). Once enabled, every successfully-built
   overlay is validated against the prepared policy bundle - see
   [CI.md](CI.md)'s Registered checks section.

Three runtime-checked exported variables (not part of the embedded
archive - the archive controls _what policies exist_, these control _how
they're applied and filtered_ once loaded) tune Kyverno validation, all
empty/no-op by default:

- `kyverno.NamespaceSelectorLabelKeys` — namespace-label keys to strip
  `namespaceSelector` gates for, since offline `kyverno apply` has no
  namespace labels available. A policy's `namespaceSelector` is only
  stripped when its `matchLabels` contains one of these keys - an
  unconfigured or non-matching selector is left untouched.
- `kyverno.IncludeComponents` — kustomize component paths (relative to
  the policy bundle's `overlays/_ci` directory, e.g.
  `"../../components/restrict-old-registry"`) layered on top of the
  bundle's `base/` when preparing policies. Defaults to base-only.
- `kyverno.ExcludedRules` — a `map[string][]string` of policy name to the
  rule names to drop from that policy's results (an empty slice excludes
  every rule under that policy). Defaults to empty (nothing excluded).
