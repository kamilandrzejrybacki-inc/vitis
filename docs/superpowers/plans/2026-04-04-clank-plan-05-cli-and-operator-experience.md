# Plan 05: CLI And Operator Experience

- **Status**: Draft
- **Date**: 2026-04-04
- **Depends on**: Plan 01, Plan 04

---

## Objective

Expose Clank as an operator-friendly CLI that is explicit about risk, state, and failure modes.

This plan is about the human-facing contract of the tool.

---

## Target Directories

- `cmd/clank/`
- `internal/cli/`
- `internal/config/`

---

## Files To Create

- `cmd/clank/main.go`
- `internal/cli/run.go`
- `internal/cli/peek.go`
- `internal/cli/doctor.go`
- `internal/cli/render.go`
- `internal/config/config.go`

---

## Work Items

### 1. Implement `clank run`

Support:

- prompt input
- prompt file input
- working directory
- timeout
- file backend options
- debug raw capture

### 2. Implement `clank peek`

Return:

- session summary
- turn history
- recent warnings
- recent state notes

### 3. Add a lightweight `doctor` command

`doctor` should probe:

- whether the provider executable is installed
- whether the executable can start in the current environment
- whether the working directory is usable
- basic terminal assumptions

### 4. Make result output honest

The CLI should clearly surface:

- final status
- response if present
- blocked/auth/rate-limit reason if present
- extraction confidence
- exit code

### 5. Add explicit safety wording

Help text and README-facing language should state:

- local-first intended usage
- unsupported hosted/shared usage
- no automatic bypass of prompts

### 6. Keep diagnostics split from machine output

Use:

- stdout for machine-readable result
- stderr for progress, warnings, and diagnostics

---

## Tests

- bad flags fail with config error
- `run` emits a structured result for non-success states
- `peek` renders file-store sessions correctly
- `doctor` detects missing provider and reports it cleanly

---

## Done When

- a user can run Clank without reading source code to understand failure modes
- the CLI does not pretend blocked/auth/rate-limit states are runtime bugs
- the operator contract matches the underlying state model
