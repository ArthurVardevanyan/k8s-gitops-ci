---
name: exemptions
description: >
  How to add, scope, and audit CI exemptions in this repository.
  Activate when asked to exempt a check, suppress a finding, or add a
  test.sh EXEMPTIONS block.
---

# Exemption Handling Skill

Full reference: [`docs/EXEMPTIONS.md`](../../../docs/EXEMPTIONS.md) and
[`docs/HOOKS.md`](../../../docs/HOOKS.md). This skill is a decision-oriented
quick-reference — read the docs for authoritative detail.

## Pick the right mode

| Situation                                                                                   | Use                             |
| ------------------------------------------------------------------------------------------- | ------------------------------- |
| You control the manifest and want a single-resource exemption visible next to the field     | Annotation on the resource      |
| Exempting every file in a directory, a resource you don't control, or a non-Kubernetes YAML | `EXEMPTIONS=(...)` in `test.sh` |

## Annotation exemption

Add directly to the resource's `metadata.annotations`:

```yaml
metadata:
  annotations:
    gitops-ci.k8s.io/exempt-<check-id>: "<value>"
```

`<value>` must exactly match the finding's `Value` (or `Token` when the
check sets one — see [Value vs. Token](../../../docs/EXEMPTIONS.md#value-vs-token)).

## `EXEMPTIONS=(...)` in `test.sh`

### Syntax

```sh
export EXEMPTIONS=(
  "check=<id>,file=<path-suffix>"
  "check=<id>,kind=<Kind>,name=<name>"
)
```

Each entry is a comma-separated set of `key=value` pairs. `check=` is
required; all other keys are optional narrowing filters. Quote each entry.
`export` prefix is supported and recommended. See the
[selector reference](../../../docs/EXEMPTIONS.md#selector-reference) for all
available keys (`file`, `kind`, `name`, `namespace`, `match`, `value`,
`path`).

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
merge across ancestors). This means **one `test.sh` at a shared parent
directory can cover multiple non-app subdirectories** that have no
`test.sh` of their own — you don't need a `test.sh` in every leaf
directory. If a subdirectory has its own `test.sh`, that one applies
instead and the parent's is never consulted for those files. Only
`check=kubeconform` selectors take effect from non-app `test.sh` files
today.

### Exemptable check IDs

See the full table in
[`docs/EXEMPTIONS.md#exemptable-check-ids`](../../../docs/EXEMPTIONS.md#exemptable-check-ids).
Commonly used IDs:

| ID               | When to use                                                                 |
| ---------------- | --------------------------------------------------------------------------- |
| `sync-options`   | Resource in a non-builtin API group that ArgoCD manages without dry-run     |
| `image-checksum` | Image that cannot be pinned to a digest (e.g. `:latest` by upstream design) |
| `kubeconform`    | Non-Kubernetes YAML (no `kind`/`apiVersion`) in a directory under `--dirs`  |
| `namespace`      | Cluster-scoped resource that a check misclassifies as namespace-scoped      |

`cluster-identity` is deliberately **non-exemptable** — never attempt to
exempt it.

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

In the app's `test.sh`, or as an annotation on the resource:

```sh
export EXEMPTIONS=(
  "check=image-checksum,file=<app>/base/deployment.yaml"
)
```

Or annotation (scoped to the exact image on that resource only):

```yaml
gitops-ci.k8s.io/exempt-image-checksum: "registry.example.com/app:latest"
```

### Exempt a CRD-backed resource from sync-options

```sh
export EXEMPTIONS=(
  "check=sync-options,kind=MyCustomKind,name=my-instance"
)
```

## Verification checklist

- [ ] Selector is as narrow as possible (prefer `file=path/to/file.yaml`
      over bare `file=filename.yaml` to avoid basename collisions).
- [ ] `check=` value is a valid exemptable ID (confirm in
      [`docs/EXEMPTIONS.md`](../../../docs/EXEMPTIONS.md#exemptable-check-ids)).
- [ ] Entry is quoted; `export` prefix present.
- [ ] A malformed entry causes a **blocking build error** — run the
      pipeline locally to confirm the entry parses cleanly before merging.
- [ ] Exemption is documented with an inline comment explaining why.
