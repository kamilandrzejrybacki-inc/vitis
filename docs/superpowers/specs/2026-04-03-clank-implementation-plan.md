# Clank Agent Bridge - Implementation Plan

**Design Spec**: `docs/superpowers/specs/2026-04-03-clank-agent-bridge-design.md`
**Date**: 2026-04-03

---

## Phase 1: Project Scaffold + Core Types

### Step 1.1: Initialize Go module

- `go mod init github.com/kamilandrzejrybacki-inc/clank`
- Create directory structure under `cmd/clank/` and `internal/`
- Add `.gitignore` for Go binaries

### Step 1.2: Define model types

**Files:**
- `internal/model/status.go` — `RunStatus` type and constants (`completed`, `timeout`, `error`, `partial`)
- `internal/model/session.go` — `Session`, `SessionPatch`
- `internal/model/turn.go` — `Turn`
- `internal/model/result.go` — `RunRequest`, `RunResult`, `ResultMeta`
- `internal/model/errors.go` — `ErrorCode` constants, `RunError` struct
- `internal/model/events.go` — `StreamEvent`, `StoredStreamEvent`, `ExitResult`

### Step 1.3: Define interfaces

**Files:**
- `internal/adapter/adapter.go` — `Adapter` interface, `SpawnSpec`, `CompletionSignal`, `ExtractionResult`, `CompletionContext`, `AdapterRegistry`
- `internal/terminal/runtime.go` — `PseudoTerminalRuntime` interface, `PseudoTerminalProcess` interface
- `internal/store/store.go` — `Store` interface

**Done when:** All types compile. No implementation yet.

---

## Phase 2: PTY Runtime

### Step 2.1: StreamBuffer

**File:** `internal/terminal/buffer.go`

- Accumulates `StreamEvent` entries
- `Append(event StreamEvent)` — thread-safe append
- `Bytes() []byte` — full transcript
- `Tail(numberOfBytes int) []byte` — last N bytes
- `LastChunkTimestamp() time.Time` — for idle detection
- `ExitCode() *int` — set when process exits
- Protected by `sync.Mutex`

**Tests:** `internal/terminal/buffer_test.go`
- Append and read back
- Tail returns correct slice
- Thread-safe concurrent writes

### Step 2.2: PseudoTerminalProcess implementation

**File:** `internal/terminal/process.go`

- Wraps `os.File` (PTY master) and `*exec.Cmd`
- `Write(data []byte)` — writes to PTY master
- `Output() <-chan StreamEvent` — goroutine reads from PTY master, timestamps chunks, sends to channel
- `Done() <-chan ExitResult` — goroutine waits on `cmd.Wait()`, sends exit result
- `Terminate(gracePeriod time.Duration)` — sends SIGINT, waits grace period, sends SIGKILL if still alive

**Tests:** `internal/terminal/process_test.go`
- Spawn `/bin/echo hello`, read output, verify exit code 0
- Spawn a process that ignores SIGINT, verify SIGKILL after grace period
- Write to stdin, verify process receives it

### Step 2.3: PseudoTerminalRuntime implementation

**File:** `internal/terminal/runtime.go` (add implementation below interface)

- Uses `creack/pty` to allocate PTY and start command
- Sets terminal dimensions (default 80x24)
- Returns `PseudoTerminalProcess`

**Dependencies:** `go get github.com/creack/pty`

**Done when:** Can spawn a real process, write to it, read output, and terminate it.

---

## Phase 3: Store Backends

### Step 3.1: File store

**File:** `internal/store/file/file_store.go`

- Constructor takes `logPath` and `debugRaw` flag
- `CreateSession` — writes `<logPath>/sessions/<sessionID>.json` (temp + rename)
- `UpdateSession` — reads, patches, writes (temp + rename)
- `AppendTurn` — appends JSON line to `<logPath>/turns/<sessionID>.jsonl`
- `PeekTurns` — reads JSONL, returns last N turns
- `AppendStreamEvent` — appends to `<logPath>/raw/<sessionID>.jsonl` (no-op if not debugRaw)
- `Close` — no-op

**Tests:** `internal/store/file/file_store_test.go`
- Create session, read back from disk
- Append turns, peek last N
- Update session, verify patch applied
- Verify atomic write (no partial files on crash)

