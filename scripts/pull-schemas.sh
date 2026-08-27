#!/usr/bin/env bash
set -euo pipefail

: "${SCHEMA_REPO:=https://github.com/ArthurVardevanyan/kubernetes-json-schema}"
: "${SCHEMA_REPO_BRANCH:=main}"
# renovate: datasource=git-refs depName=ArthurVardevanyan/kubernetes-json-schema
: "${SCHEMA_REPO_SHA:=c970b7981bf4edd36d04ee9893551e40dc1c66d7}"
: "${XDG_CACHE_HOME:=${HOME}/.cache}"
: "${SCHEMA_CACHE:=${XDG_CACHE_HOME}/k8s-gitops-ci/kubernetes-json-schema}"
: "${OUTPUT:=pkg/lint/kubeconform/schemas/schemas.tar.gz}"

mkdir -p "$(dirname "${OUTPUT}")"

# Pinned mode (default): fetch and check out the exact SCHEMA_REPO_SHA
# commit so `task update:schemas`/`task build` are reproducible for a
# given pin - bumping to a newer upstream commit is then an explicit,
# reviewable change to SCHEMA_REPO_SHA (Renovate opens this PR
# automatically via the git-refs customManager in renovate.json that
# tracks this file), not silent drift every time this script re-runs.
#
# Floating mode: only used when SCHEMA_REPO_SHA is explicitly set to
# empty - e.g. by an org overriding SCHEMA_REPO to point at a different
# fork that doesn't share this default pin's commit history, before they
# have their own known-good SHA to pin to (see docs/SCHEMAS.md).
REF="${SCHEMA_REPO_BRANCH}"
if [[ -n "${SCHEMA_REPO_SHA}" ]]; then
  REF="${SCHEMA_REPO_SHA}"
fi

# Short-circuit: in pinned mode, if the archive already exists and a
# sibling marker records the exact REF we're about to fetch, the output
# is already up to date — skip the network fetch + repack entirely. This
# is a `deps:` of test/build/lint/vulncheck and runs every CI invocation,
# so avoiding the per-run `git fetch` when the pin hasn't moved is a real
# saving. Floating mode (empty SCHEMA_REPO_SHA) always re-fetches, since
# a branch tip can move without the marker changing.
MARKER="${OUTPUT}.ref"
CURRENT_REF=""
if [[ -f "${MARKER}" ]]; then
  CURRENT_REF="$(cat "${MARKER}")"
fi
if [[ -n "${SCHEMA_REPO_SHA}" && -f "${OUTPUT}" && "${CURRENT_REF}" == "${REF}" ]]; then
  echo "${OUTPUT} already up to date for ${REF} (skipping fetch)"
  exit 0
fi

if [[ ! -d "${SCHEMA_CACHE}/.git" ]]; then
  rm -rf "${SCHEMA_CACHE}"
  mkdir -p "${SCHEMA_CACHE}"
  git -C "${SCHEMA_CACHE}" init -q
  git -C "${SCHEMA_CACHE}" remote add origin "${SCHEMA_REPO}"
fi

# Fetch + reset (rather than pull) so force-pushes / rewritten history
# upstream (e.g. after a rebase of SCHEMA_REPO_BRANCH) never cause
# "divergent branches" failures - and so fetching an arbitrary pinned SHA
# (not necessarily any branch tip) works the same way as fetching a
# branch name.
git -C "${SCHEMA_CACHE}" fetch --depth=1 origin "${REF}"
git -C "${SCHEMA_CACHE}" checkout -B ci-fetch FETCH_HEAD
git -C "${SCHEMA_CACHE}" reset --hard FETCH_HEAD
git -C "${SCHEMA_CACHE}" clean -fdx

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT
mkdir -p "${TMP_DIR}/kubernetes-json-schema"

for dir in custom-standalone-strict master-local master-standalone-strict; do
  if [[ -d "${SCHEMA_CACHE}/${dir}" ]]; then
    cp -r "${SCHEMA_CACHE}/${dir}" "${TMP_DIR}/kubernetes-json-schema/${dir}"
  fi
done

# --sort/--mtime/--owner/--group/--numeric-owner strip every source of
# non-determinism tar would otherwise pick up from the filesystem
# (directory-walk order, checkout mtimes, local uid/gid), and `gzip -n`
# drops gzip's own embedded mtime/name header field - without all of
# these, this archive gets different bytes on every run even when the
# checked-out schema content is byte-identical, which defeats Go's
# build/test cache for every package that go:embeds it (see the
# sibling embed_archive.go and docs/SCHEMAS.md).
#
# Those flags are GNU tar features: macOS's default `tar` is bsdtar,
# which rejects `--sort=name`. Resolve GNU tar explicitly (gtar via
# homebrew's gnu-tar on macOS; plain tar on Linux) and hard-fail with
# install guidance rather than silently falling back to bsdtar and
# producing a non-reproducible archive that quietly busts Go's cache.
if command -v gtar >/dev/null 2>&1; then
  TAR=gtar
elif tar --version 2>/dev/null | grep -q 'GNU tar'; then
  TAR=tar
else
  echo "ERROR: GNU tar is required to build a reproducible archive," >&2
  echo "       but only bsdtar (which lacks --sort) was found." >&2
  echo "       macOS: brew install gnu-tar (provides 'gtar')." >&2
  exit 1
fi

"${TAR}" --sort=name --mtime="UTC 1970-01-01" --owner=0 --group=0 --numeric-owner \
  -cf - -C "${TMP_DIR}" "kubernetes-json-schema" | gzip -n >"${TMP_DIR}/schemas.tar.gz"
cp "${TMP_DIR}/schemas.tar.gz" "${OUTPUT}"
# Record the ref this archive was built from so a subsequent run with an
# unchanged pin can short-circuit above.
printf '%s\n' "${REF}" >"${MARKER}"
echo "Wrote ${OUTPUT}"
