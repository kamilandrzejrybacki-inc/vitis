# Vitis N-Peer Conversations — Design Spec

**Date:** 2026-04-08
**Status:** Draft, awaiting review
**Supersedes parts of:** `2026-04-07-vitis-a2a-conversations-design.md` (extends the 2-peer model to N peers)
**Motivates:** future `vitis-ui` web UI spec (depends on this landing)

## Context

Vitis today runs `vitis converse` as strict alternation between exactly two peers
declared via `--peer-a` / `--peer-b`. This spec extends the core to support
N-peer (N ≥ 2, capped at 16) conversations with **addressed** turn routing: each
peer ends its reply with a structured `<<NEXT: peer-id>>` trailer that names the
next speaker. When the trailer is missing, invalid, or addresses the current
speaker, routing falls back to round-robin in declared order.

The UI for driving these conversations is a separate, downstream spec. This
document is strictly about the core runtime, CLI surface, persistence schema,
and test strategy required to make N-peer conversations real in the `vitis`
binary.

## Scope

### In scope

- N-peer conversations in `vitis converse` for 2 ≤ N ≤ 16.
- Addressed turn routing via `<<NEXT: peer-id>>` trailer, round-robin fallback
  in declared peer order.
- Repeatable `--peer id=...,provider=...` CLI surface.
- Repeatable `--seed id=<id>,content="..."` flag with broadcast form
  `--seed content="..."`.
- Back-compat aliases: `--peer-a`, `--peer-b`, `--peer-a-opt`, `--peer-b-opt`,
  `--seed-a`, `--seed-b`, `--opener a|b` continue to work and map to ids `a`/`b`.
- Hard cutover to persistence schema v2, with a read-only v1 compat reader for
  `vitis peek` so existing sessions remain inspectable.
- Bus interface abstraction (`internal/bus`) with an inproc implementation keyed
  by peer id. NATS impl is explicitly deferred.
- Enriched per-turn records with `from_id`, `to_id`, `reason`, `next_id_parsed`,
  `fallback_used`.

### Non-goals (deferred)

- NATS bus implementation. Interface only; impl is Plan 4.
- Judge terminator with N peers (Plan 3 concern). Only `--terminator sentinel`
  is supported here.
- Human-in-the-loop / interactive message injection. Belongs to the web UI spec.
- Quorum END, moderator-driven turn policies, tool-based addressing.
- The `vitis-ui` web UI itself. Separate spec, depends on this one.
- Schema v1 → v2 on-disk migration tooling. v1 sessions remain v1 on disk.

## Decisions log

| Decision | Choice | Rationale |
|---|---|---|
| Turn-ordering policy | Addressed / reactive with round-robin fallback | Matches group-chat UX; round-robin guarantees forward progress |
| Addressing mechanism | Structured trailer `<<NEXT: peer-id>>` on the last non-empty line | Composes with existing `<<END>>` sentinel; robust against code-fence decoys |
| Fallback on missing/invalid address | Warn + round-robin in declared order | Forward progress over strictness |
| Peer declaration | Repeatable `--peer id=...,provider=...` | Explicit ids; required because ids appear in trailers |
| Seed declaration | Repeatable `--seed id=<id>,content="..."`; broadcast `--seed content="..."` | Static flag surface; no dynamic Cobra registration |
| Back-compat | `--peer-a/b`, `--peer-a/b-opt`, `--seed-a/b`, `--opener a|b` as aliases to ids `a`/`b` | Zero-churn for existing scripts and tests |
| Terminator | Any-peer `<<END>>` wins (unchanged) | Predictable; existing docs and tests stay valid |
| Unknown addressee | Warn + round-robin fallback | Matches F1 philosophy; avoids hard failures mid-conversation |
| Persistence | Hard cutover to schema v2; v1 read-only compat in `vitis peek` | Clean model; no dual-write complexity |
| Bus | Interface abstraction + inproc impl | Unblocks N-peer today; NATS drops into same interface later |
| Concurrency | Single turn in flight (unchanged) | Group-chat visuals do not imply parallelism |

