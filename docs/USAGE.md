# GitHub Actions Integration Guide

A minimal template for running `k8s-gitops-ci` in GitHub Actions. For the
full pipeline/flag reference, see [CI.md](CI.md).

## Setup

### GitHub App (recommended)

Use a GitHub App so each run gets a short-lived, scoped token. Create one
under **Settings → Developer settings → GitHub Apps → New GitHub App**:

1. **Permissions:** set `Contents: Read` and `Pull requests: Read and
write` (required for `--comment` and the PR title/checklist checks).
2. **Install:** install the App on the target organization or repository.
3. **Private key:** generate a PEM private key and store its contents as a
   repository secret named `CI_BOT_PRIVATE_KEY`.
4. **Client ID:** copy the App's Client ID and store it as a repository
   variable named `CI_BOT_CLIENT_ID`.

The workflow below exchanges those values for a per-run token with
`actions/create-github-app-token`. A personal access token also works —
set `GH_TOKEN` directly instead of using the token step.

### Minimal workflow

Copy this into `.github/workflows/k8s-gitops-ci.yml`. It runs the full
pipeline on pull requests and posts the report as a PR comment.

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
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Generate CI bot token
        id: app-token
        uses: actions/create-github-app-token@v3
        with:
          client-id: ${{ vars.CI_BOT_CLIENT_ID }}
          private-key: ${{ secrets.CI_BOT_PRIVATE_KEY }}

      - name: Install k8s-gitops-ci
        env:
          VERSION: v0.63.0 # pin a release tag
        run: |
          curl -fsSL -o /tmp/k8s-gitops-ci.tar.gz \
            "https://github.com/ArthurVardevanyan/k8s-gitops-ci/releases/download/${VERSION}/k8s-gitops-ci_${VERSION#v}_linux_amd64.tar.gz"
          curl -fsSL -o /tmp/checksums.txt \
            "https://github.com/ArthurVardevanyan/k8s-gitops-ci/releases/download/${VERSION}/k8s-gitops-ci_${VERSION#v}_checksums.txt"
          cd /tmp && grep linux_amd64 checksums.txt | sha256sum -c -
          tar -xzf k8s-gitops-ci.tar.gz k8s-gitops-ci
          install -m 0755 k8s-gitops-ci /usr/local/bin/k8s-gitops-ci

      - name: Install lint tooling
        run: |
          npm install -g prettier@3
          npm install -g markdownlint-cli2@latest

      - name: Run k8s-gitops-ci
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
        run: |
          k8s-gitops-ci pipeline \
            --url "https://github.com/${{ github.repository }}" \
            --pr "${{ github.event.pull_request.number }}" \
            --comment
```

The release binary embeds the kubeconform schemas, so no schema download
is needed. `gh`, `kustomize`, and `shellcheck` are preinstalled on
GitHub-hosted Ubuntu runners; `prettier` and `markdownlint-cli2` are
installed above because they are not. If your repo has no Go files, the
`golangci` step skips cleanly — no `golangci-lint` install is needed.

### Container image

Use the multi-arch image when you need `linux/arm64` or want the bundled
tools instead of installing them:

```yaml
jobs:
  k8s-gitops-ci:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/arthurvardevanyan/k8s-gitops-ci:0.63.0
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Generate CI bot token
        id: app-token
        uses: actions/create-github-app-token@v3
        with:
          client-id: ${{ vars.CI_BOT_CLIENT_ID }}
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
files and you are not installing `golangci-lint`.

## Setup notes

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

## Flag quick reference

- `--lint-only` — run Linting + Static Checks only; skips overlay builds.
  Faster, and avoids needing `kustomize`/Helm for the build phase.
- `--dirs <prefixes>` — validate every file under the given prefixes
  instead of the PR diff. Useful for matrix jobs over independent trees.
- `--quiet` — in `pipeline`, omit passing sections from the PR comment
  and delete the comment when everything passes; the exit code still
  reflects findings. (`test --quiet` always exits 0.)

## Branch protection

To prevent merging until validation passes, make the `k8s-gitops-ci`
status check **required** in your repository's branch protection rules:

> **Settings → Branches → Protect matching branches → Required checks
> → Add checks** → search for `k8s-gitops-ci`.

This blocks merges until the check reports `success`, so the PR must be
clean before it can be merged.
