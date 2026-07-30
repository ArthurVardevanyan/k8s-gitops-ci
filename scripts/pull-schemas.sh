#!/usr/bin/env bash
set -euo pipefail

: "${SCHEMA_REPO:=https://github.com/ArthurVardevanyan/kubernetes-json-schema}"
: "${XDG_CACHE_HOME:=$HOME/.cache}"
: "${SCHEMA_CACHE:=$XDG_CACHE_HOME/k8s-gitops-ci/kubernetes-json-schema}"
: "${OUTPUT:=pkg/lint/kubeconform/schemas/schemas.tar.gz}"

mkdir -p "$(dirname "$OUTPUT")"
mkdir -p "$SCHEMA_CACHE"

if [[ -d "$SCHEMA_CACHE/.git" ]]; then
  # Use fetch + reset instead of pull so that force-pushes / rewritten
  # history on the upstream "production" branch (e.g. after a rebase)
  # don't cause "divergent branches" failures.
  git -C "$SCHEMA_CACHE" fetch --depth=1 origin production
  git -C "$SCHEMA_CACHE" checkout -B production FETCH_HEAD
  git -C "$SCHEMA_CACHE" reset --hard FETCH_HEAD
  git -C "$SCHEMA_CACHE" clean -fdx
else
  rm -rf "$SCHEMA_CACHE"
  git clone --depth=1 "$SCHEMA_REPO" "$SCHEMA_CACHE"
fi

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
