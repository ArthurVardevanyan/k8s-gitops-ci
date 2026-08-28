#!/usr/bin/env bash
set -euo pipefail

# Verify that every runtime validation check cites an upstream Kubernetes
# function that exists and has not changed since the check was validated.
#
# This is the enforcement behind the rule documented in docs/CI.md: a runtime
# check must be a faithful 1:1 port of a specific upstream validation rule.
# The family is always-blocking and non-exemptable, which is only defensible
# if the API server really would reject the manifest.
#
# Fetched sources are cached under XDG_CACHE_HOME, so a re-run against an
# already-verified tag does no network I/O. This runs as a step in `task ci`,
# matching how scripts/pull-schemas.sh fetches its pinned artifact.
#
# The tag is derived from the k8s.io/api version in go.mod (v0.36.3 ->
# v1.36.3) rather than pinned separately, so bumping the Kubernetes
# dependency forces re-verification instead of letting the citation pin
# silently lag behind the typed structs the checks are written against.
# Pass --tag to override, e.g. to preview an upcoming release.

cd "$(dirname "$0")/.."

exec go run ./internal/cmd/verify-upstream-refs "$@"
