#!/usr/bin/env bash
# 14_rtk_integration.sh — verify rtk integration with vitis-spawned agents
#
# What it tests:
#   - rtk binary is on PATH
#   - vitis doctor reports rtk as available for both claude-code and codex
#   - vitis doctor reports the rtk hook as installed for at least one
#     provider (a hard requirement before A2A runs benefit from rtk)
#
# This script does NOT actually run a conversation through rtk — vitis
# itself doesn't execute commands, so rtk is invisible to the broker. The
# integration is verifiable only by inspecting the spawned agent's hook
# config, which is exactly what vitis doctor does.
#
# To get end-to-end verification with real LLM token compression you would
# need to run script 08 / 13 against a real provider and inspect the
# spawned agent's tool-call traffic. That's outside the scope of this
# script.
#
# SKIPS automatically if rtk is not installed (run setup_rtk.sh first).
#
# Run: tests/manual/14_rtk_integration.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"

header "14_rtk_integration: rtk detection + hook check"

if ! command -v rtk >/dev/null 2>&1; then
  skip "rtk binary not found on PATH (run tests/manual/setup_rtk.sh first)"
fi
ok "rtk on PATH: $(command -v rtk) ($(rtk --version 2>/dev/null | head -1))"

VITIS="$(vitis_bin)"

check_provider() {
  local provider="$1"
  printf '\n%s---%s vitis doctor --provider %s\n' "$C_DIM" "$C_RESET" "${provider}"

  set +e
  out=$( "${VITIS}" doctor --provider "${provider}" 2>&1 )
  set -e
  echo "${out}"

  available=$(echo "${out}" | python3 -c '
import json, sys
try:
    print(json.loads(sys.stdin.read()).get("rtk",{}).get("available",False))
except Exception:
    print("False")
' 2>/dev/null)

  hook=$(echo "${out}" | python3 -c '
import json, sys
try:
    print(json.loads(sys.stdin.read()).get("rtk",{}).get("hook_installed",False))
except Exception:
    print("False")
' 2>/dev/null)

  if [[ "${available}" != "True" ]]; then
    fail "${provider}: rtk should be available (binary is on PATH but doctor says no)"
  fi
  ok "${provider}: rtk.available=true"

  if [[ "${hook}" == "True" ]]; then
    ok "${provider}: rtk hook is installed and active"
    HOOK_FOUND=1
  else
    warn "${provider}: rtk hook NOT installed — run 'tests/manual/setup_rtk.sh' or 'rtk init -g' (codex needs --codex)"
  fi
}

HOOK_FOUND=0
check_provider "claude-code"
check_provider "codex"

if (( HOOK_FOUND == 0 )); then
  fail "rtk is installed but no provider has the rtk hook active. Run tests/manual/setup_rtk.sh"
fi

verify "for end-to-end verification: run a real provider conversation (08 or 13) and inspect the spawned agent's tool calls — they should show rtk-rewritten outputs (compact, deduped)"