## Architecture

### Peer identity

Each peer has a stable string id matching `^[a-z][a-z0-9_-]{0,31}$`. Ids must be
unique within a conversation. The ids `a` and `b` are reserved aliases produced
by the back-compat flag layer; users MAY still declare them explicitly via
`--peer id=a,...`, but mixing `--peer-a` with `--peer id=a,...` in the same
invocation is a hard error.

### Bus interface (new package `internal/bus`)

```go
type Envelope struct {
    ConvID  string
    TurnSeq int
    FromID  string            // "" for the opener turn
    ToID    string            // target peer id
    Payload []byte
    Meta    map[string]string
}

type Bus interface {
    Publish(ctx context.Context, env Envelope) error
    Subscribe(peerID string) (<-chan Envelope, func(), error) // cancel func unsubscribes
    Close() error
}
```

The inproc implementation holds one buffered channel per subscribed peer id and
routes `Publish` calls by `ToID`. Double-subscribe is rejected. The returned
cancel func removes the subscription and closes its channel.

A future NATS implementation lands behind the same interface using subject
`vitis.conv.<conv_id>.<peer_id>` and is out of scope here.

### Turn scheduler (new package `internal/conversation/scheduler`)

The scheduler owns the turn loop. It depends on a `TurnPolicy` interface:

```go
type TurnPolicy interface {
    Next(current string, reply string, peers []string) (next string, parsed *string, fallback bool)
}
```

Ship one implementation, `AddressedPolicy`, which parses the `<<NEXT: id>>`
trailer (see *Addressing parser*) and falls back to round-robin over `peers`
skipping `current`.

### Conversation runtime wiring

```
CLI flags → ConvConfig{ peers[], seeds{}, opener, limits, terminator, ... }
          → Runtime{ bus, scheduler, policy, providers{}, persistence }
          → loop:
              prompt = seed[opener]
              current = opener
              while !done:
                  publish envelope to current
                  reply = providers[current].Turn(prompt)
                  record turn
                  if sentinel_hit: done
                  next, parsed, fallback = policy.Next(current, reply, peer_ids)
                  prompt = reply
                  current = next
```

### Module layout delta

- `internal/bus/` — **new**. `bus.go` (interface, Envelope), `inproc.go` (impl),
  `inproc_test.go`.
- `internal/conversation/scheduler/` — **new**. `policy.go` (interface),
  `addressed.go` (AddressedPolicy + trailer parser), `loop.go` (turn loop),
  tests.
- `internal/conversation/envelope.go` — **modified (schema break)**. Drop
  `PeerA` / `PeerB` fields, add `FromID` / `ToID`.
- `internal/cli/converse.go` — **rewritten**. Repeatable `--peer`, `--seed id=...`,
  back-compat alias translation layer, validation.
- `internal/persistence/` — **modified**. v2 writer, v1 read-only compat under
  `persistence/v1compat/`.
- `mockagent/` — **extended**. `--next-trailer <id>`, `--no-trailer`,
  `--bad-trailer <value>` directives for scripted integration tests.

## CLI surface

### New flags

```
--peer id=<id>,provider=<provider>[,model=...,reasoning-effort=...,cwd=...,home=...,env_KEY=VAL]
        repeatable, 2..16 peers; ids must be unique and match ^[a-z][a-z0-9_-]{0,31}$

--seed id=<peer_id>,content="..."   repeatable, one per peer; mutually exclusive with broadcast form
--seed content="..."                broadcast; same seed sent to every peer

--opener <id>                       peer that speaks first; default = first --peer declared
```

Unchanged flags (identical semantics): `--max-turns`, `--per-turn-timeout`,
`--overall-timeout`, `--terminator sentinel`, `--sentinel`, `--style`,
`--working-directory`, `--stream-turns`, `--bus inproc`, `--log-backend`.

### Back-compat aliases

