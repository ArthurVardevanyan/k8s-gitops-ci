# Release

This describes what's actually configured in `.goreleaser.yaml`, the
`VERSION` file, `cliff.toml`, and `.tekton/k8s-gitops-ci.yaml` today — see
[TEKTON.md](TEKTON.md) for the pipeline infrastructure this all runs on.

## Table of Contents

- [Versioning](#versioning)
- [Published artifacts](#published-artifacts)
- [Release flow](#release-flow)
- [Cutting a release](#cutting-a-release)
- [Release candidates](#release-candidates)
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

The latest-tag and latest-`main` checks compare `VERSION` on the PR branch
as-is, so a branch that never modified `VERSION` but is simply _behind_
`main` (it predates a newer release there) is **not** blocked by either
check: since it didn't touch the file, merging it can't downgrade anything
— the merge inherits main's newer `VERSION`. This is detected via
`git diff --name-only ${BASE_REF}...HEAD` not listing `VERSION`. Any PR
that _does_ change `VERSION` is still subject to the full checks, and when
the base ref can't be resolved (offline/local) — or the diff itself fails —
both checks stay fully in force (fail-closed).

`version:check` also enforces **bump correctness**: when a PR advances
`VERSION` (i.e. proposes a release), the bump must be at least as large as
the conventional commits since the last tag warrant — you can't ship a
`feat:` as a patch bump. It uses [git-cliff](https://git-cliff.org/)
(configured by `cliff.toml`) purely as an **advisor** to compute the
expected bump; `VERSION` still decides the actual number. Only
_under_-bumps are rejected (over-bumps — a deliberately larger release —
are allowed). Pre-1.0, breaking changes (`feat!:`/`BREAKING CHANGE:`) map
to a **minor** bump (`cliff.toml`'s `[bump] breaking_always_bump_major =
false`), not a jump to `1.0.0`. This check is skipped for non-release PRs
(`VERSION` unchanged) and when `git-cliff` isn't installed (local dev), so
it only ever hard-gates a real release bump — and it hard-gates in CI,
where `git-cliff` is provided by the toolbox image.

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
  standalone out of the box; `before.hooks` runs `task schemas:pull`/
  `task policies:pull` first so those archives exist on disk for
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
2. **Push event (merge to `main`):** the step reads `VERSION` and branches
   into one of three outcomes:
   - **GA** — `v${VERSION}` isn't tagged yet (`VERSION` advanced). The step
     re-validates format and strictly-greater-than-latest-tag
     (defense-in-depth alongside `version:check`), then **first deletes
     that version's release candidates** (`v${VERSION}-rc.*` tags +
     releases). This ordering matters: the native notes
     (`changelog.use: github-native`) derive the "previous tag" from the
     most recent existing tag, so a lingering `-rc.*` tag would make the
     changelog compare against the RC instead of the prior GA (truncated
     "What's Changed" + wrong "Full Changelog" link). With the RCs gone,
     it `git tag "v${VERSION}" "${PARAM_REVISION}"` (local only —
     GoReleaser's GitHub Releases API call with `target_commitish`
     auto-creates the tag on GitHub; the raw Git Data API isn't permitted
     for this pipeline's GitHub App token, which returns
     `403: Resource not accessible by integration`), then runs GoReleaser
     (`GORELEASER_CURRENT_TAG="v${VERSION}" goreleaser release --skip=ko --clean`)
     to publish the GitHub Release (binaries + native notes) against the
     correct previous GA.
   - **RC** — `v${VERSION}` is already the latest GA tag, and there's a
     shippable change (see [Release candidates](#release-candidates)). The
     step cuts `v<next>-rc.N` as a GitHub pre-release.
   - **CI-only** — otherwise (no `VERSION` bump and nothing shippable since
     the last GA). **This is what makes ordinary merges CI-only.**
3. **PR event:** a snapshot build only — no tag, no GitHub Release.
   `TAG="$(date -u +%Y%m%d%H%M%S)-pr-${PARAM_PR_NUMBER}-${SHORT_SHA}"`,
   then `GORELEASER_CURRENT_TAG="${TAG}" goreleaser release --snapshot
--skip=ko --clean`. This validates that a real release build succeeds
   without publishing anything.

## Cutting a release

Releases are gated by a pull request — the same review gate as any code
change — not by a local tag push or a per-merge automation:

1. Edit the repo-root **`VERSION`** file to the new version
   (`MAJOR.MINOR.PATCH`, no `v` prefix), e.g. `0.46.1` → `0.47.0`. Choose
   the bump size to match what's shipping — at least a minor if any
   `feat:` has landed since the last release (`version:check` enforces
   this; see [Versioning](#versioning)).
2. Open a PR with that change (a good place to note what's in the
   release). `task ci`/`version:check` runs on it and fails if the new
   `VERSION` is malformed, not ahead of the latest tag/`main`, or an
   under-bump for the commits since the last release.
3. **Merge the PR.** That merge is a `push` to `main`; the pipeline sees
   `v${VERSION}` doesn't exist yet, so it tags `v${VERSION}` and publishes
   the GitHub Release (binaries + native notes).

No one pushes a tag by hand; the tag is created by the pipeline as a
consequence of the merged `VERSION` bump.

## Release candidates

Between GA releases, the pipeline automatically publishes **release
candidates** for the _next_ version so changes can be validated before GA
(e.g. a personal repo can pin to an RC build to test). RCs are **binary/
asset only** — download the artifacts from the RC's GitHub pre-release;
they are not meant to be consumed as a Go module (see the note below).

**When an RC is cut.** On a merge to `main`, the pipeline cuts
`v<next>-rc.N` only when **all** of these hold:

1. `VERSION` equals the latest GA tag — i.e. we're between releases, not
   on a GA-bump merge.
2. A **binary-affecting** path changed since the latest GA tag. Only
   inputs that alter the shipped binary count:
   - `**/*.go`
   - `scripts/pull-schemas.sh`, `scripts/pull-policies.sh`,
     `scripts/pull-all.sh` — these pin (via a SHA) the embedded
     kubeconform-schema / Kyverno-policy archives baked into the binary at
     build time, so a bump there changes what the binary ships even though
     the `.tar.gz` archives themselves are gitignored.
   - `.goreleaser.yaml`
     Docs, `.tekton/`, CI/editor config, `Taskfile.yml`, and `go.mod`/
     `go.sum` (Go-module dependency bumps — low-risk, covered by tests)
     do **not** trigger an RC on their own.
3. git-cliff computes a `next` version greater than the latest GA tag
   (there's a conventional releasable commit to size it).

`<next>` is git-cliff's `--bumped-version` (the same advisor
`version:check` uses), so it tracks whether the pending release is a patch
or a minor. `N` is the next integer after the highest existing
`v<next>-rc.*`. goreleaser's `prerelease: auto` marks `-rc` tags as GitHub
pre-releases (never "Latest").

**Advisor tag scoping.** `cliff.toml`'s `tag_pattern` is anchored to
strict GA tags (`^v[0-9]+\.[0-9]+\.[0-9]+$`) — it deliberately excludes
RC tags. If pre-release tags were included in the pattern,
`git-cliff --bumped-version --unreleased` would anchor off the latest RC
(e.g. `v0.48.4-rc.1`) instead of the last GA tag (`v0.48.3`), producing a
pre-release-shaped `next` (e.g. `0.48.4-rc.2`) that fails the downstream
strict `MAJOR.MINOR.PATCH` regex gate in both the Tekton pipeline and
`version:check`, silently blocking RC cuts and the bump-correctness guard.

**Cleanup.**

- When the GA for a version is published, its `v<version>-rc.*`
  **releases and tags are deleted** (best-effort — cleanup never fails a
  publish).
- If the computed `next` shifts mid-cycle (e.g. patch RCs exist, then a
  `feat:` lands so `next` becomes a minor), the now-stale RCs for the old
  base are deleted when the first RC for the new base is cut — they would
  never become GA.

**Go-module note.** RC tags use standard semver (`v<next>-rc.N`), which is
a valid Go pre-release version, and they are deleted at GA. Deleting a tag
that the Go module proxy has cached can cause `go get` resolution errors
for anyone who pinned it. This is an accepted trade-off for the
binary-only RC workflow: don't `go get` an RC as a module. (`go get`/
`@latest` ignores pre-releases anyway.)

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

Unlike `task build`/`task test` (which depend on the `schemas:pull`/
`policies:pull` Task targets directly), a standalone `goreleaser build`/
`release` doesn't go through `Taskfile.yml` at all by default — that's
why `before.hooks` explicitly runs `task schemas:pull`/
`task policies:pull` itself (see [Published artifacts](#published-artifacts)),
so `schemas.tar.gz`/`policies.tar.gz` are always present on disk before
the `-tags=embedschemas` build's `//go:embed` needs them, even when
dry-running `goreleaser` directly like this.
