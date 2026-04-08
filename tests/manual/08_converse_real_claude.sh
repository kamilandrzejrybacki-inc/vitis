#!/usr/bin/env bash
# 08_converse_real_claude.sh — A2A with real Claude Code on both sides
#
# What it tests:
#   - vitis converse can drive a real claude session in interactive mode
#   - turn-end marker injection works against the real TUI
#   - sentinel termination works in a real conversation
#
# REQUIRES: a working `claude` install on PATH (or VITIS_CLAUDE_BINARY set
# to a working binary). SKIPS automatically if not available.
#
# This is a SLOW test (real LLM calls). Budget ~2-5 minutes per turn.
# Run with --max-turns 4 by default to keep cost bounded; override with
# VITIS_MANUAL_MAX_TURNS=N for longer runs.
#
# WARNING: this consumes Anthropic API credits / Pro session quota.
#
# Run: tests/manual/08_converse_real_claude.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "08_converse_real_claude: real claude↔claude conversation"

require_claude_code

VITIS="$(vitis_bin)"
MAX_TURNS="${VITIS_MANUAL_MAX_TURNS:-4}"

warn "this script makes REAL Claude API calls and will consume quota"
warn "press Ctrl-C in the next 5 seconds to abort"
sleep 5

info "vitis converse claude-code ↔ claude-code, max-turns ${MAX_TURNS}"
out=$( "${VITIS}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "You are a Go expert. Briefly explain what a context.Context is. End with <<END>>." \
  --seed-b "You are a curious Python developer learning Go. Ask clarifying questions about context.Context. End with <<END>> when satisfied." \
  --opener a \
  --max-turns "${MAX_TURNS}" \
  --per-turn-timeout 300 \
  --terminator sentinel \
  --sentinel "<<END>>" \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=true ) || {
    print_json "${out}" || true
    fail "vitis converse exited non-zero (check stderr above)"
  }

echo "${out}" | tail -100
status=$(json_field "${out}" conversation.status)
turns=$(json_field "${out}" conversation.turns_consumed)

case "${status}" in
  completed_sentinel)
    ok "real Claude conversation terminated via sentinel after ${turns} turns"
    ;;
  max_turns_hit)
    warn "real Claude conversation hit max-turns (${turns}) — sentinel may not have triggered, OR the model didn't follow instructions"
    ;;
  *)
    fail "unexpected conversation status=${status}"
    ;;
esac

verify "human review: read the streamed turns above and confirm the two Claude personas had a coherent multi-turn dialogue"
