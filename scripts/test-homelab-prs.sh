#!/usr/bin/env bash
# test-homelab-prs.sh — Regression replay: run k8s-gitops-ci against the last N
# merged PRs of a real GitOps repo (default ArthurVardevanyan/HomeLab) and emit a
# Markdown pass/fail report.
#
# This is the org-agnostic core's own end-to-end replay harness. It exercises the
# native `k8s-gitops-ci` binary (no downstream/org layer) against the corpus the
# engine was designed around, so a real regression surfaces as a real failure —
# there is deliberately no "known warning" downgrade classifier here.
#
# Caveats (see docs/DEVELOPMENT.md, "End-to-end / regression replay"):
#   1. Replaying merged PRs can't cover every validator edge, and it is blind
#      to false negatives — a deterministic golden suite (documented there as
#      future work) would be needed for that.
#   2. A newer, intentional check change may legitimately fail an older merged PR.
#      Eyeball the diff; don't treat a single red as a regression on its own.
#
# Usage:
#   ./scripts/test-homelab-prs.sh [--count N] [--repos R1,R2] [--output FILE]
#                                 [--binary PATH] [--parallel N] [--enable-avp]
#
# Examples:
#   ./scripts/test-homelab-prs.sh --count 10
#   ./scripts/test-homelab-prs.sh --repos ArthurVardevanyan/HomeLab --output report.md

set -o errexit
set -o nounset
set -o pipefail
set -o errtrace
shopt -s failglob
shopt -s inherit_errexit

# Any unexpected command failure (missing binary, gh/network error, etc.)
# aborts via errexit; normalize that to exit code 2 ("harness error") so CI
# can tell a setup failure apart from the exit-1 "some PRs failed" signal.
# Deliberate `exit 0`/`exit 1` at the end do NOT trigger ERR, so they win.
#
# Inline rather than a named function, matching the other scripts here: a
# handler only ever reached through `trap` looks unreachable to static
# analysis, and the two codes reported for that (SC2317, SC2329) differ by
# linter version - so a named handler needs a disable whose correctness
# depends on which version a given machine ships. `$?` is expanded while the
# echo command is built, so it is still the failing command's status.
trap 'echo "test-homelab-prs: harness error (exit $?)" >&2; exit 2' ERR

# --- Defaults ---
COUNT=15
REPOS="ArthurVardevanyan/HomeLab"
OUTPUT=""
BINARY="${K8S_GITOPS_CI_BINARY:-./bin/k8s-gitops-ci}"
PARALLEL=5
# AVP (secret rendering) needs org-specific secret-backend access, which a
# generic replay of a public repo does not have — off by default here.
ENABLE_AVP=false
# A replay is only evidence if it runs the pipeline the way the replayed
# repo's own CI runs it. --assume-openshift exempts API groups that ship with
# OpenShift but also run standalone (OLM, Prometheus Operator, ...) from the
# sync-options check, so replaying an OpenShift repo without it reports
# findings that repo's CI never reported — a PR touching OLM manifests failed
# this gate while its real CI had passed it clean.
#
# It is per-repo, not global: it is a fact about the repo being replayed, not
# about the replay. A corpus mixing an OpenShift repo with a vanilla-Kubernetes
# one needs the flag for the former and would be wrong for the latter, so
# repos that need it are listed here rather than the flag being applied to
# whatever REPOS happens to contain.
OPENSHIFT_REPOS="ArthurVardevanyan/HomeLab"

# --- Parse args ---
while [[ $# -gt 0 ]]; do
  case "$1" in
  --count | -n)
    COUNT="$2"
    shift 2
    ;;
  --repos | -r)
    REPOS="$2"
    shift 2
    ;;
  --output | -o)
    OUTPUT="$2"
    shift 2
    ;;
  --binary | -b)
    BINARY="$2"
    shift 2
    ;;
  --parallel | -p)
    PARALLEL="$2"
    shift 2
    ;;
  --enable-avp)
    ENABLE_AVP=true
    shift
    ;;
  --openshift-repos)
    OPENSHIFT_REPOS="$2"
    shift 2
    ;;
  --help | -h)
    echo "Usage: $0 [--count N] [--repos R1,R2] [--output FILE] [--binary PATH] [--parallel N] [--enable-avp] [--openshift-repos R1,R2]"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    exit 1
    ;;
  esac
done

