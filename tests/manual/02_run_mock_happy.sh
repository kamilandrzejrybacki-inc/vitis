#!/usr/bin/env bash
# 02_run_mock_happy.sh — vitis run via the mock agent: happy path
#
# What it tests:
#   - vitis run uses the VITIS_*_BINARY override correctly
#   - the mock agent's response is captured and surfaced as JSON
#   - status=completed, response is non-empty
#   - peek can read the resulting session
#
# Run: tests/manual/02_run_mock_happy.sh
#
# This script does NOT need a real Claude/Codex install.
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "02_run_mock_happy: single-shot run via mock agent"

VITIS="$(vitis_bin)"
MOCK="$(mockagent_bin)"

# Point vitis's claude-code adapter at the mock binary so we get a fast,
# deterministic, network-free run.
export VITIS_CLAUDE_BINARY="${MOCK}"
export MOCK_RESPONSE="2 + 2 = 4"
export MOCK_MODE="happy"

info "vitis run --provider claude-code --prompt 'what is 2+2?'"
out=$( "${VITIS}" run --provider claude-code --prompt "what is 2+2?" --log-path "${TEST_LOG_DIR}" --timeout 10 )

echo "${out}"
assert_status "${out}" "completed"
assert_nonempty_response "${out}"

session_id="$(json_field "${out}" session_id)"
ok "session_id=${session_id}"

info "vitis peek the session log"
peek_out=$( "${VITIS}" peek --session-id "${session_id}" --log-path "${TEST_LOG_DIR}" --last 10 )
echo "${peek_out}"
# Peek output shape: { "session_id": "...", "turns": [ {turn_index, role, content, ...}, ... ] }
if [[ -z "$(json_field "${peek_out}" turns.0.content)" ]]; then
  fail "peek returned no turns"
fi
ok "peek returned non-empty turn list"
