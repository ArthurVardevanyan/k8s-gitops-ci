---
name: exemptions
description: >
  How to add, scope, and audit CI exemptions in this repository.
  Activate when asked to exempt a check, suppress a finding, make a CI
  check pass, or add a test.sh EXEMPTIONS block.
---

# Exemption Handling Skill

Full reference: [`docs/EXEMPTIONS.md`](../../../docs/EXEMPTIONS.md) and
[`docs/HOOKS.md`](../../../docs/HOOKS.md). This skill is a self-contained
decision guide — the docs are for deep dives only.

**Prefer annotation when possible** — it ships with the PR so it takes
effect immediately without the PR trust-model gotcha. Use `EXEMPTIONS=(...)`
only when: the check doesn't support annotation mode, the resource is
outside the repo, or you need to exempt a whole directory.

## Pick the right mode

| Situation                                                                                   | Use                             | Visible in your own PR run?           |
| ------------------------------------------------------------------------------------------- | ------------------------------- | ------------------------------------- |
| You control the manifest and want a single-resource exemption visible next to the field     | Annotation on the resource      | Yes (ships with the PR)               |
| Exempting every file in a directory, a resource you don't control, or a non-Kubernetes YAML | `EXEMPTIONS=(...)` in `test.sh` | Only after merge, or via `/hook-test` |

## Complete table of exemptable check IDs

