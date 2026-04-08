#!/usr/bin/env bash
# 06_converse_asymmetric_seeds.sh — peer-a and peer-b receive different seeds
#
# What it tests:
#   - --seed-a and --seed-b are honored independently
#   - The first envelope to each peer contains their own seed
#   - --opener=b reverses the alternation
#
# Run: tests/manual/06_converse_asymmetric_seeds.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"
setup_tmp_logs

header "06_converse_asymmetric_seeds: --seed-a, --seed-b, --opener"

VITIS="$(vitis_bin)"
MOCK="$(mockagent_bin)"
export VITIS_CLAUDE_BINARY="${MOCK}"
export MOCK_RESPONSE="ok"
export MOCK_MULTI_TURN=1

info "vitis converse with asymmetric seeds, opener=b"
out=$( "${VITIS}" converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "you are the architect" \
  --seed-b "you are the security reviewer" \
  --opener b \
  --max-turns 4 \
  --per-turn-timeout 5 \
  --log-path "${TEST_LOG_DIR}" \
  --stream-turns=false )

assert_conv_status "${out}" "max_turns_hit"
seed_a=$(json_field "${out}" conversation.seed_a)
seed_b=$(json_field "${out}" conversation.seed_b)
opener=$(json_field "${out}" conversation.opener)

[[ "${seed_a}" == "you are the architect"        ]] || fail "seed_a not stored correctly: ${seed_a}"
[[ "${seed_b}" == "you are the security reviewer" ]] || fail "seed_b not stored correctly: ${seed_b}"
[[ "${opener}" == "b" ]]                              || fail "opener not stored correctly: ${opener}"
ok "asymmetric seeds + opener=b stored correctly"

verify "human review: in the conversation log, turn 1's envelope is delivered to peer-b and contains 'you are the security reviewer'; turn 2 alternates to peer-a"
