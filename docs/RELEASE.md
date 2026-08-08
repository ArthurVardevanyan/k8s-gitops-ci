# Release

This describes what's actually configured in `.goreleaser.yaml`, the
`VERSION` file, and `.tekton/k8s-gitops-ci.yaml` today — see
[TEKTON.md](TEKTON.md) for the pipeline infrastructure this all runs on.

## Table of Contents

- [Versioning](#versioning)
- [Published artifacts](#published-artifacts)
- [Release flow](#release-flow)
- [Cutting a release](#cutting-a-release)
- [Dry-run locally](#dry-run-locally)

## Versioning

Semantic versioning, with the repo-root **`VERSION`** file
(`MAJOR.MINOR.PATCH`, no `v` prefix — e.g. `0.47.0`) as the **single
source of truth**. A release happens only when a merge to `main` advances
`VERSION` past the latest existing tag; the pipeline then creates the tag
`v${VERSION}` and publishes. Ordinary merges — where `VERSION` is
unchanged — run CI only and produce no release. There is no automatic
per-commit version bump: bumping `VERSION` is a deliberate, reviewed
change (see [Cutting a release](#cutting-a-release)).

`task version:check` (part of `task ci`, so it runs on every PR) validates
that `VERSION` is well-formed `MAJOR.MINOR.PATCH` and never goes
**backwards** — it must be `>=` both the latest release tag and the
`VERSION` currently on `main` (`origin/main`). `VERSION` may _equal_ those
(the common, non-release case: most PRs don't touch it), but a PR that
_lowers_ `VERSION` — even to a value still ahead of the latest tag — fails
CI and can't merge. (The `origin/main` comparison is enforced whenever
that ref is resolvable — always in the Tekton PR run; a purely local
`task version:check` with no network falls back to the tag comparison.)

Tags are always `v`-prefixed (`vMAJOR.MINOR.PATCH`, e.g. `v0.47.0`) — this
is required for the module to be resolvable as a Go dependency (`go get`/
`go install` only recognize `v`-prefixed tags as module versions). The
release step derives the tag as `v${VERSION}`. Releases up through
`0.38.0` were originally tagged without the `v` prefix (invisible to Go's
module resolver); those have since been mirrored as `v`-prefixed tags
(same commits, same release notes) and the bare originals removed.

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
- **GitHub Releases**
  (`release.github: {owner: ArthurVardevanyan, name: k8s-gitops-ci}`).
  The release body is GitHub's **native auto-generated release notes**
  (`.goreleaser.yaml`'s `changelog.use: github-native`): a "What's
  Changed" list of merged PRs, a "New Contributors" section, and a Full
  Changelog compare link.

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

Everything runs inside the single Tekton build step described in
[TEKTON.md](TEKTON.md) — there's no separate release job/Task.

1. **On every PR/push, always:** `task ci` (the full local CI pipeline —
   version check, deps check, format check, schema/policy refresh, lint,
   test+race+coverage, build) must pass before anything release-related
   runs. `task ci`'s `version:check` step validates the `VERSION` file.
2. **Push event (merge to `main`):** the step reads `VERSION` and forms
   `NEW_SEMVER="v${VERSION}"`, then:
   - If the tag `${NEW_SEMVER}` **already exists**, this merge did not
     bump `VERSION` (the common case) — or the pipeline is re-running
     after a prior successful release. Either way there's nothing to
     release, so the release path is skipped cleanly. **This is what makes
     ordinary merges CI-only.**
   - Otherwise `VERSION` advanced. The step re-validates the format and
     that it's strictly greater than the latest tag (defense-in-depth
     alongside `version:check`), then `git tag "${NEW_SEMVER}"
"${PARAM_REVISION}"` creates the tag **locally only** — GoReleaser's
     GitHub Releases API call (`target_commitish`) auto-creates the tag on
     GitHub itself; creating it via the raw Git Data API isn't permitted
     for this pipeline's GitHub App token (`403: Resource not accessible
by integration`). Then
     `GORELEASER_CURRENT_TAG="${NEW_SEMVER}" goreleaser release --skip=ko
--clean` builds both binaries and creates the GitHub Release, whose
     body is GitHub's native auto-generated release notes
     (`changelog.use: github-native`).
3. **PR event:** a snapshot build only — no tag, no GitHub Release.
   `TAG="$(date -u +%Y%m%d%H%M%S)-pr-${PARAM_PR_NUMBER}-${SHORT_SHA}"`,
   then `GORELEASER_CURRENT_TAG="${TAG}" goreleaser release --snapshot
--skip=ko --clean`. This validates that a real release build succeeds
   without publishing anything.

## Cutting a release

Releases are gated by a pull request — the same review gate as any code
change — not by a local tag push or a per-merge automation:

1. Edit the repo-root **`VERSION`** file to the new version
   (`MAJOR.MINOR.PATCH`, no `v` prefix), e.g. `0.46.1` → `0.47.0`.
2. Open a PR with that change (a good place to note what's in the
   release). `task ci`/`version:check` runs on it and fails if the new
   `VERSION` is malformed or not ahead of the latest tag.
3. **Merge the PR.** That merge is a `push` to `main`; the pipeline sees
   `v${VERSION}` doesn't exist yet, so it tags `v${VERSION}` and publishes
   the GitHub Release (binaries + native notes).

No one pushes a tag by hand; the tag is created by the pipeline as a
consequence of the merged `VERSION` bump.

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