Every check registered via `check.Register` (`namespace`, `psa-labels`,
`rbac-readonly`, `rbac-wildcards`, `crb`, `sync-options`, `image-checksum`,
`named-ports`, `podspec-defaults`, `placeholder`) becomes exemptable via
the **selector** mode automatically (under its own ID). The large file
check (`large-file`) and the kubeconform lint step (`kubeconform`) are
registered as exemptable manually via `exempt.RegisterExemptable`.
`cluster-identity` is also registered but is **deliberately non-exemptable** (infraID mismatch,
invalid JSON — structural findings that should not be waved away).
Annotation-mode support is separate: a check only honors
`gitops-ci.k8s.io/exempt-<id>` if it sets both `Finding.Value` **and**
`Finding.Annotations` on its findings (see [Value vs. Token](../../../docs/EXEMPTIONS.md#value-vs-token)).

| ID                                                | Annotation mode? | What the annotation or `value=` must equal                                  |
| ------------------------------------------------- | ---------------- | --------------------------------------------------------------------------- |
| `image-checksum`                                  | yes              | The exact image reference (report's `Image` column)                         |
| `cluster-name`                                    | yes              | The foreign cluster-name token (report's `Value` column)                    |
| `project-ref`                                     | yes              | The foreign project number or project ID                                    |
| `rbac-wildcards`                                  | yes              | One or more wildcarded rule fields, comma-separated (`verbs,resources`)     |
| `named-ports`                                     | yes              | The numeric port as a string                                                |
| `podspec-defaults`                                | yes              | The joined missing-fields list (e.g. `securityContext, resources.requests`) |
| `namespace`                                       | no               | — (selector-only: `kind`/`name`/`file`)                                     |
| `psa-labels`                                      | no               | — (selector-only)                                                           |
| `rbac-readonly`                                   | no               | — (selector-only)                                                           |
| `crb`                                             | no               | — (selector-only)                                                           |
| `sync-options`                                    | no               | — (selector-only: `kind`/`name`/`file`)                                     |
| `placeholder`                                     | no               | — (selector-only; `value=`/`match=` match the flagged token)                |
| `kubeconform`                                     | no               | — (file-level only: `file=`)                                                |
| `large-file`                                      | no               | — (file-level only: `file=`)                                                |
| `cluster-identity`                                | **never**        | Deliberately non-exemptable (infraID mismatch, invalid JSON)                |
| `<family>/<category>/<rule>` (runtime validation) | **never**        | Deliberately non-exemptable — see below                                     |

**Runtime-validation check IDs are never exemptable.** Any ID of the form
`<family>/<category>/<rule>` (e.g.
`kubernetes/apps/daemonset-min-ready-seconds-invalid`,
`kubernetes/batch/backoff-limit-invalid`,
`kubernetes/container/duplicate-container-names`,
`kubernetes/storage-class/...`, `kubernetes/pod-spec/...`) comes from
`pkg/validator/runtime/<family>/` and appears in the
**"Runtime Validation"** report section, not "Resource Compliance". These
are 1:1 ports of the Kubernetes API server's own validation, so they only
fire on manifests the cluster itself would reject — an exemption would
just defer the failure to apply time. The runtime adapter implements
`check.NonExemptable`, so `check.Register` never registers these IDs as
exemptable; neither an annotation nor a `check=` selector can match one.
**Fix the manifest.** Never write an `EXEMPTIONS` entry for a finding in
the Runtime Validation section.

**NAD validation** (`pkg/validator/nad`) is not part of the `check.Register`
framework at all — hard-error NAD findings are never exemptable.
`cluster-identity` is a deliberately non-exemptable structural bucket;
never attempt to exempt it.

## Annotation exemption

Add directly to the resource's `metadata.annotations`:

```yaml
metadata:
  annotations:
    gitops-ci.k8s.io/exempt-<check-id>: "<value>"
```

`<value>` must exactly match the entry's "what it must equal" column above.
`exempt.Accepts` fails closed: an empty annotation value, or no annotations
at all, never matches — even against an empty-valued finding.

## `EXEMPTIONS=(...)` in `test.sh`

### Syntax

Use the **multi-line array form only**. Each quoted line is **one entry**;
the `key=value` pairs within the line are comma-separated:

```sh
export EXEMPTIONS=(
  "check=image-checksum,value=registry.example.com/app:latest"
  "check=sync-options,kind=ArgoCD,name=my-argocd-instance"
)
```

**Never use** `EXEMPTIONS="check=x,file=y"` (single-line, multi-key form):
the parser (`parseExemptionSingle`) splits on commas into **entries**, not
keys — this produces a wide-open `check=x` selector (exempts the entire
check for the whole run) **plus** a blocking "missing check" error for
`file=y`. Only single-key single-line entries work (`EXEMPTIONS="check=sync-options"`).

**`dir=` is not a recognized key** in `EXEMPTIONS=(...)`. Writing
`dir=...` is a blocking "unknown exemption key" error. Use per-file
`file=` entries or a shared-parent `test.sh` to cover directories.

### Selector key semantics

| Key          | Match                                                                                               |
| ------------ | --------------------------------------------------------------------------------------------------- |
| `check=`     | Required — the check ID this selector exempts                                                       |
| `file=`      | Exact basename, **or** suffix match with `/` prepended (the selector value must NOT start with `/`) |
| `kind=`      | Exact match                                                                                         |
| `name=`      | Exact match                                                                                         |
| `namespace=` | Exact match                                                                                         |
| `value=`     | Exact match against the finding's `Value`/`Token`                                                   |
| `match=`     | Substring match against the finding's `Value`/`Token`                                               |
| `path=`      | Suffix-aligned dot/bracket path; `[]` wildcards the index, `[N]` pins it                            |

Selectors are merged **flatly across all apps** in a run, not scoped to the
app that declared them — keep selectors narrow.

### Where to put `test.sh`

**Kustomize app directory** (has `base/`, `overlays/`, or `components/`):
place `test.sh` at the app root. It is read during the Build YAML
phase whenever any file in that app is in the changeset.

**Non-app directory** (no kustomize structure — e.g. `okd/node-config/`):
place `test.sh` in that directory, or in any **ancestor** directory.
Resolution walks upward from a changed file's own directory toward the
repository root during the Linting phase (`resolveNonAppHookConfigs`),
stopping at the **nearest ancestor that declares a `test.sh`**
(closest-match-wins — like `.gitignore`/`.editorconfig` cascading, not a
merge across ancestors). All check selectors take effect from non-app
`test.sh` files — they are merged into both the Linting and Post-Build
phases, covering `kubeconform`, `placeholder`, `named-ports`, and all
other exemptable checks.

**PR runs read `test.sh` from main** — `SourceMain` is the default for
pull_request events. A new `EXEMPTIONS` entry added in the PR has no
effect on the PR's own CI run until it merges, or until you trigger
SourcePR via a `/hook-test` comment. Annotation exemptions are unaffected
because they ship with the manifest in the PR.

### Built-in Tekton-PaC exemption

`PipelineRun` resources under a `.tekton/` directory are **already exempt**
from `sync-options`/`namespace` by default (built-in selectors in
`pkg/validator/tekton_exemptions.go`). You do not need a `test.sh` for this.
To disable the default, set `validator.TektonPACDir = ""` at process startup.

## What the finding's `File` actually is

This is the most common reason exemptions "don't work." All resource-compliance
doc-checks are **render-sensitive**: for files composed into a rendered overlay,
the finding's `File` is the **overlay directory** (e.g.
`apps/foo/overlays/prod`), **not** the raw source manifest (`base/deployment.yaml`).

**What to do:** read `Kind`/`Name`/`Image`/`Value` straight off the
failing report row, and build the selector from those columns. Prefer
`value=` (exact match on the image, cluster name, port, etc.) or
`kind=`+`name=` (resource identity) for rendered-path findings. Using
`file=` on an overlay basename (e.g. `file=prod`) is fragile — many
overlays share the same name and the selector is merged across all apps.

## Verify locally

1. Build: `task build` (in this repo) → `bin/k8s-gitops-ci`.
2. In the GitOps repo being fixed, run one of:
   - `bin/k8s-gitops-ci test --all` — uncommitted working-tree diff (fastest iteration)
   - `bin/k8s-gitops-ci test <changed-dir>` — full tree walk under a directory
   - `bin/k8s-gitops-ci build-yaml --app <app> --cluster <cluster>` — single app/overlay
   - Append `--lint-only` to skip build checks, `--verbose` for details.
3. Local runs **automatically resolve test.sh from the working tree** (`SourceLocal`)
   — no flag needed. Annotation exemptions are always visible because they
   ship with the manifest.
4. Success: the failed row disappears, the "Accepted Exemptions" block lists
   the applied exemption (except kubeconform, which is silent), and the
   process exits 0.

## Common patterns

### Exempt several files under a shared directory from kubeconform (non-Kubernetes YAML)

Place **one** `test.sh` at the shared parent directory rather than one per
leaf directory — ancestor walk-up means files in subdirectories with no
`test.sh` of their own still match:

```sh
# okd/test.sh — covers okd/install-config.yaml directly, AND
# okd/node-config/*.yaml even though node-config/ has no test.sh itself.
export EXEMPTIONS=(
  "check=kubeconform,file=install-config.yaml"
  "check=kubeconform,file=node-config/gpu-1.yaml"
  "check=kubeconform,file=node-config/worker-1.yaml"
)
```

> **`file=` path-suffix matching:** use a partial path suffix
> (`node-config/foo.yaml`) or bare basename (`foo.yaml`) — not the full
> repo-relative path (`okd/node-config/foo.yaml`). The suffix check
> requires a `/` separator before the value, so a root-relative path
> never self-matches.

### Exempt an image from digest pinning

**Using an annotation** (recommended for single-resource exemptions):

```yaml
gitops-ci.k8s.io/exempt-image-checksum: "registry.example.com/app:latest"
```

The value is the exact image reference from the report's `Image` column.

**Using EXEMPTIONS selector** (e.g. for images across many resources):

```sh
# Prefer value= for rendered-path findings (see "What the finding's File actually is")
export EXEMPTIONS=(
  "check=image-checksum,value=registry.example.com/app:latest"
)
```

### Exempt a CRD-backed resource from sync-options

```sh
export EXEMPTIONS=(
  "check=sync-options,kind=MyCustomKind,name=my-instance"
)
```

### Exempt binary/large files from the LargeFile check

```sh
export EXEMPTIONS=(
  "check=large-file,file=gwe.db"
  "check=large-file,file=High_Speed.curaprofile"
)
```

The `file=` key matches by basename or `/`-prefix suffix. For files in a
shared directory, use a partial suffix (e.g. `cura/High_Speed.curaprofile`).

## Things that can't be exempted / when exemptions are the wrong tool

- **`cluster-identity`** — deliberately non-exemptable (infraID mismatch,
  invalid JSON). These are structural findings about malformed data.
- **Runtime-validation findings** (the "Runtime Validation" report
  section; IDs shaped `<family>/<category>/<rule>`) — deliberately non-exemptable
  via `check.NonExemptable`. The API server would reject the manifest, so
  an exemption only defers the failure to sync time. Fix the manifest.
- **NAD hard errors** — outside the `check.Register` framework; never exemptable.
- **`--disable-checks <id>`** — disables an entire check across the whole
  run. This is a different mechanism from exemptions. Use it when an
  environment genuinely can't provision a given tool (a missing lint tool
  means the pipeline didn't actually check what it claims to have checked).
  Each check/step has a string ID for this mechanism (`DisabledChecks`/
  `EnabledChecks` in `Options`).

## Verification checklist

- [ ] Selector is as narrow as possible (`value=` or `kind=`+`name=`
      preferred for rendered-path findings; `file=` on overlay basenames
      is fragile because selectors merge across all apps).
- [ ] `check=` value is one of the IDs in the full table above (not
      `cluster-identity`, not a runtime-validation
      `<family>/<category>/<rule>` ID,
      and not a NAD error).
- [ ] Entry is quoted; `export` prefix present.
- [ ] Multi-line array form used — never `EXEMPTIONS="check=x,file=y"`
      (comma-splitting produces a wide-open selector + blocking error).
- [ ] `dir=` not used (blocking "unknown exemption key" error).
- [ ] Annotation value copied verbatim from the report row (check the
      `Image`/`Value` column of the failed row).
- [ ] Local verification done: `task build` then `test --all` or `test <dir>`
      (local runs auto-resolve `test.sh` from the working tree — no flag
      needed). PR runs use `SourceMain`; test PR-branch `test.sh` via
      `/hook-test` or after merge.
- [ ] Exemption documented with an inline comment explaining why.
