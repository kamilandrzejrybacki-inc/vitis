#!/usr/bin/env bash
# 05_converse_mock_sentinel.sh — A2A terminates via sentinel token
#
# What it tests:
#   - sentinel terminator detects <<END>> emitted by a peer
#   - conversation finalizes with status=completed_sentinel
#   - the sentinel is stripped from the response forwarded to the other peer
#     (verified by inspecting the conversation log on disk)
#   - the conversation ends BEFORE max-turns
#
# Run: tests/manual/05_converse_mock_sentinel.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "05_converse_mock_sentinel: sentinel terminator early-exits"

CLANK="$(clank_bin)"
MOCK="$(mockagent_bin)"
export CLANK_CLAUDE_BINARY="${MOCK}"
export MOCK_RESPONSE="ok"
export MOCK_MULTI_TURN=1
export MOCK_SENTINEL_AT_TURN=3   # peer-B emits <<END>> on its 3rd local turn

info "clank converse, max-turns 20, sentinel on peer-B turn 3 → expected total ~6 turns"
out=$( "${CLANK}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed "discuss until sentinel" \
  --max-turns 20 \
  --per-turn-timeout 5 \
  --terminator sentinel \
  --sentinel "<<END>>" \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=false ) || {
    fail "clank converse exited non-zero"
  }

print_json "${out}"
assert_conv_status "${out}" "completed_sentinel"

turns_consumed=$(json_field "${out}" conversation.turns_consumed)
info "turns_consumed=${turns_consumed} (expected ~6, must be < 20)"
if (( turns_consumed >= 20 )); then
  fail "conversation hit max-turns; sentinel was NOT detected"
fi
ok "conversation terminated at turn ${turns_consumed} via sentinel"

# Verify sentinel is NOT leaked into the persisted log of subsequent turns.
log_jsonl="${TEST_LOG_DIR}/conversations/$(json_field "${out}" conversation.conversation_id).jsonl"
if [[ -f "${log_jsonl}" ]]; then
  if grep -qF "<<END>>" "${log_jsonl}"; then
    warn "the sentinel string IS present somewhere in the log — verify by hand it is only in the FINAL turn's response, not in any envelope"
  else
    ok "sentinel is correctly stripped from forwarded envelopes"
  fi
fi
