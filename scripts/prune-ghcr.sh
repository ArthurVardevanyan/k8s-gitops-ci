#!/usr/bin/env bash
# Prune the GHCR package down to the published tags (`main`, `latest`,
# `X.Y.Z`) plus the per-platform images those tags reference.
#
# GHCR cannot untag: the registry Tag DELETE endpoint is unsupported, so the
# only way to remove anything is the GitHub Packages API, which deletes a
# whole package *version* (the manifest itself).  Deleting a version that an
# index still references breaks that index.  Therefore this script:
#
#   1. lists every package version;
#   2. for every version carrying a kept tag, fetches the index through the
#      registry API and collects its child digests;
#   3. aborts, deleting nothing, if any kept tag cannot be read or any child
#      is missing;
#   4. deletes every version that is neither a kept-tag version nor a child
#      of one;
#   5. re-checks that every child of every kept tag still resolves.
#
# Environment:
#   GH_TOKEN   token with packages:write (required)
#   REPO       owner/name; defaults to $GITHUB_REPOSITORY
#   DRY_RUN    "1" lists what would be deleted without deleting anything
set -euo pipefail

REPO="$(printf '%s' "${REPO:-${GITHUB_REPOSITORY:?REPO or GITHUB_REPOSITORY required}}" | tr '[:upper:]' '[:lower:]')"
: "${GH_TOKEN:?GH_TOKEN required}"
DRY_RUN="${DRY_RUN:-0}"
OWNER="${REPO%%/*}"
PKG="${REPO#*/}"
KEEP_RE='^(main|latest|[0-9]+\.[0-9]+\.[0-9]+)$'
ACCEPT='application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json'

OWNER_TYPE="$(gh api "/users/${OWNER}" --jq .type)"
case "$OWNER_TYPE" in
Organization) API="/orgs/${OWNER}/packages/container/${PKG}/versions" ;;
*) API="/users/${OWNER}/packages/container/${PKG}/versions" ;;
esac

REG_TOKEN="$(curl -sf --user "x-access-token:${GH_TOKEN}" \
  "https://ghcr.io/token?service=ghcr.io&scope=repository:${REPO}:pull" | jq -r .token)"
if [ -z "$REG_TOKEN" ] || [ "$REG_TOKEN" = "null" ]; then
  echo "::error::Could not obtain a GHCR registry token; nothing pruned."
  exit 1
fi

# registry_get <ref> [curl args...] - GET a manifest by tag or digest.
registry_get() {
  local ref="$1"
  shift
  curl -sf -H "Authorization: Bearer ${REG_TOKEN}" -H "Accept: ${ACCEPT}" \
    "$@" "https://ghcr.io/v2/${REPO}/manifests/${ref}"
}

VERSIONS="$(gh api --paginate "${API}?per_page=100" --jq '.[]' | jq -s '.')"
echo "Package has $(jq length <<<"$VERSIONS") version(s)."

# Versions that carry at least one kept tag.
KEPT_TAGGED="$(jq -c --arg re "$KEEP_RE" \
  '[.[] | select((.metadata.container.tags // []) | any(test($re)))]' <<<"$VERSIONS")"
if [ "$(jq length <<<"$KEPT_TAGGED")" -eq 0 ]; then
  echo "::error::No version carries a kept tag (main/latest/X.Y.Z); refusing to prune."
  exit 1
fi

# Digests that must survive: the kept-tag versions and every child they list.
KEEP_DIGESTS="$(jq -r '.[].name' <<<"$KEPT_TAGGED")"
CHILDREN=""
while read -r TAG; do
  [ -z "$TAG" ] && continue
  if ! BODY="$(registry_get "$TAG")"; then
    echo "::error::Kept tag ${TAG} cannot be read from the registry; nothing pruned."
    exit 1
  fi
  for D in $(jq -r '.manifests[]?.digest' <<<"$BODY"); do
    CHILDREN="${CHILDREN}${D}"$'\n'
    if ! registry_get "$D" -o /dev/null -I; then
      echo "::error::Tag ${TAG} references ${D}, which is already missing; nothing pruned."
      exit 1
    fi
  done
done < <(jq -r --arg re "$KEEP_RE" '.[].metadata.container.tags[] | select(test($re))' <<<"$KEPT_TAGGED" | sort -u)
KEEP_DIGESTS="$(printf '%s\n%s' "$KEEP_DIGESTS" "$CHILDREN" | sed '/^$/d' | sort -u)"
echo "Keeping $(wc -l <<<"$KEEP_DIGESTS") digest(s):"
while read -r D; do echo "  $D"; done <<<"$KEEP_DIGESTS"

FAILED=0
COUNT=0
while IFS=$'\t' read -r ID DIGEST TAGS; do
  [ -z "$ID" ] && continue
  if grep -qFx "$DIGEST" <<<"$KEEP_DIGESTS"; then
    continue
  fi
  COUNT=$((COUNT + 1))
  if [ "$DRY_RUN" = "1" ]; then
    echo "Would delete ${ID} ${DIGEST:0:19} [${TAGS}]"
    continue
  fi
  echo "Deleting ${ID} ${DIGEST:0:19} [${TAGS}]"
  if ! gh api -X DELETE "${API}/${ID}" >/dev/null; then
    echo "::warning::Failed to delete version ${ID}"
    FAILED=$((FAILED + 1))
  fi
done < <(jq -r '.[] | [.id, .name, ((.metadata.container.tags // []) | join(","))] | @tsv' <<<"$VERSIONS")
echo "Processed ${COUNT} unreferenced version(s) (dry run: ${DRY_RUN})."

if [ "$DRY_RUN" != "1" ]; then
  # Re-check: every kept tag must still resolve, with every child present.
  while read -r TAG; do
    [ -z "$TAG" ] && continue
    BODY="$(registry_get "$TAG")" || {
      echo "::error::Post-prune: ${TAG} is unreadable."
      exit 1
    }
    for D in $(jq -r '.manifests[]?.digest' <<<"$BODY"); do
      registry_get "$D" -o /dev/null -I || {
        echo "::error::Post-prune: ${TAG} lost child ${D}."
        exit 1
      }
    done
  done < <(jq -r --arg re "$KEEP_RE" '.[].metadata.container.tags[] | select(test($re))' <<<"$KEPT_TAGGED" | sort -u)
  echo "Post-prune verification passed."
fi

[ "$FAILED" -eq 0 ] || {
  echo "::error::${FAILED} deletion(s) failed."
  exit 1
}
