# Vitis

**Vitis** is a local-first orchestrator for driving AI agent CLIs (Claude Code, Codex, …) through a real PTY. It started as a single-prompt-and-extract harness and has grown into a multi-turn agent-to-agent (A2A) broker with pluggable token-efficiency layers.

In one sentence: **Vitis lets two interactive AI agents have a structured, persistent, observable conversation through your terminal — locally, deterministically, and cheaply.**

```
┌──────────────┐  envelope    ┌──────────────────────┐  envelope    ┌──────────────┐
│   Peer A     │  ──────────► │  Vitis Conversation  │  ──────────► │   Peer B     │
│  (claude /   │              │       Broker         │              │  (claude /   │
│   codex /    │  ◄────────── │  • alternation       │  ◄────────── │   codex /    │
│   mock /     │  reply       │  • marker injection  │  reply       │   mock /     │
│   portkey)   │              │  • sentinel/judge    │              │   portkey)   │
└──────────────┘              │  • event bus         │              └──────────────┘
                              └──────────────────────┘
```

## What it does

Two execution modes, both backed by the same PTY runtime and persistence layer:

| Mode | Command | Purpose |
|---|---|---|
| **Single-shot** | `vitis run` | Send one prompt, capture the agent's response, exit. |
| **A2A multi-turn** | `vitis converse` | Drive two long-lived peer agents through alternating turns until a sentinel, judge, or max-turns cap fires. |

Plus inspection helpers:

| Command | Purpose |
|---|---|
| `vitis peek --session-id <id>` | Read back turns from a recorded session. |
| `vitis doctor [--provider <id>]` | Verify the local environment: provider availability, version, and rtk integration status. |

## Quick start

```bash
# 1. Build it
go build -o vitis ./cmd/vitis

# 2. Single-shot run against real Claude Code
./vitis run --provider claude-code --prompt "what is 2+2?"

# 3. Two-peer A2A conversation, alternating until <<END>>
./vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "You are a Go expert. Briefly explain channels. End with <<END>>." \
  --seed-b "You are a critic. Find one flaw. End with <<END>>." \
  --max-turns 6 \
  --terminator sentinel \
  --style caveman-full

# 4. Check what's wired up
./vitis doctor --provider claude-code | jq .
```

## A2A features in one table

| Feature | Flag | Default | Notes |
|---|---|---|---|
| Peer URI scheme | `--peer-a`, `--peer-b` | required | `provider:claude-code`, `provider:codex`, `provider:mock` (test builds), future `vitis://`, `stdio://` |
| Per-peer options | `--peer-a-opt key=value` (repeatable) | — | `model`, `reasoning-effort`, `cwd`, `home`, allowlisted `env_KEY=value` |
| Single seed | `--seed "..."` | — | Same starter prompt for both peers |
| Asymmetric seeds | `--seed-a "..." --seed-b "..."` | — | Per-peer briefings (debate, role-play, etc.) |
| Opener | `--opener a\|b` | `a` | Which peer speaks first |
| Hard turn cap | `--max-turns N` | 50 | Always-on safety rail; range 1..500 |
| Per-turn timeout | `--per-turn-timeout SEC` | 300 | Max seconds for any single peer reply; capped at 3600 |
| Overall timeout | `--overall-timeout SEC` | `max-turns × per-turn-timeout` | Capped at 86400 (24h) |
| Terminator | `--terminator sentinel` | sentinel | `judge` arrives in plan 3 |
| Sentinel token | `--sentinel "<<END>>"` | `<<END>>` | Line-anchored match (peer must emit on its own line) |
| Reply style | `--style caveman-{lite,full,ultra}` | `normal` | Embedded [JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman) rules; ~75% reply-token compression |
| Bus backend | `--bus inproc` | inproc | NATS arrives in plan 4 |
| Log backend | `--log-backend file` | file | Postgres arrives in plan 3 |
| Working directory | `--working-directory PATH` | cwd | Validated against path-traversal |
| Stream turns | `--stream-turns` | true | Emit each turn as JSONL on stdout during the run |

## Token-efficiency stack

