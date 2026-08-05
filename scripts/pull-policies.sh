#!/usr/bin/env bash
set -euo pipefail

: "${OUTPUT:=pkg/lint/kyverno/policies/policies.tar.gz}"

mkdir -p "$(dirname "${OUTPUT}")"

# Policies are intentionally a placeholder. Replace with real policies when
# ready - at that point this should gain a real upstream ref/SHA pin the
# same way pull-schemas.sh does, and PLACEHOLDER_REF below becomes that pin.

# Short-circuit: mirrors pull-schemas.sh's SCHEMA_REPO_SHA marker - if the
# archive already exists and a sibling marker records the exact content
# version we're about to (re)generate, skip regenerating entirely. This is
# a `deps:` of test/build/lint/vulncheck and runs on every CI invocation;
# bump PLACEHOLDER_REF whenever the placeholder content below changes.
PLACEHOLDER_REF="v1"
MARKER="${OUTPUT}.ref"
CURRENT_REF=""
if [[ -f "${MARKER}" ]]; then
  CURRENT_REF="$(cat "${MARKER}")"
fi
if [[ -f "${OUTPUT}" && "${CURRENT_REF}" == "${PLACEHOLDER_REF}" ]]; then
  echo "${OUTPUT} already up to date for placeholder ${PLACEHOLDER_REF} (skipping regeneration)"
  exit 0
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT
mkdir -p "${TMP_DIR}/kyverno-policies"

cat >"${TMP_DIR}/kyverno-policies/README.md" <<'INNER'
# Kyverno Policies

This archive is a placeholder. Populate `base/`, `components/` and
`overlays/_ci/kustomization.yaml` before enabling policy validation.
INNER

# --sort/--mtime/--owner/--group/--numeric-owner strip every source of
# non-determinism tar would otherwise pick up from the filesystem
# (directory-walk order, generation-time mtimes, local uid/gid), and
# `gzip -n` drops gzip's own embedded mtime/name header field - without
# all of these, this archive gets different bytes on every run even
# though its content is always identical, which defeats Go's build/test
# cache for every package that go:embeds it (pkg/lint/kyverno/policies,
# pkg/lint/kyverno, pkg/validator, pkg/pipeline, cmd/k8s-gitops-ci - see
# the sibling embed_archive.go and docs/SCHEMAS.md).
tar --sort=name --mtime="UTC 1970-01-01" --owner=0 --group=0 --numeric-owner \
  -cf - -C "${TMP_DIR}" "kyverno-policies" | gzip -n >"${TMP_DIR}/policies.tar.gz"
cp "${TMP_DIR}/policies.tar.gz" "${OUTPUT}"
printf '%s\n' "${PLACEHOLDER_REF}" >"${MARKER}"
echo "Wrote placeholder ${OUTPUT}"
