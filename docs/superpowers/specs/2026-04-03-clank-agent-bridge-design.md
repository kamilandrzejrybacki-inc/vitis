# Clank Agent Bridge - Design Spec

- **Status**: Approved
- **Date**: 2026-04-03
- **Language**: Go
- **MVP Provider**: Claude Code
- **Mode**: Synchronous, 1:1 prompt/response (single-turn)
- **Consumer**: CLI only, JSON to stdout

---

## 1. Overview

Clank is a Go CLI tool that drives AI agent CLIs (Claude Code, OpenCode, etc.) through a simulated terminal (PTY). It sends a prompt, detects when the agent is done, extracts the final response, logs the session, and outputs structured JSON.

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Excellent PTY/process handling, single binary, aligns with botl |
| Runtime | PTY for all providers | Universal abstraction across CLIs, battle-tests heuristics from day one |
| Multi-turn | MVP single-turn, PTY enables future multi-turn | Ship fast, don't paint into a corner |
| Storage | File + Postgres from day one | Go interfaces make this cheap, both consumers exist |
| Adapter priority | Interface is load-bearing | Second provider arriving within weeks |
| Consumer | CLI only, JSON to stdout | Unix-philosophy, n8n/Prefect call as subprocess |
| Naming | Full descriptive names, no shorthand | Readability over brevity |

### Key Departures from RFC v2

1. Go instead of TypeScript
2. No stdout/stderr split in PTY events (PTY multiplexes both onto a single stream)
3. Adapter does not touch PTY directly — provides specs + heuristics, orchestrator wires them
4. Store failures are non-blocking warnings (never prevent response delivery)
5. Binary named `clank`, not `agent-bridge`

---

## 2. Architecture

```
+----------------------------------------------+
|                    CLI                        |
|         (cobra: run, peek commands)           |
+--------------------+-------------------------+
                     |
+--------------------v-------------------------+
|              Orchestrator                     |
|  - validates request                          |
|  - creates session                            |
|  - delegates to adapter + PTY runtime         |
|  - collects result, persists, returns JSON    |
+--------+--------------------+----------------+
         |                    |
+--------v--------+  +--------v--------+
|  Adapter        |  |  PTY Runtime    |
|  Registry       |  |  (creack/pty    |
|                 |  |   goroutines    |
| - claude-code   |  |   channels)     |
| - opencode      |  |                 |
| - ...           |  +---------+------+
+--------+--------+            |
         |                     |
         +----------+----------+
                    |
+-------------------v--------------------------+
|              Store (interface)                |
|         +----------+-----------+             |
|         | FileStore| DBStore   |             |
|         | (JSONL)  | (Postgres)|             |
|         +----------+-----------+             |
+----------------------------------------------+
```

The adapter provides behavioral descriptions (spawn spec, completion heuristics, response extraction). The PTY runtime owns the process lifecycle. The orchestrator wires them together. The store persists sessions and turns.

---

## 3. Adapter Interface

```go
type SpawnSpec struct {
    Command string
    Args    []string
    Env     map[string]string
    Cwd     string
}

type CompletionSignal struct {
    Status     RunStatus  // completed | timeout | partial
    Confidence float64    // 0.0-1.0
    Reason     string     // human-readable explanation
}

type ExtractionResult struct {
    Response         string
    ParserConfidence float64
    Notes            []string // warnings, ambiguities
}

type CompletionContext struct {
    StreamTail []byte   // last N bytes of output
    ElapsedMs  int64    // time since prompt was sent
    IdleMs     int64    // time since last output chunk
    ExitCode   *int     // non-nil if process exited
}

type Adapter interface {
    ID() string
    BuildSpawnSpec(cwd string, env map[string]string) SpawnSpec
    FormatPrompt(raw string) []byte
    DetectCompletion(context CompletionContext) *CompletionSignal
    ExtractResponse(transcript []byte) ExtractionResult
}
```

### Design Notes

- `FormatPrompt` returns `[]byte` because we write raw bytes to a PTY (control characters, escape sequences, carriage returns).
- `CompletionContext` includes `ExitCode` so adapters can use process exit as a completion signal.
- `DetectCompletion` returns a pointer: `nil` means "not done yet, keep polling."
- Adapters are pure logic with no PTY access, making them trivially unit-testable with fake transcripts.

---

## 4. PTY Runtime

```go
type StreamEvent struct {
    Timestamp time.Time
    Data      []byte
}

type ExitResult struct {
    Code int
    Err  error
}

type PseudoTerminalRuntime interface {
    Spawn(spec SpawnSpec) (PseudoTerminalProcess, error)
}

type PseudoTerminalProcess interface {
    Write(data []byte) (int, error)
    Output() <-chan StreamEvent
    Done() <-chan ExitResult
    Terminate(gracePeriod time.Duration) error
}
```

### Design Notes

- Channels instead of callbacks: Go-idiomatic. The orchestrator selects on `Output()` and `Done()` in a single goroutine.
- No stdout/stderr split: PTYs multiplex both onto a single stream (`creack/pty` gives one `os.File`).
- `Terminate` is two-phase: SIGINT first (lets the CLI clean up), then SIGKILL after the grace period.
- `Write` implements `io.Writer` semantics.

