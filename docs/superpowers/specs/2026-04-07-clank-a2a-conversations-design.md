# Clank A2A Conversations — Design Spec

- **Status**: Draft (awaiting user review)
- **Date**: 2026-04-07
- **Language**: Go
- **Builds on**: `2026-04-03-clank-agent-bridge-design.md`
- **Mode**: Multi-turn, two-peer, event-driven via pluggable bus
- **Consumer**: CLI (`clank converse`, `clank converse-serve`, `clank converse-tail`)

---

## 0. Prior Art (TODO before implementation)

The development workflow mandates research/reuse before any new implementation. Before writing code for this spec, the implementation plan must complete:

- **GitHub code search** for existing Go multi-agent / A2A frameworks: `gh search repos "multi-agent go"`, `gh search code "agent to agent" language:go`, look at `eino`, `langchaingo`, any AutoGen-in-Go ports.
- **Context7 / vendor docs** for `nats.go` (current client API) and `nats-server` embeddable mode.
- **PTY multiplexing patterns** — survey how existing terminal multiplexers and TUI test harnesses solve mid-stream turn-boundary detection.
- **Marker injection prior art** — search for "agent loop sentinel token" patterns in existing agent frameworks.

Findings get folded into the implementation plan. If a battle-tested library covers 80% of the bus or peer-transport surface, we adopt it instead of writing our own.

---

## 1. Overview

Clank today is single-shot: one prompt, one response, PTY dies. This spec adds **A2A (agent-to-agent) conversations**: two long-lived peers exchange turns through a Conversation Broker, with persistent PTY processes preserving each peer's internal state across turns. Peers are interchangeable — a local PTY agent, a remote clank instance, and a stdio-piped external agent all implement the same `PeerTransport` interface.

The architecture is **event-driven** by design: the Broker, peer transports, store, terminator, and observers all communicate through a pluggable `Bus` interface. The default in-process bus is zero-dependency. An opt-in NATS backend unlocks distributed peers, live observability, and pluggable judges with no broker code changes.

### Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Topology | Symmetric, both sides interchangeable (Q1: C) | Same flag shape for local PTY, remote clank, stdio peer |
| Termination | Pluggable: `sentinel` or `judge`, with always-on max-turns cap (Q2: B+C+safety rail) | User picks soft strategy; hard cap protects against runaway costs |
| Peer addressing | URI scheme: `provider:`, `clank://`, `stdio://` (Q3: A) | One flag shape, four transports, room to grow |
| Turn protocol | System briefing on turn 1 + per-turn envelope thereafter (Q4: D) | Briefing is critical so peers know they're in dialogue; envelope gives situational awareness |
| PTY lifecycle | Persistent process per peer (Q5: B) | O(n) tokens vs O(n²); preserves in-CLI memory across turns |
| Turn-end detection | Marker injection primary + adapter idle-prompt fallback (Q6: C) | Marker is transport-portable; fallback handles uncooperative peers |
| Seed model | Single `--seed` or asymmetric `--seed-a`/`--seed-b`, `--opener` selectable (Q7: D) | Strict alternation; opener default `a` |
| Communication | Pluggable Bus interface, in-process default + NATS opt-in | Mirrors store backend pattern; unlocks observability and remote peers |

### Key Departures from the v1 Agent Bridge Spec

1. New top-level `Conversation` entity, additive to existing `Session` model — single-shot path is unchanged.
2. New persistent PTY runtime extension (`PersistentPseudoTerminalProcess` with `ConverseTurn`), additive to existing `PseudoTerminalProcess` interface.
3. New optional adapter capability (`TurnBoundaryDetector`) — runtime-asserted, never required.
4. Store becomes one of many bus subscribers rather than a special integration path.
5. New CLI subcommands (`converse`, `converse-serve`, `converse-tail`); existing `run` and `peek` untouched.

---

## 1a. Product Boundary

### Supported (this spec)

- Local single-machine conversations between two PTY peers (`provider:` × `provider:`)
- Distributed conversations across two machines via shared NATS bus (`provider:` × `clank://`)
- External-agent-driven conversations via stdio framing (`provider:` × `stdio://`)
- Two terminator strategies: sentinel-token and judge (bus-subscriber or provider-spawned)
- Live observation via `clank converse-tail` (NATS bus only)

### Unsupported

