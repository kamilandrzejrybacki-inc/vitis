#!/usr/bin/env bash
# tests/manual/lib/common.sh
#
# Shared helpers for clank manual test scripts. Source this from every
# test script:
#
#   #!/usr/bin/env bash
#   set -euo pipefail
#   SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
#   source "${SCRIPT_DIR}/lib/common.sh"
#
# Provides:
#   - color helpers (header, info, ok, warn, fail, verify)
#   - clank binary resolution (build if missing)
#   - mock-agent build helper
#   - JSON status extraction
#   - usage gating against real providers (skip if not installed)

# ---- color output ------------------------------------------------------------

if [[ -t 1 ]]; then
  C_RED=$'\033[31m'
  C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'
  C_BLUE=$'\033[34m'
  C_MAGENTA=$'\033[35m'
  C_CYAN=$'\033[36m'
  C_BOLD=$'\033[1m'
  C_DIM=$'\033[2m'
  C_RESET=$'\033[0m'
else
  C_RED= C_GREEN= C_YELLOW= C_BLUE= C_MAGENTA= C_CYAN= C_BOLD= C_DIM= C_RESET=
fi

header() { printf '\n%s=== %s ===%s\n' "$C_BOLD$C_CYAN" "$*" "$C_RESET"; }
info()   { printf '%s>>%s %s\n' "$C_BLUE" "$C_RESET" "$*"; }
ok()     { printf '%sPASS%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn()   { printf '%sWARN%s %s\n' "$C_YELLOW" "$C_RESET" "$*"; }
fail()   { printf '%sFAIL%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }
verify() { printf '%sVERIFY%s %s\n' "$C_MAGENTA" "$C_RESET" "$*"; }
skip()   { printf '%sSKIP%s %s\n' "$C_DIM" "$C_RESET" "$*"; exit 0; }

# ---- repo + binary discovery -------------------------------------------------

# Resolve REPO_ROOT regardless of where the script is invoked from.
if [[ -z "${REPO_ROOT:-}" ]]; then
  REPO_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../.." && pwd )"
fi
export REPO_ROOT

# Build directory for test artifacts. Cleaned by `make manual-clean`.
MANUAL_BUILD_DIR="${MANUAL_BUILD_DIR:-${REPO_ROOT}/tests/manual/.build}"
mkdir -p "${MANUAL_BUILD_DIR}"
export MANUAL_BUILD_DIR

# Build clank binary on demand. Always invokes `go build` because Go's build
# cache makes the no-op case essentially free, and an mtime check on a single
# source file misses changes anywhere else in the dependency graph (we have
# been bitten by stale binaries comparing only cmd/clank/main.go).
clank_bin() {
  local bin="${MANUAL_BUILD_DIR}/clank"
  ( cd "${REPO_ROOT}" && go build -o "${bin}" ./cmd/clank ) || fail "go build clank failed"
  echo "${bin}"
}

# Build the mock agent binary on demand. Same rationale as clank_bin.
mockagent_bin() {
  local bin="${MANUAL_BUILD_DIR}/mockagent"
  ( cd "${REPO_ROOT}" && go build -o "${bin}" ./internal/testutil/mockagent ) || fail "go build mockagent failed"
  echo "${bin}"
}

# ---- provider availability gating -------------------------------------------

# Returns 0 if a real provider binary is available on PATH (or via env override).
have_claude_code() {
  if [[ -n "${CLANK_CLAUDE_BINARY:-}" ]]; then
    [[ -x "${CLANK_CLAUDE_BINARY}" ]] && return 0 || return 1
  fi
  command -v claude >/dev/null 2>&1
}

have_codex() {
  if [[ -n "${CLANK_CODEX_BINARY:-}" ]]; then
    [[ -x "${CLANK_CODEX_BINARY}" ]] && return 0 || return 1
  fi
  command -v codex >/dev/null 2>&1
}

# Skip the rest of the current script unless `claude` is available.
require_claude_code() {
  if ! have_claude_code; then
    skip "claude binary not found on PATH (set CLANK_CLAUDE_BINARY=/path/to/claude or install Claude Code)"
  fi
}

require_codex() {
  if ! have_codex; then
    skip "codex binary not found on PATH (set CLANK_CODEX_BINARY=/path/to/codex or install Codex CLI)"
  fi
}

# ---- JSON helpers ------------------------------------------------------------

# json_field <json_string> <field_path>
# Extracts a top-level field from a JSON document. Uses python3 because jq
# is not always installed; falls back to grep if python3 is unavailable.
#
# Example: status=$(json_field "$out" status)
json_field() {
  local json="$1"
  local field="$2"
  if command -v python3 >/dev/null 2>&1; then
    python3 -c '
import json, sys
data = json.loads(sys.stdin.read())
parts = sys.argv[1].split(".")
for p in parts:
    if isinstance(data, list):
        data = data[int(p)]
    else:
        data = data.get(p)
    if data is None:
        break
print(data if data is not None else "")
' "${field}" <<< "${json}"
  else
    # Crude fallback — only works for top-level string fields.
    grep -oE "\"${field}\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" <<< "${json}" \
      | sed -E "s/.*: *\"([^\"]*)\".*/\1/" \
      | head -1
  fi
}

# assert_status <json> <expected_status>
assert_status() {
  local json="$1"
  local expected="$2"
  local actual
  actual="$(json_field "${json}" status)"
  if [[ "${actual}" != "${expected}" ]]; then
    fail "expected status=${expected}, got status=${actual}"
  fi
  ok "status=${actual}"
}

# assert_conv_status <json> <expected_status>
assert_conv_status() {
  local json="$1"
  local expected="$2"
  local actual
  actual="$(json_field "${json}" conversation.status)"
  if [[ "${actual}" != "${expected}" ]]; then
    fail "expected conversation.status=${expected}, got ${actual}"
  fi
  ok "conversation.status=${actual}"
}

# assert_nonempty_response <json>
assert_nonempty_response() {
  local json="$1"
  local resp
  resp="$(json_field "${json}" response)"
  if [[ -z "${resp}" ]]; then
    fail "response field is empty"
  fi
  ok "response is non-empty (${#resp} chars)"
}

# pretty-print JSON to stderr for human inspection
print_json() {
  if command -v python3 >/dev/null 2>&1; then
    echo "$1" | python3 -m json.tool >&2 2>/dev/null || echo "$1" >&2
  else
    echo "$1" >&2
  fi
}

# ---- temp dir cleanup --------------------------------------------------------

setup_tmp_logs() {
  TEST_LOG_DIR="$(mktemp -d -t clank-manual-XXXXXX)"
  trap 'rm -rf "${TEST_LOG_DIR}"' EXIT
  export TEST_LOG_DIR
}
