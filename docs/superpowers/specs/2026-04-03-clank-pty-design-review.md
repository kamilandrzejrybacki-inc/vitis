# Clank PTY Design Review And Safeguards

- **Status**: Draft
- **Date**: 2026-04-04
- **Author**: Codex / Claude-code-leak review
- **Scope**: PTY-only design review for Clank
- **Explicit non-goal**: Recommending Claude Code headless/SDK integration as the primary path

---

## 1. Purpose

This document reviews Clank's current PTY-based design against:

- the current Clank brainstorm docs
- the Claude Code codebase and distilled ADR/spec docs reviewed in `../claude-code-leak`
- public Anthropic product, help-center, and legal documentation

The goal is not to change Clank into a different product. The goal is to make the PTY-only design safer, less brittle, and less likely to drift into a product shape that creates account, policy, or operational risk.

---

## 2. Executive Summary

The current Clank design is directionally sensible as a local PTY orchestrator, but it is too optimistic in four areas:

1. It treats terminal interaction as a simple "prompt in, answer out" loop.
2. It does not distinguish "completed" from "blocked on an interactive prompt".
3. It stores PTY data too close to text-only assumptions.
4. It does not yet draw a hard enough boundary between:
   - local personal automation around a user's own Claude Code session
   - a hosted/shared product that effectively brokers Claude consumer subscriptions

The most important strategic conclusion is:

- **Local-first PTY automation is plausible.**
- **Shared or hosted access to Claude Code through consumer Claude accounts is high-risk and should be treated as out of scope unless Anthropic explicitly approves that use.**

I did **not** find a public Anthropic page that explicitly says "driving Claude Code through a PTY will get a user banned." I **did** find enough contractual and product-policy language to conclude that some product shapes of Clank would be risky enough that they should be assumed unsafe until validated directly with Anthropic.

---

## 3. Inputs Reviewed

### Clank docs

- `docs/superpowers/specs/2026-04-03-clank-agent-bridge-design.md`
- `docs/superpowers/specs/2026-04-03-clank-implementation-plan.md`

### Claude Code distilled docs

- `../claude-code-leak/docs/adr/0003-typed-tools-and-centralized-permissions.md`
- `../claude-code-leak/docs/adr/0004-concurrent-readonly-tool-execution.md`
- `../claude-code-leak/docs/adr/0006-persisted-task-model-for-background-work.md`
- `../claude-code-leak/docs/adr/0007-custom-terminal-renderer.md`
- `../claude-code-leak/docs/spec/05-permissions-and-sandboxing.md`
- `../claude-code-leak/docs/spec/07-background-tasks-agents-and-workflows.md`
- `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`
- `../claude-code-leak/docs/spec/11-auth-config-telemetry-and-operations.md`

### Claude Code source areas traced from those docs

- `../claude-code-leak/src/src/tasks/LocalShellTask/LocalShellTask.tsx`
- `../claude-code-leak/src/src/server/web/scrollback-buffer.ts`
- `../claude-code-leak/src/src/server/web/session-store.ts`
- `../claude-code-leak/src/src/server/web/session-manager.ts`
- `../claude-code-leak/src/src/server/web/pty-server.ts`
- `../claude-code-leak/src/src/server/web/auth/adapter.ts`
- `../claude-code-leak/src/src/server/web/auth/token-auth.ts`
- `../claude-code-leak/src/src/server/web/auth/apikey-auth.ts`
- `../claude-code-leak/src/src/utils/permissions/permissionSetup.ts`
- `../claude-code-leak/src/src/utils/permissions/permissions.ts`
- `../claude-code-leak/src/src/setup.ts`

### Official Anthropic pages reviewed

- Consumer Terms of Service: <https://www.anthropic.com/legal/consumer-terms>
- Commercial Terms of Service: <https://www.anthropic.com/legal/commercial-terms>
- Using Claude Code with your Pro or Max plan: <https://support.claude.com/en/articles/11145838-using-claude-code-with-your-pro-or-max-plan>
- How do usage and length limits work?: <https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work>
- Using agents according to our usage policy: <https://support.claude.com/en/articles/12005017-using-agents-according-to-our-usage-policy>

---

## 4. Policy And Account-Risk Assessment

### 4.1 What the official docs clearly imply

The public consumer terms currently say, in substance:

- consumer Claude services are distinct from API-key/commercial usage
- account credentials must not be shared or made available to others
- except via API key or explicit permission, the consumer services must not be accessed through automated or non-human means
- users must not bypass protective measures