### Step 3.2: Postgres store

**File:** `internal/store/postgres/postgres_store.go`

- Constructor takes `databaseURL`, opens `pgx` pool
- Embeds migration SQL via `embed.FS`
- Runs migrations on first connection
- Implements all `Store` methods with parameterized queries
- `Close` — closes connection pool

**File:** `internal/store/postgres/migrations/001_init.sql`

- Schema from design spec

**Dependencies:** `go get github.com/jackc/pgx/v5`

**Tests:** `internal/store/postgres/postgres_store_test.go`
- Uses testcontainers to spin up real Postgres
- Same test cases as file store (create, update, append, peek)
- Verify migrations run cleanly on fresh database

**Done when:** Both stores pass identical test scenarios.

---

## Phase 4: Claude Code Adapter

### Step 4.1: Adapter skeleton

**File:** `internal/adapter/claude_code/claude_code_adapter.go`

- `ID()` returns `"claude-code"`
- `BuildSpawnSpec` — returns `claude` command with appropriate args, passes through `cwd` and `env`
- `FormatPrompt` — appends newline to raw prompt string

### Step 4.2: Completion detection

**File:** `internal/adapter/claude_code/completion.go`

- `DetectCompletion(context CompletionContext) *CompletionSignal`
- Priority order:
  1. Process exited (`ExitCode != nil`) — confidence 1.0
  2. Known Claude CLI prompt pattern reappears in stream tail — confidence 0.9
  3. Silence heuristic: non-empty output received, then idle > 2 seconds — confidence 0.6
- Returns `nil` if none match

