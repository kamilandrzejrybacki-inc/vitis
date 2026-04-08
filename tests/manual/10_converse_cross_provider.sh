#!/usr/bin/env bash
# 10_converse_cross_provider.sh — A2A with claude-code ↔ codex (cross-provider)
#
# What it tests: the most ambitious case — two DIFFERENT real provider
# binaries talking to each other through clank as the broker. This is
# the canonical "A2A across model families" demo.
#
# REQUIRES: BOTH `claude` and `codex` available. SKIPS if either missing.
#
# WARNING: real Anthropic + OpenAI API calls. Significant cost.
#
# KNOWN LIMITATION: same as 09_converse_real_codex.sh — real codex
# multi-turn is fragile until Plan 2.5 sidecar support lands. claude-code
# side is reliable. The conversation may end early or hit max-turns
# depending on which side stalls.
#
# Run: tests/manual/10_converse_cross_provider.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "10_converse_cross_provider: claude-code ↔ codex"

require_claude_code
require_codex

CLANK="$(clank_bin)"
MAX_TURNS="${CLANK_MANUAL_MAX_TURNS:-4}"

warn "this script calls BOTH Anthropic AND OpenAI APIs — significant cost"
warn "press Ctrl-C in the next 7 seconds to abort"
sleep 7

info "clank converse claude-code (peer-a) ↔ codex (peer-b)"
out=$( "${CLANK}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:codex \
  --seed-a "You're a Go expert. Propose a one-line goroutine pool API. End your reply with <<END>>." \
  --seed-b "You're a critic. Find one flaw in any proposed Go API. End your reply with <<END>>." \
  --opener a \
  --max-turns "${MAX_TURNS}" \
  --per-turn-timeout 300 \
  --terminator sentinel \
  --sentinel "<<END>>" \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=true ) || {
    warn "clank converse exited non-zero — see streamed output for partial results"
  }

echo "${out}" | tail -120
verify "human review: did peer-a (claude) and peer-b (codex) exchange substantive turns? did the broker's strict alternation hold?"
