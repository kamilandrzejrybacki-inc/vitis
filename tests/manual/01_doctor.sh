#!/usr/bin/env bash
# 01_doctor.sh — vitis doctor: verify environment + provider availability
#
# What it tests:
#   - vitis doctor exits cleanly
#   - reports a sane summary of provider availability
#
# Run: tests/manual/01_doctor.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"

header "01_doctor: vitis doctor sanity"

VITIS="$(vitis_bin)"

info "Running vitis doctor"
if ! out=$( "${VITIS}" doctor 2>&1 ); then
  echo "${out}"
  fail "vitis doctor exited non-zero"
fi
echo "${out}"

ok "vitis doctor exited cleanly"
verify "human review: did doctor report which providers are installed and which are missing?"
