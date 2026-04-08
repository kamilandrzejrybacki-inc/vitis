# ADR: N-peer conversations with addressed routing

**Status:** Accepted (design phase complete; Phases 1–3 implemented)
**Date:** 2026-04-08
**Spec:** `docs/superpowers/specs/2026-04-08-vitis-n-peer-conversations-design.md`
**Plan:** `docs/superpowers/plans/2026-04-08-vitis-n-peer-conversations-plan.md`
**Review:** `docs/superpowers/reviews/2026-04-08-n-peer-phase-1-3-review.md`

## Context

`vitis converse` ships today as strict alternation between exactly two peers (`--peer-a`, `--peer-b`). Everything downstream — envelope routing, persistence, log paths, the CLI surface — assumes `PeerSlot ∈ {a, b}`. A planned web UI (spec TBD in a separate session) frames agent runs as *group chats*, which requires more than two participants. We need to extend the core runtime first so the UI can be designed against real semantics rather than provisional ones.

Constraints shaping the decision:

- **Back-compat is non-negotiable.** All existing 2-peer scripts, fixtures, and manual test harnesses must continue to pass unchanged. An alias layer that translates `--peer-a/b` → `--peer id=a,...` is the acceptance gate.
- **The UI must know who speaks next and why.** Turn records need enough structure to answer "was this turn addressed, a round-robin fallback, or the opener?" without re-parsing the reply.
- **Agents are LLM-driven stdout producers.** Any addressing protocol has to survive PTY output and be robust against code-fence / quoted-text false positives.
- **NATS bus is planned but not required here.** The bus interface already exists (`internal/bus`, with inproc and NATS backends). We extend routing, not re-architect it.

## Decision

Adopt **N-peer conversations (2 ≤ N ≤ 16) with addressed turn routing**:

1. **Addressing mechanism.** Each peer ends its reply with a structured trailer line `<<NEXT: peer-id>>` on the last non-empty line. The orchestrator's `AddressedPolicy` parses this trailer and routes the next envelope to the named peer.
2. **Fallback.** If the trailer is missing, unparseable, addresses an unknown peer, or addresses the current speaker, routing falls back to **round-robin in the declared peer order**. This guarantees forward progress without requiring the caller to design seeds perfectly on the first try.
3. **Termination.** `<<END>>` on the last non-empty line takes precedence over `<<NEXT>>` and terminates the conversation immediately (existing 2-peer semantics, unchanged).
4. **Identity.** Peers have stable `PeerID` strings matching `^[a-z][a-z0-9_-]{0,31}$`. The legacy slot aliases `a` and `b` are reserved for the back-compat alias layer but may also be declared explicitly.
5. **CLI.** Repeatable `--peer id=<id>,provider=<p>[,key=val,...]` and repeatable `--seed id=<id>,content="..."` (with a broadcast form `--seed content="..."`) replace the 2-slot flags as the canonical surface. The old flags translate into the new shape in a dedicated alias layer.
6. **Persistence.** Hard cutover to schema v2 keyed by peer id (`Conversation.Peers[]`, `Conversation.Seeds`, `Conversation.OpenerID`, plus enriched turn records with `FromID`, `ToID`, `Reason`, `NextIDParsed`, `FallbackUsed`). A dedicated read-only compat path upgrades v1 sessions in-memory for `vitis peek`.
7. **Bus.** Extend the existing topic convention with `conv/<id>/peer/<peer-id>/in` alongside the legacy `conv/<id>/peer-<slot>/in`. Both coexist during migration; the orchestrator reads and publishes on whichever shape matches the conversation's schema version.
8. **Concurrency.** Still one turn in flight at a time. The bus decouples routing, not parallelism. Group-chat visuals do not imply parallel peers.

## Alternatives considered

### A. Round-robin only, no addressing
Simplest extension — fixed rotation `p1 → p2 → p3 → …`. Rejected because the group-chat UX the UI wants is fundamentally about "agents talking to each other", and a fixed rotation makes every peer a passive turn-taker rather than an addressable participant.

### B. Explicit schedule via `--order a,b,c,b,a`
Caller pre-declares the full turn sequence. Rejected as v1 default because it makes every seed file specific to a turn count, and because the UI would need a turn-sequence editor before anything useful could ship. Kept as a possible future `TurnPolicy` implementation.

### C. Mention-based addressing (`@peer-c` anywhere in the reply)
Feels natural to LLMs. Rejected because `@name` tokens appear routinely in prose, quoted chat logs, and code, producing false positives the parser cannot reliably distinguish. The line-anchored `<<NEXT: id>>` trailer composes with the existing `<<END>>` sentinel and is unambiguous.

