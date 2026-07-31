# k8s-gitops-ci

Generic GitOps CI engine for Kubernetes manifests. Detects changed files,
builds Kustomize/Helm overlays, and runs a registry-driven set of
validators (namespace scope, PSA labels, RBAC, image pinning, named
ports, pod-spec defaults, sync options, cluster-identity leakage, and
more) plus wrappers around common lint tools (kubeconform, Kyverno,
golangci-lint, markdownlint, prettier, shellcheck) — distributed as a
single Go binary and a set of importable packages.

This is the org-agnostic core. Organization-specific configuration is
injected through the `provider.Providers` seams and exported package
override variables — see [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md)'s
"Design Conventions" section for how that works and how to wire your own.

## Install

```sh
go install github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/k8s-gitops-ci@latest
```

Or build from source (see [Development](#development) below), or pull the
container image built by this repo's own release pipeline.

## Usage

```sh
k8s-gitops-ci --help
```

One example per subcommand:

```sh
# Full CI pipeline: PR checks, linting, static checks, resource compliance.
k8s-gitops-ci pipeline --url https://github.com/<org>/<repo> --pr 123

# Run every validator against the working tree (no PR/remote needed).
k8s-gitops-ci test-all ./kubernetes

# Full-repo scan, printing only failing sections.
k8s-gitops-ci scan-all

# Build rendered YAML for a specific app/cluster overlay.
k8s-gitops-ci build-yaml --app my-app --cluster my-cluster

# Individual linters, each usable standalone:
k8s-gitops-ci markdownlint README.md docs/*.md
k8s-gitops-ci prettier kustomization.yaml
k8s-gitops-ci shellcheck scripts/*.sh
k8s-gitops-ci golangci ./...
k8s-gitops-ci kubeconform kubernetes/**/*.yaml
k8s-gitops-ci yaml-syntax kubernetes/**/*.yaml

# Static checks:
k8s-gitops-ci kustomize-fix kubernetes/**/kustomization.yaml
k8s-gitops-ci check-starting-csv kubernetes/**/*.yaml
k8s-gitops-ci ghost-patches kubernetes/my-app/overlays/my-cluster
k8s-gitops-ci sort-configs
k8s-gitops-ci update-scaffold-status

k8s-gitops-ci version
```

Run `k8s-gitops-ci <command> --help` for per-command flags.

### Key `pipeline` flags

- `--url` / `--pr` — `--url` is the **bare repository URL**
  (`https://github.com/org/repo`), not a pull-request URL; the PR number
  goes in the separate `--pr` flag. Passing a full PR URL
  (`.../pull/123`, `.../pulls/123`, or GitLab's `.../merge_requests/123`)
  into `--url` fails fast with an actionable error instead of a cryptic
  `git clone` failure.
- `--comment` — post a PR comment summarizing the run. **Default: off.**
  Requires repo/PR context (`--url` + `--pr`, or the equivalent
  Tekton-injected env vars) to actually be available; if that context is
  missing, comment posting is skipped with a logged reason even when
  `--comment` is passed. Any Task/script invoking
  `k8s-gitops-ci pipeline` that wants PR comments (as this repo's own
  reference/downstream Tekton Task does) must pass `--comment`
  explicitly — there is no separate `--no-comment` override; omitting
  `--comment` is sufficient to opt out.
- `--verbose` — streams every check's start/pass/fail as it runs (via an
  internal `logger.Logger`), plus a final `Summary: info=N, warn=N,
error=N` line and per-phase timing, instead of only the aggregated
  pass/fail result at the end. Also available on `test-all`,
  `build-yaml`, and `scan-all`.
- `--dirs`, `--disable-checks`, `--enable-checks`, `--hook-source`,
  `--concurrency`, `--assume-openshift`, `--app`, `--cluster` — every
  changeset-scoping and check-enablement flag `pipeline` accepts is also
  accepted by `test-all` and `scan-all`, so a failing `pipeline --url ...
--pr ...` run can be reproduced locally without a remote/PR (e.g.
  `k8s-gitops-ci test-all --dirs=kubernetes/ --disable-checks=avp`). Note
  `test-all`'s positional `[dirs...]` (a full-tree walk, replacing the
  changeset source entirely) is distinct from `--dirs` (a path-prefix
  filter applied on top of the resolved changeset) — see
  [`docs/CI.md`](docs/CI.md) for the details.

## Development

```sh
task build   # build bin/k8s-gitops-ci with version metadata
task test    # run the test suite
task lint    # run golangci-lint
task ci      # full CI pipeline: format check, lint, vulncheck, test, build
```

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) for the full Task target
reference, repository structure, and the design conventions to follow
when contributing (the `provider.Providers` seam, exported-override-var
pattern, and the generic check-enablement mechanism).

## Documentation

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — the top-level entry
  point: runtime flow, package map, and a "Where do I find X?" table
- [`docs/CI.md`](docs/CI.md) — pipeline phases, every mode
  (`pipeline`/`test-all`/`scan-all`/`build-yaml`), the full registered-
  check list, and the direct-vs-external finding classification
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — build/test/lint,
  repository structure, design conventions, how to add a new validator
- [`docs/HOOKS.md`](docs/HOOKS.md) — the `test.sh` contract
  (`SCAFFOLD=`/`AVP_EXCLUDE=`/`EXEMPTIONS=(...)`/hook directives) and
  which of them are actually wired today
- [`docs/EXCEPTIONS.md`](docs/EXCEPTIONS.md) — the exemption framework:
  annotation vs. `EXEMPTIONS` selector modes, exemptable check IDs,
  adding exemption support to a new check
- [`docs/TEKTON.md`](docs/TEKTON.md) — this repo's own Tekton
  `PipelineRun`/PaC-trigger/caching setup
- [`docs/RELEASE.md`](docs/RELEASE.md) — versioning, changelog, and
  published-artifact scope
- [`docs/SECURITY.md`](docs/SECURITY.md) — trust model, `exec.Command`
  audit, file-permission rationale
- [`docs/SCHEMAS.md`](docs/SCHEMAS.md) — how embedded kubeconform
  schemas / Kyverno policies work, and how to supply your own CRD
  schemas or real Kyverno policies
