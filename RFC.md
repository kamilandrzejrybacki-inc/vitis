# RFC v2: OpenClaw Agent Bridge (Implementation-Ready)

- **Status**: Draft for implementation  
- **Date**: April 3, 2026  
- **Owner**: OpenClaw Core  
- **MVP Provider**: Claude Code  
- **Mode**: synchronous, 1:1 prompt/response  

---

## 1) Scope & Invariants

### 1.1 Scope (MVP)
- Run one prompt against Claude Code through PTY.
- Await completion.
- Return one final assistant response.
- Persist session + turn logs.
- Support `peek` of last `n` turns.
- Configurable log backend: file or DB (CLI flag).

### 1.2 Invariants
1. Exactly one user prompt per run.
2. Exactly one returned `response` string per run.
3. A `session_id` exists for any run that reaches spawn.
4. `peek` is always turn-based (not chunk-based).
5. No additional sandboxing; inherits host capabilities.

---

## 2) Repository Layout (proposed)

```text
agent-bridge/
  src/
    cli/
      main.ts
      commands/
        run.ts
        peek.ts
    core/
      orchestrator.ts
      types.ts
      errors.ts
      config.ts
    runtime/
      pty_runner.ts
      stream_buffer.ts
    adapters/
      adapter.ts
      claude_code_adapter.ts
      completion/
        heuristics.ts
      parsing/
        turn_parser.ts
        response_extractor.ts
    storage/
      store.ts
      file_store.ts
      db_store.ts
      db/
        migrations/
          001_init.sql
    util/
      clock.ts
      ids.ts
      logger.ts
  tests/
    unit/
    integration/
  package.json
  README.md
```

(Equivalent Python layout is fine if your stack prefers it.)

---

## 3) Type Contracts

### 3.1 Core types

```ts
// src/core/types.ts
export type RunStatus = "completed" | "timeout" | "error" | "partial";

export interface RunRequest {
  provider: "claude-code"; // MVP
  prompt?: string;
  promptFile?: string;
  timeoutSec: number;
  cwd?: string;
  envFile?: string;
  logBackend: "file" | "db";
  logPath?: string; // required if file
  dbUrl?: string;   // required if db
  peekLast: number; // default 10
  debugRaw?: boolean;
}

export interface RunResult {
  sessionId: string;
  provider: string;
  status: RunStatus;
  response: string;
  peek: Turn[];
  meta: {
    durationMs: number;
    exitCode: number | null;
    parserConfidence: number;
    completionConfidence: number;
  };
  error?: {
    code: string;
    message: string;
  };
}

export interface SessionRecord {
  sessionId: string;
  provider: string;
  status: RunStatus;
  startedAt: string;
  endedAt: string | null;
  durationMs: number | null;
  exitCode: number | null;
  parserConfidence: number | null;
  completionConfidence: number | null;
}

export interface Turn {
  sessionId: string;
  turnIndex: number;
  role: "user" | "assistant" | "system" | "meta";
  content: string;
  createdAt: string;
}

export interface StreamEvent {
  sessionId: string;
  ts: string;
  source: "stdin" | "stdout" | "stderr";
  chunk: string;
}
```

---

## 4) Adapter Contract & Claude Implementation

### 4.1 Adapter interface

```ts
// src/adapters/adapter.ts
export interface SpawnSpec {
  command: string;
  args: string[];
  env?: Record<string, string>;
  cwd?: string;
}

export interface CompletionSignal {
  status: "completed" | "timeout" | "partial";
  confidence: number;
  reason: string;
}

export interface ExtractionResult {
  response: string;
  parserConfidence: number;
  notes?: string[];
}

export interface AgentAdapter {
  id(): string;
  buildSpawnSpec(cfg: { cwd?: string; env?: Record<string, string> }): SpawnSpec;
  formatPrompt(raw: string): string; // e.g., ensure newline/terminator
  detectCompletion(ctx: {
    streamTail: string;
    elapsedMs: number;
    idleMs: number;
  }): CompletionSignal | null;
  extractFinalResponse(transcript: string): ExtractionResult;
}
```

### 4.2 Claude adapter behavior (MVP)

- `id() = "claude-code"`
- `buildSpawnSpec` uses installed Claude CLI command.
- `formatPrompt` appends newline and submit sequence if needed.
- Completion detection uses:
  1. known Claude CLI completion markers (if observed),
  2. prompt reappearance pattern,
  3. silence heuristic.
- Extraction strategy:
  - parse last assistant block by delimiter/prompt transitions,
  - fallback to last non-empty output block.

