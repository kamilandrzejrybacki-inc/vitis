#!/usr/bin/env bash
# 03_run_mock_modes.sh — exercise every MOCK_MODE the mock agent supports
#
# What it tests:
#   - vitis run correctly classifies different mock outcomes:
#       happy      → completed
#       blocked    → blocked_on_input
#       auth       → auth_required
#       rate_limit → rate_limited
#       partial    → partial
#       crash      → crashed
#       ansi       → completed (response stripped of ANSI escapes)
#
# This is the most useful "is the run-status classifier still honest"
# regression check before shipping any change to the orchestrator,
# observers, or adapters.
#
# Run: tests/manual/03_run_mock_modes.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "03_run_mock_modes: status classification across mock modes"

VITIS="$(vitis_bin)"
MOCK="$(mockagent_bin)"
export VITIS_CLAUDE_BINARY="${MOCK}"

run_mode() {
  local mode="$1"
  local expected_status="$2"
  local extra_env="${3:-}"

  printf '\n%s---%s mode=%s expected_status=%s\n' "$C_DIM" "$C_RESET" "${mode}" "${expected_status}"
  (
    export MOCK_MODE="${mode}"
    if [[ -n "${extra_env}" ]]; then
      export ${extra_env}
    fi
    out=$( "${VITIS}" run \
        --provider claude-code \
        --prompt "test prompt" \
        --log-path "${TEST_LOG_DIR}/${mode}" \
        --timeout 10 ) || true
    actual=$(json_field "${out}" status)
    if [[ "${actual}" == "${expected_status}" ]]; then
      ok "${mode} → ${actual}"
    else
      echo "${out}"
      warn "${mode} expected ${expected_status}, got ${actual} (verify whether this is a regression or expected drift)"
    fi
  )
}

run_mode "happy"      "completed"        "MOCK_RESPONSE=hello"
run_mode "ansi"       "completed"        "MOCK_RESPONSE=hello"
run_mode "rate_limit" "rate_limited"     ""
run_mode "blocked"    "blocked_on_input" ""
run_mode "auth"       "auth_required"    ""
run_mode "crash"      "crashed"          ""
run_mode "partial"    "partial"          "MOCK_RESPONSE=truncated"

verify "review WARN lines (if any) — they indicate the observer/extractor classification has drifted from the expected baseline"