- More than two peers per conversation (strict pairwise alternation only)
- Free-form turn-taking, peer-initiated interrupts, or peer addressing (`@peer-b: ...`)
- Automatic recovery from peer crashes (crash terminates the conversation)
- Resumption of a dead conversation from its turn log
- Hosted/multi-tenant brokering — operator still owns the bus and machines

### Isolation

Each peer transport inherits the existing single-shot isolation guarantees: HOME, `.claude`, environment variables, and working directory are caller-controlled per peer (`--peer-a-opt cwd=...`). The Broker passes them through unchanged. No sandboxing layer in this spec.

---

## 2. Architecture

```
+------------------------------------------------------+
|                    clank converse                    |
|                                                      |
|                +----------------------+              |
|                |   Conversation       |              |
|                |   Broker             |              |
|                |   (pure router)      |              |
|                +----+-------------+---+              |
|                     |   publish   |                  |
|                     v             v                  |
|     +---------------------------------------------+  |
|     |                Bus (interface)               |  |
|     |   in-process channels  |   NATS (opt-in)    |  |
|     +--+-------+--------+------+----------+-------+  |
|        |       |        |                 |          |
|   +----v---+ +-v------+ +v------+    +----v-----+    |
|   | Peer A | | Peer B | | Store |    | Observer |    |
|   | trans- | | trans- | | sub   |    | (tail,   |    |
|   | port   | | port   | |       |    |  judge)  |    |
|   +--------+ +--------+ +-------+    +----------+    |
+------------------------------------------------------+

Peer transports:
  provider:<id>  — local persistent PTY via adapter
  clank://       — remote clank peer across the bus
  stdio://       — this process's stdin/stdout (JSONL framed)
```

### Flow

1. `clank converse` parses flags, builds a `Conversation` and two `PeerSpec`s.
2. Broker initialises the chosen Bus backend (`inproc` or `nats`).
3. Both peer transports `Start()` in parallel — local PTYs spawn, `clank://` peers handshake over the bus, `stdio://` opens its frame loop.
4. Broker subscribes the Store, the Terminator, and any observers to the conversation topics.
5. Broker builds turn-1 envelope for the opener (briefing + seed + per-turn marker token), calls `opener.Deliver(envelope)`.
6. Peer transport drives its PTY/wire, captures the response, returns a `ConversationTurn`.
7. Broker publishes the turn to `conv/<id>/turn`. Store, Terminator, and observers see it.
8. Broker drains `conv/<id>/control` non-blocking for verdicts/crashes/blocks. If anything terminal: finalize.
9. Otherwise: increment turn counter, check max-turns hard cap, build the next envelope for the other peer, repeat.
10. On finalize: Broker publishes a final control message, calls `Stop()` on both transports, returns `FinalResult`.

### Topics (logical, slash-separated; NATS uses dotted equivalent)

| Topic | Producers | Subscribers | Purpose |
|---|---|---|---|
| `conv/<id>/peer-a/in` | Broker | Peer A transport | Envelope delivery to peer A |
| `conv/<id>/peer-b/in` | Broker | Peer B transport | Envelope delivery to peer B |
| `conv/<id>/turn` | Peer transports | Broker, Store, Terminator, Observers | Per-turn responses |
| `conv/<id>/control` | Peer transports, Terminator | Broker (authoritative), Store, Observers | Verdicts, crashes, blocks, finalization |

---

## 3. Data Model

New entities live in `internal/model/conversation.go`. The existing `Session` / `Turn` model is unchanged.