Vitis itself doesn't compress tokens — but it cleanly composes with three external tools that do, each tackling a different layer of the conversation flow:

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                                                          │
│   Tool-call OUTPUT                  ┌──────────────┐                     │
│   compressed by rtk    ────────────►│ Agent input  │◄──── Other peer's   │
│   (60-90% on shell)                 │   context    │      reply (caveman │
│                                     └──────┬───────┘      style; ~75%   │
│                                            │              compression)   │
│                                            ▼                             │
│                                     ┌──────────────┐                     │
│                                     │   Inference  │   (cheap upstream:  │
│                                     │   via free   │    Groq, Deepseek,  │
│                                     │   provider   │    NVIDIA NIM via   │
│                                     │   gateway    │    Portkey gateway) │
│                                     └──────┬───────┘                     │
│                                            ▼                             │
│                                     ┌──────────────┐                     │
│                                     │ Reply output │ ──► Vitis broker    │
│                                     │   shrunk by  │    forwards as next │
│                                     │   caveman    │    envelope         │
│                                     └──────────────┘                     │
└──────────────────────────────────────────────────────────────────────────┘
```

| Layer | Tool | Repo | What it shrinks |
|---|---|---|---|
| Tool call **input** to agent | [rtk](https://github.com/rtk-ai/rtk) | rtk-ai/rtk | git/ls/cat/grep/test runners; ~60-90% |
| Model **output** from agent | [caveman](https://github.com/JuliusBrussee/caveman) | JuliusBrussee/caveman | Filler, articles, hedging in replies; ~75% |
| LLM provider | [portkeyagent](https://github.com/kamilrybacki/portkeyagent) | kamilrybacki/portkeyagent | Routes to free upstream models via the [Portkey AI Gateway](https://github.com/Portkey-AI/gateway) |

Combined effect on a multi-turn A2A run: roughly **3-4× more turns within a fixed context budget**, against zero-cost LLMs, with no broker code changes.

Setup helpers in [`tests/manual/`](tests/manual/) wire all three:

```bash
./tests/manual/setup_rtk.sh                  # install rtk + register hook for both providers
go install github.com/kamilrybacki/portkeyagent@latest
# Edit tests/manual/.portkey.env with your Portkey credentials
./tests/manual/13_converse_portkey.sh        # real conversation through Portkey
./tests/manual/14_rtk_integration.sh         # verify rtk hook is active
./tests/manual/15_converse_caveman.sh        # measure caveman compression delta
```

## Status

| Layer | Implemented | In design / planned |
|---|---|---|
| PTY runtime | Single-shot + persistent (multi-turn) | — |
| Adapters | claude-code, codex (single-shot adapter + persistent-mode bypass) | sidecar JSONL detection (Plan 2.5) |
| Conversation broker | Full state machine, strict alternation, max-turns cap, control draining, peer-crash handling | — |
| Bus | In-process channel fan-out (`inproc`) | NATS (Plan 4) |
| Terminator | Sentinel (line-anchored) | Judge with bus + provider modes (Plan 3) |
| Peer transport | `provider:` (local PTY), `mock` (test-only) | `vitis://` remote (Plan 4), `stdio://` framed (Plan 5) |
| Persistence | File store (sessions + conversations + raw events) | Postgres conversations (Plan 3) |
| CLI | `run`, `peek`, `doctor`, `converse` | `converse-serve`, `converse-tail` (Plan 4) |
| Reply style | `normal`, `caveman-{lite,full,ultra}` | per-peer style override |
| rtk integration | Doctor probes both providers; setup helper installs hooks | — |
| Observability | JSONL stream of turns; FinalResult JSON | NATS-based fan-out for live tail (Plan 4) |
| Pre-existing flake | `internal/orchestrator.TestRunHappyPath` (PTY timing race, predates A2A work) | tracked separately |

## Test layout

| Layer | Where | How |
|---|---|---|
| **Unit tests** (Go `*_test.go`) | alongside production code in `internal/...` | `go test -race -count=1 ./internal/...` |
| **CLI validation tests** | `internal/cli/*_test.go` | flag parsing, error paths, exit codes |
| **End-to-end CLI tests** | `internal/cli/*_e2e_test.go` | drive `RunCommand` / `ConverseCommand` against the bundled mock agent |
| **Orchestrator integration tests** | `internal/orchestrator/integration_test.go` | full single-shot path through every `MOCK_MODE` |
| **Top-level integration test** | `tests/integration/run_integration_test.go` | exercises the compiled `vitis` binary |
| **Manual test suite** | `tests/manual/*.sh` | 15 scripts covering doctor, run, converse, persistence, security, rtk, caveman, portkey — see [`tests/manual/README.md`](tests/manual/README.md) |
| **Acceptance docs** | `tests/acceptance/*.md` | manual scenarios for real Claude/Codex installs |

```bash
# Whole-suite green check (sequential to avoid PTY contention)
go test -race -count=1 -p 1 -timeout 300s ./...

# Manual suite, mock-only (no real LLM calls)
./tests/manual/run_all.sh --quick

# Manual suite, full (uses real LLMs if installed; auto-skips otherwise)
./tests/manual/run_all.sh
```

## Project layout

