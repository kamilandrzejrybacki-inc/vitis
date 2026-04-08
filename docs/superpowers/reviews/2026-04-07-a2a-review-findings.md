# A2A Review Findings — 2026-04-07

Consolidated findings from four parallel review passes (backend-patterns, golang-patterns, golang-testing, security) on commits `b266611..a508355` of the `feat/fill-plan-gaps` branch.

The codex second-opinion review was started but stalled mid-analysis after ~5 minutes of file inspection without producing a final report; cancelled to keep the overnight budget on track. The four completed reviews have substantial overlap and the must-fix list is well-determined without it.

**Reviewers:**
- backend-patterns (Sonnet) — 4 HIGH, 5 MEDIUM, 4 LOW
- golang-patterns (Sonnet) — 4 HIGH, 7 MEDIUM, 3 LOW
- golang-testing (Sonnet) — 5 HIGH, 7 MEDIUM, 5 LOW
- security (Sonnet) — 4 HIGH, 5 MEDIUM, 4 LOW

After dedup, **17 unique HIGH/CRITICAL findings**, **~14 MEDIUM**, and several LOW.

---

## Must-Fix (HIGH/CRITICAL)

### H1 — Briefing rendered but never delivered to peer
- **File:** `internal/conversation/envelope.go:14-30`
- **Reviewer:** backend
- **Bug:** `BuildEnvelopeTurn1` populates `model.Envelope.Briefing` but the PTY transport only writes `env.Body`. Peers never receive the system briefing → no slot identity, no max-turns awareness, no sentinel instruction.
- **Fix:** Update `BuildEnvelopeTurn1` so the body actually contains the briefing. The minimal change is to construct `body` as `briefing + "\n\n" + renderBody(...)` when `IncludeBriefing` is true. Update the envelope test to assert the body contains the briefing text and then update broker_test/E2E to confirm it still passes.

### H2 — `streamTurnsTo` goroutine races against `bus.Close()` and the final stdout write
- **File:** `internal/cli/converse.go:172-174`
- **Reviewer:** golang, backend
- **Bug:** The streaming goroutine writes turns to `stdout` while the main goroutine writes the final `FinalResult` JSON to the same `stdout`. After `br.Run` returns, `defer b.Close()` fires while the goroutine may still be in flight. Concurrent writes to `stdout` are a data race; bus close while subscribed is also a race.
- **Fix:** Use a `sync.WaitGroup`, cancel `runCtx` explicitly before waiting, and `wg.Wait()` before encoding the final result. Also drain the streaming goroutine before invoking `b.Close()`.

### H3 — Deferred peer `Stop` passes the already-cancelled run context
- **File:** `internal/conversation/broker.go:70-72`
- **Reviewer:** backend, golang
- **Bug:** When `Run` exits via context cancellation, the deferred `PeerA.Stop(ctx, ...)` / `PeerB.Stop(ctx, ...)` fire with `ctx` already done. Any IO inside `Stop` that respects ctx fails immediately, defeating the grace period.
- **Fix:** Replace `ctx` in the defer with a fresh background context that has its own timeout (e.g. `stopCtx, _ := context.WithTimeout(context.Background(), time.Second)`). Apply the same fix to the deferred `Terminator.Stop` call so behavior is consistent.

### H4 — `drainControl` is dead code; `drainControlTimed` 5ms window is fragile
- **File:** `internal/conversation/broker.go:208-224` (dead) and `:138` (5ms hardcoded)
- **Reviewer:** golang, testing, backend
- **Bug:** `drainControl` is defined but never called. `drainControlTimed(5ms)` is a sleep-based race against the sentinel goroutine's scheduling — under load or `-race`, the verdict can be missed for one turn cycle, allowing one extra unintended peer turn before termination.
- **Fix:** Delete `drainControl` entirely. Make the timed-drain window a configurable field on `BrokerDeps` with a default of `50 * time.Millisecond` (10x the current value) — that is still imperceptible to users but vastly more robust under load. Add a unit test that injects a deliberately-slow sentinel and confirms the broker still observes its verdict.

### H5 — `provider:mock` is reachable in production binaries; spawns arbitrary binary
- **File:** `internal/peer/provider/spawner.go:65-86`
- **Reviewer:** security, golang, backend
- **Bug:** `mockProviderAdapter` is in a non-test file with no build tag. A user can pass `--peer-a provider:mock --peer-a-opt bin=/anything` to execute an arbitrary binary as a PTY peer. The `MOCK_BIN` env var is also read directly.
- **Fix:** Move `mockProviderAdapter` and the `case "mock":` branch of `resolveAdapter` into a new file `internal/peer/provider/spawner_mock_test.go` (so it only compiles in tests), or behind a `//go:build a2a_test_mock` build tag. The CLI E2E test then needs the build-tag flag to compile. After the change, `vitis converse --peer-a provider:mock` from a release binary returns "unknown provider".