```go
type Conversation struct {
    ID            string
    CreatedAt     time.Time
    EndedAt       *time.Time
    Status        ConversationStatus
    MaxTurns      int
    PerTurnTimeout time.Duration
    OverallTimeout time.Duration
    Terminator    TerminatorSpec
    PeerA         PeerSpec
    PeerB         PeerSpec
    SeedA         string
    SeedB         string
    Opener        PeerSlot
    TurnsConsumed int
}

type ConversationStatus string

const (
    ConvRunning              ConversationStatus = "running"
    ConvCompletedSentinel    ConversationStatus = "completed_sentinel"
    ConvCompletedJudge       ConversationStatus = "completed_judge"
    ConvMaxTurnsHit          ConversationStatus = "max_turns_hit"
    ConvPeerCrashed          ConversationStatus = "peer_crashed"
    ConvPeerBlocked          ConversationStatus = "peer_blocked"
    ConvTimeout              ConversationStatus = "timeout"
    ConvInterrupted          ConversationStatus = "interrupted"
    ConvError                ConversationStatus = "error"
)

type PeerSlot string

const (
    PeerSlotA PeerSlot = "a"
    PeerSlotB PeerSlot = "b"
)

type PeerSpec struct {
    URI     string             // provider:claude-code | clank://... | stdio://
    Options map[string]string  // model, reasoning-effort, cwd, env, etc.
}

type TerminatorSpec struct {
    Kind     string  // "sentinel" | "judge"
    Sentinel string  // literal token (sentinel kind), default "<<END>>"
    JudgeURI string  // bus://<topic> | provider:<id> (judge kind)
}

type ConversationTurn struct {
    ConversationID       string
    Index                int
    From                 PeerSlot
    Envelope             string
    Response             string
    MarkerToken          string
    StartedAt            time.Time
    EndedAt              time.Time
    CompletionConfidence float64
    ParserConfidence     float64
    Warnings             []string
}
```

### Envelope format on the wire

**Turn 1 to the opener** (includes one-shot briefing):

```
<system briefing for this peer + terminator instructions>

[conversation: <conv-id>  turn 1 of 50  from: seed]
<seed text>

When you finish your reply, output the token <<TURN_END_a7f3c1>> on its own line.
```

**Turn N > 1**:

```
[conversation: <conv-id>  turn N of 50  from: peer-<other>]
<other peer's previous response, verbatim, sentinel stripped>

When you finish your reply, output the token <<TURN_END_<random>>> on its own line.
```

### Briefing template (per peer, turn 1 only)

Sentinel mode:

```
You are participating in a multi-turn conversation with another AI agent through Clank.
The other agent's messages will be delivered to you as plain text wrapped in a header
line indicating the turn number and sender. You should reply as if speaking to a
collaborator.

You are: peer-<slot> ("<name from --seed-a/-b or generic>").
Maximum turns in this conversation: <MaxTurns>.

When you believe the conversation has reached its goal or natural end, end your final
reply with the literal token <<END>> on its own line BEFORE the turn-end marker.

After every reply, you MUST output a per-turn marker token as instructed in the
incoming message. This marker tells the broker your turn is complete. If you forget
the marker, your turn will time out.
```

Judge mode is identical except the `<<END>>` instruction is omitted (the judge decides termination silently).

### Bus message envelope

```go
type BusMessage struct {
    ConversationID string
    Topic          string
    Kind           string    // "envelope" | "turn" | "control"
    Payload        []byte    // JSON-encoded type per Kind
    Timestamp      time.Time
}

type ControlMsg struct {
    ConversationID string
    Kind           string // "verdict" | "peer_crashed" | "peer_blocked" | "finalize"
    Slot           PeerSlot // for crashed/blocked
    Reason         string
    Status         ConversationStatus // for verdict/finalize
    Detail         string
}
```

### Storage

The `Store` interface gains two methods (additive):

```go
type Store interface {
    // ... existing single-shot methods unchanged ...
    CreateConversation(c Conversation) error
    UpdateConversation(id string, patch ConversationPatch) error
    AppendConversationTurn(turn ConversationTurn) error
    PeekConversationTurns(id string, lastN int) ([]ConversationTurn, error)
}

type ConversationPatch struct {
    Status        *ConversationStatus
    EndedAt       *time.Time
    TurnsConsumed *int
}
```

**File backend** writes:

- `<logPath>/conversations/<id>.json` (conversation summary)
- `<logPath>/conversations/<id>.jsonl` (turn log, append-only)

**Postgres backend** adds:

```sql
CREATE TABLE conversations (
    conversation_id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    max_turns INT NOT NULL,
    turns_consumed INT NOT NULL,
    terminator_kind TEXT NOT NULL,
    peer_a_uri TEXT NOT NULL,
    peer_b_uri TEXT NOT NULL,
    opener TEXT NOT NULL
);

CREATE TABLE conversation_turns (
    id BIGSERIAL PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    turn_index INT NOT NULL,
    from_peer TEXT NOT NULL,
    envelope TEXT NOT NULL,
    response TEXT NOT NULL,
    marker_token TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NOT NULL,
    completion_confidence REAL,
    parser_confidence REAL,
    warnings JSONB
);

CREATE UNIQUE INDEX conversation_turns_id_index_unique
    ON conversation_turns(conversation_id, turn_index);
```

