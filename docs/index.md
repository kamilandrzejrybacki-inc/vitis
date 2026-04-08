---
hide:
  - navigation
  - toc
---

# Vitis

Vitis is a local orchestrator for AI agent CLIs. It runs `claude`, `codex`, and similar tools through a real terminal so you can drive them from a script the same way you would drive any other process.

It does two things:

- **Single-shot runs.** You send one prompt to an agent, capture its full response, and write it to a JSON report.
- **Agent-to-agent conversations.** Two long-lived agents take turns talking to each other through a broker, with strict alternation, marker-based turn detection, and a sentinel that ends the conversation when either side decides it's done.

Both modes share the same recording layer and JSON output shape, so you can `peek` a single-shot session the same way you `peek` a multi-turn conversation.

## What you get

| Feature | What it means in practice |
|---|---|
| Run any prompt against Claude Code or Codex | One command, JSON out, transcript persisted to disk |
| Two-peer conversations | Set up two agents, give them seeds, let them talk for N turns |
| Reply-style compression | `--style caveman-full` cuts model output by roughly 60 to 75 percent without losing technical content |
| Free-LLM testing | Point Vitis at the [portkeyagent](https://github.com/kamilrybacki/portkeyagent) shim and route conversations through Groq, Deepseek, or NVIDIA NIM via the Portkey gateway |
| Rich diagnostics | `vitis doctor` reports provider versions and rtk hook status |
| File-store persistence | Every session and conversation is saved as JSON plus JSONL turn logs, with 0600 permissions |

## Where to next

| If you want to... | Read |
|---|---|
| Install Vitis | [Install](install.md) |
| See it work in 5 minutes | [Quickstart](quickstart.md) |
| Drive a single agent prompt | [Single-shot runs](usage/run.md) |
| Drive a two-peer conversation | [Conversations](usage/converse.md) |
| Look up a flag | [Configuration reference](reference/configuration.md) |
| Make it cheaper and faster | [Cost and speed](usage/cost-and-speed.md) |
| Diagnose a failing run | [Troubleshooting](troubleshooting.md) |