# A repo slug can never contain whitespace, so squeezing it out lets the list
# be written either way ("a/one,b/two" or "a/one, b/two"). The REPOS elements
# are already trimmed individually where they are consumed; without the same
# normalization here, a list written the second way would match only its first
# element and silently drop --assume-openshift for every repo after it - the
# exact class of silently-omitted flag this file exists to prevent.
OPENSHIFT_REPOS="${OPENSHIFT_REPOS//[[:space:]]/}"

# --- Validate binary exists ---
if [[ ! -x "${BINARY}" ]]; then
  echo "Binary not found at ${BINARY}. Building via 'task build'..."
  # 'task build' bakes in version ldflags and the embedded kubeconform schemas /
  # Kyverno policies (embedschemas tag) that a bare 'go build' would skip.
  task build
fi

# --- Temp files ---
RESULTS_DIR=$(mktemp -d "${TMPDIR:-/tmp}/test-homelab-prs.XXXXXX")
# Wall-clock start for the whole replay (elapsed time, distinct from the
# summed per-PR durations — the runs are parallel, so wall < sum).
SCRIPT_START=$(date +%s)
# generate_report writes the number of failed PRs here so the top-level can
# derive its exit code even when the report is generated inside a redirect.
FAIL_TALLY_FILE="${RESULTS_DIR}/.fail_tally"
TOTAL_TESTS=0
trap 'rm -rf "${RESULTS_DIR}"' EXIT

# Get current progress count (counts completed result files)
get_progress() {
  find "${RESULTS_DIR}" -name "*.txt" 2>/dev/null | wc -l | xargs
}

# --- Fetch merged PRs and run tests ---
IFS=',' read -ra REPO_LIST <<<"${REPOS}"

run_test() {
  local REPO="$1"
  local PR_NUMBER="$2"
  local PR_TITLE="$3"
  local RESULT_FILE="${RESULTS_DIR}/${REPO//\//_}_${PR_NUMBER}.txt"

  local START_TIME
  START_TIME=$(date +%s)

  local URL="https://github.com/${REPO}"
  local EXIT_CODE=0
  local OUTPUT

  local EXTRA_FLAGS=()
  if [[ "${ENABLE_AVP}" == "false" ]]; then
    EXTRA_FLAGS+=(--disable-checks avp)
  fi
  # Whole-element match: the commas around both sides mean "org/homelab-docs"
  # cannot match a list entry of "org/homelab".
  if [[ ",${OPENSHIFT_REPOS}," == *",${REPO},"* ]]; then
    EXTRA_FLAGS+=(--assume-openshift)
  fi

  # Unset GH_TOKEN for the binary so it can never post/update PR comments.
  # `--comment` is off by default too, but this is belt-and-suspenders. The
  # script's own `gh` calls use the user's auth independently.
  OUTPUT=$(GH_TOKEN="" "${BINARY}" pipeline --url "${URL}" --pr "${PR_NUMBER}" "${EXTRA_FLAGS[@]}" 2>&1) || EXIT_CODE=$?

  local END_TIME
  END_TIME=$(date +%s)
  local DURATION=$((END_TIME - START_TIME))

  # Extract error count and messages from output.
  local ERROR_COUNT=0
  local ERRORS=""
  local RESULT="pass"
  if [[ ${EXIT_CODE} -ne 0 ]]; then
    ERROR_COUNT=$(echo "${OUTPUT}" | grep -c "ERROR" || true)
    # Structured [ERROR]/FAIL lines plus their stderr/detail continuations.
    ERRORS=$(echo "${OUTPUT}" | grep -E "\[ERROR\]|HOOK FAIL|[[:space:]]FAIL:|^  stderr:|^  detail:" | sed 's/.*\] //' | head -30 || true)
    if [[ -z "${ERRORS}" ]]; then
      ERRORS=$(echo "${OUTPUT}" | grep -A20 "Failed sections:" | tail -n +2 | grep "^[[:space:]]*-" || true)
    fi
    if [[ -z "${ERRORS}" ]]; then
      ERRORS=$(echo "${OUTPUT}" | grep -i "FAILED:" || true)
    fi
    RESULT="fail"
  fi

  # Write structured result (quote values to handle special chars in titles).
  mkdir -p "$(dirname "${RESULT_FILE}")"
  printf '%s\n' \
    "REPO='${REPO}'" \
    "PR='${PR_NUMBER}'" \
    "TITLE='${PR_TITLE//\'/\'\\\'\'}'" \
    "EXIT_CODE='${EXIT_CODE}'" \
    "DURATION='${DURATION}'" \
    "ERROR_COUNT='${ERROR_COUNT}'" \
    "RESULT='${RESULT}'" >"${RESULT_FILE}"
  echo "${ERRORS}" >"${RESULT_FILE%.txt}.errors"

  local PROGRESS
  PROGRESS=$(get_progress)
  local PREFIX="[${PROGRESS}/${TOTAL_TESTS}]"
  if [[ ${EXIT_CODE} -eq 0 ]]; then
    echo "  ${PREFIX} ✓ ${REPO}#${PR_NUMBER} (${DURATION}s)"
  else
    echo "  ${PREFIX} ✗ ${REPO}#${PR_NUMBER} (${DURATION}s) — ${ERROR_COUNT} error(s)"
  fi
}

