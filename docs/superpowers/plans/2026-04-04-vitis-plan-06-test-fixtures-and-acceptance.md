# Plan 06: Test Fixtures And Acceptance Harness

- **Status**: Draft
- **Date**: 2026-04-04
- **Depends on**: Plans 02 through 05

---

## Objective

Prove that Vitis works against real PTY behavior rather than a simplified toy model.

This plan is where confidence is earned.

---

## Target Directories

- `internal/testutil/`
- `internal/adapter/claude_code/testdata/`
- `tests/integration/`
- `tests/acceptance/`

---

## Files To Create

- `internal/testutil/mockagent/main.go`
- `tests/integration/run_integration_test.go`
- `tests/acceptance/README.md`
- `tests/acceptance/transcript-capture.md`
- `tests/acceptance/cases.md`

---

## Work Items

### 1. Keep the mock provider, but demote it

The mock provider is still useful for:

- deterministic lifecycle tests
- failure injection
- CLI contract testing

But it should not be treated as sufficient evidence that the Claude adapter is correct.

### 2. Build a captured transcript corpus

Collect and store real Claude Code PTY transcripts for:

- successful simple response
- blocked interactive prompt
- permission prompt
- auth prompt or login-required state
- usage/rate-limit state
- ANSI-heavy or noisy output
- partial output before crash

### 3. Add transcript-driven adapter tests

These should run without spawning Claude Code and validate:

- state detection
- extraction
- confidence and notes

### 4. Add end-to-end integration tests

Use the mock provider to validate:

- result JSON schema
- file store behavior
- timeout behavior
- interruption behavior

### 5. Add manual acceptance procedure

Document:

- exact commands to run against a local Claude Code install
- how to capture transcript fixtures
- how to redact or sanitize captures
- how to decide whether a new transcript belongs in the corpus

### 6. Add regression gates

Before release, require:

- transcript fixture tests green
- CLI integration tests green
- manual happy-path and blocked-prompt checks against a real local Claude Code install

---

## Acceptance Matrix

At minimum, acceptance must cover:

- `completed`
- `blocked_on_input`
- `auth_required`
- `rate_limited`
- `timeout`
- `partial`

---

## Done When

- Vitis behavior is validated against real Claude Code transcript examples
- the corpus is large enough to catch common parser and detector regressions
- manual acceptance is documented well enough that another engineer can reproduce it without tribal knowledge
