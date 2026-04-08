# Conversations

`vitis converse` drives two long-lived AI agents through alternating turns. The broker hands an envelope to peer A, captures its reply, hands the reply to peer B, captures B's reply, and repeats until either peer emits the sentinel token, the max-turns cap fires, or the wall-clock timeout elapses.

Use `vitis converse` when you want one agent to discuss something with another agent: debate, code review, role-play, design exploration, anything that benefits from a back-and-forth between two perspectives.

## Minimum invocation

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed "Discuss the trade-offs of channels vs mutexes in Go. End with <<END>>." \
  --max-turns 6
```

`--peer-a` and `--peer-b` are required. Each is a URI describing one peer. The `provider:` scheme spawns a local PTY agent (Claude Code or Codex). Future schemes (`vitis://` for remote peers, `stdio://` for piped peers) follow the same flag shape.

`--seed` is the opening prompt delivered to peer A on turn 1. Both peers receive the same seed; for asymmetric setups use `--seed-a` and `--seed-b` instead.

## Asymmetric seeds

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "You are a Go expert. Propose a one-line goroutine pool API. End your reply with <<END>>." \
  --seed-b "You are a critic. Find one flaw in any proposed Go API. End your reply with <<END>>." \
  --opener a \
  --max-turns 8 \
  --terminator sentinel
```

`--seed-a` and `--seed-b` are mutually exclusive with `--seed`. `--opener` chooses which peer speaks first (default `a`).

## Per-peer options

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-a-opt model=claude-sonnet-4-6 \
  --peer-a-opt cwd=/repo/backend \
  --peer-b provider:codex \
  --peer-b-opt model=gpt-5 \
  --peer-b-opt reasoning-effort=high \
  --seed "Stress-test this design"
```

Each `--peer-{a,b}-opt key=value` sets an option for one peer. Recognised keys:

| Key | Purpose |
|---|---|
| `model` | Model id passed to the provider (forwarded as `--model` to the spawned binary) |
| `reasoning-effort` | Reasoning level for codex (`low`, `medium`, `high`) |
| `cwd` | Working directory for the spawned process |
| `home` | `HOME` env var override |
| `env_KEY=value` | Forward an env var to the spawned process; only allowlisted keys (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `MOCK_RESPONSE`, `MOCK_SENTINEL_AT_TURN`, `MOCK_DELAY_MS`) get through |

The `env_KEY` allowlist is intentional: it prevents the broker from forwarding `LD_PRELOAD`, `VITIS_CLAUDE_ARGS`, or other env vars that could bypass safety mechanisms in the spawned agent.

## Termination

A conversation ends when one of these things happens, in priority order:

1. The wall-clock `--overall-timeout` budget elapses (default `--max-turns × --per-turn-timeout`)
2. A peer crashes or gets blocked on a permission/auth/rate-limit prompt
3. The `--max-turns` hard cap is reached (default 50, max 500)
4. The terminator publishes a verdict (sentinel match by default)
5. The user interrupts via Ctrl-C

Sentinel termination is the cooperative path. You instruct each peer in the seed to end with `<<END>>` when it believes the conversation has reached its goal, and Vitis watches for that token on its own line in each reply. The default sentinel is `<<END>>`; override with `--sentinel "..."`.

```bash
vitis converse \
  ... \
  --terminator sentinel \
  --sentinel "<<DONE>>"
```

The sentinel is matched line-anchored, so `<<END>>` mid-sentence in a model reply will not trigger termination. The sentinel is also stripped from the response before it is forwarded to the other peer, so it never leaks across.

## Reply style compression

`--style` injects a system-prompt block into the per-peer briefing on turn 1 that tells the model how to format its replies. Four levels:

| Style | Behavior |
|---|---|
| `normal` | Default. No instructions added. |
| `caveman-lite` | Drop filler and hedging. Keep articles and full sentences. |
| `caveman-full` | Drop articles, use sentence fragments, prefer short synonyms. |
| `caveman-ultra` | Maximum compression. Telegraphic, abbreviated, arrows for causality. |

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed "Discuss Go error handling for 6 turns. End with <<END>>." \
  --max-turns 6 \
  --style caveman-full
```

Code blocks, error messages, security warnings, and irreversible-action confirmations stay verbatim regardless of style. See [Cost and speed](cost-and-speed.md) for measured compression numbers.

## Streaming output

By default, every captured turn is emitted as one JSONL object on stdout while the conversation runs. The final `FinalResult` JSON object lands at the end. Disable with `--stream-turns=false` if you only want the final result.

```bash
vitis converse ... --stream-turns=false
```

## Persistence

Conversations write to `./logs/conversations/<conversation_id>.json` (the summary) and `./logs/conversations/<conversation_id>.jsonl` (the per-turn log). Override with `--log-path`. Files are 0600.

## Conversation statuses

| Status | Meaning |
|---|---|
| `completed_sentinel` | A peer emitted the sentinel and the conversation finalised cleanly |
| `completed_judge` | The judge terminator decided the goal was reached (Plan 3) |
| `max_turns_hit` | The hard cap fired before any other terminator |
| `peer_crashed` | A peer process exited unexpectedly |
| `peer_blocked` | A peer hit a permission/auth/rate-limit prompt |
| `timeout` | The overall wall-clock budget expired |
| `interrupted` | User interrupted via Ctrl-C |
| `error` | Internal broker error (bus failure, store failure, etc.) |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Conversation reached a terminal status |
| 1 | Runtime error |
| 2 | Configuration error |

The full `FinalResult` JSON is always written to stdout regardless of exit code, so a 0-exit conversation that hit max-turns is distinguishable from a 0-exit conversation that hit the sentinel by reading `conversation.status` from the JSON.
