# Usage Guide

How to run `k8s-gitops-ci` against a repository. Pick a CI integration
guide below to run it automatically on pull requests, or invoke the CLI
directly. For the full pipeline and flag reference, see [CI.md](CI.md).

## Table of Contents

- [Usage Guide](#usage-guide)
  - [Table of Contents](#table-of-contents)
  - [CI integration guides](#ci-integration-guides)
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

## CI integration guides

### Tekton

Run `k8s-gitops-ci` as a container step in a Tekton Pipeline. The published
image bundles the most common CLI tools, so you only need to add
`golangci-lint` (or other missing tools) via a custom image or a separate
Task step.

#### Container image

Use `ghcr.io/arthurvardevanyan/k8s-gitops-ci:0.63.0@sha256:ace79fe17227d0cd9cc65d0cc59251af719d6911747f40f5b79b0ac8bdb45336` as the base image
for Tekton Task containers. The image includes `gh`, `kustomize`,
`shellcheck`, `prettier`, and `markdownlint-cli`.

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
      image: ghcr.io/arthurvardevanyan/k8s-gitops-ci:0.63.0@sha256:ace79fe17227d0cd9cc65d0cc59251af719d6911747f40f5b79b0ac8bdb45336
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
      image: ghcr.io/arthurvardevanyan/k8s-gitops-ci:0.63.0@sha256:ace79fe17227d0cd9cc65d0cc59251af719d6911747f40f5b79b0ac8bdb45336
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