The public help-center docs also currently state:

- Claude Code usage under Pro/Max is tied to the same Claude account used elsewhere
- Claude usage across Claude product surfaces counts toward the same usage limits
- setting `ANTHROPIC_API_KEY` makes Claude Code use API billing instead of the user's Claude subscription

The public commercial terms currently say, in substance:

- customers may build products and services for their own users under the commercial/API offering
- Anthropic can suspend access if users violate policies or use restrictions

### 4.2 Practical reading for Clank

This leads to the following practical boundary:

#### Lower-risk shape

- A local CLI that a user runs on their own machine, against their own locally installed Claude Code, within their own account context.

This is still not guaranteed safe by an explicit public blessing for PTY wrapping. But it is much closer to "personal automation around your own tool" than to "building a broker around consumer access."

#### High-risk shape

- A hosted service, daemon, browser-accessible PTY, bot platform, or team product that lets other people drive Claude Code through Clank while using Claude.ai/Pro/Max/consumer auth.

This starts to look like:

- sharing or brokering consumer account access
- automated access to consumer services
- a third-party product built on top of Claude.ai subscription capacity rather than the API/commercial surface

That is the configuration most likely to trigger enforcement or account problems.

### 4.3 Recommendation

Clank should adopt the following product boundary in writing:

- **Supported MVP**: local, one-user, local-machine PTY automation around the user's own Claude Code installation.
- **Explicitly unsupported**:
  - multi-tenant hosted Clank using Claude consumer auth
  - browser-exposed PTY service using a single Claude account
  - pooled/shared Claude subscriptions
  - any design that makes one user's Claude login or session available to another user

If hosted/shared execution is ever needed, the safer direction is:

- per-user Anthropic API keys
- commercial/API terms
- isolated execution environments

---

## 5. Main Design Gaps In The Current Clank Spec

### 5.1 The run model is too simple

Current design:

- send prompt
- wait for exit or prompt or silence
- extract final answer

Current statuses are effectively:

- `completed`
- `timeout`
- `partial`
- `error`

This is too weak for real PTY control because terminal sessions often end up in states like:

- waiting for interactive confirmation
- permission approval required
- login/authentication required
- rate-limited / usage exhausted
- crashed after partial output
- interrupted by signal
- waiting on slow model output but still alive

### 5.2 Silence is not completion

The current Claude adapter plan uses:

1. process exit
2. prompt reappearance
3. silence after output

That third heuristic is dangerous.

In the Claude Code source review, the closest PTY-adjacent logic does **not** treat "no output for N seconds" as terminal completion when the tail looks like a prompt. Instead, it raises a blocked-on-input notification:

- `LocalShellTask.tsx`
- `docs/spec/07-background-tasks-agents-and-workflows.md`

Clank should do the same.

### 5.3 Raw PTY output is not guaranteed to be text

The current Postgres schema stores `stream_events.chunk` as `TEXT`.

That is a bad fit for PTY capture.

Reasons:

- escape sequences may not be valid UTF-8
- providers may emit mixed binary-like control bytes
- text decoding choices can permanently corrupt later parsing

The reviewed Claude Code source uses a byte-preserving scrollback buffer for PTY output:

- `src/src/server/web/scrollback-buffer.ts`

Clank should preserve raw bytes first, then derive normalized text second.

### 5.4 Session isolation is underspecified

`SpawnSpec` currently carries:

- command
- args
- env
- cwd

This is not enough if Clank ever runs repeated sessions, parallel sessions, or any shared/server mode.

The Claude Code PTY server isolates users with:

- user-specific home directories
- user-specific session ownership
- per-user limits
- encrypted API-key storage in server-side session state

Clank does not need all of that in MVP, but it does need a clearer stance on:

- whether `HOME` is inherited or isolated
- whether `.claude` state is shared across runs
- whether multiple Clank runs can contend on one Claude Code auth/config directory

### 5.5 Storage is overbuilt in the wrong place

Postgres from day one is less important than transcript truthfulness and detector quality.

Before adding a DB backend, Clank needs:

- a robust corpus of captured transcripts
- fixtures for auth prompts
- fixtures for permission prompts
- fixtures for rate-limit surfaces
- fixtures for blocked interactive prompts
- fixtures for ANSI-heavy output

Without that, the adapter tests will only prove that Clank works against Clank's mock agent, not against the messy states a real Claude Code PTY produces.

