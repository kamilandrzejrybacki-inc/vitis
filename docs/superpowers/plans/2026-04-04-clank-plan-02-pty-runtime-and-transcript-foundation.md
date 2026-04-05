# Plan 02: PTY Runtime And Transcript Foundation

- **Status**: Draft
- **Date**: 2026-04-04
- **Depends on**: Plan 01
- **Can overlap with**: Plan 03 once interfaces settle

---

## Objective

Build the PTY and transcript primitives that every provider and every later layer will rely on.

This plan is about process control and capture correctness, not Claude-specific parsing.

---

## Target Directories

- `cmd/clank/`
- `internal/terminal/`
- `internal/model/`
- `internal/util/`

---

## Files To Create

- `internal/model/events.go`
- `internal/model/status.go`
- `internal/terminal/runtime.go`
- `internal/terminal/process.go`
- `internal/terminal/transcript.go`
- `internal/terminal/ansi.go`
- `internal/terminal/runtime_test.go`
- `internal/terminal/process_test.go`
- `internal/terminal/transcript_test.go`

---

## Work Items

### 1. Implement PTY process lifecycle

Support:

- spawn
- write
- output stream
- exit notification
- graceful terminate
- forced kill fallback

### 2. Add terminal geometry as a real input

Carry cols and rows in the runtime interface and default them explicitly.

### 3. Build a raw-byte transcript buffer

Requirements:

- append raw chunks
- return full raw transcript
- return tail slices
- track byte count
- track last output timestamp

### 4. Add normalized-text derivation

Requirements:

- ANSI stripping
- control-sequence normalization
- preserve enough structure for downstream parsing
- do not mutate or replace the raw capture

### 5. Add transcript metadata helpers

Helpers should expose:

- total bytes seen
- idle duration
- elapsed duration
- whether any output has arrived
- recent normalized tail

### 6. Define hard failure semantics

The runtime should distinguish:

- spawn failure
- write failure
- clean exit
- signal exit
- context cancellation

---

## Tests

### Unit tests

- raw transcript buffer preserves bytes
- normalized transcript stripping does not alter raw storage
- tail reads work across ring-buffer wrap

### Process tests

- spawn a real process and capture output
- terminate politely
- force-kill after grace deadline
- handle large outputs without corruption

---

## Done When

- raw transcript bytes are preserved exactly
- normalized transcript text can be generated deterministically
- PTY lifecycle behavior is explicit and tested
- downstream adapter code can consume transcript state without needing to know process details
