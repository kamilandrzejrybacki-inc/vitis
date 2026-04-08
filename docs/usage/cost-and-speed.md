# Cost and speed

Vitis itself does not compress tokens or batch requests. It composes with three external tools, each of which targets a different layer of the conversation. You can adopt any combination of them; none are required.

## What each tool does

`rtk` ([rtk-ai/rtk](https://github.com/rtk-ai/rtk)) sits on the agent side. When the spawned agent runs a shell command like `git status`, `cat`, `grep`, or a test runner, rtk's PreToolUse hook intercepts the call and rewrites it to a compressed equivalent. The agent receives compact output instead of verbose logs, so its context fills up much more slowly. Vitis detects rtk via `vitis doctor` but never depends on it being installed.

`caveman` ([JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman)) is a prompt-only style that nudges the model toward telegraphic replies. Vitis embeds the canonical caveman rules in `internal/conversation/style.go` and exposes them via `--style caveman-{lite,full,ultra}`, so you get the compression with no external install.

`portkeyagent` ([kamilrybacki/portkeyagent](https://github.com/kamilrybacki/portkeyagent)) is a small CLI shim that fronts the [Portkey AI Gateway](https://github.com/Portkey-AI/gateway). You point `VITIS_CLAUDE_BINARY` at it and Vitis routes conversation traffic to free or low-cost upstream providers (Groq, Deepseek, NVIDIA NIM) instead of paid Anthropic or OpenAI accounts.

## Setting up rtk

Run the bundled setup helper from a clone of the Vitis repo:

```bash
./tests/manual/setup_rtk.sh
```

It installs rtk via Homebrew, cargo, or the upstream curl-installer (whichever is available), runs `rtk init -g` for both Claude Code and Codex, and patches `~/.claude/settings.json` with the rtk PreToolUse hook entry. The patch is idempotent: re-running the helper is a no-op once the hook is in place.

Verify the hook is active:

```bash
vitis doctor --provider claude-code | jq .rtk
```

You should see `"hook_installed": true`. If it's `false`, the script's stderr output will tell you why.

## Using caveman style

No install needed. Pass `--style` to `vitis converse`:

```bash
vitis converse ... --style caveman-full
```

The four levels:

| Level | Use when |
|---|---|
| `normal` | You want unconstrained replies |
| `caveman-lite` | You want professional output minus filler and hedging |
| `caveman-full` | Default caveman, drops articles and uses sentence fragments |
| `caveman-ultra` | Maximum compression, telegraphic, abbreviated |

Code blocks, error messages, security warnings, and irreversible-action confirmations stay verbatim at every level. The compression only affects natural-language prose.

A measured comparison run against Groq llama-3.3-70b-versatile produced 708 chars at `--style normal` and 284 chars at `--style caveman-ultra` for the same prompt: a 60 percent reduction with no loss of technical content.

## Setting up portkeyagent

Install the binary:

```bash
go install github.com/kamilrybacki/portkeyagent@latest
```

Set the Portkey credentials:

```bash
export PORTKEY_API_KEY=pk-xxxxx
export PORTKEY_PROVIDER=groq                # or openai, deepseek, etc.
export PORTKEY_MODEL=llama-3.3-70b-versatile
export PORTKEY_ENDPOINT=https://api.portkey.ai/v1/chat/completions
```

Then point Vitis at portkeyagent instead of the real Claude binary:

```bash
export VITIS_CLAUDE_BINARY=$(which portkeyagent)
export MOCK_MULTI_TURN=1
vitis converse \
  --peer-a provider:claude-code \
  --peer-b provider:claude-code \
  --seed "..." \
  --max-turns 6 \
  --terminator sentinel
```

Vitis spawns portkeyagent for both peers; portkeyagent reads the envelope from stdin, calls Portkey, writes the model's reply plus the per-turn marker token to stdout. From the broker's perspective it looks identical to a real Claude Code session.

## Stacking the layers

The three tools are independent and compose freely:

| Layer | What it shrinks |
|---|---|
| `rtk` | Tool-call output going INTO the agent |
| `caveman` | Model output coming OUT of the agent |
| `portkeyagent` | Replaces the upstream LLM provider entirely |

In a long A2A conversation where two agents may run dozens of tool calls per turn, having all three active means smaller agent input contexts, smaller envelopes flowing through the broker, smaller persisted logs, and zero vendor quota cost. The combined effect on a fixed-context-budget run is roughly two to four times more turns.
