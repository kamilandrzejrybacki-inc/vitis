# Vitis Event API & Vitis UI Design

- **Status**: Draft
- **Date**: 2026-04-09
- **Scope**: HTTP event API for Vitis (`vitis serve`) + SvelteKit dashboard (`vitis-ui`)
- **Primary inputs**:
  - `2026-04-08-vitis-n-peer-conversations-design.md` (N-peer model)
  - `2026-04-07-vitis-a2a-conversations-design.md` (A2A conversation model)
  - Agent Architecture Designer (https://github.com/TheJacksonCode/Agent-Architecture) — UI patterns
  - Distillery knowledge base — SSE proxy config, Hermes API patterns

---

## 1. Problem Statement

Vitis conversations are observable only through the CLI (`vitis peek`, terminal output). There is no way to:

- Watch an active conversation in real time
- Browse stored sessions and conversations visually
- Inspect turn-by-turn message flow with metadata (confidence, warnings, turn reason)
- Visualize N-peer topology and message routing
- See termination events (sentinel detection, judge verdicts) as they happen

This spec introduces two deliverables:

1. **Vitis Event API** — a new `vitis serve` subcommand exposing REST + SSE endpoints over HTTP
2. **Vitis UI** — a SvelteKit dashboard consuming that API

---

## 2. Vitis Event API

### 2.1 Architecture

New `vitis serve --port 8090` subcommand. Starts an HTTP server with direct Go-level access to the Store and Bus interfaces.

New packages:
- `internal/api/server.go` — HTTP server setup, route registration, graceful shutdown via OS signal
- `internal/api/handlers_rest.go` — REST handlers wrapping Store interface calls
- `internal/api/handlers_sse.go` — SSE handlers subscribing to Bus topics, forwarding as typed events
- `internal/api/middleware.go` — CORS (localhost origins), request logging, optional API key auth
- `internal/cli/serve.go` — CLI wiring: creates Store + Bus, passes to API server

### 2.2 REST Endpoints (historical data from Store)

All under `/api/v1/` prefix.

| Method | Path | Description | Store Method |
|--------|------|-------------|--------------|
| `GET` | `/status` | Active conversations, SSE stream count, uptime, store backend | (runtime state) |
| `GET` | `/sessions` | List stored sessions | `Store` iteration |
| `GET` | `/sessions/:id` | Session detail with metadata | `Store` lookup |
| `GET` | `/sessions/:id/turns` | Paginated turns for single-shot session | `PeekTurns` |
| `GET` | `/conversations` | List stored conversations with peer info | `Store` iteration |
| `GET` | `/conversations/:id` | Conversation detail: peers, terminator, status | `Store` lookup |
| `GET` | `/conversations/:id/turns` | Paginated conversation turns | `PeekConversationTurns` |

Query parameters for list endpoints: `?status=`, `?limit=`, `?offset=`.

Response envelope:

```json
{
  "data": [...],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

Error envelope:

```json
{
  "error": "not_found",
  "detail": "conversation abc123 does not exist"
}
```

### 2.3 SSE Endpoints (live data from Bus)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/conversations/:id/stream` | Multiplexed SSE stream for one conversation |
| `GET` | `/conversations/stream` | Lifecycle events for all conversations |

#### Per-conversation stream (`/conversations/:id/stream`)

Multiplexes three Bus topics into typed SSE events:

```
event: turn
data: {"conversation_id":"...","index":3,"from_id":"claude-a","to_id":"codex-b","reason":"addressed","response":"...","completion_confidence":0.95,"started_at":"...","ended_at":"..."}

event: envelope
data: {"conversation_id":"...","turn_index":4,"from_id":"codex-b","to_id":"claude-a","body":"...","marker_token":"<<END>>"}

event: control
data: {"conversation_id":"...","kind":"verdict","verdict":{"decision":"complete","reason":"sentinel detected","status":"completed"}}
```

Source Bus topics:
- `turn.<conv_id>` → `event: turn`
- `envelope.in.<peer_id>` (for all peers) → `event: envelope`
- `control.<conv_id>` → `event: control`

#### Lifecycle stream (`/conversations/stream`)

```
event: conversation_started
data: {"conversation_id":"...","peers":["claude-a","codex-b"],"status":"running"}

event: conversation_ended
data: {"conversation_id":"...","status":"completed","turns_consumed":12}

event: status_changed
data: {"conversation_id":"...","status":"errored","reason":"peer timeout"}
```

### 2.4 Live Event Delivery Strategy

The in-proc Bus only works when `vitis serve` and `vitis converse` share the same process. For v1:

- `vitis serve` opens the Store in read-only mode for REST queries
- For SSE, `vitis serve` watches the Store's file backend using filesystem notifications (`fsnotify`). When a new conversation turn file is written by a concurrent `vitis converse` process, the watcher detects it, reads the new data, and pushes it as an SSE event.
- When NATS arrives (Plan 4), SSE handlers switch from fsnotify to Bus subscription. The SSE event format stays identical — the transport changes, the API contract doesn't.

### 2.5 Health & Status

`GET /health` — at root, no `/api/v1/` prefix (k8s probe compatible):

```json
{
  "status": "ok",
  "store": "file",
  "store_path": "/home/user/.vitis/sessions"
}
```

`GET /api/v1/status` — operational status (mirrors Hermes `/api` pattern):

```json
{
  "active_conversations": 2,
  "active_sse_streams": 3,
  "uptime_seconds": 1842,
  "store_backend": "file",
  "version": "0.5.0"
}
```

### 2.6 Security

- `--api-key` optional flag on `vitis serve`. When set, all `/api/v1/` routes require `Authorization: Bearer <key>` header. Health endpoint remains unauthenticated.
- CORS allows `localhost` origins only by default. `--cors-origin` flag for explicit override.
- SSE connections capped at 10 concurrent streams (prevents resource exhaustion from forgotten browser tabs).

### 2.7 Deployment Notes

When `vitis serve` sits behind Caddy reverse proxy on the homelab, the Caddy block must include:

```caddyfile
reverse_proxy localhost:8090 {
    flush_interval -1
    transport http {
        read_timeout 0
    }
}
```

Without this, Caddy buffers SSE frames and drops long-lived connections. This is a known issue (Distillery knowledge base entry 32.01, also hit with Hermes streaming).

---

## 3. Vitis UI

### 3.1 Tech Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Framework | SvelteKit | Runes ($state/$derived) excel at real-time SSE streaming — surgical DOM updates, 8ms filter vs React's 47ms |
| Graph canvas | Svelte Flow | First-class xyflow port for Svelte, maintained by xyflow team |
| UI components | shadcn-svelte | Production-ready, updated for Svelte 5 + Tailwind 4 |
| Styling | Tailwind CSS | Consistent with infravision ecosystem |
| Language | TypeScript | Type safety for API contract |
| Bundle | ~5KB runtime | 9x smaller than React equivalent |

### 3.2 Project Structure

```
vitis-ui/
├── src/
│   ├── lib/
│   │   ├── api/
│   │   │   ├── rest.ts              — typed fetch wrappers for all REST endpoints
│   │   │   ├── sse.ts               — EventSource wrapper with typed events, reconnect
│   │   │   ├── types.ts             — TypeScript models (readonly, mirroring Go structs)
│   │   │   └── health.ts            — health + status polling
│   │   ├── stores/
│   │   │   ├── sessions.svelte.ts   — $state for session/conversation lists
│   │   │   ├── conversation.svelte.ts — $state for active conversation, turns, peer states
│   │   │   └── connection.svelte.ts — SSE connection state, reconnect status
│   │   ├── components/
│   │   │   ├── ui/                  — shadcn-svelte primitives (card, badge, dialog, etc.)
│   │   │   ├── SessionCard.svelte
│   │   │   ├── TurnCard.svelte
│   │   │   ├── PeerStatusCard.svelte
│   │   │   ├── MetricsBar.svelte
│   │   │   ├── TopologyCanvas.svelte
│   │   │   ├── VerdictOverlay.svelte
│   │   │   └── StatusBadge.svelte
│   │   └── utils/
│   │       ├── format.ts            — time formatting, ID truncation, confidence display
│   │       └── peer-colors.ts       — deterministic color assignment per peer ID
│   ├── routes/
│   │   ├── +layout.svelte           — shell with sidebar nav + connection indicator
│   │   ├── +page.svelte             — redirects to /sessions
│   │   ├── sessions/
│   │   │   └── +page.svelte         — Session Browser (View 1)
│   │   ├── conversations/
│   │   │   ├── +page.svelte         — Conversation Browser (View 1)
│   │   │   └── [id]/
│   │   │       ├── +page.svelte     — Turn Timeline (View 2)
│   │   │       ├── live/
│   │   │       │   └── +page.svelte — Live Monitor (View 3)
│   │   │       └── topology/
│   │   │           └── +page.svelte — Peer Topology (View 4)
│   ├── app.css                      — Tailwind base + theme tokens
│   └── app.html
├── static/
├── svelte.config.js
├── tailwind.config.js
├── vite.config.ts
├── tsconfig.json
└── package.json
```

### 3.3 Views

#### View 1: Session & Conversation Browser

Card grid listing stored sessions and conversations. Each card:

- ID (truncated), provider badge, status badge (color-coded)
- Duration, turn count, timestamp
- Click navigates to Turn Timeline for that item

Filters: status (running/completed/errored), search by ID.

Data source: REST `GET /api/v1/sessions` + `GET /api/v1/conversations`. Live updates via `GET /api/v1/conversations/stream` SSE — new/changed conversations appear without refresh.

#### View 2: Turn Timeline

Chronological vertical timeline for a selected session or conversation. Each turn card:

- **From -> To**: peer IDs with colored badges
- **Envelope body**: collapsible, shows what was sent
- **Response text**: collapsible, shows what came back
- **TurnReason badge**: addressed / round-robin / fallback
- **Confidence meters**: CompletionConfidence + ParserConfidence as small bars
- **Warnings**: highlighted if present
- **Duration**: startedAt -> endedAt

Data source: REST for historical turns. If conversation is active, SSE stream appends new turns in real time. Auto-scroll to bottom with "pinned to bottom" toggle.

#### View 3: Live Conversation Monitor

Full-width dashboard for an active conversation:

- **Peer status row** — one card per peer: ID, provider, state (idle/speaking/done), turn count
- **Active turn indicator** — which peer is currently processing an envelope
- **Metrics bar** — elapsed time, turns consumed / max turns progress bar, current turn duration
- **Inline timeline** — compact View 2 below the status row, auto-scrolling

Data source: SSE stream for the selected conversation. Peer states derived from envelope/turn/control events:
- `envelope` event with `to_id=X` → peer X state = "speaking"
- `turn` event with `from_id=X` → peer X state = "idle", turn count++
- `control` event with verdict → peer states = "done"

#### View 4: Peer Topology Canvas

Svelte Flow graph of conversation participants:

- **Nodes**: one per peer — shows peer ID, provider icon, status color ring
- **Edges**: directed, animated on envelope delivery (pulse along edge when turn flows A -> B)
- **Turn policy visualization**: round-robin shows circular arrow overlay; addressed shows directed edges based on `<<NEXT>>` trailers parsed from `Decision.Parsed`
- **Layout**: auto-layout for 2 peers (horizontal), circular for 3+ peers

Data source: REST for peer list and topology structure, SSE for live edge animations.

#### View 5: Verdict/Termination Overlay

Global modal (in `+layout.svelte`) triggered by `control` SSE events carrying verdict data:

- Verdict decision badge: continue / complete / timeout / error
- Reason text
- For sentinel: which peer triggered it, matched token
- For judge (future): judge evaluation summary, countdown timer
- Final conversation status

Appears over whichever view is active. Dismissible. Event logged in timeline.

### 3.4 Data Flow & State Management

#### API Client Layer (`src/lib/api/`)

- `rest.ts` — typed fetch wrappers returning discriminated responses. Errors return `{error, detail}`.
- `sse.ts` — EventSource wrapper parsing typed SSE events (`turn`, `envelope`, `control`, `conversation_started`, `conversation_ended`, `status_changed`) into a discriminated union. Handles reconnection with exponential backoff (1s, 2s, 4s, max 30s).
- `types.ts` — TypeScript interfaces with `readonly` properties, mirroring Go model structs: `Session`, `Conversation`, `ConversationTurn`, `Envelope`, `ControlMsg`, `Verdict`, `PeerParticipant`, `RunStatus`, `ConversationStatus`, `PeerSpec`, `TerminatorSpec`.
- `health.ts` — polls `GET /health` + `GET /api/v1/status` on 10s interval for connection indicator.

#### Svelte 5 State (`src/lib/stores/`)

- `sessions.svelte.ts` — `$state` rune holding session/conversation list arrays. Updated by REST fetch on mount + SSE lifecycle stream for incremental updates. `$derived` for filtered/sorted views.
- `conversation.svelte.ts` — `$state` for currently-viewed conversation: metadata, turns array, peer states map (`Map<PeerID, PeerState>`). SSE events mutate state surgically:
  - `turn` event → `turns.push(newTurn)`, update peer state
  - `envelope` event → set target peer state to "speaking"
  - `control` event → update conversation status, trigger verdict overlay
  - `$derived`: active peer, progress percentage, elapsed time, turn rate
- `connection.svelte.ts` — SSE connection state (connected/reconnecting/disconnected), reconnect attempt count, last event timestamp.

No global state library needed. Svelte 5 runes handle all reactivity. Stores are plain `.svelte.ts` modules imported where needed.

#### Reactive flow:

```
SSE event arrives
  -> sse.ts parses into typed event
  -> store $state mutation (e.g. turns.push(newTurn))
  -> Svelte runes trigger surgical DOM update
  -> only the new TurnCard renders, timeline scrolls
```

#### Configuration:

- `PUBLIC_VITIS_API_URL` env var (default `http://localhost:8090`)
- Set via SvelteKit `$env/static/public` at build time

### 3.5 Connection Indicator

Sidebar footer shows connection status to Vitis API:

- **Green dot** — connected, receiving events. Shows active conversation count + SSE stream count from `/api/v1/status`.
- **Yellow dot** — reconnecting. Shows attempt count.
- **Red dot** — disconnected. Shows last successful connection time.

Polls `/health` every 10 seconds. SSE reconnect is independent (exponential backoff).

### 3.6 Design Constraints

- All SSE endpoints are **read-only observation**. No write-through-SSE. This avoids the Hermes session freeze problem (injecting into active sessions causes context corruption). If write endpoints are added later, use the dedicated bridge session pattern from Hermes.
- TypeScript types are the **source of truth** for the API contract on the frontend. They must be manually kept in sync with Go model structs. When the Go models change, `types.ts` must be updated.
- SvelteKit `load` functions handle REST data fetching on initial page load. SSE subscribes client-side only (no SSR for streaming).

---

## 4. Store Interface Extensions

The existing `Store` interface needs two additions for the REST API:

```go
// List methods (new)
ListSessions(ctx context.Context, filter SessionFilter) ([]Session, int, error)
ListConversations(ctx context.Context, filter ConversationFilter) ([]Conversation, int, error)
GetSession(ctx context.Context, sessionID string) (*Session, error)
GetConversation(ctx context.Context, conversationID string) (*Conversation, error)
```

Where:

```go
type SessionFilter struct {
    Status *RunStatus
    Limit  int
    Offset int
}

type ConversationFilter struct {
    Status *ConversationStatus
    Limit  int
    Offset int
}
```

The file store implementation reads from its existing directory structure. The Postgres store (Plan 3) implements these as SQL queries.

---

## 5. Filesystem Watcher (v1 Live Events)

For v1, `vitis serve` and `vitis converse` run as separate processes. The in-proc Bus cannot bridge them. Instead:

- `internal/api/watcher.go` — uses `fsnotify` to watch the Store's file directory
- When a new turn file is written by `vitis converse`, the watcher:
  1. Detects the file creation/modification event
  2. Reads the new turn data from the file
  3. Pushes it as an SSE event to all subscribed clients for that conversation
- Conversation lifecycle events (start/end) detected by watching conversation metadata files

Latency: fsnotify delivers events within ~10ms on Linux (inotify). Acceptable for a dev tool.

When NATS arrives (Plan 4), `watcher.go` is replaced by a Bus subscriber. The SSE event format is unchanged.

---

## 6. Implementation Phases

### Phase 1: Vitis Event API foundation
- `internal/api/` package with server, REST handlers, middleware
- Store interface extensions (ListSessions, ListConversations, GetSession, GetConversation)
- File store implementation of new methods
- `vitis serve` CLI command
- `GET /health`, `GET /api/v1/status`
- REST endpoints for sessions and conversations

### Phase 2: SSE live events
- Filesystem watcher (`internal/api/watcher.go`)
- SSE handler for per-conversation stream
- SSE handler for lifecycle stream
- Connection management (max concurrent streams)

### Phase 3: Vitis UI scaffold
- SvelteKit project setup with Svelte Flow, shadcn-svelte, Tailwind
- API client layer (`rest.ts`, `sse.ts`, `types.ts`, `health.ts`)
- Svelte stores (`sessions.svelte.ts`, `conversation.svelte.ts`, `connection.svelte.ts`)
- Shell layout with sidebar nav and connection indicator

### Phase 4: UI views
- Session & Conversation Browser (View 1)
- Turn Timeline (View 2)
- Live Conversation Monitor (View 3)
- Peer Topology Canvas (View 4)
- Verdict/Termination Overlay (View 5)

### Phase 5: Integration & polish
- End-to-end testing with real `vitis converse` sessions
- Caddy proxy configuration documentation
- Performance testing with high-frequency turn streams

---

## 7. Decisions Log

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | REST + SSE hybrid, not WebSocket | SSE is simpler (unidirectional), works through proxies, sufficient for read-only observation |
| D2 | `vitis serve` as subcommand inside Vitis binary | Direct Go-level access to Store and Bus. Single binary distribution. |
| D3 | fsnotify for v1 live events | Bridges separate processes without external infrastructure. Replaced by NATS in Plan 4. |
| D4 | SvelteKit + Svelte Flow, not React | Runes excel at real-time streaming (surgical DOM updates). Svelte Flow is first-class xyflow port. 9x smaller bundle. vitis-ui is a separate project — no code sharing penalty. |
| D5 | shadcn-svelte for UI components | Production-ready, Svelte 5 + Tailwind 4 compatible. Consistent component philosophy with infravision's shadcn-ui. |
| D6 | No write-through-SSE | Avoids Hermes session freeze pattern. All SSE is read-only observation. |
| D7 | API versioning with `/api/v1/` prefix | Consistent with Distillery's API design. |
| D8 | Caddy `flush_interval -1` documented | Known issue from Distillery KB — SSE connections drop without it. |
| D9 | All five views in v1 | Full dashboard from start: session browser, turn timeline, live monitor, topology canvas, verdict overlay. |
| D10 | Max 10 concurrent SSE streams | Prevents resource exhaustion from forgotten browser tabs. |
