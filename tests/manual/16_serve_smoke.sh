#!/usr/bin/env bash
# 16_serve_smoke.sh — smoke test for vitis serve HTTP endpoints
#
# What it tests:
#   - vitis serve starts successfully on a fixed high port
#   - GET /health returns 200 with {"status":"ok"}
#   - GET /api/v1/status returns 200 with {"status":"ok"}
#   - GET /api/v1/sessions returns 200 with {"success":true}
#   - GET /api/v1/conversations returns 200 with {"success":true}
#
# Run: tests/manual/16_serve_smoke.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "16_serve_smoke: vitis serve HTTP endpoint smoke test"

PORT=18765
BASE="http://localhost:${PORT}"
SERVER_PID=""

cleanup_server() {
  if [[ -n "${SERVER_PID}" ]]; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
}
trap cleanup_server EXIT

VITIS="$(vitis_bin)"

info "Starting vitis serve on port ${PORT} (log-path=${TEST_LOG_DIR})"
"${VITIS}" serve --port "${PORT}" --log-path "${TEST_LOG_DIR}" >"${TEST_LOG_DIR}/server.log" 2>&1 &
SERVER_PID=$!

# Poll until the server is accepting connections (up to 5s).
info "Waiting for server to become ready..."
READY=0
for i in $(seq 1 10); do
  if curl -sf "${BASE}/health" >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.5
done

if (( READY == 0 )); then
  info "Server log:"
  cat "${TEST_LOG_DIR}/server.log" >&2
  fail "server did not become ready within 5 seconds on port ${PORT}"
fi
ok "server is ready on port ${PORT}"

# check <url> <expected_substring>
check() {
  local url="$1"
  local expect="$2"
  local resp
  resp=$(curl -sf "${url}") || {
    fail "GET ${url} failed (unreachable or non-2xx)"
  }
  if echo "${resp}" | grep -q "${expect}"; then
    ok "GET ${url} contains '${expect}'"
  else
    fail "GET ${url} response missing '${expect}' — got: ${resp}"
  fi
}

check "${BASE}/health"                  '"status"'
check "${BASE}/api/v1/status"           '"status"'
check "${BASE}/api/v1/sessions"         '"success":true'
check "${BASE}/api/v1/conversations"    '"success":true'

header "All checks passed — PASS"
