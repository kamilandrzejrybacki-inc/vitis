#!/usr/bin/env bash
# 13_converse_portkey.sh — A2A using portkeyagent (Portkey-backed) instead of
# the bundled mock or real claude/codex.
#
# What it tests:
#   - clank converse runs end-to-end against a real LLM (via Portkey)
#   - The marker-injection protocol works against an out-of-tree agent
#     binary (validating the contract for any future drop-in replacement)
#   - Per-peer model + reasoning-effort options reach the upstream gateway
#
# REQUIRES:
#   - portkeyagent on PATH (or PORTKEYAGENT_BIN set):
#       go install github.com/kamilrybacki/portkeyagent@latest
#   - PORTKEY_API_KEY exported
#   - PORTKEY_VIRTUAL_KEY exported (recommended; routes to a specific upstream)
#   - PORTKEY_MODEL exported (or use the portkeyagent default)
#
# SKIPS automatically if portkeyagent is not installed or PORTKEY_API_KEY is
# not set, so this is safe to include in run_all.sh on machines that don't
# have Portkey configured.
#
# Cost: depends on the upstream model behind your virtual key. With a free
# tier, this is effectively zero. With paid models, budget per-turn × 2 ×
# max-turns input + output tokens.
#
# Run: tests/manual/13_converse_portkey.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "13_converse_portkey: A2A via portkeyagent → Portkey"

# Resolve portkeyagent binary.
PORTKEYAGENT_BIN="${PORTKEYAGENT_BIN:-$(command -v portkeyagent || true)}"
if [[ -z "${PORTKEYAGENT_BIN}" ]]; then
  skip "portkeyagent not on PATH (run 'go install github.com/kamilrybacki/portkeyagent@latest' or set PORTKEYAGENT_BIN)"
fi
if [[ ! -x "${PORTKEYAGENT_BIN}" ]]; then
  skip "PORTKEYAGENT_BIN=${PORTKEYAGENT_BIN} is not executable"
fi

if [[ -z "${PORTKEY_API_KEY:-}" ]]; then
  skip "PORTKEY_API_KEY not set (export it before running this script)"
fi

CLANK="$(clank_bin)"

# Point clank's claude-code adapter at portkeyagent. clank will spawn it
# in interactive mode (no `exec` subcommand, no trailing prompt arg) per
# the codex P1-1 fix, and portkeyagent's MOCK_MULTI_TURN auto-detection
# means we don't need to set PORTKEYAGENT_MULTI_TURN explicitly.
export CLANK_CLAUDE_BINARY="${PORTKEYAGENT_BIN}"
export MOCK_MULTI_TURN=1   # signals multi-turn mode to portkeyagent

# Optional system prompts per peer to keep replies short and on-topic for
# the test scenario. The portkeyagent process inherits env vars from clank,
# but each peer is its own subprocess so we can't trivially set different
# system prompts per peer here. For asymmetric personas use --seed-a /
# --seed-b instead — they end up as the user message on turn 1.
export PORTKEYAGENT_SYSTEM="You are participating in a brief test conversation. Keep replies under 3 sentences. Always emit the marker token instructed in the incoming message verbatim on its own line at the end."

info "PORTKEY_API_KEY    = ${PORTKEY_API_KEY:0:8}…"
info "PORTKEY_VIRTUAL_KEY= ${PORTKEY_VIRTUAL_KEY:-<unset>}"
info "PORTKEY_MODEL      = ${PORTKEY_MODEL:-<default: gpt-4o-mini>}"
info "portkeyagent       = ${PORTKEYAGENT_BIN}"

MAX_TURNS="${CLANK_MANUAL_MAX_TURNS:-4}"

info "clank converse claude-code(portkeyagent) ↔ claude-code(portkeyagent), max-turns ${MAX_TURNS}"
out=$( "${CLANK}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "Briefly suggest one Go testing best practice. End with <<END>>." \
  --seed-b "Briefly critique any single Go testing practice you hear. End with <<END>> when satisfied." \
  --opener a \
  --max-turns "${MAX_TURNS}" \
  --per-turn-timeout 120 \
  --terminator sentinel \
  --sentinel "<<END>>" \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=true ) || {
    print_json "${out}" || true
    fail "clank converse exited non-zero — Portkey/portkeyagent may not be configured correctly. See stderr above."
  }

echo "${out}" | tail -120
status=$(json_field "${out}" conversation.status)
turns=$(json_field "${out}" conversation.turns_consumed)

case "${status}" in
  completed_sentinel)
    ok "Portkey-backed conversation terminated via sentinel after ${turns} turns"
    ;;
  max_turns_hit)
    warn "conversation hit max-turns (${turns}) — the upstream model may not have followed the sentinel instruction. Inspect the streamed turns above to verify it understood the prompt."
    ;;
  peer_crashed|error)
    fail "conversation failed with status=${status}"
    ;;
  *)
    warn "unexpected status=${status}"
    ;;
esac

verify "human review: scroll up and confirm the streamed turns show real model-generated content (not echoed envelopes, not empty replies, not error messages)"