**Failure policy**: identical to single-shot. Store failures are non-blocking warnings; the Broker still finishes the conversation and returns `FinalResult` even if persistence is broken.

---

## 4. Peer Transport

```go
type PeerTransport interface {
    Start(ctx context.Context, spec PeerSpec, bus Bus, conversationID string, slot PeerSlot) error
    Deliver(ctx context.Context, envelope Envelope) (ConversationTurn, error)
    Stop(ctx context.Context, grace time.Duration) error
}

type Envelope struct {
    ConversationID  string
    TurnIndex       int
    MaxTurns        int
    From            PeerSlot
    Body            string
    MarkerToken     string
    IncludeBriefing bool
    Briefing        string
}
```

### Implementations

#### `providerTransport` (local persistent PTY)

Wraps an existing `Adapter` and a new `PersistentPseudoTerminalProcess`.

```go
type PersistentPseudoTerminalProcess interface {
    PseudoTerminalProcess
    ConverseTurn(
        ctx context.Context,
        envelopeBytes []byte,
        markerToken string,
        perTurnTimeout time.Duration,
    ) (responseBytes []byte, signal CompletionSignal, err error)
}
```

`ConverseTurn` writes the envelope to the PTY, records the buffer offset, and reads forward until one of:

1. Marker token observed on output stream → success.
2. Adapter implements `TurnBoundaryDetector` and `DetectTurnReady` returns true → success with `marker_missing` warning.
3. Per-turn timeout elapsed → publish `peer_crashed` to control, return error.
4. Context cancelled → publish nothing, return error.
5. PTY process exits unexpectedly → publish `peer_crashed`, return error.

The buffer is **windowed by turn boundaries** — turn N+1 reads only from the offset recorded at the end of turn N. Prior turns' chrome noise never contaminates the next turn's extraction.

#### Adapter extension (additive, optional)

```go
type TurnBoundaryDetector interface {
    DetectTurnReady(ctx CompletionContext) bool
}
```

Adapters implement this **optionally**. The persistent PTY runtime asserts the interface at runtime; if absent, it relies entirely on the marker token plus per-turn timeout. Existing `Adapter` interface is unchanged. Single-shot path is unaffected.

#### `clankTransport` (remote clank peer)

- `Start` subscribes to its inbox topic (`conv/<id>/peer-<slot>/in`) on the shared bus.
- `Deliver` publishes the envelope to the *remote* peer's inbox (the remote clank-converse-serve process is the actual subscriber there) and awaits the corresponding `turn` message on `conv/<id>/turn`.
- `Stop` publishes a goodbye control message and unsubscribes.

Requires `--bus nats://...` — `inproc` cannot reach another process.

#### `stdioTransport` (this process's stdin/stdout)

- `Start` opens stdin/stdout, sets up JSONL frame reader.
- `Deliver` writes the envelope as a JSON line on stdout, blocks reading stdin for the response JSON line.
- `Stop` closes the frame reader, prints a goodbye marker.

Frame format:

```json
{"kind":"envelope","conversation":"<id>","turn":3,"from":"peer-a","body":"...","marker":"TURN_END_..."}
{"kind":"response","conversation":"<id>","turn":3,"response":"..."}
```

Lets an external script drive one side of a clank conversation with zero network setup.

### Failure handling table

| Situation | Behavior |
|---|---|
| Marker never appears, no `TurnBoundaryDetector`, per-turn timeout hits | Publish `peer_crashed` to control, broker finalizes `ConvPeerCrashed` |
| `DetectTurnReady` fires but marker absent | Accept turn, attach `marker_missing` warning |
| Marker appears but PTY keeps writing | Truncate response at marker, attach `post_marker_chatter` warning |
| PTY process exits mid-turn | Publish `peer_crashed`, finalize `ConvPeerCrashed` |
| Permission/auth/rate-limit prompt detected | Publish `peer_blocked` with the non-terminal status as detail, finalize `ConvPeerBlocked` |
| `clank://` peer's bus connection drops | Treat as `peer_crashed` after per-turn timeout |
| `stdio://` peer closes its stdin | Treat as `peer_crashed` immediately |

---

## 5. Conversation Broker