The alias layer translates old flags into the new `ConvConfig` shape before any
runtime code runs. All current tests exercise this layer implicitly.

| Old flag | New equivalent |
|---|---|
| `--peer-a provider:claude-code` | `--peer id=a,provider=claude-code` |
| `--peer-b provider:codex` | `--peer id=b,provider=codex` |
| `--peer-a-opt key=val` | merged into peer `a` options |
| `--peer-b-opt key=val` | merged into peer `b` options |
| `--seed-a "..."` | `--seed id=a,content="..."` |
| `--seed-b "..."` | `--seed id=b,content="..."` |
| bare `--seed "..."` (broadcast) | `--seed content="..."` |
| `--opener a\|b` | unchanged (valid subset of `--opener <id>`) |

Mixing `--peer-a` with `--peer id=a,...` in the same invocation is a hard error
("ambiguous peer declaration for id `a`").

### `key=value` parser

The `--peer` and `--seed` flag values use a comma-separated `key=value` list
where `value` MAY be double-quoted to embed commas. Example:
`--seed id=alice,content="Hello, world."`. Unquoted commas inside a value are a
parse error. Escaping inside a quoted value uses `\"` for a literal quote and
`\\` for a literal backslash.

### Validation errors (fail-fast)

- Duplicate peer id.
- Fewer than 2 peers declared.
- More than 16 peers declared.
- `--opener <id>` references an undeclared peer.
- Missing seed for any declared peer when `--seed content=...` (broadcast) is
  not set.
- Peer id that doesn't match the id regex.
- Mixing `--peer-a/b` with `--peer id=a|b` for the same id.
- `--seed id=<id>` targeting an undeclared peer id.
- Combining broadcast `--seed content=...` with any `--seed id=...` in the same
  invocation.

### Example invocations

**2-peer, unchanged back-compat script:**
```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:codex \
  --seed-a "..." --seed-b "..." \
  --max-turns 6
```

**3-peer panel:**
```bash
vitis converse \
  --peer id=alice,provider=claude-code \
  --peer id=bob,provider=codex \
  --peer id=carol,provider=claude-code \
  --seed id=alice,content="You are the optimist. Address the next speaker with <<NEXT: id>> or end with <<END>>." \
  --seed id=bob,content="You are the pessimist." \
  --seed id=carol,content="You are the moderator." \
  --opener alice \
  --max-turns 12
```

## Addressing parser

`parse_next_trailer(reply string) *string`:

1. Strip trailing whitespace and empty lines from `reply`.
2. Take the last non-empty line.
3. If it matches `^<<END>>$` → return nil (sentinel wins; handled upstream).
4. If it matches `^<<NEXT:\s*([a-z][a-z0-9_-]{0,31})\s*>>$` → return the
   captured id.
5. Otherwise → return nil.

`<<END>>` is checked before `<<NEXT>>` regardless of order in the reply;
sentinel always wins. Trailer detection is anchored to the last non-empty line
so trailers quoted inside code fences or prose are ignored, matching the
existing `<<END>>` philosophy.

Self-addressing (`<<NEXT: current-id>>`) is treated as unknown and triggers the
fallback path. Rationale: the strict-alternation guarantee lifted from 2 peers
to N peers implies a peer cannot hand the turn to itself; otherwise a single
peer could monologue and lock the conversation.

## Turn loop

