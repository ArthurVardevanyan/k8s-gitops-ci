#!/usr/bin/env bash
set -euo pipefail

: "${OUTPUT:=pkg/lint/kyverno/policies/policies.tar.gz}"

mkdir -p "$(dirname "$OUTPUT")"

# Policies are intentionally a placeholder. Replace with real policies when ready.
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
mkdir -p "$TMP_DIR/kyverno-policies"

cat > "$TMP_DIR/kyverno-policies/README.md" <<'INNER'
# Kyverno Policies

This archive is a placeholder. Populate `base/`, `components/` and
`overlays/_ci/kustomization.yaml` before enabling policy validation.
INNER

( cd "$TMP_DIR" && tar -czf "$(pwd)/policies.tar.gz" "kyverno-policies" )
cp "$TMP_DIR/policies.tar.gz" "$OUTPUT"
echo "Wrote placeholder $OUTPUT"
