# Phase 5–7 Review Report (merged findings)

**Date:** 2026-04-08 (continuation session)
**Scope:** Phases 5, 6, 7 of the N-peer implementation. Commits since `9334977` up to current HEAD.
**Reviews run:**

- `everything-claude-code:golang-patterns` — guidance loaded earlier in the session, manual review pass executed against the new code
- `everything-claude-code:golang-testing` — guidance loaded earlier, manual review pass executed
- `everything-claude-code:security-review` — guidance loaded earlier, manual review pass executed
- **`/codex:review`** — still does not exist as a slash command. Direct `codex exec --model gpt-5-codex` invocation dispatched in the background; was still running when this report was finalized. Same outcome as the previous session's attempt — Codex's `--output-last-message` only writes on completion, and the run takes longer than the session window. **If/when the file `/tmp/codex-review-p567.txt` appears, it should be appended to this report.**

## Files reviewed

Added:
- `internal/store/v1compat/reader.go`, `reader_test.go`, `testdata/v1_conversation.json`
- `internal/store/file/file_store_v2_test.go`
- `internal/conversation/broker_npeer_test.go`
- `internal/cli/converse_npeer.go`, `converse_npeer_run.go`, `converse_npeer_test.go`, `converse_npeer_cli_test.go`

Modified:
- `internal/store/file/file_store.go` — `stampSchemaVersion` helper
- `internal/conversation/broker.go` — `BrokerDeps.PeersByID/PeerOrder`, `peerIDs()`, `startAllPeers/stopAllPeers/specForPeerID`, transportFor map lookup, opener selection branching
- `internal/cli/converse.go` — `--peer` `stringSliceFlag`, branch into `runNPeerConverse`
- `internal/testutil/mockagent/main.go` — `MOCK_NEXT_TRAILER` env var, `runMultiTurn` trailer injection

## Findings

### golang-patterns

| # | Severity | File | Finding | Action |
|---|---|---|---|---|
| GP-1 | low | `internal/conversation/broker.go:295-326` | `startAllPeers` and `stopAllPeers` each create their own `WithTimeout(5s)` cleanup contexts. The cleanup-on-partial-start path also creates one in `stopPeerIDs`. Three near-identical context constructions. Refactor to a single helper if it grows further. | Optional — leave for now. |
| GP-2 | info | `internal/cli/converse_npeer.go:1-220` | Hand-written key=value parser instead of using `csv.Reader`. Justified because `csv` cannot handle the `key=value` shape and the project explicitly avoids unnecessary dependencies. | Keep. |
| GP-3 | info | `internal/conversation/broker.go:266-271` | Conditional `slotFromPeerID` branch — N-peer mode treats the slot AS the id (`PeerSlot(decision.Next)`) while legacy maps "a"/"b". Documented inline. | Keep. |
| GP-4 | low | `internal/store/v1compat/reader.go:21-30` | `Detect` swallows JSON parse errors and returns `false`. Documented as a sniff helper that branches to "treat as v2 / let downstream handle errors". Acceptable but could surprise a caller expecting parse errors to surface. | Document — already done. |

### golang-testing

| # | Severity | File | Finding | Action |
|---|---|---|---|---|
| GT-1 | info | `internal/conversation/broker_npeer_test.go` | 5 tests cover the full N-peer policy surface (round-robin fallback, addressed routing, sentinel from middle peer, unknown addressee, self-address). All using table-style structure where applicable. | None. |
| GT-2 | info | `internal/cli/converse_npeer_test.go` | 18 parser unit tests + 7 CLI integration tests. Edge cases: missing keys, invalid id regex, unknown keys, unterminated quoted values, escapes, mixing flags, duplicate ids, unknown opener, missing seed. | None. |
| GT-3 | info | `internal/store/v1compat/reader_test.go` | 7 tests including a frozen JSON fixture, no-op v2 case, opener default. | None. |
| GT-4 | low | `internal/cli/converse_npeer_test.go` | No fuzz test for `parseKeyValueList`. Hand-written parser with multiple branches (quoted/bareword/escapes) is a natural fuzz target. | Add fuzz seed corpus. |

