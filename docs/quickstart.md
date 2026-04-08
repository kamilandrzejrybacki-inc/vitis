# Quickstart

This page walks you from a fresh install to a running agent-to-agent conversation in five minutes. It assumes you have `vitis` on your `PATH` and a real `claude` install. If you don't have Claude Code, see the [Cost and speed](usage/cost-and-speed.md) guide for the free-LLM setup using `portkeyagent`.

## 1. Send one prompt

```bash
vitis run --provider claude-code --prompt "what is 2+2?"
```

You'll get a JSON object on stdout with the session id, the captured response, the duration, and a small `meta` block with confidence scores. The session is also persisted under `./logs/sessions/` and `./logs/turns/` so you can `peek` it later.

## 2. Read it back

```bash
vitis peek --session-id <session_id_from_step_1> --last 5
```

This dumps the last five turns from the session log as JSONL.

## 3. Start a two-peer conversation

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "You are a Go expert. Briefly explain channels. End with <<END>>." \
  --seed-b "You are a curious developer. Ask one clarifying question. End with <<END>> when satisfied." \
  --max-turns 6 \
  --terminator sentinel \
  --stream-turns
```

Two `claude` processes spawn in parallel. Vitis hands the first peer the seed, captures the reply, hands it to the second peer, captures its reply, and so on. The conversation ends when either peer emits `<<END>>` on its own line, or after six total turns, whichever comes first. Each turn streams as JSONL to stdout while the run is in progress, and a final `FinalResult` JSON object lands at the end.

The conversation is persisted to `./logs/conversations/<id>.json` and `./logs/conversations/<id>.jsonl`.

## 4. Make it shorter and cheaper

Add `--style caveman-full` to compress the model's replies by about 60 to 75 percent while keeping the technical content intact:

```bash
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed-a "You are a Go expert. Briefly explain channels. End with <<END>>." \
  --seed-b "You are a curious developer. Ask one clarifying question. End with <<END>> when satisfied." \
  --max-turns 6 \
  --terminator sentinel \
  --style caveman-full
```

The reply style is a system prompt block injected into the per-peer briefing on turn 1. Code blocks, error messages, and security warnings stay verbatim per the canonical caveman rules.

## 5. Verify the environment

```bash
vitis doctor --provider claude-code | jq .
vitis doctor --provider codex | jq .
```

Each call returns a JSON document with the provider version, the resolved binary path, and an `rtk` block showing whether the optional rtk hook is installed and active for that provider. If you want the hook installed, run `tests/manual/setup_rtk.sh` from a clone of the repo.

That's the whole loop. From here, the [Single-shot runs](usage/run.md) and [Conversations](usage/converse.md) pages cover every flag in detail, and [Cost and speed](usage/cost-and-speed.md) explains the optional integrations.
