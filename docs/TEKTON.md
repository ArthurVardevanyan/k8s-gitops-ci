# Tekton

This repo's own CI is a **single** Tekton `PipelineRun` with two inline
tasks (`build` and `lint`, run in parallel) — deliberately much simpler
than a typical multi-Task Tekton pipeline, since `k8s-gitops-ci` itself is
the thing that would otherwise be a pile of separate lint/test/build
Tasks. This document describes that actual footprint, not a general
Tekton architecture.

## Table of Contents

- [Directory layout](#directory-layout)
- [PaC trigger](#pac-trigger)
- [The build task](#the-build-task)
- [The lint task](#the-lint-task)
- [Caching](#caching)
- [Known limitations](#known-limitations)

## Directory layout

- **`.tekton/k8s-gitops-ci.yaml`** — the Pipelines-as-Code (PaC)
  `PipelineRun`: one `pipelineSpec` with two `tasks[]` entries (`build`,
  `lint`, running in parallel — neither has a `runAfter` on the other),
  each with its own inline `taskSpec` (its own params/workspaces/volumes/
  stepTemplate/steps, entirely independent of the other's). There is no
  separate `Pipeline` or `Task` CRD file anywhere — everything is inline
  in this one `PipelineRun`.
- **`tekton/base/repo.yaml`** — the Pipelines-as-Code `Repository` CR
  (`concurrency_limit: 1`).
- **`tekton/k8s-gitops-ci/`** — the actual deployed overlay:
  `kustomization.yaml` (namespace `homelab`, resources `repo.yaml` +
  `go-cache-pvc.yaml`), a duplicate `repo.yaml` (identical to
  `tekton/base/repo.yaml`), and `go-cache-pvc.yaml` (a 25Gi
  `ReadWriteOnce` PVC on `rook-ceph-block-ci`, holding `GOMODCACHE`,
  `GOCACHE`, `GOLANGCI_LINT_CACHE`, `XDG_CACHE_HOME`, `GOPATH`, `GOENV` —
  see [Caching](#caching) below).

## PaC trigger

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  (event == "pull_request" || event == "push") && target_branch == "main"
pipelinesascode.tekton.dev/max-concurrency: "1"
pipelinesascode.tekton.dev/max-keep-runs: "3"
pipelinesascode.tekton.dev/target-namespace: "k8s-gitops-ci"
```

Fires on both `pull_request` and `push` events targeting `main` — the
`build` task's step branches on `event` internally (see
[The build task](#the-build-task) below) rather than having two separate
trigger definitions (`lint` ignores `event` entirely — it always runs the
same `--lint-only` pass regardless). `max-concurrency: "1"` (per-event-source
concurrency) plus the `Repository` CR's `concurrency_limit: 1`
(repo-wide) together mean **only one `PipelineRun` executes at a time**
for this repo, regardless of how many PRs/pushes arrive — a second event
queues rather than running in parallel. `max-keep-runs: "3"` prunes old
completed `PipelineRun`s, keeping only the 3 most recent.

There's also a `pipelinesascode.tekton.dev/task-1` annotation pulling in
an external Clair scan Task definition from another repo — see
[Known limitations](#known-limitations); it's not currently referenced
by any task in the pipeline.

## The build task

One step (`name: build`, `image:
registry.arthurvardevanyan.com/homelab/toolbox:not_latest`), materially
different from a typical multi-Task layout — clone and build happen in
the same pod, in the same shell script, so there's no separate
`git-clone` Task or shared PVC workspace for source code:

1. **Cache setup** — see [Caching](#caching).
2. **Clone** — `git init` into a fresh directory (`/home/default/src`),
   authenticate via `gh auth login --with-token` (using the
   `git_auth_secret` workspace's token) then `gh auth setup-git` (installs
   a git credential helper for the subsequent fetch), `git remote add
origin`, `git fetch origin "${PARAM_REVISION}"` + `git checkout
FETCH_HEAD` (works for any ref/SHA reachable on the remote, not just a
   branch tip), then `git fetch --tags --force origin` (tags are needed
   separately for git-cliff's version/changelog logic).
3. **`task ci`** — the full local CI pipeline (see
   `docs/DEVELOPMENT.md`'s [Task Targets
   Reference](DEVELOPMENT.md#task-targets-reference)) must pass before
   anything below runs.
4. **Release** — branches on `${PARAM_EVENT}` (`push` vs. everything
   else); see [RELEASE.md](RELEASE.md) for the exact commands.

A commented-out `clair-action` task (`runAfter: [build]`, referencing the
external Task pulled in by the `task-1` annotation above) is present but
disabled — see [Known limitations](#known-limitations).

## The lint task

Runs in parallel with `build` (no `runAfter` between them) and has its
own, much lighter `taskSpec` — it doesn't build anything, so it needs
neither the `go-cache` workspace nor the Go-cache env/volume setup `build`
has:

1. **No separate clone step** — unlike `build`'s own manual `git
init`/`fetch`/`checkout`, this task invokes the `k8s-gitops-ci` binary
   directly (the toolbox image's own pinned, Renovate-updated install at
   `/usr/local/bin/k8s-gitops-ci` — not the one `build` is compiling from
   source this run), and `k8s-gitops-ci pipeline --url` clones into its
   own `os.MkdirTemp` directory internally (see `pkg/git.Clone`), chdirs
   there for the run, and cleans up after itself. The step still runs `gh
auth login --with-token` + `gh auth setup-git` first (same rationale
   as `build`'s own clone: installs a git credential helper for that
   internal clone) and mounts a small `emptyDir` at `/tmp` (the pod's root
   filesystem is read-only, and that internal clone's temp dir defaults to
   `/tmp`).
2. **`k8s-gitops-ci pipeline --lint-only --disable-checks golangci
--comment`** — dogfoods this repo's own tiny Kubernetes-manifest
   footprint (`.tekton/`, `tekton/`) through the CI engine's Linting +
   Static Checks phases (see [CI.md](CI.md#modes)'s `--lint-only`
   coverage). `--disable-checks golangci` skips the pipeline's own
   diff-scoped `golangci` check, since `build`'s `task ci` already runs
   `golangci-lint` across the full Go source separately — running it again
   here would be redundant. `--target-branch` is hardcoded to `main` in
   the script (this pipeline's own PaC trigger only ever fires for
   `target_branch == "main"` anyway), fed from a `target-branch` param
   bound to `{{ target_branch }}` on the shared `pipelineSpec.params`.

There's no PR-comment collision with `build`: `build` never invokes
`k8s-gitops-ci pipeline` (it runs `task ci` + `goreleaser` instead), so
`lint` is the only task in this pipeline that posts/updates the marker-
based PR comment.

## Caching

Two-tier: an optional `go-cache` PVC workspace, falling back to a pod
`emptyDir` when not bound.

- **PVC bound** (the normal case in the real cluster —
  `go-cache-pvc.yaml`'s 25Gi `ReadWriteOnce` volume;
  `ReadWriteOnce` is sufficient specifically because
  `max-concurrency: "1"` guarantees only one pod ever mounts it at a
  time): the build step's script redirects `GOMODCACHE`, `GOCACHE`,
  `GOLANGCI_LINT_CACHE`, `XDG_CACHE_HOME` (also covers the
  kubeconform-schema/Kyverno-policy download cache — see
  [SCHEMAS.md](SCHEMAS.md)), `GOPATH` (needed for `go install`/sumdb
  verification, since the toolbox image's baked-in `GOPATH` is
  read-only), and `GOENV` onto the PVC (`mkdir -p` each first).
- **PVC not bound:** the `stepTemplate`'s env defaults keep every one of
  those on the pod's local `emptyDir` `home` volume instead — cold but
  self-contained; every subdirectory (`go-mod`, `go-build`,
  `golangci-lint`, ...) is mounted under one real emptyDir mount point
  (`/home/default/.cache`) rather than relying on kubelet to
  auto-create `.cache` as an intermediate directory for several
  separately-mounted volumes (which would be root-owned and
  non-writable).
- **`GOTMPDIR`/`TMPDIR` are always redirected, PVC bound or not** — the
  pod's root filesystem is read-only
  (`securityContext.readOnlyRootFilesystem: true`), and `go
build`/`install`'s scratch work dirs (`os.TempDir()`, default `/tmp`)
  and other tools' `mktemp`/temp-file calls (`scripts/pull-schemas.sh`,
  `pull-policies.sh`, `pull-scoped-resources.sh`, ...) need a writable
  location regardless of which cache tier is active. Neither is
  persisted on the PVC even when it's bound — both are scratch space,
  deleted after each build anyway.

## Known limitations

- **`lint` dogfoods via the last release, not this PR's own source.** It
  execs the toolbox image's pinned, Renovate-updated `k8s-gitops-ci`
  install - never the code `build` is compiling from source this same
  run. Any PR that both (a) relies on `lint` to assess it and (b) needs
  validator/lint behavior newer than that pinned release (e.g. a PR fixing
  a false positive in a `testdata/invalid/` exclusion, or adding a new
  check) will see `lint` fail against its own not-yet-released fix -
  self-resolving once merged and the toolbox image's pin catches up, but
  a real, visible false failure on that PR in the meantime. Not something
  more code in this repo can close: it's inherent to validating via a
  pre-built binary rather than the sources `build` is compiling.
- **The Clair image-vulnerability scan is planned, not enabled.** The
  `pipelinesascode.tekton.dev/task-1` annotation pulls in an external
  Task definition, and a `clair-action` task block referencing it exists
  in the pipeline — but that task block is entirely commented out
  (`# TODO: clair scan (re-enable once image build is wired up)`). This
  is consistent with container-image publishing itself being inactive
  (see [RELEASE.md](RELEASE.md)) — there's no image to scan yet.
