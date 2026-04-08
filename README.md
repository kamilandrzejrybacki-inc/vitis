# Vitis

Vitis is a local PTY orchestrator for AI agent CLIs (Claude Code, Codex, others). It runs in two modes:

- `vitis run` sends one prompt to an agent and captures the response.
- `vitis converse` drives two long-lived agents through alternating turns until a sentinel, judge verdict, or max-turns cap fires.

Both modes share the same PTY runtime, persistence layer, and JSON output shape, so anything you can inspect from a single-shot run also works on a multi-turn conversation.

## Install

```bash
go install github.com/kamilandrzejrybacki-inc/vitis/cmd/vitis@latest
```

Or build from source:

```bash
git clone https://github.com/kamilandrzejrybacki-inc/vitis.git
cd vitis
go build -o vitis ./cmd/vitis
```

For production use you need a real `claude` or `codex` install on your `PATH`. For tests and demos, Vitis ships a mock agent you can substitute via `VITIS_CLAUDE_BINARY`.

## Quick start

A single-shot run against Claude Code:

```bash
vitis run --provider claude-code --prompt "what is 2+2?"
```

A two-peer conversation that ends when either side emits the sentinel:

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "You are a Go expert. Briefly explain channels. End with <<END>>." \
  --seed-b "You are a critic. Find one flaw. End with <<END>>." \
  --max-turns 6 \
  --terminator sentinel
```

An N-peer (3+) conversation using the repeatable `--peer` flag, with addressed routing via `<<NEXT: peer-id>>` trailers:

```bash
vitis converse \
  --peer 'id=alice,provider=claude-code,seed="You are the optimist. Address the next speaker with <<NEXT: id>> or end with <<END>>."' \
  --peer 'id=bob,provider=codex,seed="You are the pessimist."' \
  --peer 'id=carol,provider=claude-code,seed="You are the moderator."' \
  --opener alice \
  --max-turns 12
```

Peer ids must match `^[a-z][a-z0-9_-]{0,31}$`. Each peer's reply may end with `<<NEXT: peer-id>>` to address the next speaker explicitly; replies without a trailer fall back to round-robin in declared order. `<<END>>` always terminates the conversation. The legacy `--peer-a/--peer-b` flags continue to work unchanged for 2-peer scripts.

Check the local environment, including which providers are installed and whether the optional rtk hook is active:

```bash
vitis doctor --provider claude-code | jq .
```

## Commands

| Command | Purpose |
|---|---|
| `vitis run` | One prompt, one response, exit. Writes a JSON report and persists the session. |
| `vitis converse` | Multi-turn agent-to-agent conversation. Strict alternation between two peers. |
| `vitis peek --session-id <id>` | Read back turns from a recorded session or conversation. |
| `vitis doctor [--provider <id>]` | Verify the local environment. Reports provider availability, version, and rtk integration status. |

## A2A conversation flags

| Flag | Default | Notes |
|---|---|---|
| `--peer-a`, `--peer-b` | required (legacy 2-peer) | Peer URI: `provider:claude-code`, `provider:codex`, `provider:mock` (test builds), `vitis://` (planned), `stdio://` (planned) |
| `--peer-a-opt key=value` | repeatable | Per-peer options. Recognised keys include `model`, `reasoning-effort`, `cwd`, `home`, allowlisted `env_KEY=value`. |
| `--peer id=...,provider=...[,seed="...",...]` | repeatable, N-peer mode | N-peer (2..16) declaration. Mutually exclusive with `--peer-a/--peer-b`. Recognised keys: `id` (required, lowercase regex), `provider` (required), `seed`, `model`, `reasoning-effort`, `cwd`, `home`. Quoted values may contain commas; escapes `\"` and `\\` work. |
| `--opener <id>` | first declared peer | In N-peer mode, names which peer opens the conversation. |
| `--seed "..."` | one of seed/seed-a+b required | Same opening prompt for both peers |
| `--seed-a "..." --seed-b "..."` | alternative | Asymmetric per-peer seeds for debate or role-play |
| `--opener a\|b` | `a` | Which peer speaks first |
| `--max-turns N` | 50 | Hard cap, range 1..500 |
| `--per-turn-timeout SEC` | 300 | Per-turn timeout, max 3600 |
| `--overall-timeout SEC` | `max-turns × per-turn-timeout` | Whole-conversation timeout, max 86400 |
| `--terminator` | `sentinel` | `judge` arrives in Plan 3 |
| `--sentinel "<<END>>"` | `<<END>>` | Line-anchored token. Either peer ends the conversation by emitting it on its own line. |
| `--style` | `normal` | `caveman-lite`, `caveman-full`, `caveman-ultra` embed reply-style instructions that compress responses by roughly 60 to 75 percent. |
| `--bus` | `inproc` | NATS arrives in Plan 4 |
| `--log-backend` | `file` | Postgres arrives in Plan 3 |
| `--working-directory` | cwd | Validated against path traversal |
| `--stream-turns` | true | Emit each turn as JSONL on stdout while the conversation runs |