```go
type Broker struct {
    Conversation Conversation
    PeerA        PeerTransport
    PeerB        PeerTransport
    Terminator   Terminator
    Bus          Bus
    Store        Store
    Clock        clockwork.Clock
}

func (b *Broker) Run(ctx context.Context) (FinalResult, error)
```

Pseudocode:

```
1. Start PeerA, PeerB in parallel.
2. Subscribe Store and Terminator to conv/<id>/turn and conv/<id>/control.
3. Subscribe Broker itself to conv/<id>/control (drained per loop iteration).
4. active := opener
5. envelope := buildTurn1Envelope(active, conversation.SeedFor(active), newMarker())
6. for {
       turn, err := active.Deliver(ctx, envelope)
       if err != nil { finalize(ConvError, err); return }

       publishTurn(turn)
       conversation.TurnsConsumed += 1

       if conversation.TurnsConsumed >= conversation.MaxTurns {
           finalize(ConvMaxTurnsHit); return
       }

       drainControl(ctx) // non-blocking
       if controlVerdict != nil {
           finalize(controlVerdict.Status, controlVerdict.Reason); return
       }

       active = other(active)
       envelope = buildEnvelope(active, turn.Response, newMarker())

       select {
       case <-ctx.Done():
           finalize(ConvInterrupted); return
       default:
       }
   }
```

### Strict alternation

The Broker is the only entity that decides whose turn it is. Peer transports can never volunteer turns; `Deliver` is the only entry point and the Broker calls it at most once at a time.

### Terminator interface

```go
type Terminator interface {
    Start(ctx context.Context, conv Conversation, bus Bus) error
    Stop(ctx context.Context) error
}

type Verdict struct {
    ConversationID string
    Decision       string  // "continue" | "terminate"
    Reason         string
    Status         ConversationStatus
}
```

Terminators are bus subscribers, not inline checks. They watch `conv/<id>/turn` and publish `Verdict` (wrapped in a `ControlMsg{Kind:"verdict"}`) to `conv/<id>/control` when they decide to terminate.

#### Sentinel terminator

- Pure function. Inspects each `turn.Response` for the configured token (default `<<END>>`).
- If found, publishes `Verdict{Decision:"terminate", Status:ConvCompletedSentinel}`.
- Strips the sentinel from `turn.Response` *before* the broker builds the next envelope so the sentinel never leaks into the other peer's input.
- Zero external dependencies; runs in-process as a bus subscriber.

#### Judge terminator

Configured via `--judge <uri>`. Two URI shapes:

1. **`bus://<topic>`** — a separate process is already subscribed to the bus and publishes `Verdict` messages to `<topic>`. The judge terminator just wires that topic into the broker's control flow. Bring-your-own-judge case. Language-agnostic — Python, n8n, Prefect, anything that can speak NATS can be a judge.

2. **`provider:<id>`** — clank spawns a *third* PTY peer in single-shot mode after every turn (reusing the existing `clank run` machinery), feeds it a fixed judge briefing plus the conversation history so far, parses the response for `CONTINUE` or `TERMINATE: <reason>`, publishes the verdict.

The judge runs **after every turn** on a bounded goroutine. If the judge itself times out or crashes, the broker logs a warning and continues — judge failure is never fatal to the conversation.

### Terminator precedence (when multiple signals fire in the same loop iteration)

1. `peer_crashed` / `peer_blocked` (always wins)
2. Max-turns hard cap
3. Terminator verdict (sentinel or judge)
4. Context cancellation (user interrupt → `ConvInterrupted`)

### Final result

```go
type FinalResult struct {
    Conversation   Conversation
    Turns          []ConversationTurn
    TerminatorNote string
    Warnings       []string
}
```

Streamed output: each `ConversationTurn` is also written to stdout as JSONL during the run (when `--stream-turns` is set, which defaults on for `inproc` bus). Final `FinalResult` is written as a single JSON object on the last line.

---

## 6. Bus Interface and Backends

```go
type Bus interface {
    Publish(ctx context.Context, topic string, msg BusMessage) error
    Subscribe(ctx context.Context, topic string) (<-chan BusMessage, func(), error)
    Close() error
}
```

The Broker, peer transports, store, and terminators only ever talk to this interface.

### `inproc` backend (default)

- `map[string]*channelFanout` guarded by `sync.RWMutex`
- Each `Subscribe` adds a buffered channel to the topic's fanout slice
- `Publish` fan-outs to every subscriber channel non-blocking; full channels drop with a warning (ring-buffer semantics)
- `Close` closes all subscriber channels
- Zero dependencies. Single-process only.