### H6 — `env_` opt prefix lets caller inject arbitrary env vars including `VITIS_CLAUDE_ARGS` / `LD_PRELOAD`
- **File:** `internal/peer/provider/spawner.go:36-39`
- **Reviewer:** security
- **Bug:** Any `--peer-a-opt env_KEY=val` is forwarded as an environment variable to the spawned subprocess. Sensitive keys (`VITIS_CLAUDE_ARGS=--dangerously-skip-permissions`, `LD_PRELOAD=/tmp/evil.so`, `VITIS_CLAUDE_BINARY=/path/to/trojan`) bypass vitis's safety nets.
- **Fix:** Replace the open `env_` forwarding with an allowlist of permitted env keys. Initial allowlist: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `MOCK_RESPONSE`, `MOCK_SENTINEL_AT_TURN`, `MOCK_DELAY_MS`. Anything else is dropped with a stderr warning. Document the allowlist in the converse command help text.

### H7 — `PersistentProcess.buffer` grows without bound
- **File:** `internal/peer/provider/persistent.go:41-42, 77`
- **Reviewer:** security, backend
- **Bug:** The pump goroutine accumulates every PTY byte indefinitely. `cursor` advances but the bytes before the cursor are never freed. A long conversation, a verbose peer, or a malicious peer can exhaust memory.
- **Fix:** After a successful marker match in `ConverseTurn` (line 137), compact the buffer: `p.buffer = append([]byte(nil), p.buffer[p.cursor:]...); p.cursor = 0`. Add a hard cap (e.g. `maxBufferBytes = 64 << 20`) — if exceeded between marker matches, return a `buffer overflow` error from `ConverseTurn` so the broker finalizes with `ConvPeerCrashed`.

### H8 — `mock.PeerTransport` type name collides with the `peer.PeerTransport` interface
- **File:** `internal/peer/mock/mock.go:26`
- **Reviewer:** golang
- **Bug:** Naming the concrete struct `PeerTransport` (same as the interface it implements) is a Go anti-pattern that breaks autocomplete and confuses readers.
- **Fix:** Rename the struct to `Transport` (matching `provider.Transport`). Update all references in `mock.go`, `broker_test.go`, and any other test that imports the mock.

### H9 — `ConvPeerCrashed` and `ConvPeerBlocked` broker paths untested
- **File:** `internal/conversation/broker_test.go` (missing tests)
- **Reviewer:** testing
- **Bug:** The control-message handling for `ControlPeerCrashed` and `ControlPeerBlocked` (broker.go:145-148) has 0% coverage.
- **Fix:** Add two new tests: `TestBrokerPeerCrashedControlMessage` and `TestBrokerPeerBlockedControlMessage`. Each publishes the control message via the inproc bus from a goroutine after the first turn, asserts the broker finalizes with the corresponding status, and asserts the turn log contains turn 1 only.

