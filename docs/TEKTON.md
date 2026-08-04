# Tekton

This repo's own CI is a **single** Tekton `PipelineRun` with one inline
step — deliberately much simpler than a typical multi-Task Tekton
pipeline, since `k8s-gitops-ci` itself is the thing that would otherwise
be a pile of separate lint/test/build Tasks. This document describes
that actual footprint, not a general Tekton architecture.

## Table of Contents

- [Directory layout](#directory-layout)
- [PaC trigger](#pac-trigger)
- [The build step](#the-build-step)
- [Caching](#caching)
- [Known limitations](#known-limitations)

## Directory layout

- **`.tekton/k8s-gitops-ci.yaml`** — the Pipelines-as-Code (PaC)
  `PipelineRun`: one `pipelineSpec` with one `tasks[]` entry (`build`),
  whose `taskSpec` has exactly one `steps[]` entry. There is no separate
  `Pipeline` or `Task` CRD file anywhere — everything is inline in this
  one `PipelineRun`.
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
single build step branches on `event` internally (see
[The build step](#the-build-step) below) rather than having two separate
trigger definitions. `max-concurrency: "1"` (per-event-source
concurrency) plus the `Repository` CR's `concurrency_limit: 1`
(repo-wide) together mean **only one `PipelineRun` executes at a time**
for this repo, regardless of how many PRs/pushes arrive — a second event
queues rather than running in parallel. `max-keep-runs: "3"` prunes old
completed `PipelineRun`s, keeping only the 3 most recent.

There's also a `pipelinesascode.tekton.dev/task-1` annotation pulling in
an external Clair scan Task definition from another repo — see
[Known limitations](#known-limitations); it's not currently referenced
by any task in the pipeline.

## The build step

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

- **The Clair image-vulnerability scan is planned, not enabled.** The
  `pipelinesascode.tekton.dev/task-1` annotation pulls in an external
  Task definition, and a `clair-action` task block referencing it exists
  in the pipeline — but that task block is entirely commented out
  (`# TODO: clair scan (re-enable once image build is wired up)`). This
  is consistent with container-image publishing itself being inactive
  (see [RELEASE.md](RELEASE.md)) — there's no image to scan yet.
