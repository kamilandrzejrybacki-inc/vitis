#!/usr/bin/env bash
# 01_doctor.sh — clank doctor: verify environment + provider availability
#
# What it tests:
#   - clank doctor exits cleanly
#   - reports a sane summary of provider availability
#
# Run: tests/manual/01_doctor.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"

header "01_doctor: clank doctor sanity"

CLANK="$(clank_bin)"

info "Running clank doctor"
if ! out=$( "${CLANK}" doctor 2>&1 ); then
  echo "${out}"
  fail "clank doctor exited non-zero"
fi
echo "${out}"

ok "clank doctor exited cleanly"
verify "human review: did doctor report which providers are installed and which are missing?"