**Tests:** `internal/adapter/claude_code/completion_test.go`
- Transcript with clean exit — returns completed, confidence 1.0
- Transcript with prompt reappearance — returns completed, confidence 0.9
- Transcript with output then silence — returns completed, confidence 0.6
- Transcript still streaming — returns nil
- Empty output with silence — returns nil (don't trigger on startup delay)
- ANSI escape sequences in stream tail — still detects patterns

### Step 4.3: Response extraction

**File:** `internal/adapter/claude_code/extraction.go`

- `ExtractResponse(transcript []byte) ExtractionResult`
- Strip ANSI escape sequences
- Priority:
  1. Parse last assistant block by prompt pattern transitions — confidence 0.9
  2. Last non-empty output block — confidence 0.6
  3. Nothing found — empty string, confidence 0.1
- Store notes for ambiguous cases

**Tests:** `internal/adapter/claude_code/extraction_test.go`
- Clean transcript with single response — high confidence
- Transcript with tool usage interspersed — extracts final response only
- ANSI-heavy output — stripped cleanly
- Empty transcript — empty response, low confidence
- Use `testdata/` directory with recorded real terminal output samples

### Step 4.4: Adapter registration

**File:** `internal/adapter/adapter.go` (extend)

- `AdapterRegistry` — map of `string` to `Adapter`
- `NewDefaultRegistry()` — returns registry with Claude Code adapter registered

**Done when:** All adapter unit tests pass with representative transcript samples.

---

## Phase 5: Orchestrator

### Step 5.1: Completion loop

**File:** `internal/orchestrator/completion_loop.go`

- Polls every 100ms via ticker
- Selects on: `process.Output()`, `process.Done()`, `ticker.C`, `context.Done()`
- Feeds stream events to buffer
- Calls `adapter.DetectCompletion` on each tick
- Returns `CompletionSignal` + populated `StreamBuffer`
- On context cancellation: calls `process.Terminate(5 * time.Second)`

### Step 5.2: Orchestrator function

**File:** `internal/orchestrator/orchestrator.go`

- `Run(context, request, dependencies) (*RunResult, error)`
- Follows the 10-step flow from design spec
- Store errors are caught and logged to stderr, never returned as the primary error

**File:** `internal/orchestrator/dependencies.go`

- `Dependencies` struct with `Adapters`, `PseudoTerminal`, `Store`

**Tests:** `internal/orchestrator/orchestrator_test.go`
- Uses mock adapter, mock PTY runtime, mock store
- Happy path: prompt sent, response received, session finalized
- Timeout: completion never fires, deadline exceeded
- Spawn failure: returns `E_SPAWN` error with session created
- Prompt write failure: returns `E_PROMPT_IO`
- Store failure during session create: still returns response, logs warning
- Process exits immediately: extraction runs on whatever output exists

**Done when:** Orchestrator tests pass with all mock combinations.

---

## Phase 6: CLI

### Step 6.1: Root command

**File:** `cmd/clank/main.go`

- Cobra root command with version flag
- Wires subcommands

### Step 6.2: Run command

**File:** `internal/cli/run.go`

- Parses all flags from design spec
- Validates flag combinations (exactly one prompt source, backend-specific flags)
- Constructs `RunRequest`, `Dependencies` (real PTY runtime, chosen store, default adapter registry)
- Calls `orchestrator.Run`
- Marshals `RunResult` to JSON, writes to stdout
- On error: marshals error result to stdout, exits with code 1
- On config error: writes message to stderr, exits with code 2

### Step 6.3: Peek command

**File:** `internal/cli/peek.go`

- Flags: `--session-id`, `--last`, `--log-backend`, `--log-path`, `--database-url`, `--format`
- Opens the appropriate store
- Calls `store.PeekTurns`
- Outputs as JSON (default) or formatted table

**Tests:** Integration tests via CLI binary execution
- Build binary, run against mock PTY provider
- Verify JSON output matches schema
- Verify exit codes

**Done when:** `clank run` and `clank peek` work end-to-end against the mock PTY provider.

---

## Phase 7: Mock PTY Provider (Test Harness)

### Step 7.1: Build mock agent binary

**File:** `internal/testutil/mockagent/main.go`

- A small Go binary that simulates a terminal agent
- Reads from stdin (the prompt)
- Waits a configurable delay (via env var `MOCK_DELAY_MS`)
- Writes a canned response (via env var `MOCK_RESPONSE` or `MOCK_RESPONSE_FILE`)
- Optionally simulates ANSI output, prompt patterns, partial output
- Exits with configurable exit code (via env var `MOCK_EXIT_CODE`)
- Mode flags via env: `MOCK_MODE=happy|timeout|partial|crash`

**Done when:** Mock agent can be spawned by the real PTY runtime and produces deterministic, controllable output.

---

## Phase 8: End-to-End Integration Tests

### Step 8.1: Happy path

- Spawn `clank run` against mock agent in happy mode
- Verify: exit code 0, JSON has `status: "completed"`, response matches canned output, session exists in store

### Step 8.2: Timeout path

- Mock agent in timeout mode (never responds)
- Verify: exit code 1, JSON has `status: "timeout"`, `error.code: "E_TIMEOUT"`

### Step 8.3: Partial output

- Mock agent writes ambiguous output then exits
- Verify: `status: "partial"`, low confidence scores

### Step 8.4: File backend round-trip

- Run with `--log-backend file`, then `clank peek` on same session
- Verify turns match

### Step 8.5: Postgres backend round-trip

- Testcontainers Postgres
- Run with `--log-backend db`, then `clank peek`
- Verify turns match

### Step 8.6: JSON schema contract

- Validate every test's JSON output against a shared JSON schema file
- Schema lives in `testdata/run_result_schema.json`

**Done when:** All integration tests pass. MVP is feature-complete.

---

## Phase 9: Manual Acceptance Testing

- Run `clank run --provider claude-code --prompt "What is 2+2?"` against real Claude Code
- Tune completion heuristics based on observed terminal output
- Capture representative transcripts into `testdata/` for regression tests
- Iterate on extraction logic until confidence is consistently above 0.8 for clean interactions

---

## Dependency Summary

| Package | Purpose |
|---------|---------|
| `github.com/creack/pty` | PTY allocation |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/jackc/pgx/v5` | Postgres driver |
| `github.com/testcontainers/testcontainers-go` | Postgres in integration tests |

---

## Execution Order

```
Phase 1 (scaffold + types)
    |
Phase 2 (PTY runtime)
    |
Phase 3 (store backends) --- can parallel with Phase 4
    |
Phase 4 (Claude Code adapter)
    |
Phase 5 (orchestrator)
    |
Phase 6 (CLI)
    |
Phase 7 (mock agent)
    |
Phase 8 (integration tests)
    |
Phase 9 (manual acceptance)
```

Phases 3 and 4 are independent and can be worked in parallel.
