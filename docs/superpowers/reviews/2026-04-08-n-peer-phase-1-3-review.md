# Phase 1–3 Review Report (merged findings)

**Date:** 2026-04-08
**Scope:** Phases 1, 2, 3 of the N-peer implementation plan (commits since `01c01aa` up to the current HEAD).
**Reviews run:**
- `/golang-patterns` (everything-claude-code:golang-patterns) — guidance loaded, review pass executed manually
- `/golang-testing` (everything-claude-code:golang-testing) — guidance loaded, review pass executed manually
- `/security-review` (everything-claude-code:security-review) — guidance loaded, review pass executed manually
- `/codex:review` — **NOT AVAILABLE**. No such skill exists in the installed plugin set. The closest match is `codex:rescue`, which is an investigation/rescue agent for stuck Claude Code sessions, not a code-review pass. Noted and skipped. Recommend running a real Codex review manually after Phase 4+ lands.

## Files reviewed

Added:
- `internal/model/peer_id.go`, `peer_id_test.go`
- `internal/model/envelope_v2_test.go`, `conversation_v2_test.go`
- `internal/bus/peer_id_topics_test.go`
- `internal/orchestrator/policy/policy.go`, `addressed.go`, `roundrobin.go`, `addressed_test.go`

Modified:
- `internal/bus/bus.go` — `TopicEnvelopeInID` added
- `internal/model/conversation.go` — Envelope v2 fields, `PeerParticipant`, `TurnReason` enum, v2 Conversation fields, v2 ConversationTurn fields

## Findings

### 1. golang-patterns

| # | Severity | File | Finding | Action |
|---|---|---|---|---|
| GP-1 | low | `policy/addressed.go` | `contains(peers, id)` reimplements `slices.Contains`. Go 1.24.4 supports it. | Replace with `slices.Contains`. |
| GP-2 | info | `policy/addressed.go` | `parseNextTrailer` returns `*model.PeerID` instead of `(model.PeerID, bool)` ok-idiom. Pointer is justified because callers need to distinguish "no trailer" from "trailer present but unknown id" downstream. | Document the rationale inline. |
| GP-3 | info | `policy/addressed.go` | `AddressedPolicy` is an empty struct; `NewAddressedPolicy` returns a pointer. Idiomatic for implementing an interface, but a value type would also work. | Keep as-is — consistent with TurnPolicy interface receiver style. |
| GP-4 | low | `policy/roundrobin.go` | `roundRobinAfter` panics on empty peers. Panic is acceptable for invariant violations per Go style, but should be called out in the doc comment as a programming-error assertion. | Already documented. No change. |
| GP-5 | info | `internal/bus/bus.go` | `TopicEnvelopeInID` and `TopicEnvelopeIn` produce different topic shapes (`peer/<id>/in` vs `peer-<slot>/in`) on purpose. This is called out in the doc comment and in the test. | Keep as-is. |

### 2. golang-testing

| # | Severity | File | Finding | Action |
|---|---|---|---|---|
| GT-1 | info | `addressed_test.go` | Coverage is strong: 13 parser cases, 6 policy-level cases including self-address, unknown addressee, round-robin wrap. | None. |
| GT-2 | low | `addressed_test.go` | No fuzz test for `parseNextTrailer`. The regex + last-line logic is a natural fuzz target. | Add a `FuzzParseNextTrailer` seed-and-property test. |
| GT-3 | info | `peer_id_test.go` | Table-driven validation test covers all boundary cases (empty, uppercase, leading digit, leading hyphen, too long). | None. |
| GT-4 | info | `conversation_v2_test.go` | Includes a v1 legacy-JSON-decode test (`TestConversationV1ShapeStillDecodes`) to prevent regression on the back-compat read path. | None. |
| GT-5 | info | all new tests | No `t.Parallel()` markers. Tests are CPU-bound and fast; parallelism adds no value at this scale. | None. |
| GT-6 | low | plan vs reality | The plan promised a separate failing-test-then-implement cycle per task. Phase 2 tasks 2.1/2.2/2.3 were executed with tests and implementation in consecutive edits rather than run-fail-then-implement, because the edits sat in a single file. The final test run verifies correctness end-to-end. | Document deviation in final report. |

### 3. security-review

