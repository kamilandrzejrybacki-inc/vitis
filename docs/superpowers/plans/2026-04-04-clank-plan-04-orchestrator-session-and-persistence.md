# Plan 04: Orchestrator, Session Model, And Persistence

- **Status**: Draft
- **Date**: 2026-04-04
- **Depends on**: Plan 01, Plan 02, Plan 03

---

## Objective

Turn the PTY runtime and provider adapter into a coherent execution engine with durable results.

This plan defines how runs become sessions, how statuses are finalized, and how data is stored.

---

## Target Directories

- `internal/orchestrator/`
- `internal/model/`
- `internal/store/`
- `internal/store/file/`
- `internal/store/postgres/`

---

## Files To Create

- `internal/model/session.go`
- `internal/model/result.go`
- `internal/model/errors.go`
- `internal/orchestrator/orchestrator.go`
- `internal/orchestrator/completion_loop.go`
- `internal/orchestrator/dependencies.go`
- `internal/store/store.go`
- `internal/store/file/file_store.go`
- `internal/store/file/file_store_test.go`

Optional after file store is stable:

- `internal/store/postgres/postgres_store.go`
- `internal/store/postgres/postgres_store_test.go`
- `internal/store/postgres/migrations/001_init.sql`

---

## Work Items

### 1. Finalize the session schema

Include:

- session ID
- provider
- run status
- timestamps
- exit code
- parser confidence
- observation confidence
- auth mode
- bytes captured
- blocked reason
- warnings

### 2. Implement the completion loop

The orchestrator should:

- stream output into transcript storage
- poll the adapter observer
- stop only when a terminal state is reached or context expires
- keep non-terminal blocked/auth states visible in diagnostics

### 3. Define store contracts around raw and derived data

Store:

- session summary
- user turn
- assistant turn if extracted
- raw PTY events if enabled
- warnings and notes

### 4. Ship the file store first

Requirements:

- append-only raw event log
- JSON session summary
- JSONL turn log
- safe updates via temp file plus rename

### 5. Add Postgres only after file store is correct

If included in this phase, use:

- binary-safe raw transcript storage
- schema that does not force text-only chunks

### 6. Make error and partial-state handling explicit

Define result behavior for:

- spawn failure
- prompt write failure
- timeout
- crash after partial output
- blocked_on_input
- auth_required
- rate_limited

---

## Tests

- happy path from spawn to extracted response
- blocked prompt results in non-success state
- timeout returns best-effort result without lying about success
- file store round-trip reproduces session and turns
- store failure does not silently corrupt runtime result

---

## Done When

- Clank can execute one end-to-end local run through the orchestrator
- the result model can represent real PTY states honestly
- file persistence is reliable enough to debug failures
- Postgres is either implemented safely or deliberately deferred
