#!/usr/bin/env bash
# 07_converse_validation_errors.sh — exercise every CLI validation rejection path
#
# What it tests: vitis converse rejects each invalid flag combination with
# exit code 2 and a stderr error message that names the offending flag.
#
# Run: tests/manual/07_converse_validation_errors.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"

header "07_converse_validation_errors"

VITIS="$(vitis_bin)"

expect_validation_error() {
  local label="$1"; shift
  local expected_substring="$1"; shift
  printf '\n%s---%s %s\n' "$C_DIM" "$C_RESET" "${label}"
  set +e
  err=$( "${VITIS}" converse "$@" 2>&1 >/dev/null )
  code=$?
  set -e
  if [[ ${code} -ne 2 ]]; then
    fail "${label}: expected exit code 2, got ${code}"
  fi
  if ! grep -qF "${expected_substring}" <<< "${err}"; then
    echo "${err}"
    fail "${label}: stderr did not contain '${expected_substring}'"
  fi
  ok "${label}: exit=2, stderr contained '${expected_substring}'"
}

expect_validation_error "missing --peer-a"      "peer-a"     --seed hi
expect_validation_error "missing --seed"        "seed"       --peer-a provider:claude-code --peer-b provider:claude-code
expect_validation_error "--seed + --seed-a"     "exclusive"  --peer-a provider:claude-code --peer-b provider:claude-code --seed x --seed-a y --seed-b z
expect_validation_error "max-turns 0"           "max-turns"  --peer-a provider:claude-code --peer-b provider:claude-code --seed x --max-turns 0
expect_validation_error "max-turns 501"         "max-turns"  --peer-a provider:claude-code --peer-b provider:claude-code --seed x --max-turns 501
expect_validation_error "judge terminator unsupported" "judge" --peer-a provider:claude-code --peer-b provider:claude-code --seed x --terminator judge
expect_validation_error "bus nats unsupported"  "bus"        --peer-a provider:claude-code --peer-b provider:claude-code --seed x --bus nats://localhost:4222
expect_validation_error "log-backend db unsupported" "log-backend" --peer-a provider:claude-code --peer-b provider:claude-code --seed x --log-backend db
expect_validation_error "opener xxx"            "opener"     --peer-a provider:claude-code --peer-b provider:claude-code --seed x --opener xxx
expect_validation_error "per-turn-timeout 0"    "per-turn-timeout" --peer-a provider:claude-code --peer-b provider:claude-code --seed x --per-turn-timeout 0
expect_validation_error "per-turn-timeout 9999" "per-turn-timeout" --peer-a provider:claude-code --peer-b provider:claude-code --seed x --per-turn-timeout 9999

ok "all validation rejections matched"