```
current   = opener
prev_from = ""
prompt    = seed[current]
seq       = 0

loop {
  if seq >= max_turns            { outcome = max_turns;  break }
  if overall_timeout elapsed     { outcome = timeout;    break }

  publish Envelope{ConvID, TurnSeq: seq, FromID: prev_from, ToID: current, Payload: prompt}

  reply, err := providers[current].Turn(ctx_with_per_turn_timeout, prompt)
  start_record(seq, from=prev_from, to=current, started, content=reply)

  if err != nil { outcome = error; finalize_record(err); break }

  if sentinel_hit(reply) {
    finalize_record(reason=end, sentinel_hit=true)
    outcome   = ended_by_sentinel
    ended_by  = current
    break
  }

  next_parsed := parse_next_trailer(reply)
  if next_parsed != nil && next_parsed in peer_ids && next_parsed != current {
    next     = *next_parsed
    reason   = addressed
    fallback = false
  } else {
    if next_parsed != nil {
      log.Warn("unknown or self addressee; falling back to round-robin", got=*next_parsed)
    }
    next     = round_robin_after(current)
    reason   = fallback_roundrobin
    fallback = true
  }

  finalize_record(reason, next_id_parsed=next_parsed, fallback_used=fallback)

  prompt    = reply   // passed verbatim to the next peer
  prev_from = current
  current   = next
  seq++
}
```

### Turn record `reason` values

- `opener` — first turn; `from_id = ""`, `to_id = opener`.
- `addressed` — trailer parsed cleanly and named a known, non-self peer.
- `fallback_roundrobin` — trailer missing, unparseable, unknown id, or
  self-addressed. `fallback_used = true`, `next_id_parsed` records what was
  seen (may be nil).
- `end` — this turn emitted `<<END>>`. `to_id = ""`; no next turn follows.

### Error taxonomy

| Code | Meaning | Outcome status | Exit code (unchanged) |
|---|---|---|---|
| `provider_spawn_failed` | Peer PTY failed to start | `error` | non-zero |
| `provider_turn_timeout` | Per-turn timeout | `error` | non-zero |
| `overall_timeout` | Whole-conversation budget exhausted | `timeout` | non-zero |
| `provider_crashed` | PTY died mid-turn | `error` | non-zero |
| `cancelled` | Parent ctx cancelled (Ctrl-C) | `cancelled` | non-zero |
| `internal` | Invariant violation | `error` | non-zero |

All error paths close the bus and flush buffered turn records before process
exit.

### Concurrency model

Exactly one turn is in flight at any time. The bus exists to decouple routing,
not to parallelize. Peers are long-lived PTY processes; the addressed peer runs,
the others idle. This is explicit in the spec because the "group chat" framing
might imply parallelism — it does not.

## Persistence — schema v2

### Hard cutover

New conversations write v2 only. `vitis peek` detects v1 on read and uses a
dedicated read-only compat path under `internal/persistence/v1compat/`. There
is no dual-write, no in-place upgrade, no migration tool.

### Version marker

Every v2 conversation header carries `"schema_version": 2`. v1 files lack this
field (or carry `"schema_version": 1`); the reader branches on presence and
value.

### Conversation header (`<session-dir>/conversation.json`)

```json
{
  "schema_version": 2,
  "conv_id": "conv_01HXYZ...",
  "created_at": "2026-04-08T14:22:31Z",
  "peers": [
    {
      "id": "alice",
      "provider": "claude-code",
      "options": { "model": "...", "reasoning_effort": "..." },
      "seed": "You are the optimist. ..."
    },
    { "id": "bob",   "provider": "codex",       "options": {}, "seed": "..." },
    { "id": "carol", "provider": "claude-code", "options": {}, "seed": "..." }
  ],
  "opener": "alice",
  "limits": {
    "max_turns": 12,
    "per_turn_timeout_sec": 300,
    "overall_timeout_sec": 3600
  },
  "terminator": { "kind": "sentinel", "sentinel": "<<END>>" },
  "style": "normal",
  "bus": "inproc",
  "working_directory": "/abs/path",
  "outcome": {
    "status": "ended_by_sentinel",
    "ended_by": "bob",
    "ended_at": "2026-04-08T14:29:11Z",
    "turn_count": 9
  }
}
```

`outcome.status` enum: `ended_by_sentinel | max_turns | timeout | error | cancelled | running`.

### Turn records (`<session-dir>/turns.jsonl`)

One JSON object per line, appended in turn order:

