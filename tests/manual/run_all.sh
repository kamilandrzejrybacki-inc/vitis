#!/usr/bin/env bash
# run_all.sh — run the entire manual test suite
#
# Modes:
#   tests/manual/run_all.sh             # all tests, including real-provider tests if available
#   tests/manual/run_all.sh --quick     # mock-only, fast (<30s total)
#   tests/manual/run_all.sh --no-real   # skip real-provider tests even if available
#   tests/manual/run_all.sh --only NN   # run a specific test by number prefix
#
# Each test is independent and may fail without aborting the suite.
# A summary is printed at the end with PASS/FAIL counts.
set -uo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"

QUICK=0
NO_REAL=0
ONLY=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --quick)   QUICK=1; shift ;;
    --no-real) NO_REAL=1; shift ;;
    --only)    ONLY="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,15p' "$0"
      exit 0
      ;;
    *) fail "unknown flag: $1" ;;
  esac
done

# Quick mode = no real providers + skip slow scripts
if [[ ${QUICK} -eq 1 ]]; then
  NO_REAL=1
fi

# Test catalog. Each entry: "<id> <script> <real|mock>"
# real = needs claude/codex; will be filtered when --no-real or absent.
TESTS=(
  "01 01_doctor.sh                       mock"
  "02 02_run_mock_happy.sh               mock"
  "03 03_run_mock_modes.sh               mock"
  "04 04_converse_mock_max_turns.sh      mock"
  "05 05_converse_mock_sentinel.sh       mock"
  "06 06_converse_asymmetric_seeds.sh    mock"
  "07 07_converse_validation_errors.sh   mock"
  "08 08_converse_real_claude.sh         real"
  "09 09_converse_real_codex.sh          real"
  "10 10_converse_cross_provider.sh      real"
  "11 11_logs_and_peek.sh                mock"
  "12 12_security_path_traversal.sh      mock"
  "13 13_converse_portkey.sh             real"
)

PASS=()
FAIL=()
SKIPPED=()

for entry in "${TESTS[@]}"; do
  read -r id script flavour <<< "${entry}"

  if [[ -n "${ONLY}" && "${id}" != "${ONLY}" ]]; then
    continue
  fi
  if [[ "${flavour}" == "real" && ${NO_REAL} -eq 1 ]]; then
    SKIPPED+=("${id}: ${script} (real-provider, skipped)")
    continue
  fi

  printf '\n%s########## %s ##########%s\n' "$C_BOLD" "${id}: ${script}" "$C_RESET"
  if "${SCRIPT_DIR}/${script}"; then
    PASS+=("${id}: ${script}")
  else
    FAIL+=("${id}: ${script}")
  fi
done

# Summary
printf '\n%s================ summary ================%s\n' "$C_BOLD" "$C_RESET"
printf '%sPASS%s %d\n' "$C_GREEN" "$C_RESET" "${#PASS[@]}"
for t in "${PASS[@]}";    do printf '  + %s\n' "$t"; done
printf '%sFAIL%s %d\n' "$C_RED" "$C_RESET" "${#FAIL[@]}"
for t in "${FAIL[@]}";    do printf '  - %s\n' "$t"; done
printf '%sSKIPPED%s %d\n' "$C_DIM" "$C_RESET" "${#SKIPPED[@]}"
for t in "${SKIPPED[@]}"; do printf '  ~ %s\n' "$t"; done

if [[ ${#FAIL[@]} -gt 0 ]]; then
  exit 1
fi