### security-review

| # | Severity | File | Finding | Action |
|---|---|---|---|---|
| SR-1 | info | `internal/cli/converse_npeer.go` | All peer ids validated against the existing `model.PeerID` regex (`^[a-z][a-z0-9_-]{0,31}$`) before they reach the broker, store, or log paths. No id-injection vector. | Validated. |
| SR-2 | low | `internal/cli/converse_npeer_run.go:142-152` | `provider:` URI is constructed from the user-supplied `provider=` value via string concatenation. No validation of the provider string. Same surface as the legacy `--peer-a provider:claude-code` path, which also does no validation — not a regression, but worth flagging for the broader URI parser hardening pass. | Defer (matches legacy). |
| SR-3 | info | `internal/cli/converse_npeer_run.go:96-103` | `workingDir` is sanitised via `filepath.Clean` and validated against `os.Stat`. Same hardening as the legacy 2-peer path. | Validated. |
| SR-4 | info | `internal/cli/converse_npeer.go` parseKeyValueList | No format-string injection (uses `%q` everywhere); no shell exec; no template eval; no filesystem access. The parser is byte-level and operates on a single string. | Validated. |
| SR-5 | info | `internal/store/file/file_store.go` `stampSchemaVersion` | Pure function, no I/O, no race surface (locking is in callers and unchanged). Idempotent. | Validated. |
| SR-6 | medium | broader | Per-peer log paths still use `logs/<conv_id>/peer-<slot>/...`. With N-peer ids like `alice`, `bob`, `carol`, the `<slot>` segment is now `alice`, `bob`, etc. This is bounded by the PeerID regex (`[a-z][a-z0-9_-]{0,31}`), so no path traversal is possible. But: if a future change ever loosens the PeerID regex, log path construction must be re-audited. | Document the dependency in the ADR. |
| SR-7 | medium | `internal/testutil/mockagent/main.go` | `MOCK_NEXT_TRAILER` env var is interpolated directly into the output: `"<<NEXT: " + nextTrailer + ">>"`. There is no validation. A test that sets `MOCK_NEXT_TRAILER=' ; rm -rf'` would emit `<<NEXT:  ; rm -rf>>` as plain text — harmless because the broker only matches the regex. Test-only code, attacker-controlled-env-var threat model is null. | None. Test-only. |

### codex review (deferred — second failed attempt)

Codex `gpt-5-codex` review was dispatched twice in this branch's history (once per session). Both attempts exited cleanly with **no review output**:

- Session 1: process ran past the session window; `--output-last-message` only writes on completion, so no findings were ever flushed.
- Session 2: process exited (`exit code 0`) but produced only `"Reading additional input from stdin..."` and 39 bytes of output. The `codex exec` invocation appears to expect stdin even when given a positional prompt argument under non-tty redirection (`2>&1 | tail -100` confuses it).

Workaround for next attempt: invoke `codex exec` directly in an interactive shell, OR write the prompt to a file and pipe it via `< prompt.txt`, OR use `--prompt @prompt.txt` if the CLI supports it.

Recommendation: re-run manually after this session ends. If findings appear, file them as an addendum to this report.

## Actionable work items

Applied this session:

- **A1** (GT-4): add a fuzz test for `parseKeyValueList` to catch escape/quote edge cases under random input.
- **A2** (GP-4 / SR-6): add a comment to `stampSchemaVersion` explicitly noting it ignores I/O concurrency (caller's job), and to PeerID-keyed log paths in the ADR documenting the regex dependency.

Deferred to follow-up:

- **D1** (codex): re-run the codex review after the session ends; append findings.
- **D2** (SR-2): URI parser hardening pass — same surface as legacy. Out of scope for the additive Phase 5–7 work.
- **D3** (GP-1): broker cleanup-context refactor if a fourth caller appears.

## Gate status

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — 515 passed, 0 failed
- New code coverage: implicit via exhaustive table-driven tests; explicit `-cover` snapshot not captured.
