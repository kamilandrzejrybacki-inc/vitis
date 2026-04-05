# Plan 03: Claude Code State Detection And Extraction

- **Status**: Draft
- **Date**: 2026-04-04
- **Depends on**: Plan 01, Plan 02

---

## Objective

Implement a Claude Code provider adapter that can:

- classify PTY session state conservatively
- distinguish success from blocked/auth/rate-limit states
- extract the final response with confidence scoring

This is the plan that makes Clank useful for the actual MVP provider.

---

## Target Directories

- `internal/adapter/`
- `internal/adapter/claude_code/`
- `internal/adapter/claude_code/testdata/`

---

## Files To Create

- `internal/adapter/adapter.go`
- `internal/adapter/registry.go`
- `internal/adapter/claude_code/adapter.go`
- `internal/adapter/claude_code/observe.go`
- `internal/adapter/claude_code/extract.go`
- `internal/adapter/claude_code/patterns.go`
- `internal/adapter/claude_code/observe_test.go`
- `internal/adapter/claude_code/extract_test.go`

---

## Work Items

### 1. Implement provider spawn spec

Define:

- executable name
- args
- default env handling
- prompt formatting behavior

### 2. Implement transcript observation

The observer should classify:

- clean completion
- blocked interactive prompt
- likely permission prompt
- auth required
- rate-limited or usage exhausted
- interrupted
- crash or abnormal exit

### 3. Port conservative blocked-prompt heuristics

Model them after the PTY-side prompt-stall handling seen in the Claude Code review:

- inspect only the recent tail
- use prompt-like last-line patterns
- do not conflate blocked state with completion

### 4. Implement response extraction

Use layered extraction:

1. structured final assistant block detection
2. last plausible assistant block fallback
3. last non-empty output block fallback

### 5. Emit confidence and notes

Extraction should return:

- `response`
- parser confidence
- notes about ambiguity

Observation should return:

- status
- terminal/non-terminal flag
- confidence
- reason
- evidence tags

### 6. Build a real transcript corpus

Create `testdata/` fixtures for:

- happy path
- ANSI-heavy output
- blocked interactive prompt
- auth-required screen
- rate-limit surface
- partial/crash cases

---

## Tests

- observation classifies each transcript fixture correctly
- extraction returns the expected final response on happy path
- extraction avoids swallowing prompt text as assistant output
- ANSI-heavy transcripts normalize cleanly

---

## Done When

- the Claude adapter can explain why it believes a run completed or did not
- blocked/auth/rate-limit states are not misreported as success
- extraction quality is grounded in captured transcripts, not just a mock harness
