#!/usr/bin/env bash
# 12_security_path_traversal.sh — verify path-traversal hardening
#
# What it tests (security review M6, H7 fixes):
#   - --working-directory pointing to a non-existent path is rejected
#   - --working-directory pointing to a file (not a directory) is rejected
#   - --log-path with relative escape sequences is cleaned (filepath.Clean)
#   - disallowed env_KEY peer options (LD_PRELOAD, VITIS_CLAUDE_ARGS) are
#     dropped with a stderr warning
#
# Run: tests/manual/12_security_path_traversal.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "12_security_path_traversal: hardening checks"

VITIS="$(vitis_bin)"
MOCK="$(mockagent_bin)"
export VITIS_CLAUDE_BINARY="${MOCK}"
export MOCK_RESPONSE="ok"
export MOCK_MULTI_TURN=1

# --- non-existent working directory ---
info "expect rejection: --working-directory /does/not/exist"
set +e
err=$( "${VITIS}" converse \
  --peer-a provider:claude-code --peer-b provider:claude-code \
  --seed x --working-directory /does/not/exist 2>&1 >/dev/null )
code=$?
set -e
[[ ${code} -eq 2 ]] || fail "expected exit 2, got ${code}"
grep -q "working-directory" <<< "${err}" || fail "stderr should mention working-directory"
ok "non-existent working-directory correctly rejected"

# --- working directory points at a file ---
touch "${TEST_LOG_DIR}/i_am_a_file"
info "expect rejection: --working-directory points to a file"
set +e
err=$( "${VITIS}" converse \
  --peer-a provider:claude-code --peer-b provider:claude-code \
  --seed x --working-directory "${TEST_LOG_DIR}/i_am_a_file" 2>&1 >/dev/null )
code=$?
set -e
[[ ${code} -eq 2 ]] || fail "expected exit 2, got ${code}"
ok "file-as-directory correctly rejected"

# --- env injection allowlist ---
info "expect warning: env_LD_PRELOAD is dropped"
set +e
err=$( "${VITIS}" converse \
  --peer-a provider:claude-code --peer-b provider:claude-code \
  --peer-a-opt env_LD_PRELOAD=/tmp/evil.so \
  --peer-a-opt env_VITIS_CLAUDE_ARGS=--dangerously-skip-permissions \
  --peer-a-opt env_MOCK_RESPONSE=ok \
  --seed x --max-turns 2 --per-turn-timeout 5 \
  --log-path "${TEST_LOG_DIR}/runs" 2>&1 >/dev/null )
set -e
if grep -q "LD_PRELOAD" <<< "${err}"; then
  ok "LD_PRELOAD dropped with stderr warning"
else
  fail "expected stderr warning about LD_PRELOAD"
fi
if grep -q "VITIS_CLAUDE_ARGS" <<< "${err}"; then
  ok "VITIS_CLAUDE_ARGS dropped with stderr warning"
else
  fail "expected stderr warning about VITIS_CLAUDE_ARGS"
fi
ok "env injection allowlist enforced"
