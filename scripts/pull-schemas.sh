#!/usr/bin/env bash
set -euo pipefail

: "${SCHEMA_REPO:=https://github.com/ArthurVardevanyan/kubernetes-json-schema}"
: "${SCHEMA_REPO_BRANCH:=main}"
# renovate: datasource=git-refs depName=ArthurVardevanyan/kubernetes-json-schema
: "${SCHEMA_REPO_SHA:=095a851bf8dee866809fc99a9818fddda9ef779b}"
: "${XDG_CACHE_HOME:=$HOME/.cache}"
: "${SCHEMA_CACHE:=$XDG_CACHE_HOME/k8s-gitops-ci/kubernetes-json-schema}"
: "${OUTPUT:=pkg/lint/kubeconform/schemas/schemas.tar.gz}"

mkdir -p "$(dirname "$OUTPUT")"

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
REF="$SCHEMA_REPO_BRANCH"
if [[ -n "$SCHEMA_REPO_SHA" ]]; then
  REF="$SCHEMA_REPO_SHA"
fi

if [[ ! -d "$SCHEMA_CACHE/.git" ]]; then
  rm -rf "$SCHEMA_CACHE"
  mkdir -p "$SCHEMA_CACHE"
  git -C "$SCHEMA_CACHE" init -q
  git -C "$SCHEMA_CACHE" remote add origin "$SCHEMA_REPO"
fi

# Fetch + reset (rather than pull) so force-pushes / rewritten history
# upstream (e.g. after a rebase of SCHEMA_REPO_BRANCH) never cause
# "divergent branches" failures - and so fetching an arbitrary pinned SHA
# (not necessarily any branch tip) works the same way as fetching a
# branch name.
git -C "$SCHEMA_CACHE" fetch --depth=1 origin "$REF"
git -C "$SCHEMA_CACHE" checkout -B ci-fetch FETCH_HEAD
git -C "$SCHEMA_CACHE" reset --hard FETCH_HEAD
git -C "$SCHEMA_CACHE" clean -fdx

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
mkdir -p "$TMP_DIR/kubernetes-json-schema"

for dir in custom-standalone-strict master-local master-standalone-strict; do
  if [[ -d "$SCHEMA_CACHE/$dir" ]]; then
    cp -r "$SCHEMA_CACHE/$dir" "$TMP_DIR/kubernetes-json-schema/$dir"
  fi
done

( cd "$TMP_DIR" && tar -czf "$(pwd)/schemas.tar.gz" "kubernetes-json-schema" )
cp "$TMP_DIR/schemas.tar.gz" "$OUTPUT"
echo "Wrote $OUTPUT"
