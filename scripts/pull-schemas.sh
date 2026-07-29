#!/usr/bin/env bash
set -euo pipefail

: "${SCHEMA_REPO:=https://github.com/ArthurVardevanyan/kubernetes-json-schema}"
: "${SCHEMA_VERSION:=1.29.0}"
: "${XDG_CACHE_HOME:=$HOME/.cache}"
: "${SCHEMA_CACHE:=$XDG_CACHE_HOME/k8s-gitops-ci/kubernetes-json-schema}"
: "${OUTPUT:=pkg/lint/kubeconform/schemas/schemas.tar.gz}"

mkdir -p "$(dirname "$OUTPUT")"
mkdir -p "$SCHEMA_CACHE"

if [[ -d "$SCHEMA_CACHE/.git" ]]; then
  git -C "$SCHEMA_CACHE" pull --depth=1 origin master
else
  rm -rf "$SCHEMA_CACHE"
  git clone --depth=1 "$SCHEMA_REPO" "$SCHEMA_CACHE"
fi

VERSION_DIR=""
for suffix in "-standalone-strict" "-standalone" ""; do
  if [[ -d "$SCHEMA_CACHE/${SCHEMA_VERSION}${suffix}" ]]; then
    VERSION_DIR="${SCHEMA_VERSION}${suffix}"
    break
  fi
done

if [[ -z "$VERSION_DIR" ]]; then
  echo "ERROR: could not find schema dir for version $SCHEMA_VERSION in $SCHEMA_CACHE" >&2
  exit 1
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
mkdir -p "$TMP_DIR/kubernetes-json-schema"
cp -r "$SCHEMA_CACHE/$VERSION_DIR" "$TMP_DIR/kubernetes-json-schema/$VERSION_DIR"

( cd "$TMP_DIR" && tar -czf "$(pwd)/schemas.tar.gz" "kubernetes-json-schema" )
cp "$TMP_DIR/schemas.tar.gz" "$OUTPUT"
echo "Wrote $OUTPUT"
