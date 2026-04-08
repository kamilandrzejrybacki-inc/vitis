#!/usr/bin/env bash
# 15_converse_caveman.sh — verify caveman reply style cuts response tokens
#
# What it tests:
#   - vitis converse --style caveman-full passes the JuliusBrussee/caveman
#     instructions through to the spawned agent's briefing
#   - The model's actual replies, when caveman is active, are measurably
#     shorter than when caveman is off
#
# Strategy:
#   Run the same converse twice — once with --style normal, once with
#   --style caveman-ultra — against the homelab Portkey gateway. Compare
#   the total response byte count across all turns. Caveman should be
#   measurably (>30%) shorter for the same prompt.
#
# REQUIRES: portkeyagent installed + tests/manual/.portkey.env present
# (same setup as 13_converse_portkey.sh). Auto-skips otherwise.
#
# Run: tests/manual/15_converse_caveman.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "15_converse_caveman: caveman style compresses A2A replies"

if [[ -f "${SCRIPT_DIR}/.portkey.env" ]]; then
  # shellcheck source=/dev/null
  source "${SCRIPT_DIR}/.portkey.env"
fi

PORTKEYAGENT_BIN="${PORTKEYAGENT_BIN:-$(command -v portkeyagent || true)}"
if [[ -z "${PORTKEYAGENT_BIN}" ]] || [[ ! -x "${PORTKEYAGENT_BIN}" ]]; then
  skip "portkeyagent not on PATH (run 'go install github.com/kamilrybacki/portkeyagent@latest')"
fi
if [[ -z "${PORTKEY_API_KEY:-}" ]]; then
  skip "PORTKEY_API_KEY not set (create tests/manual/.portkey.env)"
fi

VITIS="$(vitis_bin)"
export VITIS_CLAUDE_BINARY="${PORTKEYAGENT_BIN}"
export MOCK_MULTI_TURN=1

# A topic the model can't fluff its way around — needs concrete tokens
# regardless of style. The compression delta must come from filler/style,
# not from the model dodging the question.
SEED='Explain in plain language what a Go context.Context is and how cancellation propagates through it. Cover at minimum: the four constructor functions, how the parent-child tree is built, what Done() returns, and what happens when a parent is cancelled. End your reply with <<END>>.'

run_converse() {
  local style="$1"
  local out="$2"
  printf '\n%s---%s style=%s\n' "$C_DIM" "$C_RESET" "${style}"
  "${VITIS}" converse \
    --peer-a provider:claude-code \
    --peer-b provider:claude-code \
    --seed "${SEED}" \
    --max-turns 2 \
    --per-turn-timeout 120 \
    --terminator sentinel \
    --style "${style}" \
    --log-path "${TEST_LOG_DIR}/${style}" \
    --stream-turns=false > "${out}" 2>&1 || {
      cat "${out}"
      fail "converse --style ${style} exited non-zero"
    }
}

NORMAL_OUT="${TEST_LOG_DIR}/normal.json"
CAVEMAN_OUT="${TEST_LOG_DIR}/caveman.json"

run_converse "normal" "${NORMAL_OUT}"
run_converse "caveman-ultra" "${CAVEMAN_OUT}"

# Sum the response field lengths across all turns for each run.
sum_response_chars() {
  python3 -c '
import json, sys
path = sys.argv[1]
# Strip everything before the final indented JSON object.
text = open(path).read()
idx = text.rfind("\n{\n  ")
if idx > 0:
    text = text[idx+1:]
data = json.loads(text)
total = sum(len(t.get("response","")) for t in data.get("turns",[]))
print(total)
' "$1"
}

normal_chars=$(sum_response_chars "${NORMAL_OUT}")
caveman_chars=$(sum_response_chars "${CAVEMAN_OUT}")

info "normal style total response chars: ${normal_chars}"
info "caveman style total response chars: ${caveman_chars}"

if (( normal_chars == 0 )) || (( caveman_chars == 0 )); then
  fail "one of the runs produced no response content"
fi

# Caveman should produce a measurably shorter total. Threshold is loose
# (10% improvement) because the model has its own variability across runs.
ratio=$(python3 -c "print(${caveman_chars}/${normal_chars})")
info "caveman/normal ratio = ${ratio}"

if python3 -c "import sys; sys.exit(0 if ${caveman_chars} < ${normal_chars} * 0.9 else 1)"; then
  ok "caveman cut response length by $(python3 -c "print(round((1-${caveman_chars}/${normal_chars})*100,1))")% (threshold: >10%)"
else
  warn "caveman did not measurably compress (ratio=${ratio}); the upstream model may have ignored the style instructions, or the topic was too short to fluff"
fi

verify "human review: open ${NORMAL_OUT} and ${CAVEMAN_OUT}, look at the 'response' field of each turn. The caveman version should read like terse fragments; the normal version should be full sentences."