---

## 5) PTY Runner Contract

```ts
// src/runtime/pty_runner.ts
export interface PtyProcess {
  write(data: string): Promise<void>;
  kill(signal?: string): Promise<void>;
  onData(cb: (source: "stdout"|"stderr", chunk: string) => void): void;
  onExit(cb: (code: number | null) => void): void;
}

export interface PtyRunner {
  spawn(spec: SpawnSpec): Promise<PtyProcess>;
}
```

### PTY behavior requirements
- Must capture output asynchronously.
- Must timestamp and forward all chunks to orchestrator.
- Must support graceful termination:
  1. soft interrupt
  2. hard kill on deadline

---

## 6) Orchestrator Pseudocode

```ts
async function run(req: RunRequest): Promise<RunResult> {
  validate(req);
  const sessionId = newSessionId();
  const startedAt = nowIso();

  await store.createSession({...});

  const adapter = adapters.get(req.provider);
  const spec = adapter.buildSpawnSpec({cwd: req.cwd, env: loadEnv(req.envFile)});
  const proc = await pty.spawn(spec);

  const streamBuffer = new StreamBuffer();
  proc.onData((source, chunk) => {
    streamBuffer.push(source, chunk);
    store.appendStreamEvent({sessionId, ts: nowIso(), source, chunk}); // if debugRaw
  });

  const prompt = resolvePrompt(req.prompt, req.promptFile);
  await proc.write(adapter.formatPrompt(prompt));
  await store.appendTurn(userTurn(sessionId, prompt));

  const completion = await waitForCompletionLoop({
    timeoutSec: req.timeoutSec,
    adapter,
    streamBuffer,
    proc
  });

  const transcript = streamBuffer.getTranscript();
  const extracted = adapter.extractFinalResponse(transcript);

  const assistantTurn = mkAssistantTurn(sessionId, extracted.response);
  await store.appendTurn(assistantTurn);

  const endedAt = nowIso();
  await store.updateSession({
    sessionId,
    status: mapCompletionToStatus(completion),
    endedAt,
    durationMs: diffMs(startedAt, endedAt),
    parserConfidence: extracted.parserConfidence,
    completionConfidence: completion.confidence,
    exitCode: streamBuffer.exitCode ?? null
  });

  const peek = await store.peekTurns(sessionId, req.peekLast);

  return {
    sessionId,
    provider: adapter.id(),
    status: mapCompletionToStatus(completion),
    response: extracted.response,
    peek,
    meta: {
      durationMs: diffMs(startedAt, endedAt),
      exitCode: streamBuffer.exitCode ?? null,
      parserConfidence: extracted.parserConfidence,
      completionConfidence: completion.confidence
    }
  };
}
```

---

## 7) Completion Loop Algorithm (deterministic)

```ts
async function waitForCompletionLoop(ctx): Promise<CompletionSignal> {
  const deadline = Date.now() + timeoutSec * 1000;
  let lastDataAt = Date.now();
  let seenNonEmptyAssistantOutput = false;

  while (Date.now() < deadline) {
    await sleep(100);
    const tail = ctx.streamBuffer.tailText(8000);
    const idleMs = Date.now() - ctx.streamBuffer.lastChunkAt();

    if (ctx.streamBuffer.newOutputSinceLastTick()) {
      lastDataAt = Date.now();
      if (ctx.streamBuffer.hasNonWhitespaceOutput()) seenNonEmptyAssistantOutput = true;
    }

    const adapterSignal = ctx.adapter.detectCompletion({
      streamTail: tail,
      elapsedMs: Date.now() - (deadline - timeoutSec*1000),
      idleMs
    });
    if (adapterSignal) return adapterSignal;

    // generic fallback
    if (seenNonEmptyAssistantOutput && idleMs > 2000) {
      return { status: "completed", confidence: 0.6, reason: "silence-window" };
    }
  }

  // timeout
  await trySoftInterruptThenKill(ctx.proc);
  return { status: "timeout", confidence: 1.0, reason: "deadline-exceeded" };
}
```

---

## 8) Parsing & Response Extraction Rules

### 8.1 Turn parsing (minimal robust parser)
- Keep parser conservative:
  - identify user turn from known submitted prompt text.
  - assistant turn = response blocks following prompt until completion.
- If ambiguous:
  - collapse to one assistant turn with last meaningful content block.
- Strip control sequences for parsed turn content.
- Preserve raw logs separately when enabled.

