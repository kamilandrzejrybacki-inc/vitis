#!/usr/bin/env bash
# 04_converse_mock_max_turns.sh — A2A conversation hits the max-turns hard cap
#
# What it tests:
#   - vitis converse spawns two mock agents
#   - Strict A→B alternation
#   - Conversation reaches max_turns_hit when neither peer emits the sentinel
#   - JSON FinalResult shape is well-formed
#
# Note: provider:mock is only registered in test builds, so this script
# uses two real mock-agent subprocesses driven through the production
# claude-code provider URI by overriding VITIS_CLAUDE_BINARY. This is the
# same pattern the automated E2E tests use under the hood.
#
# Run: tests/manual/04_converse_mock_max_turns.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "04_converse_mock_max_turns: A2A reaches max-turns cap"

VITIS="$(vitis_bin)"
MOCK="$(mockagent_bin)"
export VITIS_CLAUDE_BINARY="${MOCK}"
export MOCK_RESPONSE="ok"
# MOCK_MULTI_TURN is set automatically by the mock provider adapter when
# spawned in converse mode, but we need it for the claude-code wrapper too.
export MOCK_MULTI_TURN=1

info "vitis converse claude-code↔claude-code, max-turns 5, no sentinel"
out=$( "${VITIS}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed "discuss mock A2A" \
  --max-turns 5 \
  --per-turn-timeout 5 \
  --terminator sentinel \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=false ) || {
    fail "vitis converse exited non-zero"
  }

print_json "${out}"
assert_conv_status "${out}" "max_turns_hit"

turns_consumed=$(json_field "${out}" conversation.turns_consumed)
if [[ "${turns_consumed}" != "5" ]]; then
  fail "expected turns_consumed=5, got ${turns_consumed}"
fi
ok "turns_consumed=${turns_consumed}"

verify "human review: each turn alternates a→b→a→b→a, response field of each turn contains 'turn N: ok'"