### H10 — `internal/peer/provider/spawner.go` is 0% covered
- **File:** `internal/peer/provider/` (no tests for spawner.go)
- **Reviewer:** testing
- **Bug:** `resolveAdapter`, `NewTerminalSpawner`, and `mockProviderAdapter.BuildSpawnSpec` have no unit tests. Only the CLI E2E exercises the `mock` branch.
- **Fix:** Add `internal/peer/provider/spawner_test.go` with table-driven tests for `resolveAdapter` covering: `provider:claude-code`, `provider:claudecode`, `provider:codex`, `provider:mock` (under the build tag from H5), unknown provider, malformed URI (no `provider:` prefix). Test that `mockProviderAdapter.BuildSpawnSpec` forwards options to env vars (after H6's allowlist is in place).

### H11 — `PeerB.Start` failure path untested
- **File:** `internal/conversation/broker_test.go` (missing test)
- **Reviewer:** testing
- **Bug:** When `PeerB.Start` returns an error, the broker should call `PeerA.Stop` and finalize with `ConvError`. No test exercises this.
- **Fix:** Add `TestBrokerPeerBStartFailure` using a mock `PeerB` whose `Start` returns an error. Assert: `ConvError` status, 0 turns, and that `PeerA.Stop` was called (extend mock with a call counter if needed).

---

## Should-Fix (MEDIUM)

### M1 — `time.Duration` fields serialize as nanosecond integers
- **File:** `internal/model/conversation.go:57-58`
- **Fix:** Change `PerTurnTimeout` and `OverallTimeout` to `int64` (seconds) with json tags `per_turn_timeout_sec` / `overall_timeout_sec`. Convert to/from `time.Duration` at use sites (broker, CLI). Update tests.

### M2 — `Transport.Stop` ignores `grace` parameter
- **File:** `internal/peer/provider/provider.go:90`
- **Fix:** Plumb `grace time.Duration` through `PersistentProcess.Close(grace time.Duration)` to `inner.Terminate(int(grace.Milliseconds()))`. Default to 1s if zero.

### M3 — Missing compile-time interface assertions
- **Fix:** Add `var _ bus.Bus = (*Bus)(nil)` in `internal/bus/inproc/inproc.go`, `var _ peer.PeerTransport = (*Transport)(nil)` in `internal/peer/provider/provider.go` and `internal/peer/mock/mock.go`, `var _ terminator.Terminator = (*Sentinel)(nil)` in `internal/terminator/sentinel.go`. Remove the meaningless `var _ = io.EOF` from `persistent.go`.

### M4 — Sentinel `BusMessage.Timestamp` not set
- **File:** `internal/terminator/sentinel.go:104-109`
- **Fix:** Add `Timestamp: time.Now().UTC()` to the `bus.BusMessage` literal.

### M5 — `Verdict.Decision` is an unvalidated string
- **File:** `internal/model/conversation.go:93`, `internal/conversation/broker.go:142`
- **Fix:** Add `type VerdictDecision string` with constants `DecisionContinue` / `DecisionTerminate`. Update broker comparison.

### M6 — Path traversal on `--log-path` and `--working-directory`
- **File:** `internal/cli/converse.go:75-76`, `internal/store/file/file_store.go:24-35`
- **Fix:** Apply `filepath.Clean` to `--log-path` and `--working-directory`. For `--working-directory`, also `os.Stat` to confirm it exists and is a directory. No path-escape protection beyond Clean — the local single-user threat model accepts user-controlled absolute paths but not relative escapes.

### M7 — Raw ANSI escape sequences stored verbatim in `ConversationTurn.Response`
- **File:** `internal/peer/provider/provider.go:80`
- **Fix:** Apply `terminal.NormalizePTYText(string(resp))` (already in-package, already tested) before assigning to `ConversationTurn.Response`. Optionally keep raw bytes in a separate field if/when debug-raw mode is added for conversations.

### M8 — Integer overflow in `--per-turn-timeout × max-turns`
- **File:** `internal/cli/converse.go:121-127`
- **Fix:** Cap `--per-turn-timeout` at 3600s and `--overall-timeout` at 86400s. Validate both bounds before multiplication. Use `if perTurnTimeout > 0 && maxTurns > 0 && perTurnTimeout > math.MaxInt/maxTurns { error }` as a safety net.

### M9 — Sentinel collision: peer mentioning `<<END>>` in content terminates prematurely
- **File:** `internal/terminator/sentinel.go:86`
- **Fix:** Require the sentinel to appear on its own line (i.e. surrounded by newlines or at start/end of response). Update `strings.Contains` check to a line-anchored match. Update `StripSentinel` symmetrically. Add a test where the response contains the sentinel mid-line and assert NO termination.

### M10 — `drainControl` deletion (already covered by H4 but worth flagging separately)

### M11 — Mixed unlock pattern in `mock.Deliver`
- **File:** `internal/peer/mock/mock.go:69-92`
- **Fix:** Restructure with single `defer p.mu.Unlock()` at the top of the function, eliminating the manual unlocks on early returns. Keep the script-exhausted ctx-blocking behavior intact.

### M12 — Orphaned `.tmp` files on rename failure
- **File:** `internal/store/file/file_store.go:237-250`
- **Fix:** Add `defer os.Remove(tmp)` immediately after writing the temp file. After successful rename, the deferred remove is a no-op (the file no longer exists at `tmp`).

---

## Nice-to-have (LOW) — defer to follow-up

These are LOW-priority polish items captured for a future cleanup pass; they will not block tonight's commit.

- L1 — Add `PeerSlotSeed` constant in `model/conversation.go` instead of inline `model.PeerSlot("seed")` literal in envelope.go.
- L2 — Replace `warnings := []string{}` with `var warnings []string` in `broker.go:57` (nil slice is more idiomatic).
- L3 — Remove the meaningless `var _ = io.EOF` from `persistent.go` (covered by M3).
- L4 — Add `mockagent_test.go` with unit tests for `extractMarker` / `readEnvelopeMarker`.
- L5 — Strengthen `TestConverseEndToEndCompletesViaSentinel` to assert turn count and triggering peer.
- L6 — Replace 100ms negative-assertion in `sentinel_test.go` with a synchronous channel check.
- L7 — Increase `markerSuffixBytes` from 6 to 16 (eliminates a future concern when distributed mode lands).
- L8 — Validate `--sentinel` is non-empty and contains no newlines.
- L9 — Document `logs/` in `.gitignore` so default log output is not accidentally committed.
- L10 — Add concurrent-access test for conversation persistence.
- L11 — Add tests for `ContainsMarker` / `StripMarkerAndAfter` empty-token guards.
- L12 — Remove dead `decodeFinalResult` helper in `converse_test.go` or use it.
- L13 — Add CLI tests for `--bus invalid`, `--log-backend invalid`, `--per-turn-timeout=0` after M8 lands.

---

## Acceptance criteria for fix subagent

1. All H1–H11 must be fixed and committed individually (one logical commit per finding, conventional commits format).
2. M1–M12 should be fixed; M9 (sentinel line-anchoring) and M11 (mock unlock pattern) can be deferred if time-constrained.
3. After all fixes: `go test -race -count=1 -p 1 -timeout 180s ./...` must pass on every package except `internal/orchestrator` (pre-existing flake).
4. After all fixes: `go vet ./...` must be clean.
5. After all fixes: `go build ./...` must succeed.
6. The fix subagent should report each fix's commit hash and the final test-suite output.