---

## 6. PTY Mechanisms Worth Porting From Claude Code

This section maps design lessons back to the actual Claude Code docs and source that motivated them.

### 6.1 Conservative prompt-block detection

Distilled doc:

- `../claude-code-leak/docs/spec/07-background-tasks-agents-and-workflows.md`

Source:

- `../claude-code-leak/src/src/tasks/LocalShellTask/LocalShellTask.tsx`

Useful mechanism:

- check whether output has stopped growing
- inspect only the recent tail
- look for prompt-like last-line patterns
- do **not** mark the task completed
- surface a separate "waiting for interactive input" state

This should be ported almost directly into Clank's Claude provider adapter.

### 6.2 Byte-preserving scrollback ring buffer

Distilled doc:

- `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`

Source:

- `../claude-code-leak/src/src/server/web/scrollback-buffer.ts`

Useful mechanism:

- store the last N bytes of PTY output
- keep raw bytes, not normalized text
- use a fixed-capacity ring buffer

This is a better building block than a text-only transcript accumulator.

### 6.3 Grace-period resume semantics

Distilled doc:

- `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`
- `../claude-code-leak/docs/adr/0006-persisted-task-model-for-background-work.md`

Source:

- `../claude-code-leak/src/src/server/web/session-store.ts`
- `../claude-code-leak/src/src/server/web/session-manager.ts`

Useful mechanism:

- detached PTY sessions survive disconnect for a grace period
- the PTY can be reattached
- scrollback is replayed on resume
- final cleanup is two-phase and explicit

Even in a single-turn local CLI, a lightweight version of this helps:

- interrupted caller process
- parent orchestrator crash
- debugging ambiguous transcripts

### 6.4 Per-user quotas if server mode ever appears

Distilled doc:

- `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`

Source:

- `../claude-code-leak/src/src/server/web/session-manager.ts`
- `../claude-code-leak/src/src/server/web/pty-server.ts`

Useful mechanism:

- max concurrent sessions
- hourly creation quota
- retry-after calculation
- user ownership attached to sessions

If Clank ever grows beyond a local CLI, these are table stakes.

### 6.5 Secret handling and account isolation

Distilled doc:

- `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`

Source:

- `../claude-code-leak/src/src/server/web/auth/adapter.ts`
- `../claude-code-leak/src/src/server/web/auth/apikey-auth.ts`
- `../claude-code-leak/src/src/server/web/auth/token-auth.ts`

Useful mechanism:

- encrypted at-rest storage for API keys
- signed session cookies
- explicit warning that token auth without a token is wide open

Clank does not need browser cookies, but it does need the same attitude:

- auth and session state must never be "whatever happens to be in the parent shell"

### 6.6 Hard skepticism about bypass modes

Distilled doc:

- `../claude-code-leak/docs/spec/05-permissions-and-sandboxing.md`

Source:

- `../claude-code-leak/src/src/utils/permissions/permissionSetup.ts`
- `../claude-code-leak/src/src/utils/permissions/permissions.ts`
- `../claude-code-leak/src/src/setup.ts`

Useful mechanism:

- dangerous allow-rules are stripped in automated modes
- some asks remain bypass-immune
- the most dangerous permission bypasses are heavily constrained

Clank should mirror this principle:

- never normalize "just disable permissions" into the happy path

---

## 7. Recommended Spec Changes

### 7.1 Add a stronger run-state model

Replace the current small status set with:

```go
type RunStatus string

const (
    RunCompleted        RunStatus = "completed"
    RunBlockedOnInput   RunStatus = "blocked_on_input"
    RunPermissionPrompt RunStatus = "permission_prompt"
    RunAuthRequired     RunStatus = "auth_required"
    RunRateLimited      RunStatus = "rate_limited"
    RunInterrupted      RunStatus = "interrupted"
    RunTimeout          RunStatus = "timeout"
    RunPartial          RunStatus = "partial"
    RunCrashed          RunStatus = "crashed"
    RunError            RunStatus = "error"
)
```

This keeps PTY-only semantics while making results much more informative.

### 7.2 Split "completion detection" from "state detection"

Current `DetectCompletion` is doing too much.

Suggested change:

