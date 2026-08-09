# k8s-gitops-ci

## Overview

Generic, org-agnostic Kubernetes GitOps CI engine, distributed as a Go
CLI (`k8s-gitops-ci`) and a set of importable packages. Detects changed
files, builds Kustomize/Helm overlays, and runs a registry-driven set of
validators (namespace scope, PSA labels, RBAC, image pinning, named
ports, pod-spec defaults, sync options, cluster-identity leakage, and
more), plus wrappers around common lint tools (kubeconform, Kyverno,
golangci-lint, markdownlint, prettier, shellcheck).

This is the **generic core only** — no org names, domains, cluster
names, cloud-provider, or vendored-tool specifics belong anywhere under
`pkg/`. Anything that varies by org is injected through the
`provider.Providers` interfaces or an exported package override
variable with a generic (usually empty/no-op) default. See
[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md)'s "Design Conventions"
section for the full explanation and worked examples before adding
anything that looks like it might be org-specific.

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — the top-level entry
  point: runtime flow, package map, and a "Where do I find X?" table
  pointing to every doc below.
- [docs/CI.md](docs/CI.md) — the detailed pipeline reference: phases,
  every mode (`pipeline`/`test-all`/`scan-all`/`build-yaml`) with its
  exact changeset source, the full registered-check table, and the
  direct-vs-external finding classification.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) — build/test/lint commands,
  Task target reference, repository structure, and (most important for
  making changes) the design conventions: the `provider.Providers`
  seam, exported-override-var pattern, the "core data + org-injectable
  override" pattern, and the generic check-enablement mechanism
  (`DisabledChecks`/`EnabledChecks`).
- [docs/HOOKS.md](docs/HOOKS.md) — the `test.sh` hook contract and,
  importantly, which directives are actually wired vs. parsed-but-unused
  today.
- [docs/EXEMPTIONS.md](docs/EXEMPTIONS.md) — the unified exemption
  framework's two modes (annotation vs. `EXEMPTIONS=(...)` selector),
  which one actually takes effect today, and exemptable check IDs.
- [docs/TEKTON.md](docs/TEKTON.md) — this repo's own Tekton
  `PipelineRun`/Pipelines-as-Code trigger/caching setup.
- [docs/RELEASE.md](docs/RELEASE.md) — versioning (the `VERSION` file as
  the single source of truth), how a release is cut (bump `VERSION` via
  PR), and the release flow's actual published artifacts.
- [docs/SECURITY.md](docs/SECURITY.md) — trust model, `exec.Command`
  audit table, file-permission rationale.
- [docs/SCHEMAS.md](docs/SCHEMAS.md) — how embedded kubeconform
  schemas / Kyverno policies work, and how an org supplies its own
  CRD schemas or real Kyverno policies.

## Build & Test

**Always use `task` commands, not raw `go build`/`go test`/`go fmt`.**
`task build` injects version metadata via `-ldflags` that a bare
`go build` won't produce, and future `task` targets may add embedded-
resource generation steps a raw command would silently skip.

| Action                                             | Command                        |
| -------------------------------------------------- | ------------------------------ |
| Build                                              | `task build`                   |
| Test                                               | `task test`                    |
| Lint                                               | `task lint`                    |
| Format                                             | `task format`                  |
| Full CI                                            | `task ci`                      |
| Replay real merged PRs (smoke gate)                | `task test:homelab-prs`        |
| Regenerate embedded schemas                        | `task update:schemas`          |
| Regenerate resource-scope maps from a live cluster | `task update:scoped-resources` |

Run `task --list` for the full, authoritative list — see
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for what each one does.

## Key Conventions

- **Options-struct pattern:** functions accept one `Options`/`RunOptions`
  struct, not long parameter lists.
- **Result structs:** functions return typed results
  (`check.Result`, `validator.Result`, ...), not bare errors, so callers
  can distinguish "ran and found issues" from "failed to run."
- **Test files live alongside source** (`pkg/foo/foo_test.go`); prefer
  table-driven tests over one-off `Test*` functions per case.
- **`testdata/`** for fixtures, with `testdata/invalid/` for
  deliberately-malformed inputs, where a package has them — not every
  existing package does yet, but new/rewritten validators should.
- **Every `pkg/lint/*` CLI wrapper returns a typed `ErrCLINotFound`**
  (never a panic) when its underlying tool isn't installed — match this
  when adding a new wrapper. What a _caller_ does with that error varies
  by call site: `cmd/k8s-gitops-ci`'s standalone lint subcommands still
  no-op on it, but the Linting phase (`pkg/validator/phases.go`) treats a
  missing markdownlint/prettier/shellcheck/golangci-lint/kustomize CLI as
  a hard failure, not a graceful skip — a missing lint tool means the
  pipeline didn't actually check what it claims to have checked, and
  should never be indistinguishable from a clean run. Each of those is
  individually gated behind its own step ID
  (`Options.DisabledChecks`/`EnabledChecks`) so an environment that
  genuinely can't provision a given tool can opt out explicitly instead
  of always failing.
- **Generic check-enablement, not one-off flags:** if you need a new
  gateable step, give it a string ID and use the existing
  `Options.DisabledChecks`/`EnabledChecks` + `stepEnabled` mechanism
  (`pkg/validator/phases.go`) instead of adding a dedicated boolean
  flag. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Formatting & Commits

- **Go:** tabs (goimports + gofumpt enforced via `task format`).
- **YAML/Markdown/Shell:** 2-space indent (see `.editorconfig`).
- **Line endings:** LF, final newline required, trailing whitespace
  trimmed (enforced by `.editorconfig` and the `pre-commit` hooks in
  `.pre-commit-config.yaml`).
- **Commits:** Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`,
  `refactor:`, `test:`, `build:`, `ci:`), enforced by
  `conventional-pre-commit` and matching the existing `git log` history.
- After editing Markdown or YAML, run `prettier --write <files...>` (or
  `task format`) — this fixes table alignment and formatting that
  `markdownlint`/`prettier` pre-commit hooks enforce.

## Testing & Documentation Checklist

After making a code change, before considering it done:

1. Run `task test` — verify it passes.
2. Add or update tests for the new/changed behavior (see "Key
   Conventions" above).
3. Update the relevant doc under `docs/` if the change affects
   documented behavior (flags, report structure, hook/exemption syntax,
   embedded-resource sourcing) — don't let docs drift silently.
4. Run `task format` before committing.

## Security

- This is a CI tool — operator-controlled, trusted inputs only (CLI
  flags, pipeline params, git clone). Not a web service; no HTTP
  request handling.
- `exec.Command` is always called with an explicit argument slice (no
  shell interpolation) — this is intentional and should be preserved in
  any new CLI-wrapping code.

## GitHub Interactions

Use the `gh` CLI for GitHub operations (PR comments, reviews, API
calls) via `pkg/github`'s thin wrapper — don't add a second, parallel
GitHub-API client.