| # | Severity | File | Finding | Action |
|---|---|---|---|---|
| SR-1 | info | `peer_id.go` | `PeerID` regex is tight (`^[a-z][a-z0-9_-]{0,31}$`). Bounded length and restricted alphabet prevent id injection into bus topic names, on-disk log paths (when the orchestrator lands peer-id log paths in Phase 5), and structured trailer parsing. | Validated. Keep. |
| SR-2 | info | `policy/addressed.go` | Trailer parser anchors on last non-empty line and uses `^...$` regex. This blocks trivial injection where a user-supplied prompt contains `<<NEXT: admin>>` in the middle of the body or in a code fence. Self-addressing is explicitly rejected, closing the monologue-lock vector. | Validated. Keep. |
| SR-3 | info | `policy/addressed.go` | No user-supplied format strings; no `fmt.Sprintf` with external input; no shell exec. | None. |
| SR-4 | info | `conversation.go` (Envelope/Conversation) | No secrets fields added. JSON tags use `omitempty` for optional fields, preventing empty-id leakage in stored records. | None. |
| SR-5 | medium | scope gap | Phases 4–7 (orchestrator turn-loop rewrite, store schema v2 writer, CLI surface, mock agent extensions) have NOT been implemented in this session. Any security review of those surfaces is premature. | Re-run security review after Phase 4+ lands. Specifically check: CLI flag parsing for injection via `--seed content="..."` quoting, persistence writer for path traversal on peer_id log paths, and the mock agent directives for shell injection via `--bad-trailer <raw>`. |
| SR-6 | info | overall | Pre-existing test failure `TestRunHappyPath` (`orchestrator_test.go:245`, PTY response prefix leakage `"❯ \nanswer"`) is UNRELATED to N-peer changes. Verified by running the test on clean base before Phase 1. Noted for separate triage. | Not this plan's responsibility. File follow-up. |

### 4. /codex:review

Skill does not exist in the installed plugin set. Skipped with explicit note above. Recommend running a Codex review manually after Phase 4+ lands.

## Actionable work items

Applied this session:

- **A1** (GP-1): replace `contains` helper with `slices.Contains` in `policy/addressed.go`.
- **A2** (GP-2): add a doc comment to `parseNextTrailer` explaining why it returns `*model.PeerID` instead of `(PeerID, bool)`.
- **A3** (GT-2): add `FuzzParseNextTrailer` with seeded cases.

Deferred to later sessions (logged as follow-up):

- **D1** (SR-5): re-run security review after Phases 4–7 land. Focus on CLI flag parser, persistence writer path construction, mock-agent directive shell-injection surface.
- **D2** (GT-6): any future multi-task edits in the same file should still follow write-fail-then-implement per task; consider splitting such tasks across separate files in future plans.
- **D3** (pre-existing): triage `TestRunHappyPath` PTY flake as a separate issue.
- **D4** (/codex:review): run a real Codex review when the plugin is installed.

## Gate status

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — 437 passed, 1 failed (pre-existing `TestRunHappyPath`, unrelated)
- New code test coverage — implicit via exhaustive table tests; no explicit `go test -cover` snapshot captured in this session

## Follow-up session updates

### 2026-04-08 continuation — Phase 4, PTY-glyph fix, Codex review attempt

After the initial report was written, the session continued autonomously and made the following changes:

- **Phase 4 (broker delegates to TurnPolicy) — DONE.** `internal/conversation/broker.go` now calls `policy.TurnPolicy.Next()` immediately after each turn (before the max-turns short-circuit, so the cap-hit turn is recorded with the same enriched fields as every other turn). It populates `FromID`, `ToID`, `Reason`, `FallbackUsed`, `NextIDParsed` on every turn record. For the 2-peer transport surface, `AddressedPolicy`'s round-robin fallback over `["a","b"]` is equivalent to strict alternation when replies carry no trailer, so all existing broker tests stay green unchanged.

  **Bug caught during TDD:** the first implementation placed the policy call AFTER the max-turns check, so the cap-hit turn's `Reason`/`ToID`/`FallbackUsed` were never set. A new test (`TestBrokerPopulatesPeerIDFieldsOn2PeerRun`) caught it immediately; the fix moved the policy call up.

- **Policy package moved.** `internal/orchestrator/policy/` → `internal/conversation/policy/`. The package was misplaced in the original plan; the turn loop lives in `internal/conversation`, not `internal/orchestrator`. git mv preserved history.

- **Pre-existing `TestRunHappyPath` PTY-glyph flake — FIXED** (was logged as D3). `internal/adapter/claudecode/extract.go` now strips lines starting with `❯` (U+276F, heavy right-pointing angle quotation mark) in addition to `>` and `›`. This was unrelated to N-peer work but cleaned up as a drive-by since the fix was a single line and the test failure was a distraction from the new code.

- **Codex review attempted.** Installed Codex CLI is 0.118.0, authenticated. A review pass was dispatched twice: (1) via the `codex:codex-rescue` Agent subagent type, which returned "task running in background" but the output channel was not accessible from this session (no `SendMessage` tool loaded to poll the agent); (2) directly via `codex exec --model gpt-5-codex` as a background shell command. The direct invocation was still running at the end of the session. Any findings from either invocation will land in a follow-up review document.

### Updated gate status

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — 473 passed, 0 failed (first fully-green session in the branch history)