```
vitis/
├── cmd/
│   ├── vitis/             main entry point (run, peek, doctor, converse)
│   └── screendebug/       optional helper for debugging the screen package
├── internal/
│   ├── adapter/           Adapter interface + claude-code & codex implementations
│   ├── bus/               Bus interface + inproc channel-fanout backend
│   ├── cli/               run, peek, doctor, converse commands + rtk detection
│   ├── conversation/      Broker state machine, envelope builder, briefing,
│   │                      reply style (caveman embedding), marker generator
│   ├── model/             Pure data types (Session, Turn, Conversation, ...)
│   ├── orchestrator/      Single-shot orchestrator (vitis run path)
│   ├── peer/              PeerTransport interface + provider/mock implementations
│   │   ├── mock/          scripted in-memory transport for unit tests
│   │   └── provider/      local-PTY transport with PersistentProcess wrapper
│   ├── store/             Store interface, file backend, postgres stub
│   ├── terminal/          PTY runtime, ANSI normalize, screen emulator
│   ├── terminator/        Terminator interface + sentinel implementation
│   ├── testutil/          mockagent binary used by every E2E test
│   └── util/              helpers (id generation, LookPath wrapper)
├── docs/
│   ├── superpowers/
│   │   ├── specs/         design specs (the source of truth for behavior)
│   │   ├── plans/         implementation plans, executed via subagents
│   │   └── reviews/       compiled review findings
│   └── ...
└── tests/
    ├── acceptance/        manual scenarios for real Claude Code
    ├── integration/       Go integration test against the compiled binary
    └── manual/            shell scripts: 01_doctor through 15_converse_caveman
```

## Design philosophy

- **Local-first.** Vitis is designed for one user on one machine. It is **not** a hosted brokering service for consumer Claude accounts. Hosted/distributed mode is on the roadmap (NATS bus, Plan 4) but is opt-in and operator-owned.
- **PTY is the source of truth.** Raw PTY bytes are captured verbatim; normalized text is a derived view used for parsing. Audits and debugging always trace back to the raw stream.
- **Honest status classification.** Vitis never auto-answers permission prompts, auth prompts, or rate-limit messages. It detects them and surfaces them as terminal run statuses (`blocked_on_input`, `auth_required`, `rate_limited`) — the operator decides what to do.
- **Additive integrations only.** rtk, caveman, and portkeyagent live outside Vitis. The integration code in `internal/cli/rtk.go`, `internal/conversation/style.go`, and `tests/manual/setup_rtk.sh` is purely detect-and-recommend; nothing in the broker depends on any of them being present.
- **Pluggable everything.** Bus, peer transport, terminator, store — every architectural seam is an interface. Replacing the inproc bus with NATS, or sentinel with judge, is one struct away.

## Safety boundary

- Prefer local one-user operation.
- Do not position Vitis as a hosted proxy for consumer Claude accounts.
- Do not auto-answer auth, permission, or rate-limit prompts.
- Treat raw PTY bytes as the source of truth for audits and debugging.
- Sensitive credentials (Portkey, provider API keys) should live in Vault or the equivalent — never in committed config. The `tests/manual/.portkey.env` pattern uses chmod 0600 + gitignore for machine-local keys.
- The `provider:mock` URI is gated behind a test-only registration hook (`spawner_mock_test.go`); release binaries refuse it as an unknown provider.

## Where to read next

| Document | What it covers |
|---|---|
| [`docs/superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md`](docs/superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md) | The canonical A2A design spec — broker, bus, peer transport, terminator, error model |
| [`docs/superpowers/specs/2026-04-03-vitis-agent-bridge-design.md`](docs/superpowers/specs/2026-04-03-vitis-agent-bridge-design.md) | The original v1 single-shot design that everything else builds on |
| [`docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md`](docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md) | Implementation plan for the broker / bus / sentinel foundation |
| [`docs/superpowers/plans/2026-04-07-a2a-plan-2-pty-cli.md`](docs/superpowers/plans/2026-04-07-a2a-plan-2-pty-cli.md) | Implementation plan for the persistent PTY runtime + CLI |
| [`docs/superpowers/reviews/2026-04-07-a2a-review-findings.md`](docs/superpowers/reviews/2026-04-07-a2a-review-findings.md) | Consolidated findings from four parallel review passes; every HIGH and MEDIUM addressed |
| [`tests/manual/README.md`](tests/manual/README.md) | Manual test suite with 15 scripts, the rtk + caveman + portkey integration recipes |
| [`tests/acceptance/README.md`](tests/acceptance/README.md) | Acceptance test rules for real Claude Code runs |

## License

(Pick one — currently unset. Recommend MIT to match the integrated tools.)