```json
{
  "seq": 4,
  "from_id": "alice",
  "to_id": "bob",
  "reason": "addressed",
  "next_id_parsed": "bob",
  "fallback_used": false,
  "started_at": "2026-04-08T14:24:02.118Z",
  "ended_at":   "2026-04-08T14:24:07.902Z",
  "duration_ms": 5784,
  "content": "…reply text including the verbatim <<NEXT: bob>> trailer…",
  "sentinel_hit": false,
  "tokens": { "input": 812, "output": 233 },
  "error": null
}
```

Notes:

- The `content` field preserves the reply verbatim, including any `<<NEXT: ...>>`
  or `<<END>>` lines. Parsed fields (`next_id_parsed`, `sentinel_hit`) mirror
  what the parser extracted from `content`.
- `reason = opener` → `from_id = ""`, `to_id = opener`, `next_id_parsed = nil`
  by construction.
- `reason = end` → `sentinel_hit = true`, `to_id = ""`, no subsequent turn
  record follows.

### On-disk layout

Unchanged: `<sessions-root>/<conv_id>/{conversation.json, turns.jsonl, logs/...}`.
Per-peer log files live under `logs/<peer_id>/...` to accommodate N peers and
prevent id collisions.

### v1 compat reader

`internal/persistence/v1compat/` exposes a read-only reader that:

- Detects v1 by the absence of `schema_version` or the value `1`.
- Synthesizes peers `{id: "a", ...}` and `{id: "b", ...}` from legacy
  `peer_a` / `peer_b` fields.
- Maps legacy turn records into the v2 shape with `reason` inferred:
  `opener` for `seq == 0`, otherwise alternating between `a` and `b`;
  `fallback_used = false`; `next_id_parsed = nil`.
- Drops unknown v1 fields.
- Rejects any write attempt with `"v1 session, read-only; start a new conversation"`.

`vitis peek` transparently uses this reader when the header indicates v1.

## Testing strategy

### Unit tests

- `internal/conversation/scheduler/addressed_policy_test.go` — table-driven
  parser tests: clean trailer, missing trailer, unknown id, self-addressed,
  `<<END>>` precedence, whitespace variants, code-fence decoys, max-length id,
  invalid id characters, CRLF endings.
- `internal/bus/inproc_test.go` — publish/subscribe routing by `ToID`,
  buffer backpressure, close semantics, double-subscribe rejection, unsubscribe
  cancel func behavior.
- `internal/conversation/envelope_test.go` — v2 JSON round-trip, required
  fields, `reason` enum validation.
- `internal/persistence/v2_test.go` — conversation header write/read, turns.jsonl
  append and replay.
- `internal/persistence/v1compat/reader_test.go` — golden v1 fixture → v2-shaped
  read, peer id synthesis, reason inference, append rejection.
- `internal/cli/converse_test.go` — flag parsing for repeatable `--peer`,
  `--seed id=...`, back-compat aliases, alias/new flag mixing errors, all
  validation errors, `content="..."` quoting and escaping.

### Integration tests (gated by `//go:build integration`)

- **2-peer behavioral parity** — the new `--peer id=a,...` invocation produces
  identical `turns.jsonl` (modulo ids) to the back-compat `--peer-a/b`
  invocation under the same mock script.
- **3-peer addressed happy path** — mock emits `<<NEXT: id>>` trailers in a
  scripted sequence; assert turn order and `reason=addressed` on every non-opener
  turn.
- **3-peer fallback path** — mock emits no trailer; every turn has
  `reason=fallback_roundrobin`, `fallback_used=true`, and order matches
  declaration order.
- **3-peer mixed** — some addressed, some fallback; assert per-turn reasons.
- **3-peer unknown addressee** — mock emits `<<NEXT: ghost>>`; assert warning
  logged, `next_id_parsed="ghost"`, `fallback_used=true`, conversation continues.
- **3-peer self-address** — mock emits `<<NEXT: self-id>>`; assert fallback.
- **3-peer `<<END>>` from middle peer** — assert `outcome.ended_by` is that
  peer; remaining peers never prompted again.
