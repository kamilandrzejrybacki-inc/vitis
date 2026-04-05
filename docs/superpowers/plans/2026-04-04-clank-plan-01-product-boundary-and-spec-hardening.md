# Plan 01: Product Boundary And Spec Hardening

- **Status**: Draft
- **Date**: 2026-04-04
- **Depends on**: existing spec and PTY review only
- **Blocks**: all implementation work

---

## Objective

Turn the current design from an optimistic PTY concept into an implementation-safe spec.

This plan does not write Go code. It updates the Clank design documents so that later implementation work has stable contracts.

---

## Key Decisions To Lock

1. Clank MVP is local-only and one-user-first.
2. Clank does not treat Claude consumer subscriptions as safe for hosted/shared brokering.
3. Silence is not equivalent to completion.
4. Raw PTY bytes are the source of truth.
5. The run-state model is richer than `completed|timeout|partial|error`.

---

## Files To Update

- `docs/superpowers/specs/2026-04-03-clank-agent-bridge-design.md`
- `docs/superpowers/specs/2026-04-03-clank-implementation-plan.md`
- optionally `RFC.md` if it is still intended to guide implementation

---

## Changes To Make

### 1. Add a product-boundary section

Document:

- local supported use
- unsupported hosted/shared use
- explicit non-goals for MVP

### 2. Replace the current status model

Move to a richer state set such as:

- `completed`
- `blocked_on_input`
- `permission_prompt`
- `auth_required`
- `rate_limited`
- `interrupted`
- `timeout`
- `partial`
- `crashed`
- `error`

### 3. Split observation from completion

Revise the adapter contract so the provider can classify states without forcing them to be terminal.

### 4. Revise the stream event schema

Document raw bytes as canonical and normalized text as derived.

### 5. Clarify isolation assumptions

Specify how `HOME`, `.claude`, environment variables, and working directory are handled in MVP.

### 6. De-prioritize Postgres in the MVP path

Adjust the plan so transcript fidelity, state detection, and fixtures land before DB support.

---

## Deliverables

1. Revised design spec
2. Revised implementation plan
3. Updated wording for unsupported deployment shapes
4. Stable status and transcript contracts for downstream plans

---

## Done When

- the design spec no longer implies silence means success
- the design spec no longer implies text-only transcript storage
- the implementation plan puts transcript fixtures ahead of DB work
- the MVP boundary is explicit enough that a future contributor cannot accidentally build a hosted consumer-account broker and still claim they followed spec