### `nats` backend (opt-in)

- Thin wrapper around `nats.go`
- Topic slashes map to NATS subject dots: `conv/<id>/turn` → `conv.<id>.turn`
- `Publish` calls `nc.Publish(subject, payload)`
- `Subscribe` calls `nc.ChanSubscribe(subject, ch)` and wraps the channel in a `BusMessage` decoder goroutine
- Supports embedded NATS server mode for dev (`nats-server` Go library) and external server URL for production
- Connection options pulled from `--nats-creds`, `--nats-tls`, etc.

### Topic discipline

All bus backends honor the same topics from §2. Peer inboxes have exactly one subscriber expected (the peer transport). Turn and control topics fan out to multiple subscribers freely.

### Backends not in this spec

MQTT, Redis pub/sub, and JetStream durable streams are deferred. Adding any of them is a single new package implementing the `Bus` interface — no broker code changes.

---

## 7. CLI Interface

### `clank converse` (primary entry point)

```bash
clank converse \
    --peer-a provider:claude-code \
    --peer-b provider:codex \
    --seed "Design a rate limiter for a multi-tenant API" \
    --max-turns 20 \
    --terminator sentinel \
    --sentinel "<<END>>" \
    --opener a \
    --per-turn-timeout 300 \
    --overall-timeout 3600 \
    --bus inproc \
    --log-backend file \
    --log-path ./logs \
    --working-directory . \
    --stream-turns
```

Asymmetric seeds and per-peer options:

```bash
clank converse \
    --peer-a provider:claude-code \
    --peer-a-opt model=claude-sonnet-4-6 \
    --peer-a-opt cwd=/repo/backend \
    --peer-b provider:codex \
    --peer-b-opt model=gpt-5 \
    --peer-b-opt reasoning-effort=high \
    --seed-a "You are the backend architect. Propose the design." \
    --seed-b "You are the security reviewer. Stress-test the design." \
    --opener a \
    --terminator judge \
    --judge provider:claude-code \
    --bus nats://localhost:4222
```

Remote peer over NATS:

```bash
# Machine 1 (broker side)
clank converse \
    --peer-a provider:claude-code \
    --peer-b clank://conv-abc123/peer-b \
    --seed "..." \
    --bus nats://shared-broker:4222

# Machine 2 (peer side)
clank converse-serve \
    --conversation conv-abc123 \
    --slot b \
    --peer provider:codex \
    --bus nats://shared-broker:4222
```

stdio peer:

```bash
clank converse \
    --peer-a provider:claude-code \
    --peer-b stdio:// \
    --seed "..." \
    --bus inproc
```

### `clank converse-serve` (passive peer)

Attaches a local PTY peer to a conversation owned by another clank process. No broker runs on the serve side.

### `clank converse-tail` (live observer)

```bash
clank converse-tail \
    --conversation conv-abc123 \
    --bus nats://shared-broker:4222 \
    --format jsonl
```

Pure bus subscriber. Prints turns and control messages as they appear. Only meaningful with an external bus.

### Validation

- Exactly one of `--seed` or (`--seed-a` + `--seed-b`)
- `--terminator sentinel` requires `--sentinel` (defaults to `<<END>>`)
- `--terminator judge` requires `--judge`
- `--bus inproc` forbids `clank://` peers and forbids `converse-tail`
- `--max-turns` defaults to 50, minimum 1, maximum 500 (the 500 cap is hard-coded; raising it requires a code change)
- `--per-turn-timeout` defaults to 300s
- `--overall-timeout` defaults to `max-turns * per-turn-timeout`

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Conversation reached a terminal status (sentinel, judge, max-turns, interrupted) |
| 1 | Runtime error (peer crash, spawn failure, bus error) |
| 2 | Configuration error |

JSON `FinalResult` is always written to stdout regardless of exit code.

---

## 8. Project Layout

