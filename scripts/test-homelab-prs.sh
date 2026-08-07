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
# Caveats (see docs/E2E.md):
#   1. Replaying merged PRs can't cover every validator edge — that's what the
#      deterministic golden suite (task test:e2e) is for.
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

# --- Defaults ---
COUNT=15
REPOS="ArthurVardevanyan/HomeLab"
OUTPUT=""
BINARY="${K8S_GITOPS_CI_BINARY:-./bin/k8s-gitops-ci}"
PARALLEL=5
# AVP (secret rendering) needs org-specific secret-backend access, which a
# generic replay of a public repo does not have — off by default here.
ENABLE_AVP=false

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
  --help | -h)
    echo "Usage: $0 [--count N] [--repos R1,R2] [--output FILE] [--binary PATH] [--parallel N] [--enable-avp]"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    exit 1
    ;;
  esac
done

# --- Validate binary exists ---
if [[ ! -x "${BINARY}" ]]; then
  echo "Binary not found at ${BINARY}. Building via 'task build'..."
  # 'task build' bakes in version ldflags and the embedded kubeconform schemas /
  # Kyverno policies (embedschemas tag) that a bare 'go build' would skip.
  task build
fi

# --- Temp files ---
RESULTS_DIR=$(mktemp -d "${TMPDIR:-/tmp}/test-homelab-prs.XXXXXX")
TOTAL_TESTS=0
cleanup() { rm -rf "${RESULTS_DIR}"; }
trap cleanup EXIT

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
  local PR_AUTHOR="$4"
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
    "AUTHOR='${PR_AUTHOR}'" \
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
      --json number,title,author,mergedAt \
      --jq '.[] | "\(.number)\t\(.title)\t\(.author.login)\t\(.mergedAt)"'
  )

  if [[ -z "${PRS}" ]]; then
    echo "  No merged PRs found for ${REPO}"
    continue
  fi

  PR_COUNT=$(echo "${PRS}" | wc -l | xargs)
  echo "  Found ${PR_COUNT} PRs. Running tests..."

  JOB_COUNT=0
  while IFS=$'\t' read -r PR_NUMBER PR_TITLE PR_AUTHOR _PR_MERGED; do
    run_test "${REPO}" "${PR_NUMBER}" "${PR_TITLE}" "${PR_AUTHOR}" &
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

REPO="" PR="" TITLE="" AUTHOR="" EXIT_CODE="" DURATION="" ERROR_COUNT="" RESULT=""

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

  for REPO in "${REPO_LIST[@]}"; do
    REPO=$(echo "${REPO}" | xargs)
    local REPO_SLUG="${REPO//\//_}"

    echo "## ${REPO}"
    echo ""
    echo "| # | PR | Author | Result | Duration | Errors |"
    echo "|---|-----|--------|--------|----------|--------|"

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

      echo "| ${IDX} | [#${PR}](https://github.com/${REPO}/pull/${PR}) ${SHORT_TITLE} | @${AUTHOR} | ${STATUS} | ${DURATION}s | ${ERROR_COUNT} |"
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
  echo "| Total | Passed | Failed | Pass Rate |"
  echo "|-------|--------|--------|-----------|"
  echo "| ${TOTAL} | ${TOTAL_PASS} | ${TOTAL_FAIL} | ${PASS_RATE}% |"
  echo ""

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
