# Vitis Plan Set Index

- **Status**: Draft
- **Date**: 2026-04-04
- **Scope**: executable plan set for implementing Vitis in the local repo
- **Primary inputs**:
  - `../specs/2026-04-03-vitis-agent-bridge-design.md`
  - `../specs/2026-04-03-vitis-implementation-plan.md`
  - `../specs/2026-04-03-vitis-pty-design-review.md`

---

## Purpose

This directory breaks the Vitis implementation into a set of workstreams that can be executed in order or partially in parallel.

The plans assume:

- Go remains the implementation language
- PTY remains the primary integration surface
- Claude Code remains the MVP provider
- local, one-user, local-machine execution is the supported MVP boundary

---

## Plan Set

1. `2026-04-04-vitis-plan-01-product-boundary-and-spec-hardening.md`
2. `2026-04-04-vitis-plan-02-pty-runtime-and-transcript-foundation.md`
3. `2026-04-04-vitis-plan-03-claude-code-state-detection-and-extraction.md`
4. `2026-04-04-vitis-plan-04-orchestrator-session-and-persistence.md`
5. `2026-04-04-vitis-plan-05-cli-and-operator-experience.md`
6. `2026-04-04-vitis-plan-06-test-fixtures-and-acceptance.md`

---

## Recommended Execution Order

### Stage A: Lock the shape of the product

Run first:

1. Plan 01

Reason:

- it prevents implementation churn
- it decides status model, unsupported deployment shapes, and storage rules before code hardens

### Stage B: Build the PTY foundation

Run next:

2. Plan 02
3. Plan 03

Reason:

- transcript fidelity and terminal-state detection are the hardest PTY problems
- everything downstream depends on these primitives

These two plans can overlap once the transcript primitives are stable.

### Stage C: Wire the system together

Run next:

4. Plan 04
5. Plan 05

Reason:

- these plans turn the runtime into an actual tool
- they surface the state machine to users and persistence layers

These can overlap after Plan 04 defines the core result model.

### Stage D: Prove it works against reality

Run last:

6. Plan 06

Reason:

- Vitis is only useful if it handles real Claude Code PTY behavior, not just mocks

---

## Parallelization Guidance

Safe parallel pairs:

- Plan 02 and Plan 03 after transcript interfaces are agreed
- Plan 04 and Plan 05 after result/status contracts are frozen

Not safe to parallelize early:

- changing the run-state model while adapter and orchestrator code are already in progress
- adding Postgres before raw-byte transcript storage is decided

---

## Non-Goals For This Plan Set

- hosted multi-tenant PTY serving
- browser-facing PTY sessions
- consumer Claude-account brokering
- automatic confirmation of permission prompts
- replacing PTY with SDK/headless integration

---

## Success Criteria For The Whole Set

The plan set is complete when all of the following are true:

- Vitis can run a real Claude Code session locally through PTY
- Vitis distinguishes completion from blocked/auth/rate-limit states
- raw PTY bytes are preserved without lossy assumptions
- the CLI returns structured results with meaningful status and confidence
- the file backend is reliable
- the transcript corpus contains real captured Claude Code examples for the main edge cases
- the docs clearly state supported and unsupported deployment shapes