```
clank/
    cmd/
        clank/
            main.go                         # existing + new converse routing
    internal/
        cli/
            run.go                          # existing
            peek.go                         # existing
            converse.go                     # NEW
            converse_serve.go               # NEW
            converse_tail.go                # NEW
        orchestrator/                       # existing single-shot, unchanged
        conversation/                       # NEW
            broker.go                       # Broker state machine
            envelope.go                     # envelope building + marker generation
            briefing.go                     # briefing template
            result.go                       # FinalResult
        peer/                               # NEW
            transport.go                    # PeerTransport interface
            provider/
                provider.go                 # local PTY peer transport
                persistent.go               # PersistentPseudoTerminalProcess impl
            clankremote/
                clankremote.go              # clank:// peer transport
            stdio/
                stdio.go                    # stdio:// peer transport
        terminator/                         # NEW
            terminator.go                   # Terminator interface + Verdict
            sentinel.go                     # sentinel terminator
            judge.go                        # judge terminator (bus + provider modes)
        bus/                                # NEW
            bus.go                          # Bus interface + BusMessage + ControlMsg
            inproc/
                inproc.go                   # in-process channel backend
            nats/
                nats.go                     # NATS backend
        adapter/
            adapter.go                      # + TurnBoundaryDetector optional interface
            claudecode/
                adapter.go                  # + DetectTurnReady (optional)
            codex/
                adapter.go                  # + DetectTurnReady (optional)
        terminal/
            runtime.go                      # existing
            process.go                      # existing
            persistent.go                   # NEW: PersistentPseudoTerminalProcess
        store/
            store.go                        # + conversation methods
            file/
                file_store.go               # + conversation persistence
            postgres/
                postgres_store.go           # + conversation persistence
                migrations/
                    002_conversations.sql   # NEW
        model/
            session.go                      # existing
            turn.go                         # existing
            result.go                       # existing
            conversation.go                 # NEW
            errors.go                       # + conversation error codes
```

### Standards alignment

- **Many small files**: every new file targets <400 lines; package boundaries split by domain
- **Immutability**: `Conversation`, `ConversationTurn`, `Envelope`, `BusMessage`, `ControlMsg` are value structs; broker builds new envelopes per turn
- **High cohesion / low coupling**: `Bus`, `PeerTransport`, `Terminator`, `TurnBoundaryDetector` are single-purpose interfaces; broker depends on interfaces only
- **Feature/domain organization**: top-level packages named by capability (`conversation`, `peer`, `terminator`, `bus`), not by layer (`handlers`, `services`)
- **Error handling**: per-turn timeouts, peer-crash control messages, judge-failure-non-fatal, warnings collected on `FinalResult`
- **Boundary validation**: CLI parses and validates flags before constructing Broker; envelope construction asserts marker presence
- **Single-shot path untouched**: every adapter extension is optional via runtime interface assertion; no existing public type changes shape

---

## 9. Error Model

Additive to existing error codes:

```go
const (
    ErrorBus            ErrorCode = "E_BUS"
    ErrorPeerCrash      ErrorCode = "E_PEER_CRASH"
    ErrorPeerBlock      ErrorCode = "E_PEER_BLOCK"
    ErrorJudgeUnreachable ErrorCode = "E_JUDGE_UNREACHABLE"
    ErrorMarkerMissing  ErrorCode = "E_MARKER_MISSING"
)
```

| Error | Conversation Created? | Turns Logged? | Result in JSON? |
|---|---|---|---|
| `E_CONFIG` | No | No | No (stderr + exit 2) |
| `E_BUS` | Depends on when bus failed | Up to failure point | Best-effort, status `error` |
| `E_PEER_CRASH` | Yes | Through last completed turn | Status `peer_crashed`, partial transcript |
| `E_PEER_BLOCK` | Yes | Through last completed turn | Status `peer_blocked`, last buffer tail in detail |
| `E_JUDGE_UNREACHABLE` | Yes | All turns | Status reflects how the conversation actually ended (judge failure is non-fatal); warning in `Warnings` |
| `E_MARKER_MISSING` | Yes | All turns | Per-turn warning, conversation continues |
| `E_STORE` | Depends | Depends | Result still returned; store failure logged to stderr |

---

## 10. Testing Strategy

### Unit tests