### D. Moderator-driven (one peer decides turns via a protocol)
Powerful but heavy. Requires a moderator protocol, an authority check, and a recovery path when the moderator misbehaves. Out of scope for v1. Could be layered on top later as a `TurnPolicy` that wraps `AddressedPolicy` and overrides its decisions with moderator output.

### E. Tool-based addressing via structured output
The cleanest semantically — peers emit a structured routing field via a tool call rather than stdout parsing. Rejected because today's PTY-wrapped providers (Claude Code, Codex) don't expose a structured side-channel; you'd still be parsing stdout. Collapses into option A or C in practice.

## Consequences

### Positive

- **UI unblocked.** With v2 turn records carrying `Reason` and `NextIDParsed`, the UI can render "alice spoke, addressed bob" vs "alice spoke, bob was next by round-robin" without re-parsing anything.
- **Seed-iteration feedback loop.** Every fallback is loggable with the raw last line; users can debug why their seed prompts aren't emitting valid trailers.
- **Bus abstraction pays off.** The existing `Bus` interface absorbs the new topic shape without requiring a second implementation.
- **Back-compat is cheap.** The 2-peer alias layer is a thin translation that happens before the orchestrator ever sees a `model.Conversation`, keeping the turn loop free of slot-vs-id branching at runtime.

### Negative

- **Schema v2 is a hard break.** v1 sessions become read-only via a dedicated compat path. Users who want a v2 copy must re-run their conversation.
- **Trailer parser false negatives.** Agents that format the trailer with surrounding whitespace, prose, or inside fenced code will silently fall back to round-robin. Mitigation: log the raw last line on every fallback; document the exact regex.
- **Self-monologue via fallback.** If round-robin fallback always lands on the same next peer who also never emits a trailer, the conversation degenerates into rigid rotation. This is intended behavior, not a bug, but has to be documented.
- **Alias-layer debt.** The back-compat aliases are carried indefinitely unless we set a deprecation timer. Recommend: start warning in the release after Phase 7 lands, remove one release later.
- **Per-peer log paths change.** `logs/<conv_id>/peer-<slot>/...` becomes `logs/<conv_id>/peer/<peer-id>/...`. Any external tooling that scraped the old path breaks.

## Scope boundary

This ADR covers the **core runtime** only. The web UI is a separate spec and depends on this ADR's outcomes. The `vitis serve` subcommand (HTTP/WebSocket exposure of the runtime) is explicitly not part of this decision — it belongs to the UI design document.

## Implementation status

- **Phase 1 (PeerID type + bus topic extensions):** DONE
  - `internal/model/peer_id.go`, `peer_id_test.go`
  - `internal/bus/bus.go` (`TopicEnvelopeInID`), `peer_id_topics_test.go`
- **Phase 2 (Envelope + Conversation schema v2 fields):** DONE
  - `internal/model/conversation.go` (Envelope v2 fields, `PeerParticipant`, `TurnReason`, v2 Conversation and ConversationTurn fields)
  - `internal/model/envelope_v2_test.go`, `conversation_v2_test.go`
- **Phase 3 (TurnPolicy package with AddressedPolicy):** DONE
  - `internal/conversation/policy/{policy,addressed,roundrobin}.go` (moved from `internal/orchestrator/policy/` — the package was misplaced in the plan; the turn loop lives in `internal/conversation`, not `internal/orchestrator`)
  - `internal/conversation/policy/addressed_test.go`, `fuzz_test.go`
- **Phase 4 (broker turn-loop delegates to policy):** DONE
  - `internal/conversation/broker.go` — `BrokerDeps.Policy` field, `NewBroker` defaults to `AddressedPolicy`, turn loop calls `policy.Next()` immediately after each turn (before the max-turns short-circuit so the cap-hit turn gets the same enriched fields as every other turn), populates `FromID`/`ToID`/`Reason`/`FallbackUsed`/`NextIDParsed`, drives the next slot via `slotFromPeerID`.
  - For the current 2-peer transport surface, `AddressedPolicy`'s round-robin fallback over `["a","b"]` is equivalent to strict alternation when replies carry no trailer — so all existing broker tests stay green without modification.
  - New test: `broker_policy_test.go` verifies both the opener-turn pinning and the no-trailer fallback path end-to-end.
- **Phase 5 (store schema v2 writer + v1 compat reader):** NOT STARTED
- **Phase 6 (CLI repeatable flags + alias layer):** NOT STARTED
- **Phase 7 (mock agent extensions + N-peer integration tests + manual scripts):** NOT STARTED

Phases 5–7 are deferred to follow-up sessions. The code committed so far is purely additive — the existing 2-peer runtime is untouched, and the full test suite is fully green (the pre-existing `TestRunHappyPath` PTY-glyph flake was also fixed as an incidental cleanup in commit `951b37e`).
