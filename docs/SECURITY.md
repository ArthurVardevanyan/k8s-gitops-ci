# Security Model

This is a CI tool, not a web service — its trust model, attack surface,
and mitigations follow from that. This repo does not run a separate SAST
scanner in its own CI today (see [CI.md](CI.md)/[TEKTON.md](TEKTON.md)
for what does run), so this document is framed as a durable security
model rather than a tool-specific findings-triage log. If a scanner is
adopted later, add a triage-table section to this same file at that time
rather than creating a second security doc.

## Trust model

`k8s-gitops-ci` is a CLI/Tekton-task tool operated by a trusted CI
pipeline or a developer's own machine — every input is operator-
controlled: CLI flags, Tekton `PipelineRun` params, and paths from a
`git clone` the operator already trusted enough to run this tool
against. There is no HTTP request handling anywhere in this codebase and
no user-supplied input arriving over a network boundary — `pkg/github`
talks to the `gh` CLI (itself talking to GitHub's API over the
operator's own authenticated session), not the reverse.

## `exec.Command`/`exec.CommandContext` audit

Every process this tool shells out to is called with an **explicit
argument slice** — never through a shell (`sh -c "..."`), so there is no
shell-interpolation injection surface regardless of what a filename or
flag value contains. This is intentional and must be preserved in any
new CLI-wrapping code (see `AGENTS.md`).

| File                                    | External command(s)                                                                                                                                                                                                                                                                            |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/lint/kubeconform/pull.go`          | `git clone`/`git pull` (refreshing the embedded schema source)                                                                                                                                                                                                                                 |
| `pkg/lint/kyverno/kyverno.go`           | `kyverno` (policy engine), `kustomize build`                                                                                                                                                                                                                                                   |
| `pkg/lint/golangci/golangci.go`         | `golangci-lint run`                                                                                                                                                                                                                                                                            |
| `pkg/lint/markdownlint/markdownlint.go` | `markdownlint-cli2`                                                                                                                                                                                                                                                                            |
| `pkg/lint/prettier/prettier.go`         | `prettier`                                                                                                                                                                                                                                                                                     |
| `pkg/kustomize/kustomize.go`            | `kustomize edit fix --vars` (see [CI.md](CI.md)'s Kustomize Fix section — like every other `pkg/lint/*` CLI wrapper's own missing-CLI handling, treated as a hard failure, not a graceful skip, if missing; also invokes `prettier.Write`, i.e. `prettier --write`, as a formatting follow-up) |
| `pkg/lint/shellcheck/shellcheck.go`     | `shellcheck` (also the sole exec call site reused by `pkg/lint/shellcheck/tekton.go`/`embedded.go`'s extraction — those two files call `Run`, which shells out; they don't invoke `exec.Command` a second time themselves)                                                                     |
| `pkg/changeset/changeset.go`            | `git ls-files`, `git diff` (×3 call sites)                                                                                                                                                                                                                                                     |
| `pkg/ghostpatch/ghostpatch.go`          | `git show`                                                                                                                                                                                                                                                                                     |
| `pkg/git/git.go`                        | `git` (×5: generic passthrough, `show`, `diff` ×2, `merge-base`)                                                                                                                                                                                                                               |
| `pkg/github/github.go`                  | `gh` (every GitHub API interaction routes through this one wrapper — see `pkg/github/comment.go`'s note on why call sites must go through `c.gh`, not a bare `exec.Command`, so `GH_REPO` is always set consistently)                                                                          |
| `pkg/overlay/overlay.go`                | `argocd-vault-plugin generate -` (see [CI.md](CI.md)'s Build Strategies section — implemented, but not wired into any real call site today)                                                                                                                                                    |
| `pkg/scaffold/scaffold.go`              | the `scafctl` binary (`Binary` var), `diff -rq`                                                                                                                                                                                                                                                |
| `pkg/configdiff/configdiff.go`          | `git merge-base`, `git show`                                                                                                                                                                                                                                                                   |

## File-permission rationale

Every explicit file-permission mode in this codebase is deliberately the
narrowest mode that still works for its purpose:

| Mode                                                                     | Used for                                                                                                                                                                                                                                                                                                                                                                                    | Where                                                                                                                   |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `0o600`                                                                  | The pipeline's own log file; rendered overlay output written to disk                                                                                                                                                                                                                                                                                                                        | `pkg/logger/logger.go`, `pkg/overlay/overlay.go`                                                                        |
| `0o644`                                                                  | General file writes (sorted configs, scratch `kustomization.yaml` copies for `CheckFix`'s real-CLI dry-run comparison - the actual fixed file on disk is written by the real `kustomize`/`prettier` binaries themselves, not this repo's own code, see `Fix` in `pkg/kustomize/kustomize.go` - extracted schema/policy files, generated Kyverno CLI test fixtures, scaffold README updates) | `pkg/config`, `pkg/kustomize`, `pkg/lint/kubeconform/schemas`, `pkg/lint/kyverno` (×2), `pkg/scaffold`                  |
| `0o755`                                                                  | Directories created to hold the above                                                                                                                                                                                                                                                                                                                                                       | `pkg/lint/kubeconform/schemas`, `pkg/lint/kyverno` (×2)                                                                 |
| `0o700` (Go's `os.MkdirTemp`/`os.CreateTemp` default — never overridden) | Every temp directory/file this tool creates (schema/policy extraction scratch dirs, git clone scratch dirs, scaffold's temp output dir, the shellcheck extraction package's per-script temp `.sh` files)                                                                                                                                                                                    | `pkg/lint/kubeconform/schemas`, `pkg/lint/kyverno/policies`, `pkg/git`, `pkg/scaffold`, `pkg/lint/shellcheck/tekton.go` |

`overlay.go`'s rendered-overlay output uses the more restrictive `0o600`
(not the general `0o644`) specifically because a fully-resolved overlay
is the one artifact that could carry a resolved secret value once AVP
resolution is actually wired in (see [CI.md](CI.md)) — narrower by
default, before that's even active, rather than widening later.

## Decompression-bomb guard

`pkg/lint/kubeconform/schemas/embed.go` and
`pkg/lint/kyverno/policies/embed.go` both extract a `tar.gz` archive
embedded in the binary at build time. Each per-entry extraction bounds
the copy with `io.LimitReader(tr, maxExtractedFileSize+1)`
(`maxExtractedFileSize = 512 << 20`, i.e. 512MiB) and then checks the
actual byte count copied against that same limit, erroring out if it was
exceeded — the `+1`/post-copy-check pattern (rather than trusting the
tar header's declared `Size` field, which is attacker-influenceable
metadata, not a guarantee of the actual stream length) is what makes this
a real bound on bytes _written_, not just a sanity check on a header
value that a malicious archive could lie about.

Both extractors also guard against path traversal (a "zip-slip"-style
archive entry like `../../etc/passwd`): each entry's target path is
joined against the extraction dir and then checked to still have that
dir as a prefix before anything is written, erroring out on a mismatch
rather than silently writing outside the intended directory.