- **3-peer max_turns cap** — assert `outcome.status = max_turns`, correct count.
- **3-peer per-turn timeout** on one peer — assert error outcome with
  `provider_turn_timeout`.

### Back-compat regression suite (acceptance gate)

The full existing 2-peer test suite MUST run unchanged. If any existing test
requires modification to keep passing, the alias layer is wrong — stop and
fix the alias layer, do not modify the test. v1 persistence fixtures produce
identical `vitis peek` human-visible output before and after the schema break.

### Mock agent extensions

- `--next-trailer <id>` / per-reply directive — emit a valid trailer line.
- `--no-trailer` — omit the trailer entirely (fallback test driver).
- `--bad-trailer <value>` — emit `<<NEXT: <value>>>` where `<value>` is an
  unknown or invalid id.

### Manual test scripts

- `tests/manual/13_converse_three_peer_addressed.sh`
- `tests/manual/14_converse_three_peer_fallback.sh`
- `tests/manual/15_converse_backcompat_aliases.sh`

Existing scripts `04`–`10` run untouched as the back-compat regression check.

### Coverage targets

80%+ on new packages: `internal/bus`, `internal/conversation/scheduler`,
`internal/persistence/v1compat`.

## Rollout phases

Suggested PR boundaries:

1. **Bus interface + inproc impl** — `internal/bus`, no callers yet. Unit tests
   only. Zero behavior change.
2. **Envelope v2 + persistence v2 writer + v1 compat reader** — schema break
   lands; scheduler still calls the old path. `peek` switched to v2-aware
   reader with v1 fallback. Existing 2-peer tests migrated to v2 fixtures where
   needed.
3. **Scheduler + addressed policy** — extracted turn loop and policy interface.
   2-peer conversations run through the new scheduler on the old CLI. Full
   regression suite must stay green.
4. **CLI surface: repeatable `--peer`, `--seed id=...`, back-compat aliases** —
   new flag parsing, validation, alias translation. N-peer unlocked.
5. **Mock agent extensions + N-peer integration tests + manual scripts** —
   acceptance gate.
6. **Docs update** — README, A2A conversation guide, migration note for v1
   sessions, flag reference.

## Risks

- **Back-compat alias drift.** The primary risk. Every existing test is a
  regression sentinel; the alias layer must translate old flags into the new
  `ConvConfig` shape without behavioral change. Mitigation: dedicated alias-layer
  tests in phase 4; any existing test that needs modification is a red flag.
- **Trailer parser false negatives.** Agents that format the trailer with
  surrounding whitespace, prose, or inside fenced code will silently fall back
  to round-robin. Mitigation: warn-log every fallback with the raw last line so
  users can iterate seeds; document the exact regex in the A2A guide.
- **Self-monologue via fallback.** If round-robin fallback always lands on the
  same next peer who also never emits a trailer, the conversation degenerates
  into rigid rotation. This is intended fallback behavior, not a bug; document
  it as "design seeds to teach the trailer format".
- **v1 compat reader rot.** Once nobody has v1 sessions, the compat code
  becomes dead weight. Mitigation: gate it behind a build tag in a future
  release if usage drops. Not a concern for this spec.
- **Peer id leakage into provider paths.** Today's per-provider `cwd` / `home`
  / env and log paths assume slot `a`/`b`. New code must thread peer id through
  logging and any on-disk paths; reject any id not matching the regex. Log
  paths become `logs/<conv_id>/<peer_id>/...`.

## Open questions

None at draft time. Any issues surfaced during implementation should be raised
as amendments to this spec before code lands.

## Downstream dependencies

- `vitis-ui` (separate spec, to be written after this one is implemented) will
  consume the v2 envelope, the enriched turn records, and — eventually — a
  `vitis serve` subcommand that exposes the runtime over HTTP/WebSocket. The
  `serve` subcommand is not part of this spec; it belongs to the UI spec.