echo "============================================="
echo " k8s-gitops-ci — Merged PR Replay Report"
echo "============================================="
echo ""
echo "Binary:   ${BINARY}"
echo "Repos:    ${REPOS}"
echo "Count:    ${COUNT} PRs per repo"
echo "Parallel: ${PARALLEL}"
AVP_STATE="disabled"
[[ "${ENABLE_AVP}" == "true" ]] && AVP_STATE="enabled"
echo "AVP:      ${AVP_STATE}"
echo ""

# First pass: count total PRs across all repos.
for REPO in "${REPO_LIST[@]}"; do
  REPO=$(echo "${REPO}" | xargs)
  REPO_COUNT=$(
    gh pr list \
      --repo "${REPO}" \
      --state merged \
      --limit "${COUNT}" \
      --json number \
      --jq 'length'
  )
  TOTAL_TESTS=$((TOTAL_TESTS + REPO_COUNT))
done
echo "Total PRs to test: ${TOTAL_TESTS}"
echo ""

for REPO in "${REPO_LIST[@]}"; do
  REPO=$(echo "${REPO}" | xargs) # trim whitespace
  echo "--- Fetching last ${COUNT} merged PRs from ${REPO}..."

  PRS=$(
    gh pr list \
      --repo "${REPO}" \
      --state merged \
      --limit "${COUNT}" \
      --json number,title,mergedAt \
      --jq '.[] | "\(.number)\t\(.title)\t\(.mergedAt)"'
  )

  if [[ -z "${PRS}" ]]; then
    echo "  No merged PRs found for ${REPO}"
    continue
  fi

  PR_COUNT=$(echo "${PRS}" | wc -l | xargs)
  echo "  Found ${PR_COUNT} PRs. Running tests..."

  JOB_COUNT=0
  while IFS=$'\t' read -r PR_NUMBER PR_TITLE _PR_MERGED; do
    run_test "${REPO}" "${PR_NUMBER}" "${PR_TITLE}" &
    JOB_COUNT=$((JOB_COUNT + 1))
    if ((JOB_COUNT >= PARALLEL)); then
      wait -n 2>/dev/null || true
      JOB_COUNT=$((JOB_COUNT - 1))
    fi
  done <<<"${PRS}"

  wait
  echo ""
done

# --- Generate Markdown Report ---
echo "Generating report..."
echo ""

REPO="" PR="" TITLE="" EXIT_CODE="" DURATION="" ERROR_COUNT="" RESULT=""

