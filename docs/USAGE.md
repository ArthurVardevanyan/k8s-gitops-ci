# Usage Guide

How to run `k8s-gitops-ci` against a repository. Start with the local
usage section to invoke the CLI directly, then pick
a CI integration guide below to run it automatically on pull requests. For the full pipeline and flag reference,
see [CI.md](CI.md).

## Table of Contents

- [Usage Guide](#usage-guide)
  - [Table of Contents](#table-of-contents)
  - [Local usage](#local-usage)
    - [Install](#install)
      - [Release Binary](#release-binary)
      - [Install from `go`](#install-from-go)
      - [Build from source](#build-from-source)
      - [Verify Installation](#verify-installation)
    - [Quick check — does it work?](#quick-check--does-it-work)
    - [Reproduce a CI failure locally](#reproduce-a-ci-failure-locally)
    - [Standalone Static checks](#standalone-static-checks)
    - [Build rendered YAML](#build-rendered-yaml)
    - [Selective check enablement](#selective-check-enablement)
    - [Quiet mode for scripting](#quiet-mode-for-scripting)
    - [External tool dependencies](#external-tool-dependencies)
      - [Fast pass: `--lint-only`](#fast-pass---lint-only)
    - [Tekton](#tekton)
      - [Container image](#container-image)
      - [Minimal Tekton Task](#minimal-tekton-task)
      - [PipelineRun with Pipelines-as-Code](#pipelinerun-with-pipelines-as-code)
      - [Non-PaC Deployments](#non-pac-deployments)
      - [PR comment integration](#pr-comment-integration)
      - [Tool dependencies](#tool-dependencies)
      - [Opting out of checks](#opting-out-of-checks)
    - [GitHub Actions](#github-actions)
      - [GitHub App setup](#github-app-setup)
      - [Workflow](#workflow)
      - [Setup notes](#setup-notes)
      - [Flag quick reference](#flag-quick-reference)
      - [Branch protection](#branch-protection)

## Local usage

Progressive examples for running `k8s-gitops-ci` locally against your
own GitOps repository — install, verify it works, and reproduce a CI
failure on the command line. For the full pipeline flag reference, see
[CI.md](CI.md).

### Install

#### Release Binary

Download the pre-compiled binary for your platform from the
[release page](https://github.com/ArthurVardevanyan/k8s-gitops-ci/releases/latest).

**macOS (Apple Silicon):**

```sh
curl -LO https://github.com/ArthurVardevanyan/k8s-gitops-ci/releases/latest/download/k8s-gitops-ci_darwin_arm64.tar.gz
tar -xzf k8s-gitops-ci_darwin_arm64.tar.gz
mv k8s-gitops-ci /usr/local/bin/
```

**Linux (x86_64):**

```sh
curl -LO https://github.com/ArthurVardevanyan/k8s-gitops-ci/releases/latest/download/k8s-gitops-ci_linux_amd64.tar.gz
tar -xzf k8s-gitops-ci_linux_amd64.tar.gz
sudo mv k8s-gitops-ci /usr/local/bin/
```

**Windows (x86_64):**

```powershell
# Download the .tar.gz, extract k8s-gitops-ci.exe
# then place it somewhere on your PATH, e.g.
# C:\Program Files\k8s-gitops-ci\
```

Verify the checksum matches the one in the corresponding `*_checksums.txt` file:

```sh
sha256sum -c k8s-gitops-ci_checksums.txt  # Linux/macOS
```

> **Production note.** The examples above use `latest` for convenience.
> For production, pin to a specific release (e.g. `v0.63.0`) and verify
> the SHA-256 digest matches the one in `*_checksums.txt` to prevent
> supply-chain attacks on the `latest` tag.

#### Install from `go`

```sh
go install github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/k8s-gitops-ci@latest
```

#### Build from source

Requires Go 1.27+ and [`task`](https://taskfile.dev)

```sh
task build  # writes bin/k8s-gitops-ci with version metadata
```

#### Verify Installation

```sh
k8s-gitops-ci version
```

Run `k8s-gitops-ci --help` or `k8s-gitops-ci <command> --help` for the
full CLI reference, or check the per-command descriptions in [CI.md](CI.md)'s
[Modes](CI.md#modes) section.

### Quick check — does it work?

`test` is the primary local validation command. It runs every enabled
check against a **changeset** — by default, `git diff` + `git diff
--cached` (uncommitted changes only):

```sh
# Scan uncommitted changes in the working tree
k8s-gitops-ci test

# Full-tree walk under a specific path (not a diff)
k8s-gitops-ci test ./kubernetes

# Full repository scan: every file on disk, every overlay, ignores git state
k8s-gitops-ci test --all

# Streams every check's start/pass/fail as it runs
k8s-gitops-ci test --all --verbose
```

> **Note:** `--all` takes priority over `--dirs` when both are set.
> Positional path args (`test ./kubernetes`) take priority over the
> `--dirs` flag when both are given.

### Reproduce a CI failure locally

A failing `pipeline --url <repo> --pr <n>` can be reproduced with `test`
using an equivalent flag set — `test` accepts the same changeset-scoping
and check-enablement flags as `pipeline`:

```sh
# Validator phases only, against the PR's changed files (no PR-title or
# comment posting) — no PR context needed beyond the file list.
k8s-gitops-ci test --url https://github.com/org/repo --pr 123

# Same checks, local changeset only
k8s-gitops-ci test --dirs=kubernetes/ --disable-checks=avp

# Linting-only pass (faster, skips overlay builds)
k8s-gitops-ci test --all --lint-only

# Linting-only via pipeline flag (equivalent)
k8s-gitops-ci pipeline --url https://github.com/org/repo --pr 123 --lint-only
```

> **`--comment` is off by default.** Do not pass `--comment` when
> reproducing locally — it requires PR context (`--url` + `--pr`) and an
> authenticated `gh`/`glab` CLI to post a PR comment. Omit it to avoid
> unnecessary errors.
> **Hook source.** Local `test`/`build-yaml` runs use the working-tree
> `test.sh` by default. A `--hook-source` flag (accepts `main`/`pr`/`local`)
> can override this. See [HOOKS.md](HOOKS.md) for details.

### Standalone Static checks

```sh
# Normalize kustomization.yaml field ordering (writes files)
k8s-gitops-ci kustomize-fix -all

# Validate startingCSV folder versions
k8s-gitops-ci check-starting-csv kubernetes/**/*.yaml

# Detect kustomize patches matching no resource
k8s-gitops-ci ghost-patches kubernetes/my-app/overlays/my-cluster

# Sort repo config files
k8s-gitops-ci sort-configs

# Update scaffold status README table
k8s-gitops-ci update-scaffold-status
```

> **Standalone vs. pipeline behavior.** Standalone linter subcommands
> silently **no-op with exit 0** when their CLI is missing (e.g. a repo
> without `golangci-lint` installed). The pipeline's **Linting phase**
> treats a missing CLI as a **hard failure** (blocking). The exception
> is `kubeconform`/`yaml-syntax`, which run in-process and never
> require an external binary — the embedded schemas cover schema
> validation, and YAML syntax is parsed natively in Go. Use
> `k8s-gitops-ci kustomize-fix` for a hard-failure on missing
> `kustomize`.

### Build rendered YAML

`build-yaml` validates every overlay matching the `--app`/`--cluster`
target, runs the full validator against the rendered output, and prints
the report to the console. It is **not** a validation-free dry run:

```sh
# Validate a specific app/cluster overlay
k8s-gitops-ci build-yaml --app my-app --cluster prod

# Add --verbose for per-overlay detail
k8s-gitops-ci build-yaml --app my-app --cluster dev --verbose

# Repeatable flags — `--app a --app b --cluster prod` validates both apps
k8s-gitops-ci build-yaml --app my-app --app another --cluster prod --cluster staging
```

`build-yaml` uses the native Kustomize SDK and Helm rendering engine,
so it does **not** require an external `kustomize` or `helm` binary.

### Selective check enablement

The `--disable-checks` and `--enable-checks` flags accept a
**comma-separated** list of step/check IDs:

```sh
# Disable multiple checks at once
k8s-gitops-ci test --disable-checks=shellcheck,golangci,prettier

# Enable checks that are off by default (kyverno, scaffold-readme)
k8s-gitops-ci test --enable-checks=kyverno,scaffold-readme
```

Repeated flags **overwrite** each other (Go `flag` behavior) — use
comma-separated lists. Unknown IDs produce a warning (not a hard error)
to allow sharing configs across repos with different tool versions.
See [CI.md](CI.md)'s [Registered checks](CI.md#registered-checks) table
for the full list.

> **Default-off steps.** `kyverno` and `scaffold-readme` default to
> disabled (off) — they require per-org policy configuration or
> infrastructure to run. All other steps default to enabled and are
> only turned off via `--disable-checks`.

### Quiet mode for scripting

```sh
# Only print failed/warned sections; always exit 0
k8s-gitops-ci test --all --quiet
```

> **Warning for gating.** `test --quiet` always exits 0 regardless of
> findings. It is suitable for pre-commit reporting or CI dashboards
> where you only want to see what you broke, but **not** as a blocking
> gate. For a blocking gate, run `test` without `--quiet` so a failed
> run returns a non-zero exit code.
> **`pipeline --quiet`** differs from `test --quiet`: in pipeline mode
> it omits passing sections from the PR comment and deletes the comment
> when everything passes, but the exit code still reflects findings
> (non-zero on failure).

### External tool dependencies

`k8s-gitops-ci` itself requires no runtime dependencies — the core
binary is a single Go executable. The **checks** it runs depend on
external CLI tools. A missing CLI **hard-fails** the relevant check in
the pipeline (blocking) — it never silently skips — unless the step is
explicitly disabled via `--disable-checks <id>`.

| Tool                                 | Used by                                     | Missing behavior in pipeline/test                          | Opt out                                 |
| ------------------------------------ | ------------------------------------------- | ---------------------------------------------------------- | --------------------------------------- |
| `kustomize`                          | `kustomize-fix` check, `kustomize-fix` CLI  | Hard failure (blocking)                                    | `--disable-checks kustomize-fix`        |
| `kubeconform`                        | `kubeconform` check (embedded schemas)      | **No binary needed** — runs in-process                     | `--disable-checks kubeconform`          |
| `kyverno`                            | `kyverno` check                             | Hard failure if absent and step enabled                    | Default off — `--enable-checks kyverno` |
| `shellcheck`                         | `shellcheck` check                          | Hard failure if binary absent and shell files in changeset | `--disable-checks shellcheck`           |
| `prettier`                           | `prettier` check, `kustomize-fix` follow-up | Hard failure (blocking)                                    | `--disable-checks prettier`             |
| `markdownlint` / `markdownlint-cli2` | `markdownlint` check                        | Hard failure (blocking)                                    | `--disable-checks markdownlint`         |
| `golangci-lint`                      | `golangci` check                            | Hard failure (blocking)                                    | `--disable-checks golangci`             |
| `argocd-vault-plugin`                | `avp` check                                 | Hard failure if binary absent and AVP indicators detected  | `--disable-checks avp`                  |

> **`gh` / `glab`** are required for any pipeline run with `--url` +
> `--pr` context (PR title validation, PR comment posting, PR checklist).
> Without them, the pipeline continues but skips PR-specific checks.
> **Standalone subcommand behavior.** Standalone lint subcommands
> (`k8s-gitops-ci kubeconform *.yaml`, `shellcheck *.sh`, etc.) silently
> **no-op with exit 0** when their CLI is missing — they are not a
> substitute for a pipeline run when blocking behavior is needed.

#### Fast pass: `--lint-only`

For a quick validation pass that avoids all optional CLI dependencies
(except `markdownlint`/`prettier`/`shellcheck`/`golangci` which you
may want to skip with `--disable-checks`):

```sh
k8s-gitops-ci test --all --lint-only --disable-checks=golangci,kyverno
```

This skips the overlay build phase entirely (no `kustomize` or `helm`
needed) and runs only the Linting + Static Checks phases.

### Tekton

Run `k8s-gitops-ci` as a container step in a Tekton Pipeline. The published
image bundles the most common CLI tools, so you only need to add
`golangci-lint` (or other missing tools) via a custom image or a separate
Task step.

#### Container image

Use `ghcr.io/arthurvardevanyan/k8s-gitops-ci:latest` as the base image
for Tekton Task containers. The image includes `gh`, `kustomize`,
`shellcheck`, `prettier`, and `markdownlint-cli`.

> **Production note.** Use a specific version tag and SHA-256 digest
> (e.g. `ghcr.io/arthurvardevanyan/k8s-gitops-ci:v0.63.0@sha25:...`)
> in production environments to pin the image and prevent unexpected
> changes when a new release is published.

#### Minimal Tekton Task

A single Task that runs `k8s-gitops-ci pipeline` against a PR:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: k8s-gitops-ci
spec:
  params:
    - name: repo-url
      type: string
    - name: pr-number
      type: string
    - name: target-branch
      type: string
  workspaces:
    - name: source
    - name: git-auth
      optional: true
  steps:
    - name: run-ci
      image: ghcr.io/arthurvardevanyan/k8s-gitops-ci:<VERSION>@<SHA>
      script: |
        #!/usr/bin/env bash
        k8s-gitops-ci pipeline \
          --url "$(params.repo-url)" \
          --pr "$(params.pr-number)" \
          --target-branch "$(params.target-branch)" \
          --comment \
          --verbose
```

#### PipelineRun with Pipelines-as-Code

Use the Pipelines-as-Code annotations to trigger the PipelineRun on pull
requests, and bind PaC environment variables to pipeline parameters:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: gitops-ci
  annotations:
    pipelinesascode.tekton.dev/max-keep-runs: "1"
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request" && target_branch == "main"
    pipelinesascode.tekton.dev/target-namespace: "your-namespace"
    pipelinesascode.tekton.dev/task: "[tekton/tasks/gitops-ci.yaml]"
spec:
  params:
    - name: git-commit
      value: "{{ revision }}"
    - name: git-url
      value: "{{ repo_url }}"
    - name: git-pr-number
      value: "{{ pull_request_number }}"
    - name: target-branch
      value: "{{ target_branch }}"
  pipelineSpec:
    params:
      - name: git-commit
        type: string
      - name: git-url
        type: string
        description: Repository URL to clone from.
      - name: git-pr-number
        type: string
      - name: target-branch
        type: string
    workspaces:
      - name: git_auth_secret
    tasks:
      - name: k8s-gitops-ci
        params:
          - name: git-commit
            value: $(params.git-commit)
          - name: git-url
            value: $(params.git-url)
          - name: git-pr-number
            value: $(params.git-pr-number)
          - name: target-branch
            value: $(params.target-branch)
        taskRef:
          name: k8s-gitops-ci
          kind: Task
        workspaces:
          - name: basic-auth
            workspace: git_auth_secret
  taskRunTemplate:
    serviceAccountName: pipeline
  workspaces:
    - name: git_auth_secret
      secret:
        secretName: "{{ git_auth_secret }}"
```

#### Non-PaC Deployments

For non-Pac deployments, see <https://tekton.dev/docs/triggers/>.

#### PR comment integration

Pass `--comment` to post or update the report as a PR comment. The
PipelineRun example above passes `--comment` (along with `--url`, `--pr`,
and `--target-branch`). The GitHub token comes from a SecretRef workspace
bound to the Task, matching the pattern used in this repo's own
`git_auth_secret` workspace.

#### Tool dependencies

The container image includes `gh`, `kustomize`, `shellcheck`, `prettier`,
and `markdownlint-cli`. If you use checks that require other tools, install
them in your own image or as a separate Task step.

#### Opting out of checks

When a tool cannot be installed in your Tekton environment, use
`--disable-checks <id>` to skip that check:

- `--disable-checks golangci` — skip Go linting if `golangci-lint` is
  not installed in your image.
- `--disable-checks kyverno` — skip Kyverno policy validation if the
  binary is absent.
- `--disable-checks avp` — skip Argo CD Vault Plugin secret resolution
  if AVP is not in your image.

For a minimal pass that avoids all optional CLI dependencies, combine
`--lint-only` with `--disable-checks` for the tools unavailable in your
environment.

### GitHub Actions

The job runs inside the published container image, so every tool is
pre-installed and the environment is consistent.

#### GitHub App setup

Use a GitHub App so each run gets a short-lived, scoped token. Create one
under **Settings → Developer settings → GitHub Apps → New GitHub App**:

1. **Permissions:** set `Contents: Read` and `Pull requests: Read and
write` (required for `--comment` and the PR title/checklist checks).
1. **Install:** install the App on the target organization or repository.
1. **Private key:** generate a PEM private key and store its contents as a
   repository secret named `CI_BOT_PRIVATE_KEY`.
1. **Client ID:** copy the App's Client ID and store it as a repository
   secret named `CI_BOT_CLIENT_ID`.

The workflow below exchanges those values for a per-run token with
`actions/create-github-app-token`. A personal access token also works —
set `GH_TOKEN` directly instead of using the token step.

#### Workflow

Copy this into `.github/workflows/k8s-gitops-ci.yml`. It runs the full
pipeline on pull requests and posts the report as a PR comment. The job
runs inside the published container image so every tool is pre-installed
and the environment is consistent.

```yaml
name: k8s-gitops-ci
on:
  pull_request:
    branches: [main]
    paths:
      - "kubernetes/**"
      - "**.yaml"
      - "**.yml"
      - "**.sh"
      - "**.md"

permissions:
  contents: read
  pull-requests: write

jobs:
  k8s-gitops-ci:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/arthurvardevanyan/k8s-gitops-ci:<VERSION>@<SHA>
    steps:
      - uses: actions/checkout@v7.0.1
        with:
          fetch-depth: 0

      - name: Generate CI bot token
        id: app-token
        uses: actions/create-github-app-token@v3.3.0
        with:
          client-id: ${{ secrets.CI_BOT_CLIENT_ID }}
          private-key: ${{ secrets.CI_BOT_PRIVATE_KEY }}

      - name: Run k8s-gitops-ci
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
        run: |
          k8s-gitops-ci pipeline \
            --url "https://github.com/${{ github.repository }}" \
            --pr "${{ github.event.pull_request.number }}" \
            --comment
```

The image includes `gh`, `kustomize`, `shellcheck`, `prettier`, and
`markdownlint-cli`, but not `golangci-lint`, `kyverno`, or
`argocd-vault-plugin`. Add `--disable-checks golangci` if your repo has Go
files and you want to skip the golangci-lint check.

#### Setup notes

- **Permissions:** `contents: read` and `pull-requests: write`. The latter
  is needed for `--comment` and the PR title/checklist checks.
- **`gh` auth:** pass `GH_TOKEN`. The GitHub App step generates it
  per-run; a PAT secret also works. PR checks and comments use `gh`; an
  unauthenticated run skips commenting with a warning but fails the PR
  checks.
- **`fetch-depth: 0`:** required for diff-based and merge-base checks.
- **Private repos:** `pipeline --url` clones into its own temp directory,
  so the token from `actions/checkout` is not reused. Add a global
  `insteadOf` rewrite before the pipeline step:

  ```yaml
  - env:
      GH_TOKEN: ${{ steps.app-token.outputs.token }}
    run: |
      git config --global \
        url."https://x-access-token:${GH_TOKEN}@github.com/".insteadOf \
        "https://github.com/"
  ```

#### Flag quick reference

- `--lint-only` — run Linting + Static Checks only; skips overlay builds.
  Faster, and avoids needing `kustomize`/Helm for the build phase.
- `--dirs <prefixes>` — validate every file under the given prefixes
  instead of the PR diff. Useful for matrix jobs over independent trees.
- `--quiet` — in `pipeline`, omit passing sections from the PR comment
  and delete the comment when everything passes; the exit code still
  reflects findings. (`test --quiet` always exits 0.)

#### Branch protection

To prevent merging until validation passes, make the `k8s-gitops-ci`
status check **required** in your repository's branch protection rules:

> **Settings → Branches → Protect matching branches → Required checks
> → Add checks** → search for `k8s-gitops-ci`.

This blocks merges until the check reports `success`, so the PR must be
clean before it can be merged.