```go
type TranscriptObservation struct {
    Status     RunStatus
    Terminal   bool
    Confidence float64
    Reason     string
    Evidence   []string
}

type CompletionContext struct {
    RawTail         []byte
    NormalizedTail  string
    ElapsedMs       int64
    IdleMs          int64
    ExitCode        *int
    BytesSeen       int64
    LastWriteMs     int64
}

type Adapter interface {
    ID() string
    BuildSpawnSpec(cwd string, env map[string]string) SpawnSpec
    FormatPrompt(raw string) []byte
    Observe(context CompletionContext) *TranscriptObservation
    ExtractResponse(rawTranscript []byte, normalizedTranscript string) ExtractionResult
}
```

The important difference is:

- not every observed state is terminal
- not every non-running state is success

### 7.3 Make prompt-block detection first-class

Add provider-specific detection for:

- yes/no prompts
- "press enter to continue"
- "login required"
- "permission required"
- "rate limit / usage exhausted"

At minimum, the Claude Code adapter should emit distinct reasons for:

- blocked on interactive input
- auth flow required
- usage/rate-limit screen

### 7.4 Preserve raw bytes and normalized text separately

Change storage model:

- raw PTY chunks: `[]byte`
- normalized chunks: derived lazily or stored as a second field

Suggested Postgres change:

```sql
CREATE TABLE stream_events (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ NOT NULL,
    chunk_raw BYTEA NOT NULL,
    chunk_text TEXT,
    chunk_encoding TEXT NOT NULL DEFAULT 'raw'
);
```

### 7.5 Extend session metadata

Current `Session` is missing fields that matter for diagnosing PTY behavior.

Suggested additions:

- `AuthMode string` (`subscription`, `api_key`, `unknown`)
- `BlockedReason *string`
- `ProviderVersion *string`
- `TerminalCols *int`
- `TerminalRows *int`
- `BytesCaptured *int64`
- `Warnings []string`

### 7.6 Add isolation to `SpawnSpec`

Suggested additions:

```go
type SpawnSpec struct {
    Command      string
    Args         []string
    Env          map[string]string
    Cwd          string
    HomeDir      string
    TerminalCols int
    TerminalRows int
}
```

For MVP, `HomeDir` can default to inherited `HOME`.

But the concept needs to exist early so Clank can later isolate sessions instead of baking in hidden shared-state assumptions.

### 7.7 Make "local-only" an explicit product constraint

Add to the design spec:

- Clank MVP is a local CLI for one user on one machine.
- Clank MVP is not a hosted PTY broker.
- Clank MVP does not expose browser sessions or shared session IDs.
- Any future server mode must define:
  - per-user identity
  - per-user home isolation
  - per-user quotas
  - secret encryption

### 7.8 Reorder implementation priorities

Current plan order:

- PTY runtime
- store backends
- adapter
- orchestrator

Recommended order:

1. Core types
2. PTY runtime
3. Byte-preserving transcript buffer plus ANSI normalizer
4. Claude adapter state detector
5. Real transcript fixture corpus
6. Orchestrator
7. File backend
8. CLI
9. Integration tests
10. Postgres backend

Postgres is not where the MVP risk is.

### 7.9 Improve the test matrix

Add transcript fixtures and E2E cases for:

- permission prompt appears and waits
- auth/login prompt appears
- rate-limit / usage exhausted message
- Claude exits cleanly with no answer
- Claude prints answer, then asks a follow-up confirmation
- ANSI-heavy output
- wrapped output due to narrow terminal width
- process survives but emits no output after initial banner
- parent interrupt while process is still running

### 7.10 Add an "unsafe modes" section to the spec

Clank should formally state:

- it will not auto-confirm prompts
- it will not pass dangerous bypass flags by default
- it will not interpret stalled prompt states as success
- it will not claim that consumer Claude subscriptions are safe for multi-user brokering

---

## 8. Recommended MVP Scope

### 8.1 What MVP should include

- local CLI
- PTY runtime
- byte-preserving transcript capture
- Claude Code provider adapter
- conservative blocked-input detection
- response extraction with confidence
- file-based persistence
- transcript fixture corpus

### 8.2 What MVP should exclude

- Postgres
- daemon/server mode
- browser-facing PTY
- session sharing
- multi-tenant auth
- automatic permission answering
- automatic login flows

### 8.3 Acceptable MVP positioning

Safe positioning:

- "A local PTY orchestrator for your own installed agent CLI"

Unsafe positioning:

- "A platform that lets teams use Claude Code through Clank"
- "A hosted Claude Code bridge"
- "A way to turn one Claude Pro/Max account into shared automation capacity"