### 8.2 Final response extraction
Priority:
1. last parsed assistant turn with non-empty text.
2. last non-empty stdout block.
3. if nothing found: empty string + low confidence + `partial` status.

---

## 9) Storage Interface + Implementations

### 9.1 Interface

```ts
// src/storage/store.ts
export interface LogStore {
  createSession(rec: SessionRecord): Promise<void>;
  updateSession(patch: Partial<SessionRecord> & { sessionId: string }): Promise<void>;
  appendTurn(turn: Turn): Promise<void>;
  peekTurns(sessionId: string, lastN: number): Promise<Turn[]>;
  appendStreamEvent?(ev: StreamEvent): Promise<void>;
}
```

### 9.2 File backend

Per session:
- `sessions/<session_id>.json` (session summary)
- `turns/<session_id>.jsonl` (turn stream)
- optional `raw/<session_id>.jsonl` (raw events, gated by `debugRaw`)

Atomic write strategy:
- append-only JSONL for turns/events
- temp-file + rename for session summary updates

### 9.3 DB backend (DDL)

```sql
-- 001_init.sql
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

CREATE UNIQUE INDEX turns_session_turn_idx_uq
  ON turns(session_id, turn_index);

CREATE TABLE stream_events (
  id BIGSERIAL PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
  ts TIMESTAMPTZ NOT NULL,
  source TEXT NOT NULL,
  chunk TEXT NOT NULL
);

CREATE INDEX stream_events_session_ts_idx
  ON stream_events(session_id, ts);
```

---

## 10) CLI Spec (exact)

### 10.1 `run`

```bash
agent-bridge run \
  --provider claude-code \
  (--prompt "text" | --prompt-file ./prompt.txt) \
  --log-backend (file|db) \
  [--log-path ./logs] \
  [--db-url postgres://...] \
  [--peek-last 10] \
  [--timeout-sec 600] \
  [--cwd /path] \
  [--env-file .env] \
  [--debug-raw]
```

Validation:
- exactly one of `--prompt` / `--prompt-file`
- `--log-path` required when `file`
- `--db-url` required when `db`

### 10.2 `peek`

```bash
agent-bridge peek \
  --session-id sess_123 \
  --last-n 10 \
  [--format json|table]
```

---

## 11) Error Model

| Code | Meaning | Retry? |
|---|---|---|
| `E_SPAWN` | Could not start provider process | maybe |
| `E_PROMPT_IO` | Failed writing prompt to PTY | maybe |
| `E_TIMEOUT` | Completion deadline exceeded | maybe |
| `E_EXTRACT` | Could not reliably extract final response | maybe |
| `E_STORE` | Log backend failed | depends |
| `E_CONFIG` | Invalid CLI arguments | no |

Return shape (on error):
```json
{
  "session_id": "sess_...",
  "status": "error",
  "response": "",
  "error": { "code": "E_SPAWN", "message": "..." },
  "peek": []
}
```

---

## 12) Test Plan (must-pass MVP)

### 12.1 Unit tests
- adapter spawn spec generation
- completion heuristics
- response extraction with representative transcripts
- arg validation matrix

### 12.2 Integration tests
- `run` happy path (mock PTY provider)
- timeout path
- ambiguous output path -> partial + low confidence
- file backend persistence + peek last n turns
- db backend persistence + peek last n turns

### 12.3 Contract tests
- `RunResult` JSON schema validation
- status mapping consistency

---

## 13) Milestones & Delivery

### M1 (core runnable)
- PTY runner
- Claude adapter basic
- file backend
- CLI run + peek

### M2 (production hardening)
- db backend
- improved completion/extraction confidence
- robust error mapping
- integration tests

### M3 (extensibility)
- adapter SDK docs
- add OpenCode / Pi adapters
- concurrency guardrails

---

## 14) Implementation Notes for Claude (direct instructions)

1. Implement **interfaces first** (`types`, `adapter`, `store`).
2. Build a **mock PTY integration harness** before real provider wiring.
3. Keep completion/extraction logic modular and test-driven.
4. Make CLI output schema stable from day one.
5. Prioritize deterministic behavior over clever parsing.
6. Add verbose debug mode with raw tail output for troubleshooting.
7. Do not add sandbox restrictions unless explicitly asked.

---

## 15) Acceptance Checklist

- [ ] `run` returns one final response for one prompt.
- [ ] `peek` returns last `n` turns.
- [ ] file backend works.
- [ ] db backend works.
- [ ] timeout yields structured status/error.
- [ ] extraction includes confidence.
- [ ] tests pass for unit + integration suites.