### Orchestrator Loop

```go
process, _ := runtime.Spawn(adapter.BuildSpawnSpec(workingDirectory, environment))
process.Write(adapter.FormatPrompt(prompt))

ticker := time.NewTicker(100 * time.Millisecond)
for {
    select {
    case event := <-process.Output():
        buffer.Append(event)
    case exit := <-process.Done():
        // process exited - run final extraction
    case <-ticker.C:
        signal := adapter.DetectCompletion(buildContext(buffer))
        if signal != nil { /* done */ }
    case <-context.Done():
        process.Terminate(5 * time.Second)
    }
}
```

---

## 5. Store Interface

```go
type Store interface {
    CreateSession(session Session) error
    UpdateSession(sessionID string, patch SessionPatch) error
    AppendTurn(turn Turn) error
    PeekTurns(sessionID string, lastN int) ([]Turn, error)
    AppendStreamEvent(event StoredStreamEvent) error
    Close() error
}

type Session struct {
    ID                   string
    Provider             string
    Status               RunStatus
    StartedAt            time.Time
    EndedAt              *time.Time
    DurationMs           *int64
    ExitCode             *int
    ParserConfidence     *float64
    CompletionConfidence *float64
}

type SessionPatch struct {
    Status               *RunStatus
    EndedAt              *time.Time
    DurationMs           *int64
    ExitCode             *int
    ParserConfidence     *float64
    CompletionConfidence *float64
}

type Turn struct {
    SessionID string
    Index     int
    Role      string    // "user" | "assistant" | "system" | "meta"
    Content   string
    CreatedAt time.Time
}

type StoredStreamEvent struct {
    SessionID string
    Timestamp time.Time
    Chunk     []byte
}
```

### File Backend

Per session:
- `<logPath>/sessions/<sessionID>.json` (session summary)
- `<logPath>/turns/<sessionID>.jsonl` (turn stream)
- `<logPath>/raw/<sessionID>.jsonl` (raw events, gated by `debugRaw`)

Atomic writes: append-only JSONL for turns/events, temp-file + rename for session summary updates.

### Postgres Backend

```sql
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_ms BIGINT,
    exit_code INT,
    parser_confidence REAL,
    completion_confidence REAL
);

CREATE TABLE turns (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    turn_index INT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX turns_session_turn_index_unique
    ON turns(session_id, turn_index);

CREATE TABLE stream_events (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ NOT NULL,
    chunk TEXT NOT NULL
);

CREATE INDEX stream_events_session_timestamp_index
    ON stream_events(session_id, timestamp);
```

Connection via `pgx` pool. Migrations embedded with `embed.FS`.

### Store Failure Policy

Store failures never block the response. If the PTY interaction succeeded but Postgres is down, the caller still gets their answer. Store errors are logged to stderr as warnings.

---

## 6. Orchestrator

```go
func Run(
    context context.Context,
    request RunRequest,
    dependencies Dependencies,
) (*RunResult, error) {
    // 1. Validate request
    // 2. Resolve adapter from registry
    // 3. Create session in store
    // 4. Spawn PTY process via adapter's SpawnSpec
    // 5. Write formatted prompt to PTY
    // 6. Log user turn
    // 7. Run completion loop (poll adapter heuristics every 100ms)
    // 8. Extract response from transcript
    // 9. Log assistant turn
    // 10. Finalize session in store
    // 11. Return RunResult as JSON
}

type Dependencies struct {
    Adapters       AdapterRegistry
    PseudoTerminal PseudoTerminalRuntime
    Store          Store
}
```

### Design Notes

- Stateless function, no hidden state. Everything flows through arguments.
- `Dependencies` struct for dependency injection: swap in mock PTY, mock store, mock adapter for tests.
- Turn index is hardcoded 0 (user) / 1 (assistant) for MVP single-turn. Becomes a counter when multi-turn is added.
- Error results still create a session so `peek` always works, even on failure.

---

## 7. CLI Interface

### `clank run`

```bash
clank run \
    --provider claude-code \
    --prompt "Explain the main function" \
    --timeout 600 \
    --log-backend file \
    --log-path ./logs \
    --peek-last 10 \
    --working-directory /path/to/repo \
    --env-file .env \
    --debug-raw
```

Or with prompt from file:

```bash
clank run \
    --provider claude-code \
    --prompt-file ./prompt.txt \
    --timeout 600 \
    --log-backend db \
    --database-url postgres://...
```

Output: single JSON object to stdout. Diagnostics to stderr.

### `clank peek`

```bash
clank peek \
    --session-id sess_abc123 \
    --last 10 \
    --log-backend file \
    --log-path ./logs \
    --format json
```

Supports `--format json` (default) and `--format table`.

### Validation Rules

