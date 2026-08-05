# Release

This describes what's actually configured in `.goreleaser.yaml`,
`cliff.toml`, and `.tekton/k8s-gitops-ci.yaml` today — see
[TEKTON.md](TEKTON.md) for the pipeline infrastructure this all runs on.

## Table of Contents

- [Versioning](#versioning)
- [Published artifacts](#published-artifacts)
- [Release flow](#release-flow)
- [Dry-run locally](#dry-run-locally)

## Versioning

Semantic versioning, driven entirely by
[git-cliff](https://git-cliff.org/) reading [Conventional
Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`,
`chore:`, `refactor:`, `test:`, `build:`, `ci:` — see `AGENTS.md`) — there
is no manual version bump anywhere. `cliff.toml`'s `commit_parsers` group
commits into changelog sections: `feat` → Features, `fix` → Bug Fixes,
`doc` → Documentation, `perf` → Performance, `refactor` → Refactor,
`style` → Styling, `test` → Testing, `chore` → Miscellaneous Tasks
(except `chore(release): prepare for...`, which is skipped entirely),
`build` → Build, `ci` → CI, and any commit whose body matches `.*security`
→ Security (in addition to whatever group its subject line already put
it in).

Tags are always `v`-prefixed (`vMAJOR.MINOR.PATCH`, e.g. `v0.39.0`) — this
is required for the module to be resolvable as a Go dependency (`go get`/
`go install` only recognize `v`-prefixed tags as module versions).
`cliff.toml`'s `[git] tag_pattern = "v[0-9]*"` keeps `git-cliff`'s bump/
changelog math anchored to that tag set, and the release step in
`.tekton/k8s-gitops-ci.yaml` forces the `v` prefix explicitly on whatever
`git-cliff --bumped-version` returns. Releases up through `0.38.0` were
originally tagged without the `v` prefix (invisible to Go's module
resolver); those have since been mirrored as `v`-prefixed tags (same
commits, same release notes) and the bare originals removed.

## Published artifacts

Only what's actually active in `.goreleaser.yaml` today:

- **Go binaries** for `linux/amd64` and `darwin/arm64` only. The
  `builds[].goos`/`goarch` cartesian product is `{linux,darwin} ×
{amd64,arm64}`, but the `ignore` list removes `darwin/amd64`,
  `linux/arm64`, and `windows/arm64` (`windows` is fully commented out of
  `goos` besides), leaving exactly those two platforms. `builds[].flags`
  passes `-tags=embedschemas` (matching `task build` — see
  [SCHEMAS.md](SCHEMAS.md)), so every published binary has the
  kubeconform schema archive/Kyverno policy archive baked in and works
  standalone out of the box; `before.hooks` runs `task update:schemas`/
  `task update:policies` first so those archives exist on disk for
  `//go:embed` to pick up (a no-op in the real Tekton pipeline, where
  `task ci` already generated them moments earlier in the same step —
  see [Dry-run locally](#dry-run-locally) for why it matters standalone).
- **GitHub Releases** (`release.github: {owner: ArthurVardevanyan, name:
k8s-gitops-ci}`), with the git-cliff-generated changelog as the release
  body.

**Not currently active** — present in config but not shipping:

- **Container images.** `.goreleaser.yaml`'s `kos:` block (targeting
  `registry.arthurvardevanyan.com/homelab/k8s-gitops-ci`,
  `linux/amd64`+`linux/arm64`) exists, but every real `goreleaser
release` invocation in `.tekton/k8s-gitops-ci.yaml` passes `--skip=ko`.
  Taskfile's own `image:build`/`image:publish` targets are commented out
  ("until the registry is wired up"). Don't describe a published
  container image as an existing release artifact.
- There is no Homebrew formula, GCS blob publishing, or any other
  distribution channel beyond the two items above.

## Release flow

Both paths run inside the single Tekton build step described in
[TEKTON.md](TEKTON.md) — there's no separate release job/Task.

1. **On every PR/push, always:** `task ci` (the full local CI pipeline —
   deps check, format check, schema/policy refresh, lint, test+race+
   coverage, build) must pass before anything release-related runs.
2. **Push event (merge to `main`):**
   - `NEW_SEMVER=$(git-cliff --bumped-version)` computes the next semver
     from commits since the last tag.
   - `git tag "${NEW_SEMVER}" "${PARAM_REVISION}"` creates the tag
     **locally only** — GoReleaser's GitHub Releases API call
     (`target_commitish`) auto-creates the tag on GitHub itself; creating
     it there directly via the raw Git Data API isn't permitted for this
     pipeline's GitHub App token (`403: Resource not accessible by
integration`).
   - `git-cliff --bump --unreleased -o "${TMPDIR}/CHANGELOG.md"` writes
     the changelog to a path **outside the git worktree**, specifically
     so it doesn't trip GoReleaser's dirty-working-tree guard.
   - `GORELEASER_CURRENT_TAG="${NEW_SEMVER}" goreleaser release
--skip=ko --clean --release-notes "${TMPDIR}/CHANGELOG.md"` builds
     both binaries and creates the GitHub Release.
3. **PR event:** a snapshot build only — no tag, no GitHub Release.
   `TAG="$(date -u +%Y%m%d%H%M%S)-pr-${PARAM_PR_NUMBER}-${SHORT_SHA}"`,
   then `GORELEASER_CURRENT_TAG="${TAG}" goreleaser release --snapshot
--skip=ko --clean`. This validates that a real release build succeeds
   without publishing anything.

## Dry-run locally

```sh
# Full snapshot release (binaries only, no publish, no tag needed).
goreleaser release --clean --skip=publish --snapshot

# Single-target build (fastest sanity check, current OS/arch only).
goreleaser build --single-target --snapshot --clean
```

Both require `GORELEASER_CURRENT_TAG` to be unset or to point at an
existing local tag — `goreleaser build` doesn't need one, but `release`
without `--snapshot` does.

Unlike `task build`/`task test` (which depend on the `update:schemas`/
`update:policies` Task targets directly), a standalone `goreleaser build`/
`release` doesn't go through `Taskfile.yml` at all by default — that's
why `before.hooks` explicitly runs `task update:schemas`/
`task update:policies` itself (see [Published artifacts](#published-artifacts)),
so `schemas.tar.gz`/`policies.tar.gz` are always present on disk before
the `-tags=embedschemas` build's `//go:embed` needs them, even when
dry-running `goreleaser` directly like this.
