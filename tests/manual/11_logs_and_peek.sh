#!/usr/bin/env bash
# 11_logs_and_peek.sh — verify file-store persistence and peek output
#
# What it tests:
#   - clank run writes session.json + turns.jsonl under <log-path>/
#   - clank converse writes conversations/<id>.json + .jsonl under <log-path>/
#   - clank peek can read both shapes
#   - file permissions are 0600 (readable only by owner)
#
# Run: tests/manual/11_logs_and_peek.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "11_logs_and_peek: persistence shape and permissions"

CLANK="$(clank_bin)"
MOCK="$(mockagent_bin)"
export CLANK_CLAUDE_BINARY="${MOCK}"
export MOCK_RESPONSE="hello world"
export MOCK_MULTI_TURN=1

# --- single-shot run ---
info "clank run, write session log to ${TEST_LOG_DIR}"
unset MOCK_MULTI_TURN
export MOCK_MODE="happy"
out=$( "${CLANK}" run --provider claude-code --prompt "ping" --log-path "${TEST_LOG_DIR}" --timeout 10 )
session_id=$(json_field "${out}" session_id)

[[ -f "${TEST_LOG_DIR}/sessions/${session_id}.json" ]] || fail "missing session JSON"
[[ -f "${TEST_LOG_DIR}/turns/${session_id}.jsonl" ]]   || fail "missing turn JSONL"

session_perms=$(stat -c '%a' "${TEST_LOG_DIR}/sessions/${session_id}.json")
[[ "${session_perms}" == "600" ]] || fail "session.json should be 600, got ${session_perms}"
ok "session log written with 0600 perms"

info "clank peek session"
"${CLANK}" peek --session-id "${session_id}" --log-path "${TEST_LOG_DIR}" --last 5 | head -40
ok "peek session worked"

# --- A2A conversation ---
info "clank converse, write conversation log to ${TEST_LOG_DIR}"
export MOCK_MULTI_TURN=1
out=$( "${CLANK}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed "log persistence test" \
  --max-turns 4 \
  --per-turn-timeout 5 \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=false )
conv_id=$(json_field "${out}" conversation.conversation_id)

[[ -f "${TEST_LOG_DIR}/conversations/${conv_id}.json"  ]] || fail "missing conversation JSON"
[[ -f "${TEST_LOG_DIR}/conversations/${conv_id}.jsonl" ]] || fail "missing conversation turns JSONL"

conv_perms=$(stat -c '%a' "${TEST_LOG_DIR}/conversations/${conv_id}.json")
[[ "${conv_perms}" == "600" ]] || fail "conversation.json should be 600, got ${conv_perms}"
ok "conversation log written with 0600 perms"

# Count lines in jsonl
line_count=$(wc -l < "${TEST_LOG_DIR}/conversations/${conv_id}.jsonl")
[[ "${line_count}" == "4" ]] || fail "expected 4 turn lines in JSONL, got ${line_count}"
ok "JSONL contains ${line_count} turns"

verify "inspect ${TEST_LOG_DIR}/conversations/${conv_id}.jsonl by hand if you want to see the raw envelope/response shape"