- Exactly one of `--prompt` / `--prompt-file`
- `--log-path` required when `--log-backend file`
- `--database-url` required when `--log-backend db`
- `--timeout` defaults to 600 seconds
- `--peek-last` defaults to 10

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Completed successfully |
| 1 | Runtime error (spawn, timeout, extraction) |
| 2 | Configuration error (bad flags) |

JSON result always goes to stdout regardless of exit code.

---

## 8. Error Model

```go
type ErrorCode string

const (
    ErrorSpawn    ErrorCode = "E_SPAWN"
    ErrorPromptIO ErrorCode = "E_PROMPT_IO"
    ErrorTimeout  ErrorCode = "E_TIMEOUT"
    ErrorExtract  ErrorCode = "E_EXTRACT"
    ErrorStore    ErrorCode = "E_STORE"
    ErrorConfig   ErrorCode = "E_CONFIG"
)
```

| Error | Session Created? | Turns Logged? | Response in JSON? |
|-------|-----------------|---------------|-------------------|
| `E_CONFIG` | No | No | No (stderr + exit 2) |
| `E_SPAWN` | Yes | No | Empty string, error field populated |
| `E_PROMPT_IO` | Yes | User turn only | Empty string, error field populated |
| `E_TIMEOUT` | Yes | Both (partial) | Best-effort extraction, `status: "timeout"` |
| `E_EXTRACT` | Yes | User turn only | Empty string, `status: "partial"`, low confidence |
| `E_STORE` | Depends | Depends | Response still returned; store failure logged to stderr |

---

## 9. Project Layout

```
clank/
    cmd/
        clank/
            main.go                         # cobra root command
    internal/
        cli/
            run.go                          # "run" command
            peek.go                         # "peek" command
        orchestrator/
            orchestrator.go                 # Run function
            dependencies.go                 # Dependencies struct
            completion_loop.go              # polling loop
        adapter/
            adapter.go                      # Adapter interface + registry
            claude_code/
                claude_code_adapter.go      # Claude Code heuristics
                completion.go               # completion detection logic
                extraction.go               # response extraction logic
            opencode/
                opencode_adapter.go         # second provider (weeks out)
        terminal/
            runtime.go                      # PseudoTerminalRuntime interface
            process.go                      # PseudoTerminalProcess implementation
            buffer.go                       # StreamBuffer (accumulates output)
        store/
            store.go                        # Store interface
            file/
                file_store.go               # JSONL file backend
            postgres/
                postgres_store.go           # Postgres backend
                migrations/
                    001_init.sql
        model/
            session.go                      # Session, SessionPatch
            turn.go                         # Turn
            result.go                       # RunRequest, RunResult, RunStatus
            errors.go                       # Error codes
    go.mod
    go.sum
```

### Package Responsibilities

- `internal/` keeps everything unexported. The CLI binary is the only public interface.
- Each adapter gets its own package with isolated heuristics.
- `terminal/` avoids collision with `creack/pty` package name.
- `model/` has zero dependencies; everything else depends on it.
- Nested where warranted (store backends, adapter implementations), flat otherwise.

---

## 10. Testing Strategy

### Unit Tests

- **Adapter heuristics**: feed recorded transcripts to `DetectCompletion` and `ExtractResponse`, assert correct signals. Each adapter has a `testdata/` directory with real captured terminal output.
- **Completion edge cases**: empty output, immediate exit, rapid bursts, ANSI escape sequences.
- **Store serialization**: `Turn` and `Session` round-trip through JSON and SQL.
- **CLI validation**: flag combinations, missing required flags, conflicting flags.

### Integration Tests

- **Mock PTY provider**: a small Go binary that simulates a terminal agent (reads stdin, waits, writes canned response, exits). Replaces real Claude Code in tests.
- **Happy path**: `clank run` against mock provider, verify JSON output, session and turns in store.
- **Timeout path**: mock provider that never responds, verify `E_TIMEOUT` and graceful termination.
- **Partial output**: mock provider with ambiguous output, verify `status: "partial"` and low confidence.
- **File backend**: verify JSONL written, `clank peek` reads correctly.
- **Postgres backend**: testcontainers with real Postgres, verify migrations, writes, reads.

### Contract Tests

- **JSON schema validation**: `RunResult` output validated against a JSON schema on every test run. Stability guarantee for n8n/Prefect consumers.

### Not Tested in MVP (Manual Acceptance)

- Real Claude Code integration. Heuristics need iterative tuning against real output. Mock PTY + recorded transcripts cover the automated side.

---

## 11. Milestones

### M1: Core Runnable
- PTY runtime with `creack/pty`
- Claude Code adapter (basic heuristics)
- File backend
- CLI `run` + `peek`
- Unit + integration tests with mock provider

### M2: Production Hardening
- Postgres backend
- Improved completion/extraction confidence
- Robust error mapping
- Full integration test suite with testcontainers

### M3: Extensibility
- Second provider adapter (OpenCode or similar)
- Adapter development documentation
- Concurrency guardrails (if needed for multi-session scenarios)

### Future: Multi-Turn
- Keep PTY process alive across turns
- Turn counter on session
- Conversation state management
- New CLI subcommand or interactive mode