generate_report() {
  local TIMESTAMP
  TIMESTAMP=$(date -u +"%Y-%m-%d %H:%M:%S UTC")

  echo "# k8s-gitops-ci — Merged PR Replay Report"
  echo ""
  echo "Generated: ${TIMESTAMP}"
  echo ""
  echo "| Binary | Repos | PRs per repo |"
  echo "|--------|-------|--------------|"
  echo "| \`${BINARY}\` | ${REPOS} | ${COUNT} |"
  echo ""

  local TOTAL_PASS=0
  local TOTAL_FAIL=0
  local TOTAL_DURATION=0

  for REPO in "${REPO_LIST[@]}"; do
    REPO=$(echo "${REPO}" | xargs)
    local REPO_SLUG="${REPO//\//_}"

    echo "## ${REPO}"
    echo ""
    echo "| # | PR | Result | Duration | Errors |"
    echo "|---|-----|--------|----------|--------|"

    local PASS=0
    local FAIL=0
    local IDX=0

    # shellcheck disable=SC2312
    while IFS= read -r RESULT_FILE; do
      # shellcheck source=/dev/null
      source "${RESULT_FILE}"
      IDX=$((IDX + 1))

      local STATUS
      if [[ "${EXIT_CODE}" -eq 0 ]]; then
        STATUS="✅ Pass"
        PASS=$((PASS + 1))
      else
        STATUS="❌ Fail"
        FAIL=$((FAIL + 1))
      fi

      local SHORT_TITLE="${TITLE}"
      if [[ ${#SHORT_TITLE} -gt 50 ]]; then
        SHORT_TITLE="${SHORT_TITLE:0:47}..."
      fi

      echo "| ${IDX} | [#${PR}](https://github.com/${REPO}/pull/${PR}) ${SHORT_TITLE} | ${STATUS} | ${DURATION}s | ${ERROR_COUNT} |"
      TOTAL_DURATION=$((TOTAL_DURATION + DURATION))
    done < <(find "${RESULTS_DIR}" -name "${REPO_SLUG}_*.txt" -print0 | sort -z -t_ -k3 -rn | tr '\0' '\n')

    TOTAL_PASS=$((TOTAL_PASS + PASS))
    TOTAL_FAIL=$((TOTAL_FAIL + FAIL))

    echo ""
    local REPO_TOTAL=$((PASS + FAIL))
    echo "**Summary:** ${PASS} passed, ${FAIL} failed out of ${REPO_TOTAL} PRs"
    echo ""
  done

  echo "---"
  echo ""
  echo "## Overall Summary"
  echo ""
  local TOTAL=$((TOTAL_PASS + TOTAL_FAIL))
  local PASS_RATE=0
  if [[ ${TOTAL} -gt 0 ]]; then
    PASS_RATE=$((TOTAL_PASS * 100 / TOTAL))
  fi
  local WALL_TIME=$(($(date +%s) - SCRIPT_START))
  echo "| Total | Passed | Failed | Pass Rate | Wall Time | Sum of PR Times |"
  echo "|-------|--------|--------|-----------|-----------|-----------------|"
  echo "| ${TOTAL} | ${TOTAL_PASS} | ${TOTAL_FAIL} | ${PASS_RATE}% | ${WALL_TIME}s | ${TOTAL_DURATION}s |"
  echo ""

  # Surface the failure tally to the caller (generate_report runs in a
  # subshell/pipe when redirected to a file, so a bare variable wouldn't
  # propagate — write it to a sentinel file the top-level reads for its
  # exit code).
  echo "${TOTAL_FAIL}" >"${FAIL_TALLY_FILE}"

  if [[ ${TOTAL_FAIL} -gt 0 ]]; then
    echo "### Failed PRs"
    echo ""
    for RESULT_FILE in "${RESULTS_DIR}"/*.txt; do
      # shellcheck source=/dev/null
      source "${RESULT_FILE}"
      if [[ "${RESULT}" == "fail" ]]; then
        echo "#### [${REPO}#${PR}](https://github.com/${REPO}/pull/${PR}): ${TITLE}"
        echo ""
        local ERRORS_FILE="${RESULT_FILE%.txt}.errors"
        if [[ -s "${ERRORS_FILE}" ]]; then
          echo '```log'
          cat "${ERRORS_FILE}"
          echo '```'
        else
          echo "_(no error details captured)_"
        fi
        echo ""
      fi
    done
    echo ""
  fi
}

if [[ -n "${OUTPUT}" ]]; then
  generate_report >"${OUTPUT}"
  if command -v prettier &>/dev/null; then
    prettier --write "${OUTPUT}" >/dev/null 2>&1
    echo "Formatted: ${OUTPUT} (prettier)"
  fi
  echo "Report written to: ${OUTPUT}"
else
  echo ""
  generate_report
fi

echo ""
echo "Done."

# Exit-code contract (consumed by CI to classify the replay):
#   0 — every replayed PR passed
#   1 — one or more PRs FAILED (a "warn"/eyeball-the-diff signal, not
#       necessarily a regression — a newer, stricter check may legitimately
#       fail an older merged PR)
#   2 — harness/setup error (missing binary, gh failure, etc.) — these abort
#       earlier via `set -o errexit`, which propagates the failing command's
#       own status; the wrapper below normalizes any such non-zero abort to 2.
# The report (stdout/--output) is always fully generated first, regardless.
TOTAL_FAIL_COUNT=0
if [[ -f "${FAIL_TALLY_FILE}" ]]; then
  TOTAL_FAIL_COUNT=$(cat "${FAIL_TALLY_FILE}")
fi
if [[ "${TOTAL_FAIL_COUNT}" -gt 0 ]]; then
  echo "${TOTAL_FAIL_COUNT} replayed PR(s) failed."
  exit 1
fi
exit 0