---

## 9. Proposed Additions To The Existing Docs

### 9.1 Amend the design spec

Add:

- product-boundary section
- richer run-state model
- raw-bytes transcript storage
- blocked-input detection
- isolation assumptions
- unsafe modes and unsupported deployment shapes

### 9.2 Amend the implementation plan

Add new phases or steps:

- transcript normalizer
- state detector for blocked/auth/rate-limit cases
- real captured transcript fixtures
- manual acceptance tests for prompt-block states
- policy boundary notes in CLI help and README

---

## 10. Suggested Wording For The Design Spec

Suggested text:

> Clank is a local PTY orchestrator for AI agent CLIs. It is designed for local execution against a user's own installed CLI and account context. Clank MVP does not provide hosted, pooled, or multi-tenant access to Claude Code or other subscription-backed consumer AI products.

Suggested text:

> In PTY environments, "no output" is not sufficient evidence of completion. Clank distinguishes successful completion from blocked interactive prompts, authentication requirements, permission prompts, and rate-limit states.

Suggested text:

> Clank stores raw PTY bytes as the source of truth and derives normalized text for parsing and display. Parsing must not depend on lossy text conversion of the capture stream.

---

## 11. Open Questions

These questions should be answered before implementation hardens:

1. Will Clank ever be allowed to pass through `HOME`, or must it always isolate `.claude` state per run?
2. Will Clank ever intentionally support more than one concurrently active Claude-authenticated session for one local user?
3. If Claude Code prompts for login, should Clank:
   - fail immediately
   - classify `auth_required`
   - or support a future attach/resume flow?
4. Does Clank need a `doctor` or `probe` command to capture provider fingerprints and transcript markers before normal runs?
5. Do we want a transcript-corpus directory checked into the repo from day one?

---

## 12. Final Recommendation

Proceed with Clank as a PTY-only local tool if and only if the project adopts these principles:

- local-first
- one-user-first
- no auto-confirmation of interactive prompts
- no default bypass of Claude Code safety prompts
- no assumption that consumer subscriptions are safe for brokering or shared access
- transcript fidelity over early infrastructure complexity

The PTY-only direction is viable.

The current spec is not yet robust enough for real-world Claude Code terminal behavior.

The biggest win is not changing the transport.

The biggest win is upgrading the PTY model from:

- "wait, guess, scrape"

to:

- "observe, classify, preserve, and only then extract"

---

## Appendix A: Traceability Notes

### Distilled docs to source

- PTY blocked-input behavior:
  - `../claude-code-leak/docs/spec/07-background-tasks-agents-and-workflows.md`
  - `../claude-code-leak/src/src/tasks/LocalShellTask/LocalShellTask.tsx`

- Resume and scrollback:
  - `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`
  - `../claude-code-leak/src/src/server/web/scrollback-buffer.ts`
  - `../claude-code-leak/src/src/server/web/session-store.ts`
  - `../claude-code-leak/src/src/server/web/session-manager.ts`

- Session quotas and ownership:
  - `../claude-code-leak/docs/spec/09-remote-bridge-and-server-surfaces.md`
  - `../claude-code-leak/src/src/server/web/pty-server.ts`
  - `../claude-code-leak/src/src/server/web/session-manager.ts`

- Secret handling:
  - `../claude-code-leak/src/src/server/web/auth/adapter.ts`
  - `../claude-code-leak/src/src/server/web/auth/apikey-auth.ts`
  - `../claude-code-leak/src/src/server/web/auth/token-auth.ts`

- Dangerous bypass handling:
  - `../claude-code-leak/docs/spec/05-permissions-and-sandboxing.md`
  - `../claude-code-leak/src/src/utils/permissions/permissionSetup.ts`
  - `../claude-code-leak/src/src/utils/permissions/permissions.ts`
  - `../claude-code-leak/src/src/setup.ts`

### Official external references

- Consumer Terms of Service:
  - <https://www.anthropic.com/legal/consumer-terms>

- Commercial Terms of Service:
  - <https://www.anthropic.com/legal/commercial-terms>

- Using Claude Code with your Pro or Max plan:
  - <https://support.claude.com/en/articles/11145838-using-claude-code-with-your-pro-or-max-plan>

- How do usage and length limits work?:
  - <https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work>

- Using agents according to our usage policy:
  - <https://support.claude.com/en/articles/12005017-using-agents-according-to-our-usage-policy>