## How it stays cheap

Vitis itself does not compress anything. It composes with three external tools, each of which targets a different layer of the conversation:

`rtk` ([rtk-ai/rtk](https://github.com/rtk-ai/rtk)) sits on the agent side and rewrites common shell commands like `git status`, `cat`, `grep`, and test runners so the agent receives compact output instead of verbose logs. Vitis detects rtk via `vitis doctor`, and the `tests/manual/setup_rtk.sh` helper installs the PreToolUse hook for both Claude Code and Codex.

`caveman` ([JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman)) is a prompt-only style that tells the model to drop filler words and articles while preserving code blocks, error messages, and security warnings verbatim. Vitis embeds the canonical caveman rules in `internal/conversation/style.go`, so `--style caveman-full` works without installing the upstream package.

`portkeyagent` ([kamilrybacki/portkeyagent](https://github.com/kamilrybacki/portkeyagent)) is a small CLI shim that fronts the [Portkey AI Gateway](https://github.com/Portkey-AI/gateway). You point `VITIS_CLAUDE_BINARY` at it and Vitis sends conversation traffic to free upstream providers (Groq, Deepseek, NVIDIA NIM) instead of paid Anthropic or OpenAI accounts.

None of these are required. Vitis works without all three; the integration is detect-and-recommend.

## Status

| Component | State |
|---|---|
| PTY runtime, single-shot and persistent | shipped |
| Adapters: claude-code, codex | shipped |
| Conversation broker, strict alternation, max-turns cap | shipped |
| In-process bus | shipped |
| Sentinel terminator (line-anchored) | shipped |
| Provider peer transport | shipped |
| File-store persistence (sessions and conversations) | shipped |
| `vitis run`, `peek`, `doctor`, `converse` | shipped |
| Reply style: `caveman-{lite,full,ultra}` | shipped |
| rtk integration (doctor + setup helper) | shipped |
| Sidecar JSONL detection for real claude-code multi-turn | Plan 2.5 |
| Judge terminator | Plan 3 |
| Postgres backend for conversations | Plan 3 |
| NATS bus, remote `vitis://` peers | Plan 4 |
| `stdio://` peer | Plan 5 |

## Test layout

Vitis follows the standard Go convention: unit tests live next to the code they cover under `internal/...`. Run the full suite with the race detector:

```bash
go test -race -count=1 -p 1 -timeout 300s ./...
```

The `tests/` tree holds the higher layers. `tests/integration/` exercises the compiled `vitis` binary end-to-end against the bundled mock agent. `tests/manual/` is a numbered shell script suite (15 scripts) covering doctor, run, converse, persistence, security hardening, rtk, caveman, and Portkey integration. Each script is independent and either auto-skips or fails cleanly. See [`tests/manual/README.md`](tests/manual/README.md) for the full catalog. `tests/acceptance/` is markdown documentation for manual scenarios that need a real Claude Code or Codex install.

```bash
# Mock-only sweep, no real LLM calls, around 30 seconds total
./tests/manual/run_all.sh --quick

# Full sweep, with real-provider tests if installed
./tests/manual/run_all.sh
```

## Project layout

```
vitis/
├── cmd/vitis/             entry point: run, peek, doctor, converse
├── internal/
│   ├── adapter/           Adapter interface, claude-code and codex implementations
│   ├── bus/               Bus interface, in-process channel-fanout backend
│   ├── cli/               CLI commands plus rtk detection
│   ├── conversation/      Broker state machine, envelope builder, briefing,
│   │                      reply style (caveman embedding), marker generator
│   ├── model/             Pure data types
│   ├── orchestrator/      Single-shot orchestrator (vitis run path)
│   ├── peer/              PeerTransport interface plus provider and mock implementations
│   ├── store/             Store interface, file backend, postgres stub
│   ├── terminal/          PTY runtime, ANSI normalize, screen emulator
│   ├── terminator/        Terminator interface, sentinel implementation
│   ├── testutil/          mockagent binary used by every E2E test
│   └── util/              ID generation, LookPath wrapper
├── docs/superpowers/      design specs, implementation plans, review reports
└── tests/                 acceptance docs, integration tests, manual scripts
```

## Design notes

Vitis is built for one user on one machine. Hosted brokering of consumer Claude accounts is explicitly out of scope. The raw PTY byte stream is the source of truth for every recording, and normalized text is treated as a derived view used only for parsing.

Vitis never auto-answers permission, authentication, or rate-limit prompts from the spawned agents. When the observer detects one, it surfaces the corresponding terminal status (`blocked_on_input`, `auth_required`, `rate_limited`) and lets the operator decide what to do.

Every architectural seam is an interface: bus, peer transport, terminator, store. Replacing the in-process bus with NATS, or sentinel with a judge, is a constructor swap.

## Dependencies and acknowledgements

Runtime Go modules:

| Module | Purpose | License |
|---|---|---|
| [creack/pty](https://github.com/creack/pty) | The pseudo-terminal that every spawned agent runs in | MIT |
| [stretchr/testify](https://github.com/stretchr/testify) | Test assertions and fixtures | MIT |
| [jackc/pgx/v5](https://github.com/jackc/pgx) | Postgres driver for the planned conversation backend | MIT |

Companion projects developed alongside Vitis:

| Project | Purpose | License |
|---|---|---|
| [kamilrybacki/portkeyagent](https://github.com/kamilrybacki/portkeyagent) | CLI shim that fronts the Portkey LLM gateway. Built as a `VITIS_CLAUDE_BINARY` substitute so manual tests can run against free upstream LLMs. | MIT |

Token-efficiency tools that Vitis detects and recommends:

| Project | Purpose | License |
|---|---|---|
| [rtk-ai/rtk](https://github.com/rtk-ai/rtk) | CLI proxy that compresses common shell command outputs by 60 to 90 percent. | MIT |
| [JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman) | A telegraphic reply style that cuts model output by roughly 75 percent while preserving technical accuracy. The MIT-licensed canonical rules are embedded in `internal/conversation/style.go`. | MIT |
| [Portkey-AI/gateway](https://github.com/Portkey-AI/gateway) | Open-source LLM gateway that routes to upstream providers via a single OpenAI-compatible API. portkeyagent uses it. | MIT |

AI agent CLIs Vitis drives:

| Project | Notes |
|---|---|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | Anthropic's official CLI for Claude. The canonical real-world peer for `vitis converse`. |
| [Codex CLI](https://github.com/openai/codex) | OpenAI's Codex command-line agent. Vitis spawns the interactive form for converse mode. |

Inspiration and prior art:

- [botl](https://github.com/kamilrybacki-inc/botl), the Go CLI that runs Claude Code in ephemeral Docker containers. Same author, complementary use case (Vitis is for orchestration, botl is for sandboxing).
- The `superpowers:brainstorming`, `superpowers:writing-plans`, and `superpowers:subagent-driven-development` skills shaped how the entire feature was designed and built. Every spec and plan in `docs/superpowers/` came from those workflows.

## License

MIT. See [`LICENSE`](LICENSE).

## Where to read more

| Document | Contents |
|---|---|
| [`docs/superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md`](docs/superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md) | The canonical A2A design spec: broker, bus, peer transport, terminator, error model. |
| [`docs/superpowers/specs/2026-04-03-vitis-agent-bridge-design.md`](docs/superpowers/specs/2026-04-03-vitis-agent-bridge-design.md) | The original v1 single-shot design that everything else builds on. |
| [`docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md`](docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md) | Implementation plan for the broker, bus, and sentinel foundation. |
| [`docs/superpowers/plans/2026-04-07-a2a-plan-2-pty-cli.md`](docs/superpowers/plans/2026-04-07-a2a-plan-2-pty-cli.md) | Implementation plan for the persistent PTY runtime and CLI. |
| [`docs/superpowers/reviews/2026-04-07-a2a-review-findings.md`](docs/superpowers/reviews/2026-04-07-a2a-review-findings.md) | Consolidated findings from the parallel review passes. |
| [`tests/manual/README.md`](tests/manual/README.md) | Manual test suite catalog with the rtk and caveman setup recipes. |
