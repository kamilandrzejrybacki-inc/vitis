#!/usr/bin/env bash
# 09_converse_real_codex.sh — A2A with real Codex CLI on both sides
#
# What it tests:
#   - vitis converse can drive a real codex session in interactive mode
#     (NOT `codex exec` which is one-shot — see the codex P1-1 fix)
#   - real codex emits the marker token correctly when instructed
#
# REQUIRES: a working `codex` install on PATH (or VITIS_CODEX_BINARY set).
# SKIPS automatically if not available.
#
# WARNING: this consumes OpenAI API credits. Real LLM calls. Slow.
#
# KNOWN LIMITATION: real codex multi-turn through the marker-injection
# approach is unreliable in this version of vitis — sidecar JSONL support
# (Plan 2.5 in the design spec) is the proper fix. This script may hit
# max-turns or timeout depending on whether codex emits the marker on its
# own line.
#
# Run: tests/manual/09_converse_real_codex.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "09_converse_real_codex: real codex↔codex conversation"

require_codex

VITIS="$(vitis_bin)"
MAX_TURNS="${VITIS_MANUAL_MAX_TURNS:-3}"

warn "this script makes REAL OpenAI API calls and will consume quota"
warn "real codex multi-turn is a KNOWN-FRAGILE area (see Plan 2.5)"
warn "press Ctrl-C in the next 5 seconds to abort"
sleep 5

info "vitis converse codex ↔ codex, max-turns ${MAX_TURNS}"
out=$( "${VITIS}" converse \
  --peer-a provider:codex \
  --peer-b provider:codex \
  --peer-a-opt model=gpt-5 \
  --peer-b-opt model=gpt-5 \
  --seed-a "Briefly suggest a Go function signature for a rate limiter. End with <<END>>." \
  --seed-b "Critique the proposed signature for thread-safety. End with <<END>> when done." \
  --opener a \
  --max-turns "${MAX_TURNS}" \
  --per-turn-timeout 300 \
  --terminator sentinel \
  --sentinel "<<END>>" \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=true ) || {
    print_json "${out}" || true
    warn "vitis converse exited non-zero (this may be expected — see KNOWN LIMITATION above)"
  }

echo "${out}" | tail -100
status=$(json_field "${out}" conversation.status 2>/dev/null || echo "unknown")
turns=$(json_field "${out}" conversation.turns_consumed 2>/dev/null || echo "?")

case "${status}" in
  completed_sentinel)
    ok "real codex conversation terminated via sentinel after ${turns} turns"
    ;;
  max_turns_hit)
    warn "real codex hit max-turns (${turns}) — likely the marker-injection approach didn't fire reliably; expected until Plan 2.5"
    ;;
  peer_crashed|error)
    warn "real codex run failed with status=${status} — capture stderr/stdout for the Plan 2.5 design notes"
    ;;
  *)
    warn "unexpected conversation status=${status}"
    ;;
esac

verify "human review: even if status != completed_sentinel, the streamed turns should show real model-generated content (not echoed envelopes or empty replies)"