- **Broker state machine**: scripted `MockPeerTransport` returning canned turns, verify alternation, max-turns enforcement, terminator precedence, control-message draining.
- **Sentinel terminator**: feed turns containing/missing the sentinel, verify verdict + stripping behavior.
- **Judge terminator (bus mode)**: publish verdicts manually, verify broker honors them.
- **Envelope builder**: turn-1 vs turn-N rendering, marker token uniqueness, briefing inclusion.
- **InprocBus**: fan-out semantics, multiple subscribers, full-channel drop behavior, close cleanup.
- **PersistentPseudoTerminalProcess**: scripted PTY output (recorded fixtures), verify marker detection, idle-prompt fallback, timeout behavior, post-marker truncation.
- **Adapter `DetectTurnReady`** for claudecode and codex: recorded transcripts of multi-turn sessions, assert correct ready signals.
- **Conversation persistence**: round-trip `Conversation` and `ConversationTurn` through file backend and Postgres backend.
- **CLI validation**: every flag combination, conflicting flags, missing required flags.

### Integration tests

- **Two mock-agent processes** scripted into a 3-turn exchange with sentinel termination; verify full loop, JSONL stream, final result.
- **Max-turns hard cap**: mock agents that never emit sentinel; verify broker terminates at the cap.
- **Peer crash mid-conversation**: kill one mock agent on turn 2; verify `ConvPeerCrashed` and partial transcript.
- **Judge mode (bus)**: third process publishing verdicts; verify broker honors them.
- **NATS backend**: testcontainer with `nats-server`; full conversation across two clank processes (`converse` + `converse-serve`).
- **stdio peer**: external script driving peer-b via JSONL frames; verify round trip.

### Contract tests

- **JSON schema** for `FinalResult`, `ConversationTurn`, and `BusMessage` validated on every test run. Stability guarantee for downstream consumers.
- **Topic discipline**: assert that the broker only ever publishes/subscribes to the documented topic set.

### Real provider acceptance (manual)

- `claudecode ↔ codex` at `--max-turns 5` with a known-good seed, sentinel terminator, file backend. Document the expected JSON shape and a sample transcript.

### Coverage target

80% line coverage minimum per `~/.claude/rules/testing.md`. Broker, terminator, bus, and envelope code expected to be at or above 95%.

---

## 11. Milestones

### M1 — Conversation Core (in-process only)

- `Bus` interface + inproc backend
- `Conversation` model + file-backend persistence
- `Broker` state machine
- Sentinel terminator
- `providerTransport` + `PersistentPseudoTerminalProcess`
- Optional `TurnBoundaryDetector` for claudecode and codex
- `clank converse` CLI with `provider:` × `provider:` only
- Unit + integration tests with mock agents
- Manual `claudecode ↔ codex` smoke test

### M2 — Pluggable Termination and Postgres

- Judge terminator (bus mode and provider mode)
- Postgres conversation persistence + migration
- Stream-turns JSONL output
- Test harness for judge integrations

### M3 — Distributed and Observable

- NATS bus backend (external + embedded)
- `clankTransport` (`clank://` peer)
- `clank converse-serve` subcommand
- `clank converse-tail` subcommand
- Integration tests across two clank processes via NATS testcontainer
- Documentation: bring-your-own-judge recipe, n8n/Prefect subscription example

### M4 — stdio peer and polish

- `stdioTransport` + JSONL frame protocol
- Conversation resume / replay (load from store, not yet supported)
- Performance pass on persistent PTY buffer windowing

### Future (out of scope)

- More than two peers
- Free-form turn-taking and peer addressing
- Automatic peer-crash recovery
- MQTT bus backend (interface ready, implementation deferred)
- JetStream durable conversation log
- Hosted/multi-tenant brokering

---

## 12. Open Questions for Implementation Plan

These are flagged for the implementation plan to resolve, not for this spec:

1. **Marker token format**: literal `<<TURN_END_<6 hex chars>>>` or wrap in ANSI invisible escape? Hex chars are simpler and human-debuggable; ANSI is harder for the model to forget but harder to debug. Lean toward hex, decide during prototype.
2. **NATS embedded mode default**: should `clank converse --bus nats` without a URL spin up an embedded server automatically? Convenient but adds startup latency. Probably yes for dev ergonomics, off by default in CI.
3. **Briefing template overrides**: should users be able to provide a custom briefing file (`--briefing-template path.txt`)? Yes, but defer to M2 — ship the canonical template first.
4. **Per-peer log paths**: should each peer's persistent PTY raw stream go to its own file under `<logPath>/conversations/<id>/peer-<slot>.raw.jsonl`? Yes; reuses the existing `debugRaw` scheme.
5. **Judge prompt template**: where does the canned judge briefing live for `provider:` mode? Suggest `internal/terminator/judge_briefing.txt` embedded via `embed.FS`.
